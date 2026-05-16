package taskpack

import (
	"path/filepath"
	"testing"
)

func TestListIsSortedAndContainsRunnablePacks(t *testing.T) {
	packs := List()
	if len(packs) < 2 {
		t.Fatalf("expected multiple packs, got %d", len(packs))
	}
	for i := 1; i < len(packs); i++ {
		if packs[i-1].ID > packs[i].ID {
			t.Fatalf("packs are not sorted: %s before %s", packs[i-1].ID, packs[i].ID)
		}
	}
	for _, id := range []string{"embedding-smoke", "llm-smoke"} {
		pack, ok := Get(id)
		if !ok {
			t.Fatalf("missing pack %q", id)
		}
		if len(pack.Files) == 0 {
			t.Fatalf("pack %q has no files", id)
		}
		for _, file := range pack.Files {
			if file.URI == "" || file.Path == "" {
				t.Fatalf("pack %q has incomplete file: %#v", id, file)
			}
		}
	}
}

func TestManifestBuildsBatchDestinations(t *testing.T) {
	root := t.TempDir()
	manifest, err := MustManifest("llm-smoke", root)
	if err != nil {
		t.Fatalf("MustManifest: %v", err)
	}
	if manifest.Source != "taskpack" || manifest.Repo != "llm-smoke" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	bf := manifest.Batch()
	if len(bf.Jobs) != len(manifest.Files) {
		t.Fatalf("jobs = %d, files = %d", len(bf.Jobs), len(manifest.Files))
	}
	if want := filepath.Join(root, "llm-smoke", "config.json"); bf.Jobs[0].Dest != want {
		t.Fatalf("first dest = %q, want %q", bf.Jobs[0].Dest, want)
	}
}
