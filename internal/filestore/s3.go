package filestore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"SuperBotGo/internal/model"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	s3DefaultUploadPartSize = 8 * 1024 * 1024
	s3MaxUploadParts        = 10000
	s3MaxObjectSize         = 5 * 1024 * 1024 * 1024 * 1024
	s3UploadConcurrency     = 2
)

// S3StoreConfig holds configuration for the S3-backed FileStore.
type S3StoreConfig struct {
	Bucket         string
	Region         string
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Prefix         string // e.g. "files/"
}

// S3Store implements FileStore using S3-compatible object storage.
// Data is stored as <prefix><id>.data, metadata as <prefix><id>.meta.json.
type S3Store struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	prefix    string
}

// NewS3Store creates an S3-backed FileStore.
func NewS3Store(ctx context.Context, cfg S3StoreConfig) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("filestore s3: bucket name is required")
	}

	var opts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("filestore s3: load aws config: %w", err)
	}

	s3Opts := s3Options(cfg.Endpoint)
	client := s3.NewFromConfig(awsCfg, s3Opts...)

	presignClient := client
	if cfg.PublicEndpoint != "" && cfg.PublicEndpoint != cfg.Endpoint {
		presignClient = s3.NewFromConfig(awsCfg, s3Options(cfg.PublicEndpoint)...)
	}

	return &S3Store{
		client:    client,
		presigner: s3.NewPresignClient(presignClient),
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
	}, nil
}

func s3Options(endpoint string) []func(*s3.Options) {
	var opts []func(*s3.Options)
	if endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		})
	}
	return opts
}

func (s *S3Store) dataKey(id string) string { return s.prefix + id + ".data" }
func (s *S3Store) metaKey(id string) string { return s.prefix + id + ".meta.json" }

func (s *S3Store) Store(ctx context.Context, meta FileMeta, data io.Reader) (model.FileRef, error) {
	s.prepareMetaDefaults(&meta)

	size, err := s.uploadData(ctx, meta, data)
	if err != nil {
		return model.FileRef{}, err
	}
	meta.Size = size

	if err := s.putMeta(ctx, meta); err != nil {
		// Cleanup data on meta failure.
		_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s.dataKey(meta.ID)),
		})
		return model.FileRef{}, err
	}

	return meta.Ref(), nil
}

func (s *S3Store) uploadData(ctx context.Context, meta FileMeta, data io.Reader) (int64, error) {
	knownSize := knownUploadSize(meta)
	partSize, err := uploadPartSize(knownSize)
	if err != nil {
		return 0, fmt.Errorf("filestore s3: prepare data %q: %w", meta.ID, err)
	}

	input := &transfermanager.UploadObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.dataKey(meta.ID)),
		Body:        data,
		ContentType: aws.String(meta.MIMEType),
	}
	if knownSize > 0 {
		input.ContentLength = aws.Int64(knownSize)
	}

	uploader := transfermanager.New(s.client, func(o *transfermanager.Options) {
		o.PartSizeBytes = partSize
		o.MultipartUploadThreshold = partSize
		o.Concurrency = s3UploadConcurrency
		o.MaxUploadParts = s3MaxUploadParts
	})

	out, err := uploader.UploadObject(ctx, input, func(o *transfermanager.Options) {
		o.ChecksumAlgorithm = ""
	})
	if err != nil {
		return 0, fmt.Errorf("filestore s3: put data %q: %w", meta.ID, err)
	}

	return aws.ToInt64(out.ContentLength), nil
}

func knownUploadSize(meta FileMeta) int64 {
	if meta.Size > 0 {
		return meta.Size
	}
	return meta.ExpectedSize
}

func uploadPartSize(size int64) (int64, error) {
	if size > s3MaxObjectSize {
		return 0, fmt.Errorf("object size %d exceeds S3 object size limit", size)
	}

	partSize := int64(s3DefaultUploadPartSize)
	if size > 0 {
		minPartSize := size / s3MaxUploadParts
		if size%s3MaxUploadParts != 0 {
			minPartSize++
		}
		if minPartSize > partSize {
			partSize = minPartSize
		}
	}
	return partSize, nil
}

func (s *S3Store) Get(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error) {
	meta, err := s.Meta(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("filestore s3: get data %q: %w", id, err)
	}

	return out.Body, meta, nil
}

func (s *S3Store) GetRange(ctx context.Context, id string, offset, length int64) (io.ReadCloser, *FileMeta, error) {
	if offset < 0 {
		return nil, nil, fmt.Errorf("filestore s3: negative offset %d", offset)
	}

	meta, err := s.Meta(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if meta.Size <= 0 || offset >= meta.Size {
		return io.NopCloser(bytes.NewReader(nil)), meta, nil
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	}
	switch {
	case length > 0:
		end := meta.Size - 1
		if remaining := meta.Size - offset; remaining > 0 && length < remaining {
			end = offset + length - 1
		}
		input.Range = aws.String(fmt.Sprintf("bytes=%d-%d", offset, end))
	case offset > 0:
		input.Range = aws.String(fmt.Sprintf("bytes=%d-", offset))
	}

	out, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("filestore s3: get data range %q: %w", id, err)
	}

	return out.Body, meta, nil
}

func (s *S3Store) Meta(ctx context.Context, id string) (*FileMeta, error) {
	return s.loadMeta(ctx, id)
}

func (s *S3Store) Delete(ctx context.Context, id string) error {
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	})
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.metaKey(id)),
	})
	return nil
}

func (s *S3Store) CreateDirectUpload(ctx context.Context, meta FileMeta, expiry time.Duration) (DirectUpload, error) {
	s.prepareMetaDefaults(&meta)
	meta.State = FileStatePending
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	expiresAt := time.Now().Add(expiry)
	meta.ExpiresAt = &expiresAt

	if err := s.putMeta(ctx, meta); err != nil {
		return DirectUpload{}, err
	}

	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.dataKey(meta.ID)),
		ContentType: aws.String(meta.MIMEType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		_ = s.Delete(ctx, meta.ID)
		return DirectUpload{}, fmt.Errorf("filestore s3: presign put %q: %w", meta.ID, err)
	}

	headers := make(map[string]string, len(req.SignedHeader))
	for key, values := range req.SignedHeader {
		if strings.EqualFold(key, "host") {
			continue
		}
		if len(values) == 0 {
			continue
		}
		headers[key] = values[0]
	}

	return DirectUpload{
		FileID:    meta.ID,
		Method:    req.Method,
		URL:       req.URL,
		Headers:   headers,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *S3Store) CompleteDirectUpload(ctx context.Context, id string) (model.FileRef, error) {
	meta, err := s.loadMeta(ctx, id)
	if err != nil {
		return model.FileRef{}, err
	}
	if meta.State == FileStateReady {
		return meta.Ref(), nil
	}
	if meta.State != FileStatePending {
		return model.FileRef{}, fmt.Errorf("filestore s3: upload %q is not pending", id)
	}
	if meta.ExpiresAt != nil && meta.ExpiresAt.Before(time.Now()) {
		return model.FileRef{}, fmt.Errorf("filestore s3: upload %q expired", id)
	}

	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	})
	if err != nil {
		return model.FileRef{}, fmt.Errorf("filestore s3: head data %q: %w", id, err)
	}

	meta.Size = aws.ToInt64(head.ContentLength)
	meta.State = FileStateReady
	meta.ExpiresAt = nil
	if err := s.putMeta(ctx, *meta); err != nil {
		return model.FileRef{}, err
	}
	return meta.Ref(), nil
}

// URL returns a presigned GET URL for downloading the file directly from S3.
func (s *S3Store) URL(ctx context.Context, id string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = 1 * time.Hour
	}
	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("filestore s3: presign %q: %w", id, err)
	}
	return out.URL, nil
}

// Cleanup lists all .meta.json objects, checks ExpiresAt, and deletes expired files.
func (s *S3Store) Cleanup(ctx context.Context) (int, error) {
	now := time.Now()
	removed := 0

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return removed, fmt.Errorf("filestore s3: list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if len(key) < 10 || key[len(key)-10:] != ".meta.json" {
				continue
			}

			meta, err := s.loadMetaByKey(ctx, key)
			if err != nil || meta.ID == "" {
				continue
			}
			if meta.ExpiresAt != nil && meta.ExpiresAt.Before(now) {
				_ = s.Delete(ctx, meta.ID)
				removed++
			}
		}
	}

	return removed, nil
}

func (s *S3Store) loadMeta(ctx context.Context, id string) (*FileMeta, error) {
	return s.loadMetaByKey(ctx, s.metaKey(id))
}

func (s *S3Store) loadMetaByKey(ctx context.Context, key string) (*FileMeta, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("filestore s3: get meta %q: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("filestore s3: read meta %q: %w", key, err)
	}

	var meta FileMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("filestore s3: unmarshal meta %q: %w", key, err)
	}
	return &meta, nil
}

func (s *S3Store) exists(ctx context.Context, key string) bool {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false
		}
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			return false
		}
	}
	return err == nil
}

var _ FileStore = (*S3Store)(nil)
var _ DirectUploadStore = (*S3Store)(nil)
var _ RangeReader = (*S3Store)(nil)

func (s *S3Store) prepareMetaDefaults(meta *FileMeta) {
	if meta.ID == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		meta.ID = hex.EncodeToString(b)
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}
	if meta.State == "" {
		meta.State = FileStateReady
	}
}

func (s *S3Store) putMeta(ctx context.Context, meta FileMeta) error {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("filestore s3: marshal meta %q: %w", meta.ID, err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.metaKey(meta.ID)),
		Body:          bytes.NewReader(metaBytes),
		ContentLength: aws.Int64(int64(len(metaBytes))),
		ContentType:   aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("filestore s3: put meta %q: %w", meta.ID, err)
	}
	return nil
}
