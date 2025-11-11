package state

import (
	"database/sql"
	"testing"
	"time"
)

func TestDB_UpsertMetadata(t *testing.T) {
	db := testDB(t)

	meta := &ModelMetadata{
		DownloadURL:  "https://example.com/model.gguf",
		Dest:         "/path/to/model.gguf",
		ModelName:    "Test Model",
		ModelID:      "test/model",
		Version:      "v1.0",
		Source:       "huggingface",
		Description:  "A test model",
		Author:       "TestAuthor",
		AuthorURL:    "https://example.com/author",
		License:      "MIT",
		Tags:         []string{"test", "example"},
		ModelType:    "LLM",
		Quantization: "Q4_K_M",
		FileSize:     1024 * 1024,
		FileFormat:   ".gguf",
	}

	// Test insert
	err := db.UpsertMetadata(meta)
	if err != nil {
		t.Fatalf("UpsertMetadata() error = %v", err)
	}

	// Verify insert
	retrieved, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if retrieved.ModelName != meta.ModelName {
		t.Errorf("ModelName = %q, want %q", retrieved.ModelName, meta.ModelName)
	}

	if retrieved.Source != meta.Source {
		t.Errorf("Source = %q, want %q", retrieved.Source, meta.Source)
	}

	if len(retrieved.Tags) != len(meta.Tags) {
		t.Errorf("Tags length = %d, want %d", len(retrieved.Tags), len(meta.Tags))
	}

	// Test update
	meta.Description = "Updated description"
	meta.UserRating = 5
	meta.Favorite = true

	err = db.UpsertMetadata(meta)
	if err != nil {
		t.Fatalf("UpsertMetadata() update error = %v", err)
	}

	// Verify update
	updated, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() after update error = %v", err)
	}

	if updated.Description != "Updated description" {
		t.Errorf("Description = %q, want %q", updated.Description, "Updated description")
	}

	if updated.UserRating != 5 {
		t.Errorf("UserRating = %d, want %d", updated.UserRating, 5)
	}

	if !updated.Favorite {
		t.Error("Favorite should be true")
	}
}

func TestDB_GetMetadata_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.GetMetadata("https://nonexistent.com/model.gguf")
	if err != sql.ErrNoRows {
		t.Errorf("GetMetadata() error = %v, want %v", err, sql.ErrNoRows)
	}
}

func TestDB_ListMetadata(t *testing.T) {
	db := testDB(t)

	// Insert test data
	models := []ModelMetadata{
		{
			DownloadURL: "https://example.com/model1.gguf",
			ModelName:   "Model 1",
			Source:      "huggingface",
			ModelType:   "LLM",
			Tags:        []string{"llama", "gguf"},
			UserRating:  5,
			Favorite:    true,
		},
		{
			DownloadURL: "https://example.com/model2.safetensors",
			ModelName:   "Model 2",
			Source:      "civitai",
			ModelType:   "Checkpoint",
			Tags:        []string{"realistic", "photo"},
			UserRating:  4,
			Favorite:    false,
		},
		{
			DownloadURL: "https://example.com/model3.safetensors",
			ModelName:   "Model 3",
			Source:      "huggingface",
			ModelType:   "LoRA",
			Tags:        []string{"character", "style"},
			UserRating:  3,
			Favorite:    true,
		},
	}

	for _, m := range models {
		if err := db.UpsertMetadata(&m); err != nil {
			t.Fatalf("UpsertMetadata() error = %v", err)
		}
	}

	tests := []struct {
		name    string
		filters MetadataFilters
		want    int
	}{
		{
			name:    "All models",
			filters: MetadataFilters{},
			want:    3,
		},
		{
			name: "Filter by source",
			filters: MetadataFilters{
				Source: "huggingface",
			},
			want: 2,
		},
		{
			name: "Filter by model type",
			filters: MetadataFilters{
				ModelType: "LLM",
			},
			want: 1,
		},
		{
			name: "Filter by favorites",
			filters: MetadataFilters{
				Favorite: true,
			},
			want: 2,
		},
		{
			name: "Filter by min rating",
			filters: MetadataFilters{
				MinRating: 4,
			},
			want: 2,
		},
		{
			name: "Filter by tags",
			filters: MetadataFilters{
				Tags: []string{"gguf"},
			},
			want: 1,
		},
		{
			name: "Limit results",
			filters: MetadataFilters{
				Limit: 2,
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := db.ListMetadata(tt.filters)
			if err != nil {
				t.Fatalf("ListMetadata() error = %v", err)
			}

			if len(results) != tt.want {
				t.Errorf("ListMetadata() count = %d, want %d", len(results), tt.want)
			}
		})
	}
}

func TestDB_SearchMetadata(t *testing.T) {
	db := testDB(t)

	// Insert test data
	models := []ModelMetadata{
		{
			DownloadURL: "https://example.com/llama.gguf",
			ModelName:   "Llama 2 7B",
			Description: "A large language model",
			Author:      "Meta",
			Tags:        []string{"llama", "text-generation"},
		},
		{
			DownloadURL: "https://example.com/realistic.safetensors",
			ModelName:   "Realistic Vision",
			Description: "Photorealistic image generation",
			Author:      "SG_161222",
			Tags:        []string{"realistic", "checkpoint"},
		},
		{
			DownloadURL: "https://example.com/style.safetensors",
			ModelName:   "Anime Style LoRA",
			Description: "Anime art style",
			Author:      "AnimeArtist",
			Tags:        []string{"lora", "anime"},
		},
	}

	for _, m := range models {
		if err := db.UpsertMetadata(&m); err != nil {
			t.Fatalf("UpsertMetadata() error = %v", err)
		}
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{
			name:  "Search by model name",
			query: "Llama",
			want:  1,
		},
		{
			name:  "Search by description",
			query: "photorealistic",
			want:  1,
		},
		{
			name:  "Search by author",
			query: "Meta",
			want:  1,
		},
		{
			name:  "Search by tag",
			query: "anime",
			want:  1,
		},
		{
			name:  "Search case insensitive",
			query: "REALISTIC",
			want:  1,
		},
		{
			name:  "No matches",
			query: "nonexistent",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := db.SearchMetadata(tt.query)
			if err != nil {
				t.Fatalf("SearchMetadata() error = %v", err)
			}

			if len(results) != tt.want {
				t.Errorf("SearchMetadata() count = %d, want %d", len(results), tt.want)
			}
		})
	}
}

func TestDB_UpdateMetadataUsage(t *testing.T) {
	db := testDB(t)

	meta := &ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		ModelName:   "Test Model",
	}

	err := db.UpsertMetadata(meta)
	if err != nil {
		t.Fatalf("UpsertMetadata() error = %v", err)
	}

	// Update usage
	err = db.UpdateMetadataUsage(meta.DownloadURL)
	if err != nil {
		t.Fatalf("UpdateMetadataUsage() error = %v", err)
	}

	// Verify usage updated
	updated, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if updated.TimesUsed != 1 {
		t.Errorf("TimesUsed = %d, want %d", updated.TimesUsed, 1)
	}

	if updated.LastUsed == nil {
		t.Error("LastUsed should not be nil")
	}

	// Update again
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp
	err = db.UpdateMetadataUsage(meta.DownloadURL)
	if err != nil {
		t.Fatalf("UpdateMetadataUsage() second call error = %v", err)
	}

	updated2, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() second call error = %v", err)
	}

	if updated2.TimesUsed != 2 {
		t.Errorf("TimesUsed = %d, want %d", updated2.TimesUsed, 2)
	}

	if !updated2.LastUsed.After(*updated.LastUsed) {
		t.Error("LastUsed should be updated to a later time")
	}
}

func TestDB_DeleteMetadata(t *testing.T) {
	db := testDB(t)

	meta := &ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		ModelName:   "Test Model",
	}

	err := db.UpsertMetadata(meta)
	if err != nil {
		t.Fatalf("UpsertMetadata() error = %v", err)
	}

	// Verify it exists
	_, err = db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() before delete error = %v", err)
	}

	// Delete
	err = db.DeleteMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("DeleteMetadata() error = %v", err)
	}

	// Verify deletion
	_, err = db.GetMetadata(meta.DownloadURL)
	if err != sql.ErrNoRows {
		t.Errorf("GetMetadata() after delete error = %v, want %v", err, sql.ErrNoRows)
	}
}

func TestDB_UpsertMetadata_RequiredFields(t *testing.T) {
	db := testDB(t)

	// Test missing download URL
	meta := &ModelMetadata{
		ModelName: "Test",
	}

	err := db.UpsertMetadata(meta)
	if err == nil {
		t.Error("UpsertMetadata() should error with missing download_url")
	}
}

// testDB creates an in-memory test database
func testDB(t *testing.T) *DB {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	db := &DB{SQL: sqlDB}

	// Initialize metadata table
	if err := db.InitMetadataTable(); err != nil {
		t.Fatalf("failed to initialize metadata table: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
