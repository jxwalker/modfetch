package metadata

import (
	"context"
	"net/http"
	"testing"
)

func TestModelScopeFetcherCanHandle(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "ModelScope model URL",
			url:  "https://modelscope.cn/models/qwen/Qwen2.5-7B-Instruct",
			want: true,
		},
		{
			name: "ModelScope www model URL",
			url:  "https://www.modelscope.cn/models/qwen/Qwen2.5-7B-Instruct/resolve/master/model.gguf",
			want: true,
		},
		{
			name: "Hugging Face URL",
			url:  "https://huggingface.co/qwen/Qwen2.5-7B-Instruct",
			want: false,
		},
		{
			name: "ModelScope non-model URL",
			url:  "https://modelscope.cn/docs",
			want: false,
		},
	}

	f := NewModelScopeFetcher(&http.Client{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.CanHandle(tt.url); got != tt.want {
				t.Fatalf("CanHandle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelScopeFetcherFetchMetadataSuccess(t *testing.T) {
	response := `{
		"Code": 200,
		"Success": true,
		"Message": "OK",
		"Data": {
			"Id": "qwen/Qwen2.5-7B-Instruct",
			"Name": "Qwen2.5-7B-Instruct",
			"Description": "Instruction tuned Qwen model",
			"Owner": "qwen",
			"License": "apache-2.0",
			"Tags": ["llm", "text-generation"],
			"Tasks": ["text-generation"],
			"Downloads": 4200,
			"ModelCover": "https://modelscope.cn/cover.png"
		}
	}`
	client := routedHTTPClient(t, "modelscope.cn", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models/qwen/Qwen2.5-7B-Instruct" {
			t.Errorf("unexpected ModelScope API path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("Revision") != "master" {
			t.Errorf("unexpected revision query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))

	f := NewModelScopeFetcher(client)
	meta, err := f.FetchMetadata(context.Background(), "https://modelscope.cn/models/qwen/Qwen2.5-7B-Instruct/resolve/master/qwen2.5.Q4_K_M.gguf")
	if err != nil {
		t.Fatalf("FetchMetadata() error = %v", err)
	}
	if meta.Source != "modelscope" {
		t.Fatalf("Source = %q, want modelscope", meta.Source)
	}
	if meta.ModelID != "qwen/Qwen2.5-7B-Instruct" {
		t.Fatalf("ModelID = %q", meta.ModelID)
	}
	if meta.ModelName != "Qwen2.5-7B-Instruct" || meta.Author != "qwen" {
		t.Fatalf("unexpected identity metadata: %+v", meta)
	}
	if meta.RepoURL != "https://modelscope.cn/models/qwen/Qwen2.5-7B-Instruct" || meta.AuthorURL != "https://modelscope.cn/profile/qwen" {
		t.Fatalf("unexpected links: %+v", meta)
	}
	if meta.ModelType != "LLM" || meta.Quantization != "Q4_K_M" || meta.FileFormat != ".gguf" {
		t.Fatalf("unexpected model specs: %+v", meta)
	}
	if meta.DownloadCount != 4200 || meta.License != "apache-2.0" {
		t.Fatalf("unexpected registry metadata: %+v", meta)
	}
}

func TestModelScopeFetcherAPIFailureFallsBack(t *testing.T) {
	client := routedHTTPClient(t, "modelscope.cn", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	f := NewModelScopeFetcher(client)
	meta, err := f.FetchMetadata(context.Background(), "https://modelscope.cn/models/qwen/Qwen2.5-7B-Instruct/resolve/master/qwen2.5.Q5_K_M.gguf")
	if err != nil {
		t.Fatalf("FetchMetadata() should fall back on API failure, got %v", err)
	}
	if meta.Source != "modelscope" || meta.ModelID != "qwen/Qwen2.5-7B-Instruct" {
		t.Fatalf("unexpected fallback metadata: %+v", meta)
	}
	if meta.Quantization != "Q5_K_M" || meta.FileFormat != ".gguf" {
		t.Fatalf("unexpected fallback file metadata: %+v", meta)
	}
}

func TestParseModelScopeURL(t *testing.T) {
	modelID, owner, repo, revision, filename, err := parseModelScopeURL("https://modelscope.cn/models/qwen/Qwen2.5-7B-Instruct/resolve/master/path/to/model.gguf")
	if err != nil {
		t.Fatalf("parseModelScopeURL() error = %v", err)
	}
	if modelID != "qwen/Qwen2.5-7B-Instruct" || owner != "qwen" || repo != "Qwen2.5-7B-Instruct" || revision != "master" || filename != "path/to/model.gguf" {
		t.Fatalf("unexpected parse result: %q %q %q %q %q", modelID, owner, repo, revision, filename)
	}
}

func TestModelScopeURLBuildersEscapeSegments(t *testing.T) {
	if got := modelScopeAPIURL("owner name", "repo/name", "release/1"); got != "https://modelscope.cn/api/v1/models/owner%20name/repo%2Fname?Revision=release%2F1" {
		t.Fatalf("modelScopeAPIURL() = %q", got)
	}
	if got := modelScopeModelURL("owner name", "repo/name"); got != "https://modelscope.cn/models/owner%20name/repo%2Fname" {
		t.Fatalf("modelScopeModelURL() = %q", got)
	}
}
