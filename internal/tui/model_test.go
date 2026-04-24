package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestUpdateNewJobEsc(t *testing.T) {
	m := &Model{newJob: true, newStep: 1, newInput: textinput.New()}
	m.updateNewJob(tea.KeyMsg{Type: tea.KeyEsc})
	if m.newJob {
		t.Fatalf("newJob should be false after esc")
	}
}

func TestUpdateBatchModeEsc(t *testing.T) {
	m := &Model{batchMode: true, batchInput: textinput.New()}
	m.updateBatchMode(tea.KeyMsg{Type: tea.KeyEsc})
	if m.batchMode {
		t.Fatalf("batchMode should be false after esc")
	}
}

func TestUpdateFilterEsc(t *testing.T) {
	m := &Model{filterOn: true, filterInput: textinput.New()}
	m.updateFilter(tea.KeyMsg{Type: tea.KeyEsc})
	if m.filterOn {
		t.Fatalf("filterOn should be false after esc")
	}
}

func TestRestoreVisibleSelectionPreservesSelectedRowWhenStillVisible(t *testing.T) {
	fi := textinput.New()
	fi.CharLimit = 4096
	m := &Model{
		activeTab: -1,
		selected:  2,
		rows: []state.DownloadRow{
			{URL: "https://example.com/alpha", Dest: "alpha.gguf", Status: "running"},
			{URL: "https://example.com/beta", Dest: "beta.gguf", Status: "running"},
			{URL: "https://example.com/gamma", Dest: "gamma.gguf", Status: "running"},
		},
		filterOn:    true,
		filterInput: fi,
	}
	m.filterInput.Focus()

	selectedKey := m.currentVisibleKey()
	m.filterInput.SetValue("gamma")
	m.restoreVisibleSelection(selectedKey)

	rows := m.visibleRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 visible row after filtering %q, got %d", m.filterInput.Value(), len(rows))
	}
	if m.selected != 0 {
		t.Fatalf("expected selected index to follow gamma to 0, got %d", m.selected)
	}
	if rows[m.selected].Dest != "gamma.gguf" {
		t.Fatalf("expected gamma to remain selected, got %q", rows[m.selected].Dest)
	}
}

func TestUpdateFilterEscPreservesSelectedRowWhenClearingFilter(t *testing.T) {
	fi := textinput.New()
	fi.CharLimit = 4096
	fi.SetValue("gam")
	fi.Focus()
	m := &Model{
		activeTab: -1,
		selected:  0,
		rows: []state.DownloadRow{
			{URL: "https://example.com/alpha", Dest: "alpha.gguf", Status: "running"},
			{URL: "https://example.com/beta", Dest: "beta.gguf", Status: "running"},
			{URL: "https://example.com/gamma", Dest: "gamma.gguf", Status: "running"},
		},
		filterOn:    true,
		filterInput: fi,
	}

	m.updateFilter(tea.KeyMsg{Type: tea.KeyEsc})

	if m.selected != 2 {
		t.Fatalf("expected selected index to follow gamma back to 2, got %d", m.selected)
	}
	if rows := m.visibleRows(); rows[m.selected].Dest != "gamma.gguf" {
		t.Fatalf("expected gamma to remain selected, got %q", rows[m.selected].Dest)
	}
}

func TestComputeCurAndTotalPlanningIgnoresPreallocatedPart(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := state.NewDB(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer func() { _ = db.Close() }()

	dest := filepath.Join(tmpDir, "model.gguf")
	part := dest + ".part"
	if err := os.WriteFile(part, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write part: %v", err)
	}

	m := &Model{
		cfg: &config.Config{General: config.General{DownloadRoot: tmpDir}},
		st:  db,
	}
	cur, total := m.computeCurAndTotal(state.DownloadRow{
		URL:    "https://example.com/model.gguf",
		Dest:   dest,
		Size:   1024,
		Status: "planning",
	})

	if cur != 0 {
		t.Fatalf("planning row should report 0 current bytes, got %d", cur)
	}
	if total != 1024 {
		t.Fatalf("expected total size 1024, got %d", total)
	}
}

func TestUpdateNormalQuestion(t *testing.T) {
	m := &Model{}
	m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Alt: false})
	if !m.showHelp {
		t.Fatalf("showHelp should be true after '?' key")
	}
}
