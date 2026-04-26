package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/state"
)

func TestVerifyCompleteChunksFindsDirtyChunks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.part")
	if err := os.WriteFile(path, []byte("aaaabbbbcccc"), 0o644); err != nil {
		t.Fatal(err)
	}
	chunks := []state.ChunkRow{
		{Index: 2, Start: 8, Size: 4, SHA256: chunkSHA("cccc"), Status: "running"},
		{Index: 1, Start: 4, Size: 4, SHA256: chunkSHA("xxxx"), Status: "complete"},
		{Index: 0, Start: 0, Size: 4, SHA256: chunkSHA("aaaa"), Status: "complete"},
		{Index: 3, Start: 8, Size: 4, SHA256: "", Status: "complete"},
	}

	dirty, err := verifyCompleteChunks(context.Background(), path, chunks, 2)
	if err != nil {
		t.Fatalf("verify chunks: %v", err)
	}
	if len(dirty) != 2 || dirty[0].Index != 1 || dirty[1].Index != 3 {
		t.Fatalf("expected chunks 1 and 3 dirty, got %+v", dirty)
	}
}

func TestVerifyCompleteChunksHonorsContextCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.part")
	if err := os.WriteFile(path, []byte("aaaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := verifyCompleteChunks(ctx, path, []state.ChunkRow{
		{Index: 0, Start: 0, Size: 4, SHA256: chunkSHA("aaaa"), Status: "complete"},
	}, 1)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func chunkSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
