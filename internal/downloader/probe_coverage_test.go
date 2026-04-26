package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestStagePartPathModes(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{General: config.General{DownloadRoot: filepath.Join(tmp, "downloads"), StagePartials: true}}
	dest := filepath.Join(tmp, "models", "model.gguf")

	got := StagePartPath(cfg, "https://example.com/model.gguf", dest)
	if !strings.HasPrefix(got, filepath.Join(cfg.General.DownloadRoot, ".parts")+string(os.PathSeparator)) {
		t.Fatalf("expected part under download_root .parts, got %q", got)
	}
	if !strings.HasSuffix(got, ".part") || !strings.Contains(filepath.Base(got), "model.gguf.") {
		t.Fatalf("unexpected stage part name: %q", got)
	}

	custom := filepath.Join(tmp, "partials")
	cfg.General.PartialsRoot = custom
	if got := stagePartPath(cfg, "https://example.com/model.gguf", dest); !strings.HasPrefix(got, custom+string(os.PathSeparator)) {
		t.Fatalf("expected custom partials root, got %q", got)
	}

	cfg.General.StagePartials = false
	if got := StagePartPath(cfg, "https://example.com/model.gguf", dest); got != dest+".part" {
		t.Fatalf("expected legacy part path, got %q", got)
	}
}

func TestRenameOrCopyAndCopyFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dst := filepath.Join(tmp, "dst.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copy file: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected copied data: %q", string(data))
	}

	renamed := filepath.Join(tmp, "renamed.txt")
	if err := renameOrCopy(dst, renamed); err != nil {
		t.Fatalf("rename or copy: %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("expected source removed after rename, err=%v", err)
	}
	if data, err := os.ReadFile(renamed); err != nil || string(data) != "payload" {
		t.Fatalf("unexpected renamed data=%q err=%v", string(data), err)
	}
	if err := renameOrCopy(filepath.Join(tmp, "missing"), filepath.Join(tmp, "other")); err == nil {
		t.Fatal("expected missing source error")
	}
}

func TestProbeURLRangeFallbackAndComputeRemoteSHA256(t *testing.T) {
	body := []byte("hash me")
	var sawAuth atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer token" {
			sawAuth.Store(true)
		}
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if r.Header.Get("Range") == "bytes=0-0" {
				w.Header().Set("Content-Range", "bytes 0-0/7")
				w.Header().Set("Content-Disposition", `attachment; filename="model.gguf"`)
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(body[:1])
				return
			}
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	cfg := &config.Config{Network: config.Network{TimeoutSeconds: 2}}
	headers := map[string]string{"Authorization": "Bearer token"}
	meta, err := ProbeURL(context.Background(), cfg, server.URL, headers)
	if err != nil {
		t.Fatalf("probe url: %v", err)
	}
	if !sawAuth.Load() {
		t.Fatal("expected probe requests to include auth header")
	}
	if meta.Size != 7 || !meta.AcceptRange || meta.Filename != "model.gguf" || meta.FinalURL != server.URL {
		t.Fatalf("unexpected probe metadata: %+v", meta)
	}

	sum, err := ComputeRemoteSHA256(context.Background(), cfg, server.URL, headers)
	if err != nil {
		t.Fatalf("compute remote sha: %v", err)
	}
	want := sha256.Sum256(body)
	if sum != hex.EncodeToString(want[:]) {
		t.Fatalf("unexpected remote sha: %s", sum)
	}
}

func TestComputeRemoteSHA256RejectsNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()

	_, err := ComputeRemoteSHA256(context.Background(), &config.Config{Network: config.Network{TimeoutSeconds: 2}}, server.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "GET status: 403") {
		t.Fatalf("expected non-success status error, got %v", err)
	}
}

func TestSafetensorsAdjustAndDeepVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.safetensors")
	writeSafetensorsFile(t, path, map[string]any{
		"tensor": map[string]any{
			"dtype":        "F32",
			"shape":        []int{2},
			"data_offsets": []int{0, 8},
		},
	}, []byte("12345678extra"))

	changed, err := adjustSafetensors(path, nil)
	if err != nil {
		t.Fatalf("adjust safetensors: %v", err)
	}
	if !changed {
		t.Fatal("expected trailing bytes to be truncated")
	}
	ok, declared, err := deepVerifySafetensors(path)
	if err != nil || !ok {
		t.Fatalf("deep verify adjusted file ok=%v declared=%d err=%v", ok, declared, err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat adjusted file: %v", err)
	}
	if fi.Size() != declared {
		t.Fatalf("expected file size %d, got %d", declared, fi.Size())
	}

	plain := filepath.Join(t.TempDir(), "plain.bin")
	if err := os.WriteFile(plain, []byte("not safetensors"), 0o644); err != nil {
		t.Fatalf("write plain: %v", err)
	}
	if changed, err := adjustSafetensors(plain, nil); err != nil || changed {
		t.Fatalf("plain adjust changed=%v err=%v", changed, err)
	}
	if ok, declared, err := deepVerifySafetensors(plain); err != nil || !ok || declared != 0 {
		t.Fatalf("plain deep verify ok=%v declared=%d err=%v", ok, declared, err)
	}
}

func TestSafetensorsValidationErrors(t *testing.T) {
	tmp := t.TempDir()
	short := filepath.Join(tmp, "short.safetensors")
	if err := os.WriteFile(short, []byte("tiny"), 0o644); err != nil {
		t.Fatalf("write short: %v", err)
	}
	if _, err := adjustSafetensors(short, nil); err == nil || !strings.Contains(err.Error(), "file too small") {
		t.Fatalf("expected small file error, got %v", err)
	}

	badSize := filepath.Join(tmp, "bad-size.safetensors")
	writeSafetensorsFile(t, badSize, map[string]any{
		"tensor": map[string]any{
			"dtype":        "F32",
			"shape":        []int{3},
			"data_offsets": []int{0, 8},
		},
	}, []byte("12345678"))
	if ok, _, err := deepVerifySafetensors(badSize); err == nil || ok || !strings.Contains(err.Error(), "size 8 != 3*4") {
		t.Fatalf("expected tensor size error, ok=%v err=%v", ok, err)
	}

	beyond := filepath.Join(tmp, "beyond.safetensors")
	writeSafetensorsFile(t, beyond, map[string]any{
		"tensor": map[string]any{
			"dtype":        "U8",
			"shape":        []int{3},
			"data_offsets": []int{0, 4},
		},
	}, []byte("123"))
	if _, err := adjustSafetensors(beyond, nil); err == nil || !strings.Contains(err.Error(), "beyond data length") {
		t.Fatalf("expected beyond data length error, got %v", err)
	}
}

func TestSafetensorsDtypeBytesAndToInt64(t *testing.T) {
	if dtypeBytes(" bf16 ") != 2 || dtypeBytes("BOOL") != 1 || dtypeBytes("unknown") != 0 {
		t.Fatal("unexpected dtype byte sizes")
	}
	if got, ok := toInt64(float64(42)); !ok || got != 42 {
		t.Fatalf("float64 conversion got %d ok=%v", got, ok)
	}
	if got, ok := toInt64(int64(43)); !ok || got != 43 {
		t.Fatalf("int64 conversion got %d ok=%v", got, ok)
	}
	if got, ok := toInt64(44); !ok || got != 44 {
		t.Fatalf("int conversion got %d ok=%v", got, ok)
	}
	if _, ok := toInt64("45"); ok {
		t.Fatal("string should not convert to int64")
	}
}

func writeSafetensorsFile(t *testing.T, path string, header map[string]any, data []byte) {
	t.Helper()
	hdr, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint64(len(hdr))); err != nil {
		t.Fatalf("write header length: %v", err)
	}
	buf.Write(hdr)
	buf.Write(data)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write safetensors: %v", err)
	}
}
