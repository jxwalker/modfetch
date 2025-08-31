package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
	ui "modfetch/internal/tui"
	cw "modfetch/internal/tui/configwizard"
	uiv2 "modfetch/internal/tui/v2"
)

func handleTUI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs (not used in TUI)")
	// --v2 kept for compatibility but unused since v2 is default
	_ = fs.Bool("v2", false, "Use TUI v2 (default)")
	useV1 := fs.Bool("v1", false, "Use legacy TUI v1 (fallback)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		}
	}
	// If still empty, default to ~/.config/modfetch/config.yml
	if *cfgPath == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return errors.New("--config is required or set MODFETCH_CONFIG")
		}
		*cfgPath = filepath.Join(h, ".config", "modfetch", "config.yml")
	}
	// Try to load config. If not found, offer to create via wizard with sensible defaults.
	c, err := config.Load(*cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create directory and run wizard
			_ = os.MkdirAll(filepath.Dir(*cfgPath), 0o755)
			defaults := &config.Config{Version: 1, General: config.General{DataRoot: "~/modfetch/data", DownloadRoot: "~/modfetch/downloads", PlacementMode: "symlink"}, Concurrency: config.Concurrency{ChunkSizeMB: 8, PerFileChunks: 4}}
			wiz := cw.New(defaults)
			p := tea.NewProgram(wiz)
			m, werr := p.Run()
			if werr != nil {
				return werr
			}
			w, ok := m.(*cw.Wizard)
			if !ok {
				return errors.New("unexpected wizard model")
			}
			cfg := w.Config()
			if cfg == nil {
				return errors.New("config wizard was cancelled")
			}
			b, merr := yaml.Marshal(cfg)
			if merr != nil {
				return merr
			}
			if err := os.WriteFile(*cfgPath, b, 0o644); err != nil {
				return err
			}
			fmt.Printf("wrote config to %s\n", *cfgPath)
			// Reload expanded config
			c, err = config.Load(*cfgPath)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	_ = logging.New(*logLevel, *jsonOut) // placeholder for future log routing
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer st.SQL.Close()
	var m tea.Model
	// Default to v2 unless legacy v1 explicitly requested
	if *useV1 {
		m = ui.New(c, st)
	} else {
		m = uiv2.New(c, st, version)
	}
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}
