package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchHuggingFaceReturnsDownloadableURI(t *testing.T) {
	old := huggingFaceBaseURL
	defer func() { huggingFaceBaseURL = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			if got := r.URL.Query().Get("search"); got != "tiny" {
				t.Fatalf("search query = %q", got)
			}
			_, _ = w.Write([]byte(`[{"id":"owner/tiny","modelId":"owner/tiny","downloads":42,"likes":7,"pipeline_tag":"text-generation","tags":["gguf"]}]`))
		case "/api/models/owner/tiny/tree/main":
			_, _ = w.Write([]byte(`[
				{"type":"file","path":"README.md","size":123},
				{"type":"file","path":"tiny.Q4_K_M.gguf","size":2048},
				{"type":"file","path":"model.safetensors","size":4096}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	huggingFaceBaseURL = server.URL

	results, err := Search(context.Background(), Options{Provider: ProviderHuggingFace, Query: "tiny", Limit: 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	got := results[0]
	if got.URI != "hf://owner/tiny/tiny.Q4_K_M.gguf?rev=main" {
		t.Fatalf("uri = %q", got.URI)
	}
	if got.Size != 2048 || got.FileType != "gguf" || got.Index != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestSearchCivitAIReturnsResolverURI(t *testing.T) {
	old := civitaiBaseURL
	defer func() { civitaiBaseURL = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":123,"name":"Tiny Checkpoint","type":"Checkpoint","tags":["sd"],"stats":{"downloadCount":9,"thumbsUpCount":2},"modelVersions":[{"id":456,"name":"v1","files":[{"name":"tiny.safetensors","type":"Model","sizeKB":10.5}]}]}]}`))
	}))
	defer server.Close()
	civitaiBaseURL = server.URL

	results, err := Search(context.Background(), Options{Provider: ProviderCivitAI, Query: "tiny", Limit: 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	got := results[0]
	if got.URI != "civitai://model/123?version=456&file=tiny.safetensors" {
		t.Fatalf("uri = %q", got.URI)
	}
	if got.Size != 10752 || got.FileType != "Model" || got.VersionID != "456" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// ---- hfFileScore ----------------------------------------------------------

func TestHfFileScore_Extensions(t *testing.T) {
	cases := []struct {
		path      string
		wantScore int
		excluded  bool
	}{
		{"model.gguf", 100, false},
		{"model.safetensors", 90, false},
		{"model.bin", 80, false},
		{"model.pt", 70, false},
		{"model.pth", 70, false},
		{"model.ckpt", 70, false},
		{"model.onnx", 50, false},
		{"README.md", 0, true},
		{"config.json", 0, true},
	}
	for _, tc := range cases {
		got := hfFileScore(tc.path)
		if tc.excluded {
			if got >= 0 {
				t.Errorf("hfFileScore(%q) = %d, want < 0 (excluded)", tc.path, got)
			}
		} else {
			if got != tc.wantScore {
				t.Errorf("hfFileScore(%q) = %d, want %d", tc.path, got, tc.wantScore)
			}
		}
	}
}

func TestHfFileScore_QuantBonus(t *testing.T) {
	if got := hfFileScore("model-q4_k_m.gguf"); got != 112 {
		t.Errorf("q4_k_m.gguf score = %d, want 112", got)
	}
	if got := hfFileScore("model-q5_k_m.gguf"); got != 110 {
		t.Errorf("q5_k_m.gguf score = %d, want 110", got)
	}
	if got := hfFileScore("model-q4.gguf"); got != 108 {
		t.Errorf("q4.gguf score = %d, want 108", got)
	}
	if got := hfFileScore("model-fp16.gguf"); got != 105 {
		t.Errorf("fp16.gguf score = %d, want 105", got)
	}
}

func TestHfFileScore_OptimizerExcluded(t *testing.T) {
	// optimizer_state.bin has base score 80 but penalty -100 → negative → skipped
	if got := hfFileScore("optimizer_state.bin"); got >= 0 {
		t.Errorf("optimizer_state.bin score = %d, want < 0", got)
	}
}

// ---- preferSmallerNonZero -------------------------------------------------

func TestPreferSmallerNonZero(t *testing.T) {
	cases := []struct {
		a, b int64
		want bool
	}{
		{0, 100, false},
		{100, 0, true},
		{50, 100, true},
		{100, 50, false},
		{50, 50, false},
		{-1, 50, false},
	}
	for _, tc := range cases {
		got := preferSmallerNonZero(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("preferSmallerNonZero(%d, %d) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// ---- zeroIntString --------------------------------------------------------

func TestZeroIntString(t *testing.T) {
	if got := zeroIntString(0); got != "" {
		t.Errorf("zeroIntString(0) = %q, want empty", got)
	}
	if got := zeroIntString(42); got != "42" {
		t.Errorf("zeroIntString(42) = %q, want \"42\"", got)
	}
}

// ---- normalizeProvider ----------------------------------------------------

func TestNormalizeProvider(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ProviderHuggingFace},
		{"hf", ProviderHuggingFace},
		{"HuggingFace", ProviderHuggingFace},
		{"hugging-face", ProviderHuggingFace},
		{"civ", ProviderCivitAI},
		{"CivitAI", ProviderCivitAI},
		{"civit-ai", ProviderCivitAI},
		{"ms", ProviderModelScope},
		{"modelscope", ProviderModelScope},
		{"model-scope", ProviderModelScope},
		{"ModelScope", ProviderModelScope},
		{"all", ProviderAll},
		{"ALL", ProviderAll},
		{"unknown", "unknown"},
	}
	for _, tc := range cases {
		got := normalizeProvider(tc.in)
		if got != tc.want {
			t.Errorf("normalizeProvider(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---- bestCivitAIFile ------------------------------------------------------

func TestBestCivitAIFile_ModelTypePreferred(t *testing.T) {
	versions := []civitaiVersion{
		{
			ID:   1,
			Name: "v1",
			Files: []civitaiFile{
				{Name: "other.txt", Type: "Other"},
				{Name: "weights.safetensors", Type: "Model"},
			},
		},
	}
	ver, file := bestCivitAIFile(versions)
	if ver.ID != 1 {
		t.Errorf("version ID = %d, want 1", ver.ID)
	}
	if file.Name != "weights.safetensors" {
		t.Errorf("file = %q, want weights.safetensors", file.Name)
	}
}

func TestBestCivitAIFile_FallbackToFirst(t *testing.T) {
	versions := []civitaiVersion{
		{
			ID:   5,
			Name: "v5",
			Files: []civitaiFile{
				{Name: "config.yaml", Type: "Config"},
				{Name: "readme.md", Type: "Doc"},
			},
		},
	}
	_, file := bestCivitAIFile(versions)
	if file.Name != "config.yaml" {
		t.Errorf("file = %q, want config.yaml (first file)", file.Name)
	}
}

func TestBestCivitAIFile_HighestVersionSelected(t *testing.T) {
	versions := []civitaiVersion{
		{ID: 2, Files: []civitaiFile{{Name: "v2.bin", Type: "Model"}}},
		{ID: 7, Files: []civitaiFile{{Name: "v7.bin", Type: "Model"}}},
		{ID: 4, Files: []civitaiFile{{Name: "v4.bin", Type: "Model"}}},
	}
	_, file := bestCivitAIFile(versions)
	if file.Name != "v7.bin" {
		t.Errorf("file = %q, want v7.bin (highest version ID)", file.Name)
	}
}

func TestBestCivitAIFile_EmptyVersions(t *testing.T) {
	ver, file := bestCivitAIFile(nil)
	if ver.ID != 0 || file.Name != "" {
		t.Errorf("expected zero values for empty versions, got ver=%+v file=%+v", ver, file)
	}
}

// ---- bestHFFile -----------------------------------------------------------

func TestBestHFFile_PicksHighestScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		files := []hfRepoFile{
			{Type: "file", Path: "README.md", Size: 100},
			{Type: "file", Path: "model.safetensors", Size: 500 * 1024 * 1024},
			{Type: "file", Path: "model-q4_k_m.gguf", Size: 4 * 1024 * 1024 * 1024},
			{Type: "directory", Path: "checkpoints", Size: 0},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	orig := huggingFaceBaseURL
	huggingFaceBaseURL = srv.URL
	defer func() { huggingFaceBaseURL = orig }()

	got, err := bestHFFile(context.Background(), srv.Client(), "owner/repo")
	if err != nil {
		t.Fatalf("bestHFFile: %v", err)
	}
	if got.Path != "model-q4_k_m.gguf" {
		t.Errorf("bestHFFile = %q, want model-q4_k_m.gguf", got.Path)
	}
}

func TestBestHFFile_NoModelFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		files := []hfRepoFile{
			{Type: "file", Path: "README.md", Size: 100},
			{Type: "file", Path: "config.json", Size: 200},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	orig := huggingFaceBaseURL
	huggingFaceBaseURL = srv.URL
	defer func() { huggingFaceBaseURL = orig }()

	_, err := bestHFFile(context.Background(), srv.Client(), "owner/repo")
	if err == nil {
		t.Fatal("expected error for repo with no downloadable model files")
	}
}

// ---- Search error cases ---------------------------------------------------

func TestSearch_EmptyQuery(t *testing.T) {
	_, err := Search(context.Background(), Options{Provider: ProviderHuggingFace, Query: "  "})
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query-required error, got %v", err)
	}
}

func TestSearch_UnknownProvider(t *testing.T) {
	_, err := Search(context.Background(), Options{Provider: "bogusprovider", Query: "gpt"})
	if err == nil || !strings.Contains(err.Error(), "unknown discovery provider") {
		t.Fatalf("expected unknown-provider error, got %v", err)
	}
}

// ---- Search ModelScope ----------------------------------------------------

func TestSearch_ModelScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/models":
			resp := modelScopeSearchResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Models []modelScopeSearchModel `json:"Models"`
				}{
					Models: []modelScopeSearchModel{
						{
							ID:        "qwen/Qwen-7B",
							Name:      "Qwen-7B",
							ChName:    "通义千问-7B",
							Owner:     "qwen",
							Downloads: 12345,
							Likes:     678,
							Tags:      []string{"llm", "chat"},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/repo/files"):
			resp := modelScopeFilesResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Files []modelScopeFile `json:"Files"`
				}{
					Files: []modelScopeFile{
						{Type: "blob", Path: "README.md", Size: 100},
						{Type: "blob", Path: "qwen-7b.Q4_K_M.gguf", Size: 4 * 1024 * 1024 * 1024},
						{Type: "tree", Path: "checkpoints"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	orig := modelScopeBaseURL
	modelScopeBaseURL = srv.URL
	defer func() { modelScopeBaseURL = orig }()

	results, err := Search(context.Background(), Options{Provider: ProviderModelScope, Query: "qwen", Limit: 5})
	if err != nil {
		t.Fatalf("Search ModelScope: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	r := results[0]
	if r.Provider != ProviderModelScope {
		t.Errorf("provider = %q, want %q", r.Provider, ProviderModelScope)
	}
	if r.ModelID != "qwen/Qwen-7B" {
		t.Errorf("model_id = %q, want qwen/Qwen-7B", r.ModelID)
	}
	wantURI := srv.URL + "/models/qwen/Qwen-7B/resolve/master/qwen-7b.Q4_K_M.gguf"
	if r.URI != wantURI {
		t.Errorf("URI = %q, want %q", r.URI, wantURI)
	}
	if r.FilePath != "qwen-7b.Q4_K_M.gguf" || r.FileName != "qwen-7b.Q4_K_M.gguf" || r.FileType != "gguf" {
		t.Errorf("unexpected file fields: path=%q name=%q type=%q", r.FilePath, r.FileName, r.FileType)
	}
	if r.Size != 4*1024*1024*1024 {
		t.Errorf("size = %d, want 4GiB", r.Size)
	}
	if r.Downloads != 12345 {
		t.Errorf("downloads = %d, want 12345", r.Downloads)
	}
}

func TestSearch_ModelScope_DropsResultsWithoutDownloadableFile(t *testing.T) {
	// When the file listing fails, the model page URL would be HTML, so the
	// result is dropped entirely rather than handed to `download --url`,
	// which would otherwise save HTML masquerading as a model artifact.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/models":
			_ = json.NewEncoder(w).Encode(modelScopeSearchResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Models []modelScopeSearchModel `json:"Models"`
				}{
					Models: []modelScopeSearchModel{
						{ID: "alice/MyModel", Name: "MyModel", Owner: "alice"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/repo/files"):
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	orig := modelScopeBaseURL
	modelScopeBaseURL = srv.URL
	defer func() { modelScopeBaseURL = orig }()

	results, err := Search(context.Background(), Options{Provider: ProviderModelScope, Query: "alice", Limit: 5})
	if err != nil {
		t.Fatalf("Search ModelScope: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results len = %d, want 0 (result must be dropped when no downloadable file is resolvable)", len(results))
	}
}

func TestSearch_ModelScope_DropsResultsWithoutModelFiles(t *testing.T) {
	// Repos that only contain README/config files (no .gguf/.safetensors/etc.)
	// have no downloadable artifact, so they must be omitted from results.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/models":
			_ = json.NewEncoder(w).Encode(modelScopeSearchResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Models []modelScopeSearchModel `json:"Models"`
				}{
					Models: []modelScopeSearchModel{
						{ID: "bob/EmptyRepo", Name: "EmptyRepo", Owner: "bob"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/repo/files"):
			_ = json.NewEncoder(w).Encode(modelScopeFilesResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Files []modelScopeFile `json:"Files"`
				}{
					Files: []modelScopeFile{
						{Type: "blob", Path: "README.md", Size: 100},
						{Type: "blob", Path: "config.json", Size: 200},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	orig := modelScopeBaseURL
	modelScopeBaseURL = srv.URL
	defer func() { modelScopeBaseURL = orig }()

	results, err := Search(context.Background(), Options{Provider: ProviderModelScope, Query: "bob", Limit: 5})
	if err != nil {
		t.Fatalf("Search ModelScope: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results len = %d, want 0 (no downloadable model files in repo)", len(results))
	}
}

func TestSearch_ModelScope_EscapesOwnerAndName(t *testing.T) {
	var capturedFilesPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/models":
			_ = json.NewEncoder(w).Encode(modelScopeSearchResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Models []modelScopeSearchModel `json:"Models"`
				}{
					// Owner/name with characters that need URL escaping.
					Models: []modelScopeSearchModel{
						{ID: "team space/My Model", Name: "My Model", Owner: "team space"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/repo/files"):
			capturedFilesPath = r.URL.EscapedPath()
			_ = json.NewEncoder(w).Encode(modelScopeFilesResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Files []modelScopeFile `json:"Files"`
				}{
					Files: []modelScopeFile{
						{Type: "blob", Path: "weights/model.safetensors", Size: 1024},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	orig := modelScopeBaseURL
	modelScopeBaseURL = srv.URL
	defer func() { modelScopeBaseURL = orig }()

	results, err := Search(context.Background(), Options{Provider: ProviderModelScope, Query: "team", Limit: 5})
	if err != nil {
		t.Fatalf("Search ModelScope: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d", len(results))
	}
	if want := "/api/v1/models/team%20space/My%20Model/repo/files"; capturedFilesPath != want {
		t.Errorf("file-list path = %q, want %q (owner/name should be URL-escaped)", capturedFilesPath, want)
	}
	wantURI := srv.URL + "/models/team%20space/My%20Model/resolve/master/weights/model.safetensors"
	if results[0].URI != wantURI {
		t.Errorf("URI = %q, want %q", results[0].URI, wantURI)
	}
}

func TestSearch_ModelScope_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(modelScopeSearchResponse{
			Code:    500,
			Success: false,
			Message: "internal server error",
		})
	}))
	defer srv.Close()

	orig := modelScopeBaseURL
	modelScopeBaseURL = srv.URL
	defer func() { modelScopeBaseURL = orig }()

	_, err := Search(context.Background(), Options{Provider: ProviderModelScope, Query: "test", Limit: 5})
	if err == nil {
		t.Fatal("expected error for ModelScope API failure")
	}
	if !strings.Contains(err.Error(), "internal server error") || !strings.Contains(err.Error(), "code=500") {
		t.Errorf("error = %q, want both code and message in error", err.Error())
	}
}

func TestSearch_ModelScope_BlankMessageGetsDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(modelScopeSearchResponse{
			Code:    503,
			Success: false,
			Message: "",
		})
	}))
	defer srv.Close()

	orig := modelScopeBaseURL
	modelScopeBaseURL = srv.URL
	defer func() { modelScopeBaseURL = orig }()

	_, err := Search(context.Background(), Options{Provider: ProviderModelScope, Query: "test", Limit: 5})
	if err == nil {
		t.Fatal("expected error for ModelScope API failure")
	}
	if !strings.Contains(err.Error(), "unknown error") || !strings.Contains(err.Error(), "code=503") {
		t.Errorf("error = %q, want default message and code in error", err.Error())
	}
}

func TestCheckModelScopeEnvelope(t *testing.T) {
	cases := []struct {
		name    string
		success bool
		code    int
		message string
		wantErr bool
	}{
		{"success_code_200", true, 200, "", false},
		{"success_code_zero", true, 0, "", false},
		{"failure_with_message", false, 500, "boom", true},
		{"failure_blank_message", false, 503, "", true},
		{"success_but_nonzero_code_other", true, 500, "weird", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkModelScopeEnvelope(tc.success, tc.code, tc.message)
			if (err != nil) != tc.wantErr {
				t.Fatalf("checkModelScopeEnvelope(%v, %d, %q) err = %v, wantErr %v", tc.success, tc.code, tc.message, err, tc.wantErr)
			}
		})
	}
}

// ---- Search all -----------------------------------------------------------

func TestSearch_All_CombinesProviders(t *testing.T) {
	hfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/tree/") {
			_ = json.NewEncoder(w).Encode([]hfRepoFile{
				{Type: "file", Path: "model.safetensors", Size: 512 * 1024 * 1024},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]hfModel{
			{ID: "org/hf-model", ModelID: "org/hf-model", Downloads: 8000},
		})
	}))
	defer hfSrv.Close()

	civSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(civitaiSearchResponse{
			Items: []civitaiModel{
				{
					ID:   200,
					Name: "CivModel",
					Type: "Checkpoint",
					Stats: struct {
						DownloadCount int64 `json:"downloadCount"`
						ThumbsUpCount int64 `json:"thumbsUpCount"`
					}{DownloadCount: 3000},
				},
			},
		})
	}))
	defer civSrv.Close()

	msSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/models":
			_ = json.NewEncoder(w).Encode(modelScopeSearchResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Models []modelScopeSearchModel `json:"Models"`
				}{
					Models: []modelScopeSearchModel{
						{ID: "alice/ms-model", Name: "ms-model", Owner: "alice", Downloads: 500},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/repo/files"):
			_ = json.NewEncoder(w).Encode(modelScopeFilesResponse{
				Code:    200,
				Success: true,
				Data: struct {
					Files []modelScopeFile `json:"Files"`
				}{
					Files: []modelScopeFile{
						{Type: "blob", Path: "model.safetensors", Size: 1024},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer msSrv.Close()

	origHF, origCiv, origMS := huggingFaceBaseURL, civitaiBaseURL, modelScopeBaseURL
	huggingFaceBaseURL = hfSrv.URL
	civitaiBaseURL = civSrv.URL
	modelScopeBaseURL = msSrv.URL
	defer func() {
		huggingFaceBaseURL = origHF
		civitaiBaseURL = origCiv
		modelScopeBaseURL = origMS
	}()

	results, err := Search(context.Background(), Options{Provider: ProviderAll, Query: "test", Limit: 25})
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	providers := map[string]bool{}
	for _, r := range results {
		providers[r.Provider] = true
	}
	for _, want := range []string{ProviderHuggingFace, ProviderCivitAI, ProviderModelScope} {
		if !providers[want] {
			t.Errorf("provider %q not found in combined results", want)
		}
	}
}

func TestSearch_LimitCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		models := make([]hfModel, 30)
		for i := range models {
			models[i] = hfModel{
				ID:      fmt.Sprintf("owner/model-%d", i),
				ModelID: fmt.Sprintf("owner/model-%d", i),
			}
		}
		_ = json.NewEncoder(w).Encode(models)
	}))
	defer srv.Close()

	orig := huggingFaceBaseURL
	huggingFaceBaseURL = srv.URL
	defer func() { huggingFaceBaseURL = orig }()

	results, err := Search(context.Background(), Options{Provider: ProviderHuggingFace, Query: "model", Limit: 30})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) > 25 {
		t.Errorf("len(results) = %d, want <= 25 (hard cap)", len(results))
	}
}

func TestSearch_IndexAssigned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/tree/") {
			_ = json.NewEncoder(w).Encode([]hfRepoFile{
				{Type: "file", Path: "model.gguf", Size: 100},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]hfModel{
			{ID: "owner/a", ModelID: "owner/a", Downloads: 200},
			{ID: "owner/b", ModelID: "owner/b", Downloads: 100},
		})
	}))
	defer srv.Close()

	orig := huggingFaceBaseURL
	huggingFaceBaseURL = srv.URL
	defer func() { huggingFaceBaseURL = orig }()

	results, _ := Search(context.Background(), Options{Provider: ProviderHuggingFace, Query: "model", Limit: 5})
	for i, r := range results {
		if r.Index != i+1 {
			t.Errorf("results[%d].Index = %d, want %d", i, r.Index, i+1)
		}
	}
}

