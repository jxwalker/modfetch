package downloader

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"modfetch/internal/config"
)

// stagePartPath returns a staging .part path under download_root/.parts or partials_root if set, unique to (url,dest).
func stagePartPath(cfg *config.Config, url, dest string) string {
	if cfg != nil && !cfg.General.StagePartials {
		return dest + ".part"
	}
	// Prefer explicit partials_root if configured; otherwise fallback to download_root/.parts
	partsDir := cfg.General.PartialsRoot
	if partsDir == "" {
		partsDir = filepath.Join(cfg.General.DownloadRoot, ".parts")
	}
	_ = os.MkdirAll(partsDir, 0o755)
	keySrc := url + "|" + dest
	h := sha1.Sum([]byte(keySrc))
	key := hex.EncodeToString(h[:])[:12]
	name := filepath.Base(dest)
	file := fmt.Sprintf("%s.%s.part", name, key)
	return filepath.Join(partsDir, file)
}

// StagePartPath is an exported helper for UI components to locate the .part file
// for an in-progress download, consistent with downloader behavior.
func StagePartPath(cfg *config.Config, url, dest string) string {
	return stagePartPath(cfg, url, dest)
}

// renameOrCopy attempts to rename, falling back to copy when cross-device.
func renameOrCopy(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err == syscall.EXDEV {
			if err2 := copyFile(src, dst); err2 != nil { return err2 }
			_ = os.Remove(src)
			return nil
		}
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil { return err }
	defer func() { _ = sf.Close() }()
	df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil { return err }
	defer func() { _ = df.Close() }()
	if _, err := io.Copy(df, sf); err != nil { return err }
	if err := df.Sync(); err != nil { return err }
	return nil
}
