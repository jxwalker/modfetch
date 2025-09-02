package tui2

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"modfetch/internal/config"
	"modfetch/internal/state"
)

func newTestModel() *Model {
	cfg := &config.Config{Version: 1}
	cfg.General.DataRoot = "/tmp" // unused in these tests
	m := &Model{
		cfg:       cfg,
		th:        defaultTheme(),
		activeTab: -1,
		prog:      progress.New(progress.WithDefaultGradient(), progress.WithWidth(16)),
	}
	return m
}

func TestInspectorCompletedShowsAvgSpeed(t *testing.T) {
	m := newTestModel()
	now := time.Now().Unix()
	row := state.DownloadRow{
		URL:       "https://example.com/file",
		Dest:      "/dest/file",
		Size:      1024 * 1024,
		Status:    "complete",
		CreatedAt: now - 10,
		UpdatedAt: now,
	}
	m.rows = []state.DownloadRow{row}
	m.selected = 0
	out := m.renderInspector()
	if !strings.Contains(out, "Avg Speed:") {
		t.Fatalf("expected inspector to include Avg Speed for completed job; got:\n%s", out)
	}
}

func TestInspectorRunningShowsStarted(t *testing.T) {
	m := newTestModel()
	now := time.Now().Unix()
	row := state.DownloadRow{
		URL:       "https://example.com/file",
		Dest:      "/dest/file",
		Size:      2048,
		Status:    "running",
		CreatedAt: now - 5,
		UpdatedAt: now - 1,
	}
	m.rows = []state.DownloadRow{row}
	m.selected = 0
	out := m.renderInspector()
	if !strings.Contains(out, "Started:") {
		t.Fatalf("expected inspector to include Started time for running job; got:\n%s", out)
	}
}
