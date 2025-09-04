package config

import "testing"

func TestLoadSampleConfig(t *testing.T) {
	path := "../../assets/sample-config/config.example.yml"
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if c.Version != 1 {
		t.Fatalf("expected version 1, got %d", c.Version)
	}
	if c.General.DataRoot == "" || c.General.DownloadRoot == "" {
		t.Fatalf("expected non-empty general paths")
	}
}
