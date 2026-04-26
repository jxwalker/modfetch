package resolver

import (
	"context"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

type fakeResolver struct{}

func (fakeResolver) CanHandle(uri string) bool {
	return uri == "fake://model"
}

func (fakeResolver) Resolve(context.Context, string, *config.Config) (*Resolved, error) {
	return &Resolved{URL: "https://example.com/model.bin"}, nil
}

type nonComparableResolver struct {
	seen map[string]bool
}

func (r nonComparableResolver) CanHandle(uri string) bool {
	return r.seen[uri]
}

func (nonComparableResolver) Resolve(context.Context, string, *config.Config) (*Resolved, error) {
	return &Resolved{URL: "https://example.com/non-comparable.bin"}, nil
}

func TestRegisterResolver(t *testing.T) {
	unregister := Register(fakeResolver{})
	defer unregister()

	res, err := Resolve(context.Background(), "fake://model", nil)
	if err != nil {
		t.Fatalf("resolve fake uri: %v", err)
	}
	if res.URL != "https://example.com/model.bin" {
		t.Fatalf("unexpected resolved URL: %s", res.URL)
	}
}

func TestUnregisterResolver(t *testing.T) {
	unregister := Register(fakeResolver{})
	unregister()

	if _, err := Resolve(context.Background(), "fake://model", nil); err == nil {
		t.Fatal("expected fake resolver to be unavailable after unregister")
	}
}

func TestUnregisterNonComparableResolver(t *testing.T) {
	unregister := Register(nonComparableResolver{seen: map[string]bool{"map://model": true}})

	res, err := Resolve(context.Background(), "map://model", nil)
	if err != nil {
		t.Fatalf("resolve non-comparable resolver: %v", err)
	}
	if res.URL != "https://example.com/non-comparable.bin" {
		t.Fatalf("unexpected resolved URL: %s", res.URL)
	}

	unregister()
	if _, err := Resolve(context.Background(), "map://model", nil); err == nil {
		t.Fatal("expected non-comparable resolver to be unavailable after unregister")
	}
}
