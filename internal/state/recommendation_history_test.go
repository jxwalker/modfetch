package state

import "testing"

func TestRecommendationHistoryUpsertListAndLookup(t *testing.T) {
	db, err := NewDB(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	row := RecommendationHistoryRow{
		Task:        "coding",
		Query:       "qwen coder gguf",
		Provider:    "huggingface",
		ModelID:     "owner/model",
		URI:         "hf://owner/model/model.gguf?rev=main",
		Action:      "selected",
		Score:       100,
		Fit:         "excellent",
		HardwareKey: "darwin/arm64/128g/unified",
	}
	if err := db.UpsertRecommendationHistory(row); err != nil {
		t.Fatalf("upsert selected: %v", err)
	}
	if err := db.UpsertRecommendationHistory(row); err != nil {
		t.Fatalf("upsert selected again: %v", err)
	}
	row.Action = "skipped"
	row.URI = "hf://owner/other/other.gguf?rev=main"
	if err := db.UpsertRecommendationHistory(row); err != nil {
		t.Fatalf("upsert skipped: %v", err)
	}

	lookup, err := db.RecommendationHistoryFor("coding", "qwen coder gguf", "darwin/arm64/128g/unified")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(lookup) != 2 {
		t.Fatalf("lookup len = %d, want 2", len(lookup))
	}
	var selected RecommendationHistoryRow
	for _, got := range lookup {
		if got.Action == "selected" {
			selected = got
		}
	}
	if selected.Count != 2 || selected.LastSelected == 0 {
		t.Fatalf("selected row = %#v, want count 2 and last_selected", selected)
	}

	listed, err := db.ListRecommendationHistory(1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed len = %d, want 1", len(listed))
	}
}
