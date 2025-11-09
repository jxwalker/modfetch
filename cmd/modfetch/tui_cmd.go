package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/tui"
	cw "github.com/jxwalker/modfetch/internal/tui/configwizard"
	"gopkg.in/yaml.v3"
)

func handleTUI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs (not used in TUI)")
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
	defer func() { _ = st.SQL.Close() }()

	m := tui.New(c, st, version)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}
