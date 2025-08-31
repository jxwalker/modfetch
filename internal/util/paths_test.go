package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeFileName(t *testing.T) {
	cases := map[string]string{
		"foo/bar":                           "foo-bar",
		"foo\\bar":                          "foo-bar",
		"  spaced name  ":                  "spaced-name",
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

func TestUniquePath(t *testing.T) {
	d := t.TempDir()
	base := "ModelX - file.bin"
	p1, err := UniquePath(d, base, "")
	if err != nil { t.Fatal(err) }
if filepath.Base(p1) != "ModelX---file.bin" { t.Fatalf("got %s want %s", filepath.Base(p1), "ModelX---file.bin") }
	// create p1
	if err := os.WriteFile(p1, []byte("x"), 0o644); err != nil { t.Fatal(err) }
	// Next should try version hint
	p2, err := UniquePath(d, base, "12")
	if err != nil { t.Fatal(err) }
if filepath.Base(p2) != "ModelX---file (v12).bin" {
		t.Fatalf("unexpected p2: %s", filepath.Base(p2))
	}
	// create p2
	if err := os.WriteFile(p2, []byte("x"), 0o644); err != nil { t.Fatal(err) }
	// Next should try numeric suffix
	p3, err := UniquePath(d, base, "12")
	if err != nil { t.Fatal(err) }
if filepath.Base(p3) != "ModelX---file (2).bin" {
		t.Fatalf("unexpected p3: %s", filepath.Base(p3))
	}
}

