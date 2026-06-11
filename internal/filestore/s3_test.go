package filestore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestS3StoreStoreAcceptsNonSeekableReader(t *testing.T) {
	ctx := context.Background()
	payload := []byte("telegram file payload")

	var (
		mu          sync.Mutex
		objects     = map[string][]byte{}
		handlerErrs []error
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			mu.Lock()
			handlerErrs = append(handlerErrs, fmt.Errorf("method = %s, want PUT", r.Method))
			mu.Unlock()
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			mu.Lock()
			handlerErrs = append(handlerErrs, fmt.Errorf("read body: %w", err))
			mu.Unlock()
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}

		mu.Lock()
		objects[r.URL.Path] = body
		mu.Unlock()

		w.Header().Set("ETag", `"test-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := NewS3Store(ctx, S3StoreConfig{
		Bucket:    "bucket",
		Region:    "us-east-1",
		Endpoint:  server.URL,
		AccessKey: "access-key",
		SecretKey: "secret-key",
		Prefix:    "files/",
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	ref, err := store.Store(ctx, FileMeta{
		ID:       "file-id",
		Name:     "photo.jpg",
		MIMEType: "image/jpeg",
	}, struct{ io.Reader }{Reader: bytes.NewReader(payload)})
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	if ref.Size != int64(len(payload)) {
		t.Fatalf("ref.Size = %d, want %d", ref.Size, len(payload))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(handlerErrs) > 0 {
		t.Fatalf("handler errors: %v", handlerErrs)
	}

	if got := objects["/bucket/files/file-id.data"]; !bytes.Equal(got, payload) {
		t.Fatalf("stored data = %q, want %q", got, payload)
	}

	var meta FileMeta
	if err := json.Unmarshal(objects["/bucket/files/file-id.meta.json"], &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Size != int64(len(payload)) {
		t.Fatalf("meta.Size = %d, want %d", meta.Size, len(payload))
	}
}

func TestS3StoreStoreUsesMultipartForLargeNonSeekableReader(t *testing.T) {
	ctx := context.Background()
	payload := bytes.Repeat([]byte("x"), s3DefaultUploadPartSize+123)

	var (
		mu                 sync.Mutex
		objects            = map[string][]byte{}
		uploadedParts      = map[int][]byte{}
		createdMultipart   bool
		completedMultipart bool
		handlerErrs        []error
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch {
		case r.Method == http.MethodPost && query.Has("uploads"):
			mu.Lock()
			createdMultipart = true
			mu.Unlock()
			writeXML(w, `<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>files/large-id.data</Key><UploadId>upload-id</UploadId></InitiateMultipartUploadResult>`)
		case r.Method == http.MethodPut && query.Has("partNumber"):
			partNumber, err := strconv.Atoi(query.Get("partNumber"))
			if err != nil {
				http.Error(w, "bad part number", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				mu.Lock()
				handlerErrs = append(handlerErrs, fmt.Errorf("read part %d: %w", partNumber, err))
				mu.Unlock()
				http.Error(w, "read body", http.StatusInternalServerError)
				return
			}

			mu.Lock()
			uploadedParts[partNumber] = body
			mu.Unlock()

			w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, partNumber))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && query.Has("uploadId"):
			mu.Lock()
			completedMultipart = true
			for i := 1; i <= len(uploadedParts); i++ {
				objects["/bucket/files/large-id.data"] = append(objects["/bucket/files/large-id.data"], uploadedParts[i]...)
			}
			mu.Unlock()
			writeXML(w, `<CompleteMultipartUploadResult><Location>http://example.test/bucket/files/large-id.data</Location><Bucket>bucket</Bucket><Key>files/large-id.data</Key><ETag>"complete"</ETag></CompleteMultipartUploadResult>`)
		case r.Method == http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				mu.Lock()
				handlerErrs = append(handlerErrs, fmt.Errorf("read put %s: %w", r.URL.Path, err))
				mu.Unlock()
				http.Error(w, "read body", http.StatusInternalServerError)
				return
			}

			mu.Lock()
			objects[r.URL.Path] = body
			mu.Unlock()

			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		default:
			mu.Lock()
			handlerErrs = append(handlerErrs, fmt.Errorf("unexpected request: %s %s", r.Method, r.URL.String()))
			mu.Unlock()
			http.Error(w, "unexpected request", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	store, err := NewS3Store(ctx, S3StoreConfig{
		Bucket:    "bucket",
		Region:    "us-east-1",
		Endpoint:  server.URL,
		AccessKey: "access-key",
		SecretKey: "secret-key",
		Prefix:    "files/",
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	ref, err := store.Store(ctx, FileMeta{
		ID:       "large-id",
		Name:     "video.mp4",
		MIMEType: "video/mp4",
		Size:     int64(len(payload)),
	}, struct{ io.Reader }{Reader: bytes.NewReader(payload)})
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if ref.Size != int64(len(payload)) {
		t.Fatalf("ref.Size = %d, want %d", ref.Size, len(payload))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(handlerErrs) > 0 {
		t.Fatalf("handler errors: %v", handlerErrs)
	}
	if !createdMultipart {
		t.Fatal("multipart upload was not created")
	}
	if !completedMultipart {
		t.Fatal("multipart upload was not completed")
	}
	if got := objects["/bucket/files/large-id.data"]; !bytes.Equal(got, payload) {
		t.Fatalf("stored data length = %d, want %d", len(got), len(payload))
	}

	var meta FileMeta
	if err := json.Unmarshal(objects["/bucket/files/large-id.meta.json"], &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Size != int64(len(payload)) {
		t.Fatalf("meta.Size = %d, want %d", meta.Size, len(payload))
	}
}

func TestUploadPartSizeScalesForLargeKnownObjects(t *testing.T) {
	partSize, err := uploadPartSize(5 * 1024 * 1024 * 1024 * 1024)
	if err != nil {
		t.Fatalf("uploadPartSize() error = %v", err)
	}
	if partSize <= s3DefaultUploadPartSize {
		t.Fatalf("partSize = %d, want larger than default %d", partSize, s3DefaultUploadPartSize)
	}
	if parts := (5*1024*1024*1024*1024 + partSize - 1) / partSize; parts > s3MaxUploadParts {
		t.Fatalf("parts = %d, want <= %d", parts, s3MaxUploadParts)
	}

	if _, err := uploadPartSize(5*1024*1024*1024*1024 + 1); err == nil {
		t.Fatal("uploadPartSize() error = nil, want size limit error")
	}
}

func TestS3StoreCreateDirectUploadUsesPublicEndpointForPresign(t *testing.T) {
	ctx := context.Background()

	var (
		mu    sync.Mutex
		paths []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()

		w.Header().Set("ETag", `"test-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := NewS3Store(ctx, S3StoreConfig{
		Bucket:         "bucket",
		Region:         "us-east-1",
		Endpoint:       server.URL,
		PublicEndpoint: "https://files.example.test",
		AccessKey:      "access-key",
		SecretKey:      "secret-key",
		Prefix:         "files/",
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	upload, err := store.CreateDirectUpload(ctx, FileMeta{
		ID:       "upload-id",
		Name:     "document.pdf",
		MIMEType: "application/pdf",
	}, 0)
	if err != nil {
		t.Fatalf("CreateDirectUpload() error = %v", err)
	}

	wantPrefix := "https://files.example.test/bucket/files/upload-id.data?"
	if !strings.HasPrefix(upload.URL, wantPrefix) {
		t.Fatalf("upload.URL = %q, want prefix %q", upload.URL, wantPrefix)
	}
	if strings.Contains(upload.URL, server.URL) {
		t.Fatalf("upload.URL = %q contains internal endpoint %q", upload.URL, server.URL)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 || paths[0] != "/bucket/files/upload-id.meta.json" {
		t.Fatalf("internal S3 paths = %v, want only meta PUT", paths)
	}
}

func writeXML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
