package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	cw "github.com/jxwalker/modfetch/internal/tui/configwizard"
	"gopkg.in/yaml.v3"
)

func handleConfigWizard(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("config wizard", flag.ContinueOnError)
	out := fs.String("out", "", "write YAML to this path instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Use simple defaults
	defaults := &config.Config{
		Version: 1,
		General: config.General{
			DataRoot:      "~/modfetch/data",
			DownloadRoot:  "~/modfetch/downloads",
			PlacementMode: "symlink",
		},
		Concurrency: config.Concurrency{ChunkSizeMB: 8, PerFileChunks: 4},
	}
	w := cw.New(defaults)
	p := tea.NewProgram(w)
	m, err := p.Run()
	if err != nil {
		return err
	}
	wiz, ok := m.(*cw.Wizard)
	if !ok {
		return errors.New("unexpected model type from wizard")
	}
	cfg := wiz.Config()
	if cfg == nil {
		return errors.New("no config produced")
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if *out == "" {
		fmt.Print(string(b))
		return nil
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote config to %s\n", *out)
	return nil
}
