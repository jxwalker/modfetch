package config

import (
	"fmt"
	"os"
	"strings"

	friendlyerrors "github.com/jxwalker/modfetch/internal/errors"
)

// ValidationError represents a detailed config validation error
type ValidationError struct {
	Field      string
	Value      interface{}
	Message    string
	Suggestion string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Config validation error in '%s': %s", e.Field, e.Message)
}

// ValidateDetailed performs comprehensive validation with friendly error messages
func (c *Config) ValidateDetailed() []ValidationError {
	var errs []ValidationError

	// Version check
	if c.Version != 1 {
		errs = append(errs, ValidationError{
			Field:      "version",
			Value:      c.Version,
			Message:    fmt.Sprintf("Unsupported version: %d", c.Version),
			Suggestion: "Use version: 1",
		})
	}

	// General section
	if c.General.DataRoot == "" {
		errs = append(errs, ValidationError{
			Field:      "general.data_root",
			Message:    "Required field missing",
			Suggestion: "Set to a directory for modfetch data:\n  data_root: ~/.local/share/modfetch",
		})
	}

	if c.General.DownloadRoot == "" {
		errs = append(errs, ValidationError{
			Field:      "general.download_root",
			Message:    "Required field missing",
			Suggestion: "Set to a directory for downloads:\n  download_root: ~/Downloads/modfetch",
		})
	}

	// Check for conflicting options
	if c.General.AlwaysNoResume && c.General.AutoRecoverOnStart {
		errs = append(errs, ValidationError{
			Field:      "general.always_no_resume",
			Value:      true,
			Message:    "Conflicts with auto_recover_on_start=true",
			Suggestion: "Set one of these to false",
		})
	}

	// Placement mode validation
	if c.General.PlacementMode != "" {
		validModes := []string{"symlink", "hardlink", "copy"}
		found := false
		for _, mode := range validModes {
			if c.General.PlacementMode == mode {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, ValidationError{
				Field:      "general.placement_mode",
				Value:      c.General.PlacementMode,
				Message:    "Invalid placement mode",
				Suggestion: "Use one of: symlink, hardlink, copy",
			})
		}
	}

	// Concurrency checks
	if c.Concurrency.ChunkSizeMB < 1 || c.Concurrency.ChunkSizeMB > 1000 {
		errs = append(errs, ValidationError{
			Field:      "concurrency.chunk_size_mb",
			Value:      c.Concurrency.ChunkSizeMB,
			Message:    "Must be between 1 and 1000 MB",
			Suggestion: "Recommended: 32-128 MB for best performance",
		})
	}

	if c.Concurrency.PerFileChunks < 1 {
		errs = append(errs, ValidationError{
			Field:      "concurrency.per_file_chunks",
			Value:      c.Concurrency.PerFileChunks,
			Message:    "Must be at least 1",
			Suggestion: "Recommended: 4-16 chunks",
		})
	}

	if c.Concurrency.PerFileChunks > 100 {
		errs = append(errs, ValidationError{
			Field:      "concurrency.per_file_chunks",
			Value:      c.Concurrency.PerFileChunks,
			Message:    "Unusually high (>100 chunks)",
			Suggestion: "High values may not improve performance. Try 4-16.",
		})
	}

	if c.Concurrency.MaxRetries < 0 {
		errs = append(errs, ValidationError{
			Field:      "concurrency.max_retries",
			Value:      c.Concurrency.MaxRetries,
			Message:    "Must be >= 0",
			Suggestion: "Recommended: 3-10 retries",
		})
	}

	// Network checks
	if c.Network.TimeoutSeconds < 1 {
		errs = append(errs, ValidationError{
			Field:      "network.timeout_seconds",
			Value:      c.Network.TimeoutSeconds,
			Message:    "Must be at least 1 second",
			Suggestion: "Recommended: 30-120 seconds",
		})
	}

	if c.Network.TimeoutSeconds > 3600 {
		errs = append(errs, ValidationError{
			Field:      "network.timeout_seconds",
			Value:      c.Network.TimeoutSeconds,
			Message:    "Very long timeout (>1 hour)",
			Suggestion: "Consider reducing to 30-300 seconds",
		})
	}

	if c.Network.MaxRedirects < 0 {
		errs = append(errs, ValidationError{
			Field:      "network.max_redirects",
			Value:      c.Network.MaxRedirects,
			Message:    "Must be >= 0",
			Suggestion: "Recommended: 5-10 redirects",
		})
	}

	// Backoff validation
	if c.Concurrency.Backoff.MinMS < 0 {
		errs = append(errs, ValidationError{
			Field:      "concurrency.backoff.min_ms",
			Value:      c.Concurrency.Backoff.MinMS,
			Message:    "Must be >= 0",
			Suggestion: "Recommended: 100-1000 ms",
		})
	}

	if c.Concurrency.Backoff.MaxMS < c.Concurrency.Backoff.MinMS {
		errs = append(errs, ValidationError{
			Field:      "concurrency.backoff.max_ms",
			Value:      c.Concurrency.Backoff.MaxMS,
			Message:    "max_ms must be >= min_ms",
			Suggestion: fmt.Sprintf("Set max_ms to at least %d", c.Concurrency.Backoff.MinMS),
		})
	}

	// Logging validation
	lvl := strings.ToLower(c.Logging.Level)
	validLevels := []string{"", "debug", "info", "warn", "error"}
	found := false
	for _, valid := range validLevels {
		if lvl == valid {
			found = true
			break
		}
	}
	if !found {
		errs = append(errs, ValidationError{
			Field:      "logging.level",
			Value:      c.Logging.Level,
			Message:    "Invalid log level",
			Suggestion: "Use one of: debug, info, warn, error",
		})
	}

	// Check token environment variables are set if sources are enabled
	if c.Sources.HuggingFace.Enabled {
		tokenEnv := c.Sources.HuggingFace.TokenEnv
		if tokenEnv == "" {
			tokenEnv = "HF_TOKEN"
		}
		if os.Getenv(tokenEnv) == "" {
			errs = append(errs, ValidationError{
				Field:      "sources.huggingface",
				Message:    fmt.Sprintf("HuggingFace enabled but %s not set", tokenEnv),
				Suggestion: fmt.Sprintf("Set the token:\n  export %s=hf_...\n  Get one at: https://huggingface.co/settings/tokens", tokenEnv),
			})
		}
	}

	if c.Sources.CivitAI.Enabled {
		tokenEnv := c.Sources.CivitAI.TokenEnv
		if tokenEnv == "" {
			tokenEnv = "CIVITAI_TOKEN"
		}
		if os.Getenv(tokenEnv) == "" {
			errs = append(errs, ValidationError{
				Field:      "sources.civitai",
				Message:    fmt.Sprintf("CivitAI enabled but %s not set", tokenEnv),
				Suggestion: fmt.Sprintf("Set the token:\n  export %s=...\n  Get one at: https://civitai.com/user/account", tokenEnv),
			})
		}
	}

	return errs
}

// ValidateWithFriendlyErrors returns a user-friendly validation error
func (c *Config) ValidateWithFriendlyErrors() error {
	// Run standard validation first
	if err := c.Validate(); err != nil {
		return err
	}

	// Run detailed validation
	errs := c.ValidateDetailed()
	if len(errs) == 0 {
		return nil
	}

	// Build friendly error message
	var msg strings.Builder
	msg.WriteString("Configuration validation failed:\n\n")

	for i, err := range errs {
		msg.WriteString(fmt.Sprintf("%d. %s\n", i+1, err.Error()))
		if err.Value != nil {
			msg.WriteString(fmt.Sprintf("   Current value: %v\n", err.Value))
		}
		if err.Suggestion != "" {
			lines := strings.Split(err.Suggestion, "\n")
			for _, line := range lines {
				msg.WriteString(fmt.Sprintf("   → %s\n", line))
			}
		}
		msg.WriteString("\n")
	}

	return friendlyerrors.NewFriendlyError(
		"Config validation failed",
		msg.String(),
	).WithDocs("https://github.com/jxwalker/modfetch#configuration")
}
