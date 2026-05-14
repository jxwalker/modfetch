package recommend

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jxwalker/modfetch/internal/state"
)

func HardwareKey(hw HardwareProfile) string {
	ramGB := RoundedGiB(hw.RAMBytes)
	vramGB := RoundedGiB(hw.VRAMBytes)
	switch {
	case hw.UnifiedMemory:
		return fmt.Sprintf("%s/%s/ram%dg/unified", strings.ToLower(hw.OS), strings.ToLower(hw.Arch), ramGB)
	case hw.VRAMBytes > 0:
		return fmt.Sprintf("%s/%s/vram%dg-ram%dg/discrete", strings.ToLower(hw.OS), strings.ToLower(hw.Arch), vramGB, ramGB)
	default:
		return fmt.Sprintf("%s/%s/ram%dg/system", strings.ToLower(hw.OS), strings.ToLower(hw.Arch), ramGB)
	}
}

func RoundedGiB(bytes int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return (bytes + (1<<30 - 1)) >> 30
}

func FeedbackFromHistory(st *state.DB, task, query, hardwareKey string) (map[string]Feedback, error) {
	if st == nil {
		return nil, errors.New("nil state db")
	}
	rows, err := st.RecommendationHistoryFor(task, query, hardwareKey)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Feedback)
	for _, row := range rows {
		key := FeedbackKey(row.URI)
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

func RecordHistory(st *state.DB, task, query, hardwareKey string, recs []Recommendation, action string, selectedIndex int) error {
	if st == nil {
		return errors.New("nil state db")
	}
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "selected", "skipped", "shown":
	default:
		return fmt.Errorf("unsupported recommendation history action %q", action)
	}
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
