package util

import "testing"

func TestURLPathBase(t *testing.T) {
	cases := map[string]string{
		"https://civitai.com/api/download/models/2058285?type=Archive&format=Other": "2058285",
		"https://example.com/path/to/file.bin?foo=bar": "file.bin",
		"https://example.com/": "download",
		"/just/a/path/name.txt": "name.txt",
	}
	for in, want := range cases {
		got := URLPathBase(in)
		if got != want {
			t.Fatalf("URLPathBase(%q)=%q want %q", in, got, want)
		}
	}
}
