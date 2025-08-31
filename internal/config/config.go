package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config mirrors the YAML schema. All values should be supplied via YAML; we avoid hard-coded defaults.
// Minimal validation occurs in Validate().
type Config struct {
	Version     int              `yaml:"version"`
	General     General          `yaml:"general"`
	Network     Network          `yaml:"network"`
	Concurrency Concurrency      `yaml:"concurrency"`
	Sources     Sources          `yaml:"sources"`
	Resolver    ResolverConf     `yaml:"resolver"`
	Placement   Placement        `yaml:"placement"`
	Classifier  ClassifierConfig `yaml:"classifier"`
	Logging     Logging          `yaml:"logging"`
	Metrics     Metrics          `yaml:"metrics"`
	Validation  Validation       `yaml:"validation"`
	UI          UIOptions        `yaml:"ui"`
}

type General struct {
	DataRoot       string `yaml:"data_root"`
	DownloadRoot   string `yaml:"download_root"`
	PartialsRoot   string `yaml:"partials_root"`
	PlacementMode  string `yaml:"placement_mode"` // symlink | hardlink | copy
	Quarantine     bool   `yaml:"quarantine"`
	AllowOverwrite bool   `yaml:"allow_overwrite"`
	DryRun         bool   `yaml:"dry_run"`
	// Downloads behavior
	StagePartials  bool `yaml:"stage_partials"`   // if true (default), write .part files under download_root/.parts or partials_root if set
	AlwaysNoResume bool `yaml:"always_no_resume"` // if true, do not resume partials unless overridden on CLI
}

type Network struct {
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	MaxRedirects   int    `yaml:"max_redirects"`
	TLSVerify      bool   `yaml:"tls_verify"`
	UserAgent      string `yaml:"user_agent"`
}

type ResolverConf struct {
	CacheTTLHours int `yaml:"cache_ttl_hours"`
}

type Concurrency struct {
	GlobalFiles     int     `yaml:"global_files"`
	PerFileChunks   int     `yaml:"per_file_chunks"`
	PerHostRequests int     `yaml:"per_host_requests"`
	ChunkSizeMB     int     `yaml:"chunk_size_mb"`
	MaxRetries      int     `yaml:"max_retries"`
	Backoff         Backoff `yaml:"backoff"`
}

type Backoff struct {
	MinMS  int  `yaml:"min_ms"`
	MaxMS  int  `yaml:"max_ms"`
	Jitter bool `yaml:"jitter"`
}

type Sources struct {
	HuggingFace SourceWithToken `yaml:"huggingface"`
	CivitAI     SourceWithToken `yaml:"civitai"`
}

type SourceWithToken struct {
	Enabled  bool   `yaml:"enabled"`
	TokenEnv string `yaml:"token_env"`
}

type ClassifierConfig struct {
	Rules []ClassifierRule `yaml:"rules"`
}

type ClassifierRule struct {
	Regex string `yaml:"regex"`
	Type  string `yaml:"type"`
}

type Placement struct {
	Apps    map[string]AppPlacement `yaml:"apps"`
	Mapping []MappingRule           `yaml:"mapping"`
}

type AppPlacement struct {
	Base  string            `yaml:"base"`
	Paths map[string]string `yaml:"paths"`
}

type MappingRule struct {
	Match   string          `yaml:"match"`
	Targets []MappingTarget `yaml:"targets"`
}

type MappingTarget struct {
	App     string `yaml:"app"`
	PathKey string `yaml:"path_key"`
}

type Logging struct {
	Level  string  `yaml:"level"`  // debug|info|warn|error
	Format string  `yaml:"format"` // human|json
	File   LogFile `yaml:"file"`
}

type LogFile struct {
	Enabled      bool   `yaml:"enabled"`
	Path         string `yaml:"path"`
	MaxMegabytes int    `yaml:"max_megabytes"`
	MaxBackups   int    `yaml:"max_backups"`
	MaxAgeDays   int    `yaml:"max_age_days"`
}

type Metrics struct {
	PrometheusTextfile PromTextfile `yaml:"prometheus_textfile"`
}

type PromTextfile struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type Validation struct {
	RequireSHA256                      bool `yaml:"require_sha256"`
	AcceptMD5SHA1IfProvided            bool `yaml:"accept_md5_sha1_if_provided"`
	SafetensorsDeepVerifyAfterDownload bool `yaml:"safetensors_deep_verify_after_download"`
}

type UIOptions struct {
	// RefreshHz controls the TUI refresh frequency (ticks per second). If 0, defaults to 1.
	// Values above 10 are clamped to 10 to avoid excessive CPU usage.
	RefreshHz int `yaml:"refresh_hz"`
	// ShowURL sets the initial table mode to show URL instead of DEST in the last column.
	// Deprecated in favor of ColumnMode but still honored if ColumnMode is empty.
	ShowURL bool `yaml:"show_url"`
	// ColumnMode controls which field is shown in the last column: dest | url | host
	ColumnMode string `yaml:"column_mode"`
	// Compact reduces columns in the v2 table (hides SPEED/THR) for a denser view.
	Compact bool `yaml:"compact"`
}

// Load reads, parses, expands, and validates a YAML config file.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config path is empty")
	}
	expanded, err := expandTilde(path)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(expanded)
	if err != nil {
		return nil, err
	}
	// Expand ${ENV} placeholders before unmarshalling
	b = []byte(os.ExpandEnv(string(b)))
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if err := c.expandPaths(); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) expandPaths() error {
	var err error
	if c.General.DataRoot, err = expandTilde(c.General.DataRoot); err != nil {
		return err
	}
	if c.General.DownloadRoot, err = expandTilde(c.General.DownloadRoot); err != nil {
		return err
	}
	if c.General.PartialsRoot, err = expandTilde(c.General.PartialsRoot); err != nil {
		return err
	}
	if c.Logging.File.Path, err = expandTilde(c.Logging.File.Path); err != nil {
		return err
	}
	if c.Metrics.PrometheusTextfile.Path, err = expandTilde(c.Metrics.PrometheusTextfile.Path); err != nil {
		return err
	}
	for name, app := range c.Placement.Apps {
		if app.Base != "" {
			exp, err := expandTilde(app.Base)
			if err != nil {
				return fmt.Errorf("placement.apps.%s.base: %w", name, err)
			}
			app.Base = exp
			c.Placement.Apps[name] = app
		}
	}
	return nil
}

func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d", c.Version)
	}
	if c.General.DataRoot == "" {
		return errors.New("general.data_root is required")
	}
	if c.General.DownloadRoot == "" {
		return errors.New("general.download_root is required")
	}
	if c.Resolver.CacheTTLHours < 0 {
		return fmt.Errorf("resolver.cache_ttl_hours must be >= 0")
	}
	lvl := stringsLower(c.Logging.Level)
	switch lvl {
	case "", "debug", "info", "warn", "error":
		// ok
	default:
		return fmt.Errorf("logging.level invalid: %s", c.Logging.Level)
	}
	fmtStr := stringsLower(c.Logging.Format)
	switch fmtStr {
	case "", "human", "json":
		// ok
	default:
		return fmt.Errorf("logging.format invalid: %s", c.Logging.Format)
	}
	for i, r := range c.Classifier.Rules {
		if r.Regex == "" || r.Type == "" {
			return fmt.Errorf("classifier.rules[%d]: regex and type required", i)
		}
		if _, err := regexp.Compile(r.Regex); err != nil {
			return fmt.Errorf("classifier.rules[%d].regex: %v", i, err)
		}
	}
	if c.UI.RefreshHz < 0 {
		return fmt.Errorf("ui.refresh_hz must be >= 0")
	}
	return nil
}

func expandTilde(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if p[0] != '~' {
		return p, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return h, nil
	}
	return filepath.Join(h, p[2:]), nil
}

func stringsLower(s string) string {
	b := []byte(s)
	for i := range b {
		if 'A' <= b[i] && b[i] <= 'Z' {
			b[i] = b[i] + 32
		}
	}
	return string(b)
}

// Ensure paths that should exist (optional helper for future use)
func EnsureDir(path string, perm fs.FileMode) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, perm)
}
