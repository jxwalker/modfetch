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

func TestValidateRejectsNegativeConcurrencyValues(t *testing.T) {
	base := Config{
		Version: 1,
		General: General{
			DataRoot:     t.TempDir(),
			DownloadRoot: t.TempDir(),
		},
	}

	tests := []struct {
		name string
		edit func(*Config)
	}{
		{"global files", func(c *Config) { c.Concurrency.GlobalFiles = -1 }},
		{"per file chunks", func(c *Config) { c.Concurrency.PerFileChunks = -1 }},
		{"per host requests", func(c *Config) { c.Concurrency.PerHostRequests = -1 }},
		{"chunk size", func(c *Config) { c.Concurrency.ChunkSizeMB = -1 }},
		{"max retries", func(c *Config) { c.Concurrency.MaxRetries = -1 }},
		{"backoff min", func(c *Config) { c.Concurrency.Backoff.MinMS = -1 }},
		{"backoff max", func(c *Config) { c.Concurrency.Backoff.MaxMS = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			tt.edit(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
