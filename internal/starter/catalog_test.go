package starter

import "testing"

func TestStarterCatalogLookup(t *testing.T) {
	entries := List()
	if len(entries) < 2 {
		t.Fatalf("expected starter entries, got %d", len(entries))
	}
	entry, ok := Get("starter://gpt2-config")
	if !ok {
		t.Fatal("expected gpt2-config starter")
	}
	if entry.URI != "hf://gpt2/config.json?rev=main" {
		t.Fatalf("unexpected URI: %s", entry.URI)
	}
	if _, err := MustURI("missing"); err == nil {
		t.Fatal("expected missing starter to fail")
	}
}
