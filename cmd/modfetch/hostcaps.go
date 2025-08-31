package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"modfetch/internal/config"
	"modfetch/internal/resolver"
	"modfetch/internal/state"
)

func handleHostCaps(args []string) error {
	if len(args) > 0 && args[0] == "clear" {
		return handleHostCapsClear(args[1:])
	}
	return handleHostCapsList(args)
}

func handleHostCapsList(args []string) error {
	fs := flag.NewFlagSet("hostcaps", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	jsonOut := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		}
	}
	if *cfgPath == "" {
		return errors.New("--config is required or set MODFETCH_CONFIG")
	}
	if _, err := os.Stat(*cfgPath); err != nil {
		return fmt.Errorf("config file not found: %s", *cfgPath)
	}
	c, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer st.SQL.Close()
	hc, err := st.ListHostCaps()
	if err != nil {
		return err
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(hc)
	}
	for _, h := range hc {
		fmt.Printf("%s\thead_ok=%v\taccept_ranges=%v\n", h.Host, h.HeadOK, h.AcceptRanges)
	}
	return nil
}

func handleHostCapsClear(args []string) error {
	fs := flag.NewFlagSet("hostcaps clear", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	resolverCache := fs.Bool("resolver-cache", false, "Clear resolver cache")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		}
	}
	if *cfgPath == "" {
		return errors.New("--config is required or set MODFETCH_CONFIG")
	}
	if _, err := os.Stat(*cfgPath); err != nil {
		return fmt.Errorf("config file not found: %s", *cfgPath)
	}
	c, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer st.SQL.Close()
	if err := st.ClearHostCaps(); err != nil {
		return err
	}
	fmt.Println("hostcaps: cleared all")
	if *resolverCache {
		if err := resolver.ClearCache(c); err != nil {
			return err
		}
		fmt.Println("resolver cache: cleared")
	}
	return nil
}
