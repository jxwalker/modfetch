package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modfetch/internal/config"
)

func TestCivitAIResolve_ModelLatestAndHeaders(t *testing.T) {
	// Fake CivitAI API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v1/models/") {
			w.WriteHeader(200)
			w.Write([]byte(`{
			  "modelVersions": [
			    {"id":11, "files":[{"id":1,"name":"v11.notprimary.safetensors","type":"Model","primary":false,"downloadUrl":"` + tsURLPlaceholder + `/dl/a.bin"}]},
			    {"id":12, "files":[
			      {"id":2,"name":"v12.primary.safetensors","type":"Model","primary":true,"downloadUrl":"` + tsURLPlaceholder + `/dl/primary.bin"},
			      {"id":3,"name":"v12.vae","type":"VAE","primary":false,"downloadUrl":"` + tsURLPlaceholder + `/dl/vae.bin"}
			    ]}
			  ]
			}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/model-versions/") {
			w.WriteHeader(200)
			w.Write([]byte(`{"id":12, "files":[{"id":9, "name":"mv.primary.safetensors","type":"Model","primary":true,"downloadUrl":"` + tsURLPlaceholder + `/dl/mv.bin"}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()

	// Replace placeholder with actual URL in handler output
	tsURL := ts.URL
	rewriteHandler(t, ts, tsURL)

	// Override base URL for resolver
	oldBase := civitaiBaseURL
	civitaiBaseURL = tsURL
	defer func(){ civitaiBaseURL = oldBase }()

	// Config enabling CivitAI with token env
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfgYaml := []byte("version: 1\n"+
		"general:\n  data_root: \""+tmp+"/data\"\n  download_root: \""+tmp+"/dl\"\n"+
		"sources:\n  civitai:\n    enabled: true\n    token_env: \"CIVITAI_TOKEN\"\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil { t.Fatal(err) }
	_ = os.Setenv("CIVITAI_TOKEN", "XYZ")
	cfg, err := config.Load(cfgPath)
	if err != nil { t.Fatalf("config: %v", err) }

	// Latest primary
	res, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/123", cfg)
	if err != nil { t.Fatalf("resolve: %v", err) }
	if !strings.HasSuffix(res.URL, "/dl/primary.bin") {
		t.Fatalf("unexpected url: %s", res.URL)
	}
	if got := res.Headers["Authorization"]; got != "Bearer XYZ" {
		t.Fatalf("auth header not set: %q", got)
	}
	// Suggested filename falls back to original file name when model name is absent in API
	if res.SuggestedFilename == "" || !strings.HasSuffix(res.SuggestedFilename, ".safetensors") {
		t.Fatalf("expected suggested filename safetensors, got %q", res.SuggestedFilename)
	}

	// Filter by file name substring
	res2, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/123?file=vae", cfg)
	if err != nil { t.Fatalf("resolve2: %v", err) }
	if !strings.HasSuffix(res2.URL, "/dl/vae.bin") {
		t.Fatalf("unexpected url2: %s", res2.URL)
	}
	if res2.SuggestedFilename == "" || !strings.Contains(res2.SuggestedFilename, "vae") {
		t.Fatalf("expected suggested filename containing 'vae', got %q", res2.SuggestedFilename)
	}

	// Specific version
	res3, err := (&CivitAI{}).Resolve(context.Background(), "civitai://model/123?version=12", cfg)
	if err != nil { t.Fatalf("resolve3: %v", err) }
	if !strings.HasSuffix(res3.URL, "/dl/mv.bin") {
		t.Fatalf("unexpected url3: %s", res3.URL)
	}
	if res3.SuggestedFilename == "" || !strings.Contains(res3.SuggestedFilename, ".safetensors") {
		t.Fatalf("expected suggested safetensors filename, got %q", res3.SuggestedFilename)
	}
}

// rewriteHandler swaps out a placeholder token with the actual server URL in responses.
const tsURLPlaceholder = "__TSURL__"

func rewriteHandler(t *testing.T, ts *httptest.Server, tsURL string) {
	t.Helper()
	orig := ts.Config.Handler
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &respRewriter{ResponseWriter: w, replace: tsURL}
		orig.ServeHTTP(rw, r)
	})
}

type respRewriter struct {
	http.ResponseWriter
	replace string
}

func (rw *respRewriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write([]byte(strings.ReplaceAll(string(b), tsURLPlaceholder, rw.replace)))
}

