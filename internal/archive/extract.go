package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	sevenZipCommands = []string{"7zz", "7z", "7za"}
	lookPath         = exec.LookPath
	commandContext   = exec.CommandContext
)

func Extract(ctx context.Context, src, destDir string) ([]string, error) {
	if strings.TrimSpace(src) == "" {
		return nil, errors.New("archive source is required")
	}
	if strings.TrimSpace(destDir) == "" {
		return nil, errors.New("archive destination directory is required")
	}
	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(ctx, src, destDir)
	case strings.HasSuffix(lower, ".tar"):
		f, err := os.Open(src)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		return extractTar(ctx, f, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		f, err := os.Open(src)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		return extractTar(ctx, gz, destDir)
	case strings.HasSuffix(lower, ".7z"):
		return extract7z(ctx, src, destDir)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", filepath.Ext(src))
	}
}

func extract7z(ctx context.Context, src, destDir string) ([]string, error) {
	bin, err := find7z()
	if err != nil {
		return nil, err
	}
	cleanDest, err := filepath.Abs(destDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cleanDest, 0o755); err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp(filepath.Dir(cleanDest), ".modfetch-7z-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	var stderr limitedBuffer
	stderr.max = 8 * 1024
	cmd := commandContext(ctx, bin, "x", "-y", "-o"+tmp, src)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, msg)
	}
	return moveExtractedTree(ctx, tmp, cleanDest)
}

func find7z() (string, error) {
	for _, name := range sevenZipCommands {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("7z extraction requires 7zz, 7z, or 7za on PATH")
}

func extractZip(ctx context.Context, src, destDir string) ([]string, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()

	var out []string
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		target, err := safeArchivePath(destDir, f.Name)
		if err != nil {
			return out, err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return out, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return out, err
		}
		rc, err := f.Open()
		if err != nil {
			return out, err
		}
		if err := writeFile(target, rc, f.FileInfo().Mode()); err != nil {
			_ = rc.Close()
			return out, err
		}
		if err := rc.Close(); err != nil {
			return out, err
		}
		out = append(out, target)
	}
	return out, nil
}

func extractTar(ctx context.Context, r io.Reader, destDir string) ([]string, error) {
	tr := tar.NewReader(r)
	var out []string
	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		target, err := safeArchivePath(destDir, hdr.Name)
		if err != nil {
			return out, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return out, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return out, err
			}
			if err := writeFile(target, tr, os.FileMode(hdr.Mode)); err != nil {
				return out, err
			}
			out = append(out, target)
		default:
			continue
		}
	}
}

func safeArchivePath(destDir, name string) (string, error) {
	cleanDest, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(cleanDest, filepath.Clean(name)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(cleanDest, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return target, nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o644
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func moveExtractedTree(ctx context.Context, srcDir, destDir string) ([]string, error) {
	var out []string
	cleanDest, err := filepath.Abs(destDir)
	if err != nil {
		return nil, err
	}
	cleanSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, err
	}
	if err := filepath.WalkDir(cleanSrc, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(cleanSrc, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target, err := safeArchivePath(cleanDest, rel)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive entry is a symlink: %s", rel)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive entry is not a regular file: %s", rel)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := writeFile(target, f, info.Mode()); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		out = append(out, target)
		return nil
	}); err != nil {
		return out, err
	}
	return out, nil
}

type limitedBuffer struct {
	max int
	buf []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.max <= 0 {
		return len(p), nil
	}
	if len(b.buf) < b.max {
		n := b.max - len(b.buf)
		if n > len(p) {
			n = len(p)
		}
		b.buf = append(b.buf, p[:n]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return string(b.buf)
}
