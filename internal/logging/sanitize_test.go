package logging

import "testing"

func TestSanitizeURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://user:pass@example.com/path?token=secret&x=1#frag", "https://example.com/path"},
		{"hf://owner/repo/file.txt?rev=main", "hf://owner/repo/file.txt"},
		{"not a url", "not a url"},
	}
	for _, c := range cases {
		got := SanitizeURL(c.in)
		if got != c.want {
			t.Errorf("SanitizeURL(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
