package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"modfetch/internal/config"
	"modfetch/internal/downloader"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

const version = "0.1.0-M0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command provided")
	}

	// Global flags (parsed per subcommand to avoid hard defaults)
	cmd := args[0]
	switch cmd {
	case "config":
		return handleConfig(args[1:])
	case "status":
		return handleStatus(args[1:])
	case "download":
		return handleDownload(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func usage() {
	fmt.Println(strings.TrimSpace(`modfetch - robust model fetcher (skeleton)

Usage:
  modfetch <command> [flags]

Commands:
  config validate   Validate a YAML config file
  config print      Print the loaded config as JSON
  download          Download a file via direct URL (M1 minimal)
  status            Print a simple status (skeleton)
  version           Print version
  help              Show this help

Flags:
  --config PATH     Path to YAML config file (or MODFETCH_CONFIG env var)
  --log-level L     Log level: debug|info|warn|error (per command)
  --json            JSON log output (per command)
`))
}

func handleStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	_ = fs.Parse(args)
	log := logging.New(*logLevel, *jsonOut)
	log.Infof("status: ok (skeleton)")
	return nil
}

func handleConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand required: validate | print")
	}
	sub := args[0]
	switch sub {
	case "validate":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			log.Infof("config: valid")
			return nil
		})
	case "print":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(c)
		})
	default:
		return fmt.Errorf("unknown config subcommand: %s", sub)
	}
}

func handleDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	url := fs.String("url", "", "HTTP URL to download (direct)")
	dest := fs.String("dest", "", "destination path (optional)")
	sha := fs.String("sha256", "", "expected SHA256 (optional)")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env }
	}
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	dl := downloader.NewSingle(c, log, st)
	ctx := context.Background()
	final, sum, err := dl.Download(ctx, *url, *dest, *sha)
	if err != nil { return err }
	log.Infof("downloaded: %s (sha256=%s)", final, sum)
	return nil
}

func configOp(args []string, fn func(*config.Config, *logging.Logger) error) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
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
	c, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	log := logging.New(*logLevel, *jsonOut)
	return fn(c, log)
}

