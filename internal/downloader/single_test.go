package downloader

import (
	"context"
	"os"
	"testing"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// Integration test that downloads a tiny public text file from Hugging Face.
func TestSingleDownloadHF(t *testing.T) {
	if testing.Short() { t.Skip("-short set") }
	tmp := t.TempDir()
	cfgYaml := []byte(""+
		"version: 1\n"+
		"general:\n"+
		"  data_root: \""+tmp+"/data\"\n"+
		"  download_root: \""+tmp+"/dl\"\n"+
		"  placement_mode: \"symlink\"\n"+
		"  quarantine: false\n"+
		"  allow_overwrite: true\n")
	cfgPath := tmp+"/config.yml"
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config load: %v", err) }

	log := logging.New("info", false)
	st, err := state.Open(cfg)
	if err != nil { t.Fatalf("state open: %v", err) }
	defer st.SQL.Close()

	dl := NewSingle(cfg, log, st, nil)
	final, sum, err := dl.Download(context.Background(), "https://raw.githubusercontent.com/github/gitignore/main/Go.gitignore", "", "", nil, false)
	if err != nil { t.Fatalf("download: %v", err) }
	if _, err := os.Stat(final); err != nil { t.Fatalf("final file missing: %v", err) }
	if len(sum) != 64 { t.Fatalf("unexpected sha256 length: %d", len(sum)) }
}

