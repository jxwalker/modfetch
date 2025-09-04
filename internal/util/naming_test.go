package util

import "testing"

func TestExpandPattern(t *testing.T) {
	pat := "{model_name} - {file_name} ({version_id})"
	toks := map[string]string{
		"model_name": "FooModel",
		"file_name":  "bar.safetensors",
		"version_id": "123",
	}
	got := ExpandPattern(pat, toks)
	want := "FooModel - bar.safetensors (123)"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
	// Unknown tokens left as-is
	got2 := ExpandPattern("{unknown}-{file_name}", toks)
	if got2 != "{unknown}-bar.safetensors" {
		t.Fatalf("unexpected: %q", got2)
	}
	// Empty pattern returns empty
	if ExpandPattern("", toks) != "" {
		t.Fatalf("expected empty for empty pattern")
	}
}
