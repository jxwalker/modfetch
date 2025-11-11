package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/system"
)

// Check represents a single diagnostic check
type Check struct {
	Name        string
	Run         func(ctx context.Context) CheckResult
	Critical    bool // If true, failure suggests modfetch won't work
	Description string
}

// CheckResult represents the result of a diagnostic check
type CheckResult struct {
	Passed     bool
	Warning    bool // Passed but with warnings
	Message    string
	Suggestion string
}

func handleDoctor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	verbose := fs.Bool("verbose", false, "Show detailed output for each check")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Try to load config, but don't fail if it doesn't exist
	var cfg *config.Config
	var cfgErr error

	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		} else {
			if h, err := os.UserHomeDir(); err == nil && h != "" {
				*cfgPath = filepath.Join(h, ".config", "modfetch", "config.yml")
			}
		}
	}

	if *cfgPath != "" {
		cfg, cfgErr = config.Load(*cfgPath)
	}

	fmt.Println("Running modfetch diagnostics...\n")

	// Define all checks
	checks := []Check{
		{
			Name:        "Config file exists",
			Critical:    true,
			Description: "Configuration file must exist for modfetch to work",
			Run: func(ctx context.Context) CheckResult {
				if *cfgPath == "" {
					return CheckResult{
						Passed:     false,
						Message:    "No config path specified",
						Suggestion: "Set MODFETCH_CONFIG or use --config flag\nRun 'modfetch setup' to create a configuration",
					}
				}

				if _, err := os.Stat(*cfgPath); err != nil {
					return CheckResult{
						Passed:     false,
						Message:    fmt.Sprintf("Config file not found: %s", *cfgPath),
						Suggestion: "Run 'modfetch setup' to create a configuration",
					}
				}

				return CheckResult{
					Passed:  true,
					Message: fmt.Sprintf("Found: %s", *cfgPath),
				}
			},
		},
		{
			Name:        "Config is valid YAML",
			Critical:    true,
			Description: "Configuration must be valid YAML syntax",
			Run: func(ctx context.Context) CheckResult {
				if cfgErr != nil {
					return CheckResult{
						Passed:     false,
						Message:    "Config parsing failed",
						Suggestion: fmt.Sprintf("Fix config errors:\n%v\n\nRun 'modfetch config validate' for details", cfgErr),
					}
				}

				if cfg == nil {
					return CheckResult{
						Passed:  false,
						Message: "Config not loaded",
					}
				}

				return CheckResult{
					Passed:  true,
					Message: "Valid YAML syntax",
				}
			},
		},
		{
			Name:        "Download directory exists and is writable",
			Critical:    true,
			Description: "Downloads must have a valid destination directory",
			Run: func(ctx context.Context) CheckResult {
				if cfg == nil {
					return CheckResult{Passed: false, Message: "Config not loaded"}
				}

				if cfg.General.DownloadRoot == "" {
					return CheckResult{
						Passed:     false,
						Message:    "general.download_root not set in config",
						Suggestion: "Add download_root to your config file",
					}
				}

				// Check if directory exists
				info, err := os.Stat(cfg.General.DownloadRoot)
				if err != nil {
					if os.IsNotExist(err) {
						// Try to create it
						if err := os.MkdirAll(cfg.General.DownloadRoot, 0755); err != nil {
							return CheckResult{
								Passed:     false,
								Message:    fmt.Sprintf("Directory doesn't exist and can't be created: %s", cfg.General.DownloadRoot),
								Suggestion: fmt.Sprintf("Create manually: mkdir -p %s", cfg.General.DownloadRoot),
							}
						}
						return CheckResult{
							Passed:  true,
							Warning: true,
							Message: fmt.Sprintf("Created directory: %s", cfg.General.DownloadRoot),
						}
					}
					return CheckResult{
						Passed:     false,
						Message:    fmt.Sprintf("Cannot access: %s", err),
						Suggestion: "Check file permissions",
					}
				}

				if !info.IsDir() {
					return CheckResult{
						Passed:     false,
						Message:    "Path exists but is not a directory",
						Suggestion: "Remove the file or choose a different download_root",
					}
				}

				// Test write permission
				testFile := filepath.Join(cfg.General.DownloadRoot, ".modfetch_write_test")
				if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
					return CheckResult{
						Passed:     false,
						Message:    "Directory is not writable",
						Suggestion: fmt.Sprintf("Fix permissions: chmod u+w %s", cfg.General.DownloadRoot),
					}
				}
				os.Remove(testFile)

				return CheckResult{
					Passed:  true,
					Message: fmt.Sprintf("Writable: %s", cfg.General.DownloadRoot),
				}
			},
		},
		{
			Name:        "Disk space available",
			Critical:    false,
			Description: "Sufficient disk space for downloads",
			Run: func(ctx context.Context) CheckResult {
				if cfg == nil {
					return CheckResult{Passed: false, Message: "Config not loaded"}
				}

				available, err := system.CheckAvailableSpace(cfg.General.DownloadRoot)
				if err != nil {
					return CheckResult{
						Warning: true,
						Passed:  true,
						Message: fmt.Sprintf("Could not check disk space: %v", err),
					}
				}

				availableGB := float64(available) / (1024 * 1024 * 1024)

				if availableGB < 1 {
					return CheckResult{
						Passed:     false,
						Message:    fmt.Sprintf("Very low disk space: %s", humanize.Bytes(available)),
						Suggestion: "Free up disk space before downloading large models",
					}
				}

				if availableGB < 10 {
					return CheckResult{
						Passed:  true,
						Warning: true,
						Message: fmt.Sprintf("Low disk space: %s free", humanize.Bytes(available)),
						Suggestion: "Consider freeing up more space for large model downloads",
					}
				}

				return CheckResult{
					Passed:  true,
					Message: fmt.Sprintf("%s available", humanize.Bytes(available)),
				}
			},
		},
		{
			Name:        "Database accessible",
			Critical:    true,
			Description: "State database must be accessible",
			Run: func(ctx context.Context) CheckResult {
				if cfg == nil {
					return CheckResult{Passed: false, Message: "Config not loaded"}
				}

				db, err := state.Open(cfg)
				if err != nil {
					return CheckResult{
						Passed:     false,
						Message:    fmt.Sprintf("Cannot open database: %v", err),
						Suggestion: "Check that data_root is writable and database is not corrupted",
					}
				}
				defer db.Close()

				return CheckResult{
					Passed:  true,
					Message: fmt.Sprintf("Database OK: %s", db.Path),
				}
			},
		},
		{
			Name:        "HF_TOKEN environment variable",
			Critical:    false,
			Description: "HuggingFace token for gated models",
			Run: func(ctx context.Context) CheckResult {
				token := os.Getenv("HF_TOKEN")
				if token == "" {
					return CheckResult{
						Passed:  true,
						Warning: true,
						Message: "Not set",
						Suggestion: "Set HF_TOKEN to download gated HuggingFace models:\n  export HF_TOKEN=hf_...\n  Get token at: https://huggingface.co/settings/tokens",
					}
				}

				if !strings.HasPrefix(token, "hf_") {
					return CheckResult{
						Passed:  true,
						Warning: true,
						Message: "Set but may be invalid (doesn't start with 'hf_')",
						Suggestion: "Verify your token at: https://huggingface.co/settings/tokens",
					}
				}

				return CheckResult{
					Passed:  true,
					Message: "Set (hf_...)",
				}
			},
		},
		{
			Name:        "CIVITAI_TOKEN environment variable",
			Critical:    false,
			Description: "CivitAI token for downloads",
			Run: func(ctx context.Context) CheckResult {
				token := os.Getenv("CIVITAI_TOKEN")
				if token == "" {
					return CheckResult{
						Passed:  true,
						Warning: true,
						Message: "Not set",
						Suggestion: "Set CIVITAI_TOKEN to download from CivitAI:\n  export CIVITAI_TOKEN=...\n  Get token at: https://civitai.com/user/account",
					}
				}

				return CheckResult{
					Passed:  true,
					Message: "Set",
				}
			},
		},
		{
			Name:        "Internet connectivity",
			Critical:    true,
			Description: "Network access for downloads",
			Run: func(ctx context.Context) CheckResult {
				err := system.CheckConnectivity(ctx)
				if err != nil {
					return CheckResult{
						Passed:     false,
						Message:    "Network check failed",
						Suggestion: err.Error(),
					}
				}

				return CheckResult{
					Passed:  true,
					Message: "Internet accessible",
				}
			},
		},
		{
			Name:        "Orphaned .part files",
			Critical:    false,
			Description: "Incomplete downloads from previous sessions",
			Run: func(ctx context.Context) CheckResult {
				if cfg == nil {
					return CheckResult{Passed: false, Message: "Config not loaded"}
				}

				// Count .part files in download root
				partCount := 0
				filepath.Walk(cfg.General.DownloadRoot, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if !info.IsDir() && strings.HasSuffix(info.Name(), ".part") {
						partCount++
					}
					return nil
				})

				if partCount == 0 {
					return CheckResult{
						Passed:  true,
						Message: "No orphaned .part files",
					}
				}

				return CheckResult{
					Passed:  true,
					Warning: true,
					Message: fmt.Sprintf("Found %d .part file(s)", partCount),
					Suggestion: "Clean up old partial downloads:\n  modfetch clean --days=7",
				}
			},
		},
	}

	// Run all checks
	results := make([]CheckResult, len(checks))
	passedCount := 0
	failedCount := 0
	warningCount := 0

	for i, check := range checks {
		if *verbose {
			fmt.Printf("[ ] %s...\n", check.Name)
		}

		start := time.Now()
		result := check.Run(ctx)
		duration := time.Since(start)

		results[i] = result

		// Determine symbol
		symbol := "✓"
		if !result.Passed {
			symbol = "✗"
			failedCount++
		} else if result.Warning {
			symbol = "⚠"
			warningCount++
			passedCount++
		} else {
			passedCount++
		}

		// Print result
		fmt.Printf("%s %s", symbol, check.Name)
		if *verbose {
			fmt.Printf(" (%.2fs)", duration.Seconds())
		}
		fmt.Println()

		if result.Message != "" {
			fmt.Printf("  %s\n", result.Message)
		}

		if result.Suggestion != "" {
			lines := strings.Split(result.Suggestion, "\n")
			for _, line := range lines {
				fmt.Printf("  → %s\n", line)
			}
		}

		if *verbose || !result.Passed || result.Warning {
			fmt.Println()
		}
	}

	// Summary
	totalChecks := len(checks)
	fmt.Printf("\nDiagnostic Summary:\n")
	fmt.Printf("  Total checks: %d\n", totalChecks)
	fmt.Printf("  Passed:       %d\n", passedCount)
	fmt.Printf("  Warnings:     %d\n", warningCount)
	fmt.Printf("  Failed:       %d\n", failedCount)

	if failedCount > 0 {
		fmt.Println("\n⚠ Some critical checks failed. modfetch may not work correctly.")
		fmt.Println("Please fix the issues above before using modfetch.")
		return fmt.Errorf("%d checks failed", failedCount)
	}

	if warningCount > 0 {
		fmt.Println("\n⚠ Some checks have warnings. modfetch will work but some features may be limited.")
	} else {
		fmt.Println("\n✓ All checks passed! modfetch is ready to use.")
	}

	return nil
}
