package main

import (
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/discovery"
)

func TestPrintDiscoveryResultsShowsDownloadCommand(t *testing.T) {
	out := captureStdout(t, func() {
		err := printDiscoveryResults([]discovery.Result{{
			Index:     1,
			Provider:  discovery.ProviderHuggingFace,
			Name:      "owner/tiny",
			Pipeline:  "text-generation",
			Downloads: 42,
			Likes:     7,
			FilePath:  "tiny.gguf",
			Size:      2048,
			URI:       "hf://owner/tiny/tiny.gguf?rev=main",
		}}, "tiny gpt2", false)
		if err != nil {
			t.Fatalf("print discovery results: %v", err)
		}
	})
	for _, want := range []string{"owner/tiny", "tiny.gguf", "hf://owner/tiny/tiny.gguf?rev=main", "modfetch discover download \"tiny gpt2\" --select 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
