package main

import (
	"errors"
	"flag"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
	ui "modfetch/internal/tui"
)

func handleTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs (not used in TUI)")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	_ = logging.New(*logLevel, *jsonOut) // placeholder for future log routing
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	m := ui.New(c, st)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

