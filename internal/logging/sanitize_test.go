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

func TestSanitizeText(t *testing.T) {
	in := "GET https://user:pass@example.com/file.bin?token=secret failed with Authorization: Bearer abc.def-123)"
	want := "GET https://example.com/file.bin failed with Authorization: Bearer [REDACTED])"
	if got := SanitizeText(in); got != want {
		t.Fatalf("SanitizeText()=%q want %q", got, want)
	}
}

func TestSanitizeError(t *testing.T) {
	if SanitizeError(nil) != nil {
		t.Fatal("nil error should stay nil")
	}
	err := SanitizeError(testErr("download https://example.com/a?token=secret"))
	if err == nil || err.Error() != "download https://example.com/a" {
		t.Fatalf("unexpected sanitized error: %v", err)
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }
