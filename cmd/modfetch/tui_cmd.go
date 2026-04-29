package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/tui"
	cw "github.com/jxwalker/modfetch/internal/tui/configwizard"
	"gopkg.in/yaml.v3"
)

func handleTUI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "JSON snapshot output when --snapshot is set")
	snapshot := fs.Bool("snapshot", false, "print a non-interactive TUI state snapshot and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedCfgPath, err := resolveConfigPath(*common.configPath)
	if err != nil {
		return err
	}
	// Try to load config. If not found, offer to create via wizard with sensible defaults.
	c, err := config.Load(resolvedCfgPath)
	if err != nil {
		if *snapshot {
			if os.IsNotExist(err) {
				return fmt.Errorf("config file not found: %s: %w", resolvedCfgPath, err)
			}
			return err
		}
		if os.IsNotExist(err) {
			// Create directory and run wizard
			if err := os.MkdirAll(filepath.Dir(resolvedCfgPath), 0o755); err != nil {
				return err
			}
			defaults := &config.Config{Version: 1, General: config.General{DataRoot: "~/modfetch/data", DownloadRoot: "~/modfetch/downloads", PlacementMode: "symlink"}, Concurrency: config.Concurrency{ChunkSizeMB: 8, PerFileChunks: 4}}
			wiz := cw.New(defaults)
			p := tea.NewProgram(wiz)
			m, werr := p.Run()
			if werr != nil {
				return werr
			}
			w, ok := m.(*cw.Wizard)
			if !ok {
				return errors.New("unexpected wizard model")
			}
			cfg := w.Config()
			if cfg == nil {
				return errors.New("config wizard was cancelled")
			}
			b, merr := yaml.Marshal(cfg)
			if merr != nil {
				return merr
			}
			if err := os.WriteFile(resolvedCfgPath, b, 0o644); err != nil {
				return err
			}
			fmt.Printf("wrote config to %s\n", resolvedCfgPath)
			// Reload expanded config
			c, err = config.Load(resolvedCfgPath)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	_ = logging.New(*common.logLevel, *common.jsonOut) // placeholder for future log routing
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer func() { _ = st.SQL.Close() }()

	if *snapshot {
		snap, err := buildTUISnapshot(c, st)
		if err != nil {
			return err
		}
		if *common.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		}
		printTUISnapshot(snap)
		return nil
	}

	m := tui.New(c, st, version)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

type tuiSnapshot struct {
	Version   string             `json:"version"`
	Config    tuiSnapshotConfig  `json:"config"`
	Downloads tuiDownloadSummary `json:"downloads"`
	Library   tuiLibrarySummary  `json:"library"`
}

type tuiSnapshotConfig struct {
	DataRoot      string `json:"data_root"`
	DownloadRoot  string `json:"download_root"`
	PlacementMode string `json:"placement_mode"`
}

type tuiDownloadSummary struct {
	Total     int            `json:"total"`
	ByStatus  map[string]int `json:"by_status"`
	Active    int            `json:"active"`
	Pending   int            `json:"pending"`
	Completed int            `json:"completed"`
	Failed    int            `json:"failed"`
	ErrorLike int            `json:"error_like"`
}

type tuiLibrarySummary struct {
	Total     int            `json:"total"`
	Favorites int            `json:"favorites"`
	BySource  map[string]int `json:"by_source"`
	ByType    map[string]int `json:"by_type"`
}

func buildTUISnapshot(c *config.Config, st *state.DB) (tuiSnapshot, error) {
	rows, err := st.ListDownloads()
	if err != nil {
		return tuiSnapshot{}, fmt.Errorf("list downloads: %w", err)
	}
	metadata, err := st.ListMetadata(state.MetadataFilters{})
	if err != nil {
		return tuiSnapshot{}, fmt.Errorf("list library metadata: %w", err)
	}

	snap := tuiSnapshot{
		Version: version,
		Config: tuiSnapshotConfig{
			DataRoot:      c.General.DataRoot,
			DownloadRoot:  c.General.DownloadRoot,
			PlacementMode: c.General.PlacementMode,
		},
		Downloads: tuiDownloadSummary{
			Total:    len(rows),
			ByStatus: map[string]int{},
		},
		Library: tuiLibrarySummary{
			Total:    len(metadata),
			BySource: map[string]int{},
			ByType:   map[string]int{},
		},
	}

	for _, row := range rows {
		status := normalizeSnapshotToken(row.Status)
		if status == "" {
			status = "unknown"
		}
		snap.Downloads.ByStatus[status]++
		switch classifyTUIDownloadStatus(status) {
		case "active":
			snap.Downloads.Active++
		case "pending":
			snap.Downloads.Pending++
		case "completed":
			snap.Downloads.Completed++
		case "failed":
			snap.Downloads.Failed++
		}
		if isTUIErrorLikeStatus(status) {
			snap.Downloads.ErrorLike++
		}
	}

	for _, meta := range metadata {
		if meta.Favorite {
			snap.Library.Favorites++
		}
		source := normalizeSnapshotToken(meta.Source)
		if source == "" {
			source = "unknown"
		}
		snap.Library.BySource[source]++
		modelType := normalizeSnapshotToken(meta.ModelType)
		if modelType == "" {
			modelType = "unknown"
		}
		snap.Library.ByType[modelType]++
	}

	return snap, nil
}

func classifyTUIDownloadStatus(status string) string {
	switch normalizeSnapshotToken(status) {
	case "running", "downloading", "active", "retrying":
		return "active"
	case "pending", "queued", "planning":
		return "pending"
	case "complete", "completed", "verified":
		return "completed"
	case "error", "failed", "checksum_mismatch", "verify_failed", "canceled", "cancelled":
		return "failed"
	default:
		return ""
	}
}

func isTUIErrorLikeStatus(status string) bool {
	switch normalizeSnapshotToken(status) {
	case "error", "failed", "checksum_mismatch", "verify_failed":
		return true
	default:
		return false
	}
}

func normalizeSnapshotToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func printTUISnapshot(snap tuiSnapshot) {
	fmt.Printf("TUI snapshot: downloads=%d active=%d pending=%d completed=%d failed=%d error_like=%d library=%d favorites=%d\n",
		snap.Downloads.Total,
		snap.Downloads.Active,
		snap.Downloads.Pending,
		snap.Downloads.Completed,
		snap.Downloads.Failed,
		snap.Downloads.ErrorLike,
		snap.Library.Total,
		snap.Library.Favorites,
	)
	fmt.Printf("Config: data_root=%s download_root=%s placement_mode=%s\n",
		snap.Config.DataRoot,
		snap.Config.DownloadRoot,
		snap.Config.PlacementMode,
	)
	fmt.Printf("Download statuses: %s\n", formatSnapshotCounts(snap.Downloads.ByStatus))
	fmt.Printf("Library sources: %s\n", formatSnapshotCounts(snap.Library.BySource))
	fmt.Printf("Library types: %s\n", formatSnapshotCounts(snap.Library.ByType))
}

func formatSnapshotCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, " ")
}
