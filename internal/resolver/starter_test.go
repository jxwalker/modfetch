package resolver

import (
	"context"
	"strings"
	"testing"
)

func TestStarterResolver(t *testing.T) {
	res, err := Resolve(context.Background(), "starter://gpt2-config", nil)
	if err != nil {
		t.Fatalf("resolve starter: %v", err)
	}
	if !strings.Contains(res.URL, "huggingface.co/gpt2/resolve/607a30d783dfa663caf39e06633721c8d4cfcd7e/config.json") {
		t.Fatalf("unexpected starter URL: %s", res.URL)
	}
}

func TestStarterResolverDirectHTTP(t *testing.T) {
	res, err := Resolve(context.Background(), "starter://public-1mb", nil)
	if err != nil {
		t.Fatalf("resolve direct starter: %v", err)
	}
	if res.URL != "https://proof.ovh.net/files/1Mb.dat" {
		t.Fatalf("unexpected direct starter URL: %s", res.URL)
	}
}
