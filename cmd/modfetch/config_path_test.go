package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPathPrecedence(t *testing.T) {
	home := t.TempDir()
	envPath := filepath.Join(t.TempDir(), "env.yml")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MODFETCH_CONFIG", envPath)

	if got, err := resolveConfigPath(" /tmp/explicit.yml "); err != nil || got != "/tmp/explicit.yml" {
		t.Fatalf("explicit path = %q, %v", got, err)
	}
	if got, err := resolveConfigPath(""); err != nil || got != envPath {
		t.Fatalf("env path = %q, %v", got, err)
	}
	t.Setenv("MODFETCH_CONFIG", "")
	want := filepath.Join(home, ".config", "modfetch", "config.yml")
	if got, err := resolveConfigPath(""); err != nil || got != want {
		t.Fatalf("default path = %q, %v; want %q", got, err, want)
	}
}

func TestResolveConfigPathExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	wantExplicit := filepath.Join(home, "explicit.yml")
	if got, err := resolveConfigPath("~/explicit.yml"); err != nil || got != wantExplicit {
		t.Fatalf("explicit tilde path = %q, %v; want %q", got, err, wantExplicit)
	}

	wantEnv := filepath.Join(home, "env.yml")
	t.Setenv("MODFETCH_CONFIG", "~/env.yml")
	if got, err := resolveConfigPath(""); err != nil || got != wantEnv {
		t.Fatalf("env tilde path = %q, %v; want %q", got, err, wantEnv)
	}
}

func TestLoadConfigUsesDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MODFETCH_CONFIG", "")
	cfgPath := filepath.Join(home, ".config", "modfetch", "config.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	dataRoot := filepath.Join(home, "data")
	downloadRoot := filepath.Join(home, "downloads")
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + fmt.Sprintf("%q", dataRoot) + "\n" +
		"  download_root: " + fmt.Sprintf("%q", downloadRoot) + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	c, gotPath, err := loadConfig("")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if gotPath != cfgPath {
		t.Fatalf("loaded path = %q; want %q", gotPath, cfgPath)
	}
	if c.General.DataRoot != dataRoot || c.General.DownloadRoot != downloadRoot {
		t.Fatalf("unexpected config roots: %+v", c.General)
	}
}

func TestLoadConfigAllowsTildePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MODFETCH_CONFIG", "")
	cfgPath := filepath.Join(home, "modfetch.yml")
	dataRoot := filepath.Join(home, "data")
	downloadRoot := filepath.Join(home, "downloads")
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + fmt.Sprintf("%q", dataRoot) + "\n" +
		"  download_root: " + fmt.Sprintf("%q", downloadRoot) + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	c, gotPath, err := loadConfig("~/modfetch.yml")
	if err != nil {
		t.Fatalf("load tilde config: %v", err)
	}
	if gotPath != cfgPath {
		t.Fatalf("loaded path = %q; want %q", gotPath, cfgPath)
	}
	if c.General.DataRoot != dataRoot || c.General.DownloadRoot != downloadRoot {
		t.Fatalf("unexpected config roots: %+v", c.General)
	}
}

func TestLoadConfigAllowsTildeEnvPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfgPath := filepath.Join(home, "env.yml")
	t.Setenv("MODFETCH_CONFIG", "~/env.yml")
	dataRoot := filepath.Join(home, "data")
	downloadRoot := filepath.Join(home, "downloads")
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: " + fmt.Sprintf("%q", dataRoot) + "\n" +
		"  download_root: " + fmt.Sprintf("%q", downloadRoot) + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	c, gotPath, err := loadConfig("")
	if err != nil {
		t.Fatalf("load tilde env config: %v", err)
	}
	if gotPath != cfgPath {
		t.Fatalf("loaded path = %q; want %q", gotPath, cfgPath)
	}
	if c.General.DataRoot != dataRoot || c.General.DownloadRoot != downloadRoot {
		t.Fatalf("unexpected config roots: %+v", c.General)
	}
}
