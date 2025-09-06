package state

import (
	"github.com/jxwalker/modfetch/internal/config"
	"testing"
)

func TestHostCapsUpsertGet(t *testing.T) {
	cfg := &config.Config{Version: 1, General: config.General{DataRoot: t.TempDir(), DownloadRoot: t.TempDir()}}
	dbptr, err := Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = dbptr.SQL.Close() }()

	host := "example.com"
	if err := dbptr.UpsertHostCaps(host, true, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, ok, err := dbptr.GetHostCaps(host)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatalf("expected row")
	}
	if !got.HeadOK || got.AcceptRanges {
		t.Fatalf("unexpected caps: %+v", got)
	}
}
