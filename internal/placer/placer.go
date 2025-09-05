package placer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jxwalker/modfetch/internal/classifier"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/util"
)

// ComputeTargets returns absolute destination directories for an artifact type
// according to the placement.mapping rules in config.
func ComputeTargets(cfg *config.Config, artifactType string) ([]string, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	var targets []string
	for _, rule := range cfg.Placement.Mapping {
		if rule.Match == artifactType {
			for _, t := range rule.Targets {
				app, ok := cfg.Placement.Apps[t.App]
				if !ok {
					return nil, fmt.Errorf("mapping references unknown app: %s", t.App)
				}
				rel, ok := app.Paths[t.PathKey]
				if !ok {
					return nil, fmt.Errorf("app %s missing path key: %s", t.App, t.PathKey)
				}
				dir := filepath.Join(app.Base, rel)
				targets = append(targets, dir)
			}
		}
	}
	return targets, nil
}

// Place copies/symlinks/hardlinks a file into the mapped directories.
// mode: "symlink" | "hardlink" | "copy"
// If artifactType is empty, it will be detected.
func Place(cfg *config.Config, srcPath, artifactType, mode string) ([]string, error) {
	if mode == "" {
		mode = cfg.General.PlacementMode
	}
	if artifactType == "" {
		artifactType = classifier.Detect(cfg, srcPath)
	}
	dests, err := ComputeTargets(cfg, artifactType)
	if err != nil {
		return nil, err
	}
	if len(dests) == 0 {
		return nil, fmt.Errorf("no mapping targets for type %s", artifactType)
	}

	sum, err := sha256File(srcPath)
	if err != nil {
		return nil, err
	}

	var placed []string
	for _, dir := range dests {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return placed, err
		}
		dst := filepath.Join(dir, filepath.Base(srcPath))
		if exists(dst) {
			// verify checksum if not overwriting
			if !cfg.General.AllowOverwrite {
				if same, _ := sameSHA256(dst, sum); same {
					// Already present with same content; skip
					placed = append(placed, dst)
					continue
				}
				return placed, fmt.Errorf("destination exists and differs: %s", dst)
			}
			// We will remove and replace according to mode
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return placed, err
			}
		}
		switch strings.ToLower(mode) {
		case "symlink":
			// Use relative symlink when possible
			rel, _ := filepath.Rel(dir, srcPath)
			if rel == "" || strings.HasPrefix(rel, "..") {
				rel = srcPath
			}
			if err := os.Symlink(rel, dst); err != nil {
				return placed, err
			}
		case "hardlink":
			if err := os.Link(srcPath, dst); err != nil {
				return placed, err
			}
		case "copy":
			if err := copyFile(srcPath, dst); err != nil {
				return placed, err
			}
		default:
			return placed, fmt.Errorf("unknown placement mode: %s", mode)
		}
		placed = append(placed, dst)
	}
	return placed, nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func sha256File(p string) (string, error) {
	return util.HashFileSHA256(p)
}

func sameSHA256(p string, want string) (bool, error) {
	s, err := sha256File(p)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(want)), nil
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sf.Close() }()
	df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = df.Close() }()
	_, err = io.Copy(df, sf)
	return err
}
