package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestLoadStrictSampleConfig(t *testing.T) {
	path := "../../assets/sample-config/config.example.yml"
	if _, err := LoadStrict(path); err != nil {
		t.Fatalf("LoadStrict returned error for sample config: %v", err)
	}
}

func TestLoadStrictRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(dir, "data") + "\n" +
		"  download_root: " + filepath.Join(dir, "downloads") + "\n" +
		"  download_rooot: typo\n"
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("non-strict Load should ignore unknown fields, got %v", err)
	}
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "download_rooot") {
		t.Fatalf("expected strict unknown-field error, got %v", err)
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

func TestValidateRejectsNegativeDNSCacheTTL(t *testing.T) {
	c := Config{
		Version: 1,
		General: General{
			DataRoot:     t.TempDir(),
			DownloadRoot: t.TempDir(),
		},
	}
	c.Network.DNSCacheTTLSeconds = -1
	if err := c.Validate(); err == nil {
		t.Fatal("expected negative DNS cache TTL to fail validation")
	}
}

func TestValidateS3RequiresCredentialsWhenEndpointConfigured(t *testing.T) {
	t.Setenv("MODFETCH_TEST_S3_ACCESS", "")
	t.Setenv("MODFETCH_TEST_S3_SECRET", "")
	c := Config{
		Version: 1,
		General: General{
			DataRoot:     t.TempDir(),
			DownloadRoot: t.TempDir(),
		},
		Storage: Storage{S3: S3Storage{
			Endpoint:     "http://localhost:9000",
			AccessKeyEnv: "MODFETCH_TEST_S3_ACCESS",
			SecretKeyEnv: "MODFETCH_TEST_S3_SECRET",
		}},
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "storage.s3 credentials missing") {
		t.Fatalf("expected missing s3 credential error, got %v", err)
	}
}

func TestValidateS3AcceptsConfiguredCredentials(t *testing.T) {
	t.Setenv("MODFETCH_TEST_S3_ACCESS", "access")
	t.Setenv("MODFETCH_TEST_S3_SECRET", "secret")
	c := Config{
		Version: 1,
		General: General{
			DataRoot:     t.TempDir(),
			DownloadRoot: t.TempDir(),
		},
		Storage: Storage{S3: S3Storage{
			Endpoint:     "localhost:9000",
			UseHTTP:      true,
			AccessKeyEnv: "MODFETCH_TEST_S3_ACCESS",
			SecretKeyEnv: "MODFETCH_TEST_S3_SECRET",
		}},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected s3 config to validate, got %v", err)
	}
}

func TestValidateS3RejectsInvalidEndpoint(t *testing.T) {
	t.Setenv("MODFETCH_TEST_S3_ACCESS", "access")
	t.Setenv("MODFETCH_TEST_S3_SECRET", "secret")
	c := Config{
		Version: 1,
		General: General{
			DataRoot:     t.TempDir(),
			DownloadRoot: t.TempDir(),
		},
		Storage: Storage{S3: S3Storage{
			Endpoint:     "ftp://example.com",
			AccessKeyEnv: "MODFETCH_TEST_S3_ACCESS",
			SecretKeyEnv: "MODFETCH_TEST_S3_SECRET",
		}},
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "storage.s3.endpoint") {
		t.Fatalf("expected invalid s3 endpoint error, got %v", err)
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
