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
	common := addCommonConfigLogFlags(fs, "json logs (not used in TUI)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedCfgPath, err := resolveConfigPath(*common.configPath)
	if err != nil {
		return err
	}
	// Try to load config. If not found, offer to create via wizard with sensible defaults.
	c, err := config.Load(resolvedCfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create directory and run wizard
			if err := os.MkdirAll(filepath.Dir(resolvedCfgPath), 0o755); err != nil {
				return err
			}
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
			if err := os.WriteFile(resolvedCfgPath, b, 0o644); err != nil {
				return err
			}
			fmt.Printf("wrote config to %s\n", resolvedCfgPath)
			// Reload expanded config
			c, err = config.Load(resolvedCfgPath)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	_ = logging.New(*common.logLevel, *common.jsonOut) // placeholder for future log routing
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
