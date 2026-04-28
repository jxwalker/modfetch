package resolver

import (
	"context"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

type registeredTestResolver struct{}

func (registeredTestResolver) CanHandle(uri string) bool {
	return uri == "test://model"
}

func (registeredTestResolver) Resolve(context.Context, string, *config.Config) (*Resolved, error) {
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
	unregister := Register(registeredTestResolver{})
	defer unregister()

	res, err := Resolve(context.Background(), "test://model", nil)
	if err != nil {
		t.Fatalf("resolve test uri: %v", err)
	}
	if res.URL != "https://example.com/model.bin" {
		t.Fatalf("unexpected resolved URL: %s", res.URL)
	}
}

func TestUnregisterResolver(t *testing.T) {
	unregister := Register(registeredTestResolver{})
	unregister()

	if _, err := Resolve(context.Background(), "test://model", nil); err == nil {
		t.Fatal("expected test resolver to be unavailable after unregister")
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
