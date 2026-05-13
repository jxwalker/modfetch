package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/jxwalker/modfetch/internal/discovery"
	"github.com/jxwalker/modfetch/internal/recommend"
)

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
	flagArgs, queryArgs := splitDiscoverArgs(args, map[string]bool{
		"json": true, "unified-memory": true, "download": true, "place": true, "summary-json": true, "dry-run": true, "quiet": true, "no-resume": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	hw := recommend.DetectHardware(ctx)
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
	recs, hw, err := recommend.Recommend(ctx, recommend.Options{
		Query:    query,
		Task:     *task,
		Provider: *provider,
		Limit:    *limit,
		Hardware: hw,
	})
	if err != nil {
		return err
	}
	effectiveTask := recommend.NormalizeTask(*task)
	effectiveQuery := query
	if effectiveQuery == "" {
		effectiveQuery = recommend.DefaultQuery(effectiveTask)
	}
	if *download {
		if len(recs) == 0 {
			return fmt.Errorf("no recommendations for %q", effectiveQuery)
		}
		if *selectIndex < 1 || *selectIndex > len(recs) {
			return fmt.Errorf("--select must be between 1 and %d", len(recs))
		}
		selected := recs[*selectIndex-1]
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
		fmt.Printf("    uri: %s\n", rec.URI)
		fmt.Printf("    download: %s\n", rec.DownloadCommand)
	}
	return nil
}

func gibToBytes(v float64) int64 {
	return int64(v * 1024 * 1024 * 1024)
}
