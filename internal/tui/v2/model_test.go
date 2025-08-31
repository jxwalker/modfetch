package tui2

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

func TestUpdateNormalQuestion(t *testing.T) {
	m := &Model{}
	m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Alt: false})
	if !m.showHelp {
		t.Fatalf("showHelp should be true after '?' key")
	}
}
