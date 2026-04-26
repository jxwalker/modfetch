package downloader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/storage"
)

func TestAutoDownloadsToS3Destination(t *testing.T) {
	payload := []byte("model payload")
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "13")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Length", "13")
		_, _ = w.Write(payload)
	}))
	defer source.Close()

	objects := map[string]string{}
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		objects[r.URL.EscapedPath()] = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer s3.Close()
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	tmp := t.TempDir()
	cfg := &config.Config{
		General: config.General{DataRoot: filepath.Join(tmp, "data"), DownloadRoot: filepath.Join(tmp, "downloads"), StagePartials: true},
		Storage: config.Storage{S3: config.S3Storage{
			Endpoint:         s3.URL,
			UseHTTP:          true,
			PathStyle:        true,
			UploadSHA256File: true,
		}},
	}
	db, err := state.NewDB(filepath.Join(tmp, "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer func() { _ = db.Close() }()

	dest := "s3://models/remote.bin"
	final, sum, err := NewAuto(cfg, logging.New("error", false), db, nil).Download(context.Background(), source.URL+"/model.bin", dest, "", nil, false)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if final != dest {
		t.Fatalf("expected final s3 destination %q, got %q", dest, final)
	}
	if sum == "" {
		t.Fatal("expected sha256")
	}
	if got := objects["/models/remote.bin"]; got != string(payload) {
		t.Fatalf("unexpected uploaded object %q", got)
	}
	if got := objects["/models/remote.bin.sha256"]; !strings.Contains(got, sum) {
		t.Fatalf("expected uploaded checksum sidecar to contain %s, got %q", sum, got)
	}
	local, err := storage.StagingPath(cfg, dest, source.URL+"/model.bin")
	if err != nil {
		t.Fatalf("staging path: %v", err)
	}
	if !strings.Contains(local, filepath.Join("s3-staging", "models")) {
		t.Fatalf("unexpected local staging path %q", local)
	}
	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 || rows[0].Dest != dest || rows[0].Status != "complete" {
		t.Fatalf("expected complete s3 state row, got %+v", rows)
	}
}
