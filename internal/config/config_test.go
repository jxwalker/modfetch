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

func TestValidateRejectsNegativeBandwidthLimits(t *testing.T) {
	base := Config{
		Version: 1,
		General: General{
			DataRoot:     t.TempDir(),
			DownloadRoot: t.TempDir(),
		},
	}

	c := base
	c.Network.GlobalBandwidthBytesPerSecond = -1
	if err := c.Validate(); err == nil {
		t.Fatal("expected negative global bandwidth limit to fail validation")
	}

	c = base
	c.Network.PerDownloadBandwidthBytesPerSecond = -1
	if err := c.Validate(); err == nil {
		t.Fatal("expected negative per-download bandwidth limit to fail validation")
	}
}
