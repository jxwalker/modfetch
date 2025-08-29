package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"modfetch/internal/config"
	"modfetch/internal/state"
)

func handleHostCaps(args []string) error {
	fs := flag.NewFlagSet("hostcaps", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	list := fs.Bool("list", false, "List cached host capabilities")
	clear := fs.String("clear", "", "Clear cache for a specific host")
	clearAll := fs.Bool("clear-all", false, "Clear cache for all hosts")
	jsonOut := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if _, err := os.Stat(*cfgPath); err != nil { return fmt.Errorf("config file not found: %s", *cfgPath) }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	if *clearAll {
		if err := st.ClearHostCaps(); err != nil { return err }
		fmt.Println("hostcaps: cleared all")
		return nil
	}
	if *clear != "" {
		h := strings.ToLower(strings.TrimSpace(*clear))
		if err := st.DeleteHostCaps(h); err != nil { return err }
		fmt.Printf("hostcaps: cleared %s\n", h)
		return nil
	}
	if *list || fs.NArg() == 0 {
		hc, err := st.ListHostCaps()
		if err != nil { return err }
		if *jsonOut {
			enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
			return enc.Encode(hc)
		}
		for _, h := range hc {
			fmt.Printf("%s\thead_ok=%v\taccept_ranges=%v\n", h.Host, h.HeadOK, h.AcceptRanges)
		}
		return nil
	}
	return errors.New("no action provided; use --list, --clear HOST, or --clear-all")
}
