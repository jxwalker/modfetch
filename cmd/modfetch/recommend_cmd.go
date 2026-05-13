package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/discovery"
	"github.com/jxwalker/modfetch/internal/recommend"
	"github.com/jxwalker/modfetch/internal/state"
)

const maxHardwareOverrideGiB = 16384

type recommendSummary struct {
	Query           string                     `json:"query"`
	Task            string                     `json:"task"`
	Hardware        recommend.HardwareProfile  `json:"hardware"`
	Recommendations []recommend.Recommendation `json:"recommendations"`
}

func handleRecommend(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("recommend", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print hardware-fit recommendations as JSON")
	provider := fs.String("provider", discovery.ProviderHuggingFace, "provider: huggingface|civitai|modelscope|all")
	task := fs.String("task", "chat", "use case: chat|coding|embedding|image")
	limit := fs.Int("limit", 5, "maximum recommendations to show")
	ramGB := fs.Float64("ram-gb", 0, "override detected system RAM in GiB")
	vramGB := fs.Float64("vram-gb", 0, "override detected dedicated VRAM in GiB")
	unified := fs.Bool("unified-memory", false, "treat RAM as unified CPU/GPU memory")
	selectIndex := fs.Int("select", 1, "1-based recommendation to use with --download")
	download := fs.Bool("download", false, "download the selected recommendation through the normal download pipeline")
	dest := fs.String("dest", "", "destination path when used with --download")
	placeFlag := fs.Bool("place", false, "place after successful download")
	summaryJSON := fs.Bool("summary-json", false, "print download completion summary as JSON")
	dryRun := fs.Bool("dry-run", false, "plan selected download without downloading")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs for selected download")
	noResume := fs.Bool("no-resume", false, "start selected download fresh instead of resuming")
	history := fs.Bool("history", false, "list persisted recommendation history")
	historyLimit := fs.Int("history-limit", 50, "maximum recommendation history rows to list")
	noLearn := fs.Bool("no-learn", false, "do not use or write recommendation history for this invocation")
	flagArgs, queryArgs := splitDiscoverArgs(args, map[string]bool{
		"json": true, "unified-memory": true, "download": true, "place": true, "summary-json": true, "dry-run": true, "quiet": true, "no-resume": true, "history": true, "no-learn": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	c, _, cfgErr := loadConfig(*common.configPath)
	if *history {
		if cfgErr != nil {
			return cfgErr
		}
		return printRecommendationHistory(c, *historyLimit, *common.jsonOut)
	}
	hw := recommend.DetectHardware(ctx)
	if err := validateGiBOverride("--ram-gb", *ramGB); err != nil {
		return err
	}
	if err := validateGiBOverride("--vram-gb", *vramGB); err != nil {
		return err
	}
	if *ramGB > 0 {
		hw.RAMBytes = gibToBytes(*ramGB)
		hw.Source = "override"
	}
	if *vramGB > 0 {
		hw.VRAMBytes = gibToBytes(*vramGB)
		hw.Source = "override"
	}
	if *unified {
		hw.UnifiedMemory = true
	}
	query := strings.TrimSpace(strings.Join(append(queryArgs, fs.Args()...), " "))
	effectiveTask := recommend.NormalizeTask(*task)
	effectiveQuery := query
	if effectiveQuery == "" {
		effectiveQuery = recommend.DefaultQuery(effectiveTask)
	}
	hardwareKey := recommendationHardwareKey(hw)
	feedback := map[string]recommend.Feedback(nil)
	var st *state.DB
	if !*noLearn && cfgErr == nil {
		var stErr error
		st, stErr = state.Open(c)
		if stErr == nil {
			defer func() { _ = st.Close() }()
			var feedbackErr error
			feedback, feedbackErr = recommendationFeedback(st, effectiveTask, effectiveQuery, hardwareKey)
			if feedbackErr != nil {
				return fmt.Errorf("load recommendation history: %w", feedbackErr)
			}
		} else if !*quiet && !*common.jsonOut {
			fmt.Fprintf(os.Stderr, "warning: recommendation history unavailable: %v\n", stErr)
		}
	} else if !*noLearn && cfgErr != nil && !*quiet && !*common.jsonOut {
		fmt.Fprintf(os.Stderr, "warning: recommendation history unavailable: %v\n", cfgErr)
	}
	recs, hw, err := recommend.Recommend(ctx, recommend.Options{
		Query:    query,
		Task:     *task,
		Provider: *provider,
		Limit:    *limit,
		Hardware: hw,
		Feedback: feedback,
	})
	if err != nil {
		return err
	}
	if st != nil {
		if err := recordRecommendationHistory(st, effectiveTask, effectiveQuery, hardwareKey, recs, "shown", 0); err != nil {
			return fmt.Errorf("record shown recommendations: %w", err)
		}
	}
	if *download {
		if len(recs) == 0 {
			return fmt.Errorf("no recommendations for %q", effectiveQuery)
		}
		if *selectIndex < 1 || *selectIndex > len(recs) {
			return fmt.Errorf("--select must be between 1 and %d", len(recs))
		}
		selected := recs[*selectIndex-1]
		if st != nil {
			if err := recordRecommendationHistory(st, effectiveTask, effectiveQuery, hardwareKey, recs, "selected", selected.Index); err != nil {
				return fmt.Errorf("record selected recommendation: %w", err)
			}
			if err := recordRecommendationHistory(st, effectiveTask, effectiveQuery, hardwareKey, recs, "skipped", selected.Index); err != nil {
				return fmt.Errorf("record skipped recommendations: %w", err)
			}
		}
		if !*quiet && !*common.jsonOut {
			fmt.Fprintf(os.Stderr, "Selected %d: %s (%s, fit=%s)\n", selected.Index, selected.Name, selected.URI, selected.Fit)
		}
		downloadArgs := []string{
			"--config", *common.configPath,
			"--log-level", *common.logLevel,
			"--url", selected.URI,
			"--profile", "auto",
		}
		if *common.jsonOut {
			downloadArgs = append(downloadArgs, "--json")
		}
		if strings.TrimSpace(*dest) != "" {
			downloadArgs = append(downloadArgs, "--dest", *dest)
		}
		if *placeFlag {
			downloadArgs = append(downloadArgs, "--place")
		}
		if *summaryJSON {
			downloadArgs = append(downloadArgs, "--summary-json")
		}
		if *dryRun {
			downloadArgs = append(downloadArgs, "--dry-run")
		}
		if *quiet {
			downloadArgs = append(downloadArgs, "--quiet")
		}
		if *noResume {
			downloadArgs = append(downloadArgs, "--no-resume")
		}
		return handleDownload(ctx, downloadArgs)
	}
	return printRecommendations(recommendSummary{
		Query:           effectiveQuery,
		Task:            effectiveTask,
		Hardware:        hw,
		Recommendations: recs,
	}, *common.jsonOut)
}

func printRecommendations(summary recommendSummary, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	if len(summary.Recommendations) == 0 {
		fmt.Println("No matching model recommendations found.")
		return nil
	}
	budget := recommend.MemoryBudgetBytes(summary.Hardware)
	fmt.Printf("Hardware: %s/%s", summary.Hardware.OS, summary.Hardware.Arch)
	if summary.Hardware.RAMBytes > 0 {
		fmt.Printf("  RAM=%s", humanize.Bytes(uint64(summary.Hardware.RAMBytes)))
	}
	if summary.Hardware.VRAMBytes > 0 {
		fmt.Printf("  VRAM=%s", humanize.Bytes(uint64(summary.Hardware.VRAMBytes)))
	}
	if summary.Hardware.UnifiedMemory {
		fmt.Print("  unified-memory")
	}
	if budget > 0 {
		fmt.Printf("  usable-budget=%s", humanize.Bytes(uint64(budget)))
	}
	fmt.Println()
	fmt.Printf("Recommendations for %q (%s):\n", summary.Query, summary.Task)
	for _, rec := range summary.Recommendations {
		size := "-"
		if rec.Size > 0 {
			size = humanize.Bytes(uint64(rec.Size))
		}
		required := "-"
		if rec.EstimatedRequired > 0 {
			required = humanize.Bytes(uint64(rec.EstimatedRequired))
		}
		fmt.Printf("%2d. %-40s fit=%-9s score=%3d size=%-10s need=%s\n", rec.Index, trimDisplay(rec.Name, 40), rec.Fit, rec.Score, size, required)
		meta := []string{}
		if rec.ParameterCount != "" {
			meta = append(meta, rec.ParameterCount)
		}
		if rec.Quantization != "" {
			meta = append(meta, rec.Quantization)
		}
		if rec.FileType != "" {
			meta = append(meta, rec.FileType)
		}
		if len(meta) > 0 {
			fmt.Printf("    %s\n", strings.Join(meta, " · "))
		}
		if len(rec.Reasons) > 0 {
			fmt.Printf("    why: %s\n", strings.Join(rec.Reasons, "; "))
		}
		if len(rec.RuntimeHints) > 0 {
			hints := make([]string, 0, len(rec.RuntimeHints))
			for _, hint := range rec.RuntimeHints {
				value := hint.Runtime
				if hint.PlacementPreset != "" {
					value += " -> " + hint.PlacementPreset
				}
				hints = append(hints, value)
			}
			fmt.Printf("    runtimes: %s\n", strings.Join(hints, "; "))
		}
		fmt.Printf("    uri: %s\n", rec.URI)
		fmt.Printf("    download: %s\n", rec.DownloadCommand)
	}
	return nil
}

func gibToBytes(v float64) int64 {
	return int64(v * 1024 * 1024 * 1024)
}

func validateGiBOverride(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be a finite GiB value", name)
	}
	if value < 0 {
		return fmt.Errorf("%s must be non-negative", name)
	}
	if value > maxHardwareOverrideGiB {
		return fmt.Errorf("%s value %.2f is unreasonably large; max is %d GiB", name, value, maxHardwareOverrideGiB)
	}
	return nil
}

func recommendationHardwareKey(hw recommend.HardwareProfile) string {
	mem := hw.RAMBytes
	if hw.VRAMBytes > mem {
		mem = hw.VRAMBytes
	}
	gb := int64(0)
	if mem > 0 {
		gb = (mem + (1<<30 - 1)) >> 30
	}
	shared := "discrete"
	if hw.UnifiedMemory {
		shared = "unified"
	}
	return fmt.Sprintf("%s/%s/%dg/%s", strings.ToLower(hw.OS), strings.ToLower(hw.Arch), gb, shared)
}

func recommendationFeedback(st *state.DB, task, query, hardwareKey string) (map[string]recommend.Feedback, error) {
	rows, err := st.RecommendationHistoryFor(task, query, hardwareKey)
	if err != nil {
		return nil, err
	}
	out := make(map[string]recommend.Feedback)
	for _, row := range rows {
		key := recommend.FeedbackKey(row.URI)
		fb := out[key]
		switch row.Action {
		case "selected":
			fb.Selected += row.Count
		case "skipped":
			fb.Skipped += row.Count
		case "shown":
			fb.Shown += row.Count
		}
		out[key] = fb
	}
	return out, nil
}

func recordRecommendationHistory(st *state.DB, task, query, hardwareKey string, recs []recommend.Recommendation, action string, selectedIndex int) error {
	rows := make([]state.RecommendationHistoryRow, 0, len(recs))
	for _, rec := range recs {
		switch action {
		case "selected":
			if rec.Index != selectedIndex {
				continue
			}
		case "skipped":
			if rec.Index == selectedIndex {
				continue
			}
		}
		rows = append(rows, state.RecommendationHistoryRow{
			Task:        task,
			Query:       query,
			Provider:    rec.Provider,
			ModelID:     rec.ModelID,
			URI:         rec.URI,
			Action:      action,
			Score:       rec.Score,
			Fit:         rec.Fit,
			HardwareKey: hardwareKey,
		})
	}
	return st.BatchUpsertRecommendationHistory(rows)
}

func printRecommendationHistory(cfg *config.Config, limit int, jsonOut bool) error {
	st, err := state.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	rows, err := st.ListRecommendationHistory(limit)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	for _, row := range rows {
		fmt.Printf("%s\t%s\t%s\tcount=%d\tfit=%s\tscore=%d\t%s\n", row.Task, row.Action, row.HardwareKey, row.Count, row.Fit, row.Score, row.URI)
	}
	return nil
}
