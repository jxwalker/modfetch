package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
)

func resolveConfigPath(flagPath string) (string, error) {
	if p := strings.TrimSpace(flagPath); p != "" {
		return expandHomeConfigPath(p)
	}
	if env := strings.TrimSpace(os.Getenv("MODFETCH_CONFIG")); env != "" {
		return expandHomeConfigPath(env)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("--config is required or set MODFETCH_CONFIG")
	}
	return filepath.Join(home, ".config", "modfetch", "config.yml"), nil
}

func expandHomeConfigPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return "", errors.New("--config is required or set MODFETCH_CONFIG")
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func loadConfig(flagPath string) (*config.Config, string, error) {
	cfgPath, err := resolveConfigPath(flagPath)
	if err != nil {
		return nil, "", err
	}
	c, err := config.Load(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, cfgPath, fmt.Errorf("config file not found: %s", cfgPath)
		}
		return nil, cfgPath, err
	}
	return c, cfgPath, nil
}
