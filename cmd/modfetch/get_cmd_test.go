package main

import (
	"context"
	"strings"
	"testing"
)

func TestGetSizePreset(t *testing.T) {
	tests := []struct {
		name    string
		size    string
		small   bool
		medium  bool
		large   bool
		want    string
		wantErr bool
	}{
		{name: "default", want: ""},
		{name: "explicit", size: "small", want: "small"},
		{name: "flag", large: true, want: "large"},
		{name: "duplicate", size: "small", medium: true, wantErr: true},
		{name: "unknown", size: "huge", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSizePreset(tt.size, tt.small, tt.medium, tt.large)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getSizePreset err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("getSizePreset = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveGetProfile(t *testing.T) {
	tests := []struct {
		task     string
		size     string
		wantTask string
		wantQ    string
		wantID   string
	}{
		{task: "coding", size: "small", wantTask: "coding", wantQ: "qwen2.5 coder 1.5b gguf"},
		{task: "embeddings", size: "large", wantTask: "embedding", wantQ: "bge large embedding"},
		{task: "comfyui", size: "medium", wantTask: "image", wantQ: "stable diffusion safetensors"},
		{task: "starter", wantID: "gpt2-tokenizer"},
	}
	for _, tt := range tests {
		t.Run(tt.task+"/"+tt.size, func(t *testing.T) {
			got, err := resolveGetProfile(tt.task, tt.size)
			if err != nil {
				t.Fatalf("resolveGetProfile: %v", err)
			}
			if got.Task != tt.wantTask {
				t.Fatalf("task = %q, want %q", got.Task, tt.wantTask)
			}
			if got.Query != tt.wantQ {
				t.Fatalf("query = %q, want %q", got.Query, tt.wantQ)
			}
			if got.StarterID != tt.wantID {
				t.Fatalf("starter = %q, want %q", got.StarterID, tt.wantID)
			}
		})
	}
}

func TestBuildGetRecommendArgs(t *testing.T) {
	args := buildGetRecommendArgs(getRunOptions{
		configPath:  "/tmp/config.yml",
		logLevel:    "debug",
		jsonOut:     true,
		task:        "coding",
		provider:    "huggingface",
		query:       "qwen2.5 coder 1.5b gguf",
		limit:       3,
		selectIndex: 2,
		download:    true,
		dryRun:      true,
		noLearn:     true,
	})
	got := strings.Join(args, "\x00")
	for _, want := range []string{
		"--config\x00/tmp/config.yml",
		"--log-level\x00debug",
		"--provider\x00huggingface",
		"--task\x00coding",
		"--limit\x003",
		"--select\x002",
		"--json",
		"--download",
		"--dry-run",
		"--no-learn",
		"--\x00qwen2.5 coder 1.5b gguf",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("recommend args missing %q in %#v", want, args)
		}
	}
	if strings.Index(got, "--download") > strings.Index(got, "--\x00") {
		t.Fatalf("query delimiter must come after flags: %#v", args)
	}
}

func TestGetHelp(t *testing.T) {
	var runErr error
	out := captureStdout(t, func() {
		runErr = handleGet(context.Background(), []string{"help"})
	})
	if runErr != nil {
		t.Fatalf("get help: %v", runErr)
	}
	for _, want := range []string{"modfetch get TASK", "coding", "embedding", "starter"} {
		if !strings.Contains(out, want) {
			t.Fatalf("get help missing %q in:\n%s", want, out)
		}
	}
}

func TestGetStarterRejectsFreeformQuery(t *testing.T) {
	err := handleGet(context.Background(), []string{"starter", "extra words"})
	if err == nil || !strings.Contains(err.Error(), "starter does not accept free-form query terms") {
		t.Fatalf("starter query error = %v", err)
	}
}
