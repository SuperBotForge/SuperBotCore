package api

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParsePluginUploadAcceptsRawWASM(t *testing.T) {
	t.Parallel()

	upload, err := parsePluginUpload("schedule.wasm", strings.NewReader("wasm"))
	if err != nil {
		t.Fatalf("parsePluginUpload() error = %v", err)
	}
	if string(upload.WasmBytes) != "wasm" {
		t.Fatalf("WasmBytes = %q, want %q", string(upload.WasmBytes), "wasm")
	}
	if len(upload.FrontendFiles) != 0 {
		t.Fatalf("FrontendFiles len = %d, want 0", len(upload.FrontendFiles))
	}
}

func TestParsePluginUploadAcceptsBundle(t *testing.T) {
	t.Parallel()

	raw := zipBundle(t, map[string]string{
		"plugin.wasm":            "wasm",
		"frontend/index.html":    "<html></html>",
		"frontend/assets/app.js": "console.log('ok')",
	})

	upload, err := parsePluginUpload("schedule.zip", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parsePluginUpload() error = %v", err)
	}
	if string(upload.WasmBytes) != "wasm" {
		t.Fatalf("WasmBytes = %q, want %q", string(upload.WasmBytes), "wasm")
	}
	if len(upload.FrontendFiles) != 2 {
		t.Fatalf("FrontendFiles len = %d, want 2", len(upload.FrontendFiles))
	}
}

func TestParsePluginUploadRejectsUnsafeBundlePath(t *testing.T) {
	t.Parallel()

	raw := zipBundle(t, map[string]string{
		"plugin.wasm":           "wasm",
		"frontend/index.html":   "<html></html>",
		"frontend/../secret.js": "bad",
	})

	_, err := parsePluginUpload("schedule.zip", bytes.NewReader(raw))
	if err == nil {
		t.Fatal("parsePluginUpload() error = nil, want error")
	}
}

func TestParsePluginUploadRequiresFrontendIndex(t *testing.T) {
	t.Parallel()

	raw := zipBundle(t, map[string]string{
		"plugin.wasm":     "wasm",
		"frontend/app.js": "console.log('ok')",
	})

	_, err := parsePluginUpload("schedule.zip", bytes.NewReader(raw))
	if err == nil {
		t.Fatal("parsePluginUpload() error = nil, want error")
	}
}

func TestParsePluginUploadRejectsNonRegularBundleEntry(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mustWriteZipFile(t, zw, "plugin.wasm", "wasm")
	mustWriteZipFile(t, zw, "frontend/index.html", "<html></html>")

	header := &zip.FileHeader{Name: "frontend/link.js"}
	header.SetMode(os.ModeSymlink | 0o777)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatalf("zip create symlink: %v", err)
	}
	if _, err := w.Write([]byte("target.js")); err != nil {
		t.Fatalf("zip write symlink: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	_, err = parsePluginUpload("schedule.zip", bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("parsePluginUpload() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("error = %q, want regular file rejection", err.Error())
	}
}

func TestParsePluginUploadRejectsHighCompressionRatio(t *testing.T) {
	t.Parallel()

	raw := zipBundle(t, map[string]string{
		"plugin.wasm":         "wasm",
		"frontend/index.html": strings.Repeat("a", 1<<20),
	})

	_, err := parsePluginUpload("schedule.zip", bytes.NewReader(raw))
	if err == nil {
		t.Fatal("parsePluginUpload() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "compression ratio") {
		t.Fatalf("error = %q, want compression ratio rejection", err.Error())
	}
}

func zipBundle(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		mustWriteZipFile(t, zw, name, content)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func mustWriteZipFile(t *testing.T, zw *zip.Writer, name, content string) {
	t.Helper()

	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create %q: %v", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("zip write %q: %v", name, err)
	}
}
