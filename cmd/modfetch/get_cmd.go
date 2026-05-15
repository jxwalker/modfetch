package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
)

type getProfile struct {
	Name        string
	Task        string
	Provider    string
	Query       string
	Limit       int
	StarterID   string
	Description string
}

func handleGet(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printGetUsage()
		if len(args) == 0 {
			return errors.New("usage: modfetch get TASK [flags]")
		}
		return nil
	}

	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print command output as JSON")
	provider := fs.String("provider", "", "provider: huggingface|civitai|modelscope|all")
	query := fs.String("query", "", "override the curated search query")
	limit := fs.Int("limit", 5, "maximum recommendations to inspect")
	selectIndex := fs.Int("select", 1, "1-based recommendation to use with --download")
	download := fs.Bool("download", false, "download the selected recommendation")
	dest := fs.String("dest", "", "destination path when downloading")
	placeFlag := fs.Bool("place", false, "place after successful download")
	summaryJSON := fs.Bool("summary-json", false, "print download completion summary as JSON")
	dryRun := fs.Bool("dry-run", false, "plan selected download without downloading")
	runHelp := fs.Bool("run-help", false, "show local runtime guidance for the selected artifact; implies --dry-run unless --download is set")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs")
	noResume := fs.Bool("no-resume", false, "start selected download fresh instead of resuming")
	ramGB := fs.Float64("ram-gb", 0, "override detected system RAM in GiB")
	vramGB := fs.Float64("vram-gb", 0, "override detected dedicated VRAM in GiB")
	unified := fs.Bool("unified-memory", false, "treat RAM as unified CPU/GPU memory")
	small := fs.Bool("small", false, "prefer a small first download")
	medium := fs.Bool("medium", false, "prefer a balanced local model")
	large := fs.Bool("large", false, "prefer larger/high-quality candidates")
	size := fs.String("size", "", "size preset: small|medium|large")
	starterID := fs.String("starter-id", "", "starter artifact ID when TASK is starter")
	noLearn := fs.Bool("no-learn", false, "do not use or write recommendation history")
	flagArgs, positional := splitDiscoverArgs(args, map[string]bool{
		"json": true, "download": true, "place": true, "summary-json": true, "dry-run": true, "quiet": true,
		"run-help": true, "no-resume": true, "unified-memory": true, "small": true, "medium": true, "large": true, "no-learn": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	positional = append(positional, fs.Args()...)
	if len(positional) == 0 {
		printGetUsage()
		return errors.New("usage: modfetch get TASK [flags]")
	}
	task := strings.TrimSpace(positional[0])
	queryWords := strings.TrimSpace(strings.Join(positional[1:], " "))
	sizePreset, err := getSizePreset(*size, *small, *medium, *large)
	if err != nil {
		return err
	}
	profile, err := resolveGetProfile(task, sizePreset)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*provider) != "" {
		profile.Provider = strings.TrimSpace(*provider)
	}
	if *limit > 0 {
		profile.Limit = *limit
	}
	if strings.TrimSpace(*query) != "" {
		profile.Query = strings.TrimSpace(*query)
	} else if queryWords != "" {
		profile.Query = queryWords
	}
	selectedDryRun := *dryRun || (*runHelp && !*download)
	if profile.StarterID != "" {
		if queryWords != "" || strings.TrimSpace(*query) != "" {
			return errors.New("starter does not accept free-form query terms; use --starter-id")
		}
		id := profile.StarterID
		if strings.TrimSpace(*starterID) != "" {
			id = strings.TrimSpace(*starterID)
		}
		return starterDownload(ctx, buildGetStarterArgs(getRunOptions{
			configPath:  *common.configPath,
			logLevel:    *common.logLevel,
			jsonOut:     *common.jsonOut,
			id:          id,
			dest:        *dest,
			place:       *placeFlag,
			summaryJSON: *summaryJSON,
			dryRun:      selectedDryRun,
			runHelp:     *runHelp,
			quiet:       *quiet,
			noResume:    *noResume,
		}))
	}
	return handleRecommend(ctx, buildGetRecommendArgs(getRunOptions{
		configPath:  *common.configPath,
		logLevel:    *common.logLevel,
		jsonOut:     *common.jsonOut,
		task:        profile.Task,
		provider:    profile.Provider,
		query:       profile.Query,
		limit:       profile.Limit,
		selectIndex: *selectIndex,
		download:    *download || selectedDryRun || strings.TrimSpace(*dest) != "" || *placeFlag || *summaryJSON,
		dest:        *dest,
		place:       *placeFlag,
		summaryJSON: *summaryJSON,
		dryRun:      selectedDryRun,
		runHelp:     *runHelp,
		quiet:       *quiet,
		noResume:    *noResume,
		ramGB:       *ramGB,
		vramGB:      *vramGB,
		unified:     *unified,
		noLearn:     *noLearn,
	}))
}

func printGetUsage() {
	fmt.Println(strings.TrimSpace(`Usage:
  modfetch get TASK [--small|--medium|--large] [--download|--dry-run] [download flags]

Tasks:
  coding       coding assistant GGUF recommendations
  chat         general chat/instruct GGUF recommendations
  embedding    embedding model recommendations
  image        Stable Diffusion / ComfyUI checkpoint recommendations
  starter      download a beginner-safe starter artifact

Examples:
  modfetch get coding --small
  modfetch get coding --small --run-help
  modfetch get coding --small --download
  modfetch get embedding --download --summary-json
  modfetch get image --small --dry-run
  modfetch get starter --dry-run`))
}

type getRunOptions struct {
	configPath  string
	logLevel    string
	jsonOut     bool
	task        string
	provider    string
	query       string
	limit       int
	selectIndex int
	download    bool
	id          string
	dest        string
	place       bool
	summaryJSON bool
	dryRun      bool
	runHelp     bool
	quiet       bool
	noResume    bool
	ramGB       float64
	vramGB      float64
	unified     bool
	noLearn     bool
}

func buildGetRecommendArgs(opts getRunOptions) []string {
	args := []string{
		"--config", opts.configPath,
		"--log-level", opts.logLevel,
		"--provider", opts.provider,
		"--task", opts.task,
		"--limit", fmt.Sprint(opts.limit),
		"--select", fmt.Sprint(opts.selectIndex),
	}
	if opts.jsonOut {
		args = append(args, "--json")
	}
	if opts.download {
		args = append(args, "--download")
	}
	if opts.dest != "" {
		args = append(args, "--dest", opts.dest)
	}
	if opts.place {
		args = append(args, "--place")
	}
	if opts.summaryJSON {
		args = append(args, "--summary-json")
	}
	if opts.dryRun {
		args = append(args, "--dry-run")
	}
	if opts.runHelp {
		args = append(args, "--run-help")
	}
	if opts.quiet {
		args = append(args, "--quiet")
	}
	if opts.noResume {
		args = append(args, "--no-resume")
	}
	if opts.ramGB > 0 {
		args = append(args, "--ram-gb", fmt.Sprint(opts.ramGB))
	}
	if opts.vramGB > 0 {
		args = append(args, "--vram-gb", fmt.Sprint(opts.vramGB))
	}
	if opts.unified {
		args = append(args, "--unified-memory")
	}
	if opts.noLearn {
		args = append(args, "--no-learn")
	}
	if opts.query != "" {
		args = append(args, "--", opts.query)
	}
	return args
}

func buildGetStarterArgs(opts getRunOptions) []string {
	args := []string{
		"--config", opts.configPath,
		"--log-level", opts.logLevel,
		"--id", opts.id,
	}
	if opts.jsonOut {
		args = append(args, "--json")
	}
	if opts.dest != "" {
		args = append(args, "--dest", opts.dest)
	}
	if opts.place {
		args = append(args, "--place")
	}
	if opts.summaryJSON {
		args = append(args, "--summary-json")
	}
	if opts.dryRun {
		args = append(args, "--dry-run")
	}
	if opts.runHelp {
		args = append(args, "--run-help")
	}
	if opts.quiet {
		args = append(args, "--quiet")
	}
	if opts.noResume {
		args = append(args, "--no-resume")
	}
	return args
}

func getSizePreset(explicit string, small, medium, large bool) (string, error) {
	selected := strings.ToLower(strings.TrimSpace(explicit))
	count := 0
	if selected != "" {
		count++
	}
	for _, enabled := range []bool{small, medium, large} {
		if enabled {
			count++
		}
	}
	if count > 1 {
		return "", errors.New("choose only one size preset: --small, --medium, --large, or --size")
	}
	if small {
		selected = "small"
	}
	if medium {
		selected = "medium"
	}
	if large {
		selected = "large"
	}
	switch selected {
	case "", "small", "medium", "large":
		return selected, nil
	default:
		return "", fmt.Errorf("unknown size preset %q (valid: small, medium, large)", explicit)
	}
}

func resolveGetProfile(task, size string) (getProfile, error) {
	task = strings.ToLower(strings.TrimSpace(task))
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "" {
		size = "medium"
	}
	switch task {
	case "starter", "start", "smoke":
		return getProfile{
			Name:        "starter",
			StarterID:   "gpt2-tokenizer",
			Description: "beginner-safe Hugging Face tokenizer smoke download",
		}, nil
	case "coding", "code", "developer", "programming":
		return getProfile{
			Name:        "coding-" + size,
			Task:        "coding",
			Provider:    "huggingface",
			Query:       sizedQuery(size, "qwen2.5 coder 1.5b gguf", "qwen coder gguf", "deepseek coder qwen gguf"),
			Limit:       5,
			Description: "coding assistant GGUF recommendation",
		}, nil
	case "chat", "assistant", "general":
		return getProfile{
			Name:        "chat-" + size,
			Task:        "chat",
			Provider:    "huggingface",
			Query:       sizedQuery(size, "qwen2.5 1.5b instruct gguf", "llama instruct gguf", "large instruct gguf"),
			Limit:       5,
			Description: "general chat/instruct GGUF recommendation",
		}, nil
	case "embedding", "embeddings", "embed", "search":
		return getProfile{
			Name:        "embedding-" + size,
			Task:        "embedding",
			Provider:    "huggingface",
			Query:       sizedQuery(size, "bge small embedding", "embedding model", "bge large embedding"),
			Limit:       5,
			Description: "embedding model recommendation",
		}, nil
	case "image", "sd", "stable-diffusion", "comfyui":
		return getProfile{
			Name:        "image-" + size,
			Task:        "image",
			Provider:    "huggingface",
			Query:       sizedQuery(size, "sd 1.5 safetensors", "stable diffusion safetensors", "sdxl safetensors"),
			Limit:       5,
			Description: "Stable Diffusion / ComfyUI checkpoint recommendation",
		}, nil
	default:
		return getProfile{}, fmt.Errorf("unknown get task %q (valid: coding, chat, embedding, image, starter)", task)
	}
}

func sizedQuery(size, small, medium, large string) string {
	switch size {
	case "small":
		return small
	case "large":
		return large
	default:
		return medium
	}
}
