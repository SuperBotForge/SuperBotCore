package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

const (
	pluginBundleWasmName       = "plugin.wasm"
	pluginBundleFrontendPrefix = "frontend/"

	maxPluginFrontendAssets    = 512
	maxPluginFrontendAssetSize = 10 << 20
	maxPluginFrontendTotalSize = 25 << 20

	maxPluginBundleCompressionRatio uint64 = 100
)

type pluginUpload struct {
	Filename      string
	WasmBytes     []byte
	FrontendFiles []pluginFrontendFile
}

type pluginFrontendFile struct {
	Path        string
	Data        []byte
	ContentType string
	Size        int64
}

// readPluginUploadFromForm accepts the legacy .wasm upload and the bundled
// .zip format: plugin.wasm (or one root .wasm) plus optional frontend/* files.
func readPluginUploadFromForm(w http.ResponseWriter, r *http.Request) (pluginUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return pluginUpload{}, false
	}

	file, header, err := r.FormFile("wasm")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'wasm' file in form")
		return pluginUpload{}, false
	}
	defer file.Close()

	upload, err := parsePluginUpload(header.Filename, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return pluginUpload{}, false
	}
	return upload, true
}

func parsePluginUpload(filename string, src io.Reader) (pluginUpload, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return pluginUpload{}, fmt.Errorf("failed to read uploaded file")
	}

	lowerName := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lowerName, ".wasm"):
		return pluginUpload{Filename: filename, WasmBytes: raw}, nil
	case strings.HasSuffix(lowerName, ".zip"):
		upload, err := parsePluginBundle(filename, raw)
		if err != nil {
			return pluginUpload{}, err
		}
		return upload, nil
	default:
		return pluginUpload{}, fmt.Errorf("file must have .wasm or .zip extension")
	}
}

func parsePluginBundle(filename string, raw []byte) (pluginUpload, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return pluginUpload{}, fmt.Errorf("invalid plugin bundle zip")
	}

	var wasmCandidates []pluginFrontendFile
	var frontendFiles []pluginFrontendFile
	seenAssets := make(map[string]struct{})
	var totalFrontendSize int64

	for _, entry := range reader.File {
		cleanName, err := cleanBundlePath(entry.Name)
		if err != nil {
			return pluginUpload{}, err
		}
		if shouldIgnoreBundleEntry(cleanName) {
			continue
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.FileInfo().Mode().IsRegular() {
			return pluginUpload{}, fmt.Errorf("bundle entry %q must be a regular file", cleanName)
		}
		if err := validateZipCompressionRatio(entry); err != nil {
			return pluginUpload{}, fmt.Errorf("bundle entry %q: %w", cleanName, err)
		}

		if strings.HasPrefix(cleanName, pluginBundleFrontendPrefix) {
			assetPath := strings.TrimPrefix(cleanName, pluginBundleFrontendPrefix)
			if assetPath == "" {
				continue
			}
			if _, exists := seenAssets[assetPath]; exists {
				return pluginUpload{}, fmt.Errorf("duplicate frontend asset %q", assetPath)
			}
			if len(frontendFiles) >= maxPluginFrontendAssets {
				return pluginUpload{}, fmt.Errorf("frontend contains too many files")
			}

			data, err := readZipFile(entry, maxPluginFrontendAssetSize)
			if err != nil {
				return pluginUpload{}, fmt.Errorf("read frontend asset %q: %w", assetPath, err)
			}
			totalFrontendSize += int64(len(data))
			if totalFrontendSize > maxPluginFrontendTotalSize {
				return pluginUpload{}, fmt.Errorf("frontend assets are too large")
			}

			seenAssets[assetPath] = struct{}{}
			frontendFiles = append(frontendFiles, pluginFrontendFile{
				Path:        assetPath,
				Data:        data,
				ContentType: detectFrontendContentType(assetPath, data),
				Size:        int64(len(data)),
			})
			continue
		}

		if path.Dir(cleanName) == "." && strings.HasSuffix(strings.ToLower(cleanName), ".wasm") {
			data, err := readZipFile(entry, maxUploadSize)
			if err != nil {
				return pluginUpload{}, fmt.Errorf("read wasm module %q: %w", cleanName, err)
			}
			wasmCandidates = append(wasmCandidates, pluginFrontendFile{
				Path: cleanName,
				Data: data,
				Size: int64(len(data)),
			})
		}
	}

	wasmBytes, err := selectBundleWasm(wasmCandidates)
	if err != nil {
		return pluginUpload{}, err
	}
	if len(frontendFiles) > 0 {
		if _, ok := seenAssets["index.html"]; !ok {
			return pluginUpload{}, fmt.Errorf("frontend/index.html is required in plugin bundle")
		}
	}

	return pluginUpload{
		Filename:      filename,
		WasmBytes:     wasmBytes,
		FrontendFiles: frontendFiles,
	}, nil
}

func cleanBundlePath(name string) (string, error) {
	if name == "" ||
		strings.Contains(name, "\x00") ||
		strings.Contains(name, "\\") ||
		path.IsAbs(name) {
		return "", fmt.Errorf("invalid bundle path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid bundle path %q", name)
		}
	}
	cleanName := path.Clean(name)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", fmt.Errorf("invalid bundle path %q", name)
	}
	return cleanName, nil
}

func shouldIgnoreBundleEntry(name string) bool {
	return name == "__MACOSX" ||
		strings.HasPrefix(name, "__MACOSX/") ||
		path.Base(name) == ".DS_Store"
}

func validateZipCompressionRatio(entry *zip.File) error {
	uncompressedSize := entry.UncompressedSize64
	if uncompressedSize == 0 {
		return nil
	}

	compressedSize := entry.CompressedSize64
	if compressedSize == 0 {
		return fmt.Errorf("invalid compressed size")
	}

	ratio := uncompressedSize / compressedSize
	hasRemainder := uncompressedSize%compressedSize > 0
	if ratio > maxPluginBundleCompressionRatio ||
		(ratio == maxPluginBundleCompressionRatio && hasRemainder) {
		return fmt.Errorf("compression ratio exceeds %d:1", maxPluginBundleCompressionRatio)
	}
	return nil
}

func readZipFile(entry *zip.File, limit int64) ([]byte, error) {
	if entry.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("file exceeds size limit")
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds size limit")
	}
	return data, nil
}

func selectBundleWasm(candidates []pluginFrontendFile) ([]byte, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("plugin bundle must contain %s or one root .wasm file", pluginBundleWasmName)
	}

	var fallback []byte
	for _, candidate := range candidates {
		if candidate.Path == pluginBundleWasmName {
			return candidate.Data, nil
		}
		fallback = candidate.Data
	}
	if len(candidates) == 1 {
		return fallback, nil
	}
	return nil, fmt.Errorf("plugin bundle contains multiple root .wasm files; name the main one %s", pluginBundleWasmName)
}

func detectFrontendContentType(filePath string, data []byte) string {
	if contentType := mime.TypeByExtension(path.Ext(filePath)); contentType != "" {
		return contentType
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return http.DetectContentType(sample)
}

func cleanFrontendAssetPath(assetPath string) (string, bool) {
	if assetPath == "" {
		return "", true
	}
	if strings.Contains(assetPath, "\x00") ||
		strings.Contains(assetPath, "\\") ||
		path.IsAbs(assetPath) {
		return "", false
	}
	for _, part := range strings.Split(assetPath, "/") {
		if part == ".." {
			return "", false
		}
	}
	cleanPath := path.Clean(assetPath)
	if cleanPath == "." {
		return "", true
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", false
	}
	return cleanPath, true
}
