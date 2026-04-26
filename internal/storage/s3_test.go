package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestPutFileSignsAndUploadsPathStyleObject(t *testing.T) {
	var gotPath, gotAuth, gotHash, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		gotHash = r.Header.Get("X-Amz-Content-Sha256")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "model.bin")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	client, err := NewS3ClientFromConfig(&config.Config{Storage: config.Storage{S3: config.S3Storage{Endpoint: srv.URL, UseHTTP: true}}})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := client.PutFile(context.Background(), "s3://models/path/to/model.bin", src, "application/octet-stream"); err != nil {
		t.Fatalf("put file: %v", err)
	}

	if gotPath != "/models/path/to/model.bin" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=test-access/") {
		t.Fatalf("expected SigV4 authorization header, got %q", gotAuth)
	}
	if gotHash == "" {
		t.Fatal("expected payload hash header")
	}
	if gotBody != "payload" {
		t.Fatalf("unexpected body %q", gotBody)
	}
}

func TestStagingPathIsStableForS3Destination(t *testing.T) {
	cfg := &config.Config{General: config.General{DataRoot: t.TempDir()}}
	a, err := StagingPath(cfg, "s3://bucket/models/file.bin", "https://example.com/file.bin")
	if err != nil {
		t.Fatalf("staging path: %v", err)
	}
	b, err := StagingPath(cfg, "s3://bucket/models/file.bin", "https://example.com/file.bin")
	if err != nil {
		t.Fatalf("staging path second call: %v", err)
	}
	if a != b {
		t.Fatalf("expected stable staging path, got %q and %q", a, b)
	}
	if !strings.Contains(a, filepath.Join("s3-staging", "bucket")) {
		t.Fatalf("expected staging path under s3-staging bucket dir, got %q", a)
	}
}

func TestNewS3ClientRejectsEndpointWithoutHost(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	_, err := NewS3ClientFromConfig(&config.Config{Storage: config.Storage{S3: config.S3Storage{Endpoint: "http://"}}})
	if err == nil || !strings.Contains(err.Error(), "storage.s3.endpoint") {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}
}
