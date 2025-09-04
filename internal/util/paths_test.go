package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeFileName(t *testing.T) {
	cases := map[string]string{
		"foo/bar":                           "foo-bar",
		"foo\\bar":                          "foo-bar",
		"  spaced name  ":                   "spaced-name",
		"2058285?type=Archive&format=Other": "2058285-type-Archive-format-Other",
		"":                                  "download",
	}
	for in, want := range cases {
		got := SafeFileName(in)
		if got != want {
			t.Fatalf("SafeFileName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestUniquePath_VersionHintSanitized(t *testing.T) {
	d := t.TempDir()
	base := "file.bin"
	// occupy base so hint path is tried
	p1, _ := UniquePath(d, base, "")
	_ = os.WriteFile(p1, []byte("x"), 0o644)

	p2, err := UniquePath(d, base, "../v12/../../evil")
	if err != nil {
		t.Fatal(err)
	}
	b := filepath.Base(p2)
	if strings.Contains(b, "/") || strings.Contains(b, "\\") {
		t.Fatalf("versionHint not sanitized in filename: %q", b)
	}
	if !strings.Contains(b, "v12") && !strings.Contains(b, "evil") {
		t.Fatalf("expected sanitized hint to be present in name: %q", b)
	}
}

func TestUniquePath(t *testing.T) {
	d := t.TempDir()
	base := "ModelX - file.bin"
	p1, err := UniquePath(d, base, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p1) != "ModelX---file.bin" {
		t.Fatalf("got %s want %s", filepath.Base(p1), "ModelX---file.bin")
	}
	// create p1
	if err := os.WriteFile(p1, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Next should try version hint
	p2, err := UniquePath(d, base, "12")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p2) != "ModelX---file (v12).bin" {
		t.Fatalf("unexpected p2: %s", filepath.Base(p2))
	}
	// create p2
	if err := os.WriteFile(p2, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Next should try numeric suffix
	p3, err := UniquePath(d, base, "12")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p3) != "ModelX---file (2).bin" {
		t.Fatalf("unexpected p3: %s", filepath.Base(p3))
	}
}
