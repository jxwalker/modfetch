package tui

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

// setupTestModel creates a basic test model for navigation testing
func setupTestModel(t *testing.T) (*Model, *state.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cfg := &config.Config{
		General: config.General{
			DownloadRoot: tmpDir,
		},
	}

	model := New(cfg, db, "test-version").(*Model)
	model.w = 120
	model.h = 40

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	}

	return model, db, cleanup
}

// TestNavigation_TabSwitching tests switching between all tabs
func TestNavigation_TabSwitching(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	tests := []struct {
		name        string
		key         string
		expectedTab int
	}{
		{"View All (key 0)", "0", -1},
		{"Pending (key 1)", "1", 0},
		{"Active (key 2)", "2", 1},
		{"Completed (key 3)", "3", 2},
		{"Failed (key 4)", "4", 3},
		{"Library (key 5)", "5", 4},
		{"Library (key l)", "l", 4},
		{"Settings (key 6)", "6", 5},
		{"Settings (key m)", "m", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updatedModel, _ := model.Update(msg)
			m := updatedModel.(*Model)

			if m.activeTab != tt.expectedTab {
				t.Errorf("Expected activeTab=%d, got %d", tt.expectedTab, m.activeTab)
			}
		})
	}
}

// TestNavigation_LibraryNavigation tests library-specific navigation
func TestNavigation_LibraryNavigation(t *testing.T) {
	model, db, cleanup := setupTestModel(t)
	defer cleanup()

	// Add test data
	createTestMetadata(t, db, 5)

	// Switch to library tab
	model.activeTab = 4
	model.refreshLibraryData()

	if len(model.libraryRows) != 5 {
		t.Fatalf("Expected 5 library rows, got %d", len(model.libraryRows))
	}

	t.Run("Navigate down with j key", func(t *testing.T) {
		initialSelected := model.librarySelected
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySelected != initialSelected+1 {
			t.Errorf("Expected librarySelected=%d, got %d", initialSelected+1, m.librarySelected)
		}
	})

	t.Run("Navigate down with down arrow", func(t *testing.T) {
		initialSelected := model.librarySelected
		msg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySelected != initialSelected+1 {
			t.Errorf("Expected librarySelected=%d, got %d", initialSelected+1, m.librarySelected)
		}
	})

	t.Run("Navigate up with k key", func(t *testing.T) {
		model.librarySelected = 2 // Start in middle
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySelected != 1 {
			t.Errorf("Expected librarySelected=1, got %d", m.librarySelected)
		}
	})

	t.Run("Navigate up with up arrow", func(t *testing.T) {
		model.librarySelected = 2 // Start in middle
		msg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySelected != 1 {
			t.Errorf("Expected librarySelected=1, got %d", m.librarySelected)
		}
	})

	t.Run("Cannot navigate below zero", func(t *testing.T) {
		model.librarySelected = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySelected != 0 {
			t.Errorf("Expected librarySelected=0, got %d", m.librarySelected)
		}
	})

	t.Run("Cannot navigate beyond max", func(t *testing.T) {
		model.librarySelected = 4 // Last item (5 total)
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySelected != 4 {
			t.Errorf("Expected librarySelected=4, got %d", m.librarySelected)
		}
	})
}

// TestNavigation_LibraryDetailView tests entering and exiting detail view
func TestNavigation_LibraryDetailView(t *testing.T) {
	model, db, cleanup := setupTestModel(t)
	defer cleanup()

	// Add test data
	createTestMetadata(t, db, 3)

	// Switch to library tab
	model.activeTab = 4
	model.refreshLibraryData()

	t.Run("Enter detail view with Enter key", func(t *testing.T) {
		model.librarySelected = 0
		model.libraryViewingDetail = false

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if !m.libraryViewingDetail {
			t.Error("Expected libraryViewingDetail=true after Enter")
		}
		if m.libraryDetailModel == nil {
			t.Error("Expected libraryDetailModel to be initialized")
		}
	})

	t.Run("Exit detail view with Escape key", func(t *testing.T) {
		model.libraryViewingDetail = true
		model.libraryDetailModel = &state.ModelMetadata{}

		msg := tea.KeyMsg{Type: tea.KeyEsc}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.libraryViewingDetail {
			t.Error("Expected libraryViewingDetail=false after Escape")
		}
		if m.libraryDetailModel != nil {
			t.Error("Expected libraryDetailModel to be nil after exit")
		}
	})
}

// TestNavigation_LibrarySearch tests library search activation and cancellation
func TestNavigation_LibrarySearch(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	// Switch to library tab
	model.activeTab = 4

	t.Run("Activate search with / key", func(t *testing.T) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if !m.librarySearchActive {
			t.Error("Expected librarySearchActive=true after / key")
		}
	})

	t.Run("Cancel search with Escape", func(t *testing.T) {
		model.librarySearchActive = true
		model.librarySearchInput = textinput.New()

		msg := tea.KeyMsg{Type: tea.KeyEsc}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.librarySearchActive {
			t.Error("Expected librarySearchActive=false after Escape")
		}
	})
}

// TestNavigation_HelpToggle tests help menu toggle
func TestNavigation_HelpToggle(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	t.Run("Show help with ? key", func(t *testing.T) {
		model.showHelp = false
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if !m.showHelp {
			t.Error("Expected showHelp=true after ? key")
		}
	})

	t.Run("Hide help with ? key toggle", func(t *testing.T) {
		model.showHelp = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if m.showHelp {
			t.Error("Expected showHelp=false after ? key toggle")
		}
	})
}

// TestNavigation_InspectorToggle tests inspector panel toggle
func TestNavigation_InspectorToggle(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	t.Run("Toggle inspector with i key", func(t *testing.T) {
		model.showInspector = false
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*Model)

		if !m.showInspector {
			t.Error("Expected showInspector=true after i key")
		}

		// Toggle again
		updatedModel, _ = m.Update(msg)
		m = updatedModel.(*Model)

		if m.showInspector {
			t.Error("Expected showInspector=false after second i key")
		}
	})
}

// TestNavigation_DownloadTabs tests navigation between download status tabs
func TestNavigation_DownloadTabs(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	tests := []struct {
		name        string
		startTab    int
		key         string
		expectedTab int
	}{
		{"From Library to Pending", 4, "1", 0},
		{"From Settings to Active", 5, "2", 1},
		{"From Pending to Completed", 0, "3", 2},
		{"From Active to Failed", 1, "4", 3},
		{"From any tab to Library with l", 2, "l", 4},
		{"From any tab to Settings with m", 3, "m", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.activeTab = tt.startTab
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updatedModel, _ := model.Update(msg)
			m := updatedModel.(*Model)

			if m.activeTab != tt.expectedTab {
				t.Errorf("Expected activeTab=%d, got %d", tt.expectedTab, m.activeTab)
			}
		})
	}
}

// TestNavigation_SelectionReset tests that selection resets when changing tabs
func TestNavigation_SelectionReset(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	model.selected = 5
	model.activeTab = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*Model)

	if m.selected != 0 {
		t.Errorf("Expected selected=0 after tab switch, got %d", m.selected)
	}
}

// TestNavigation_QuitKey tests quitting the application
func TestNavigation_QuitKey(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{"Ctrl+C", tea.KeyMsg{Type: tea.KeyCtrlC}},
		{"q key", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := model.Update(tt.msg)

			// The cmd should be tea.Quit
			if cmd == nil {
				t.Error("Expected quit command, got nil")
			}
		})
	}
}

// TestNavigation_WindowResize tests window resize handling
func TestNavigation_WindowResize(t *testing.T) {
	model, _, cleanup := setupTestModel(t)
	defer cleanup()

	msg := tea.WindowSizeMsg{Width: 200, Height: 50}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*Model)

	if m.w != 200 {
		t.Errorf("Expected width=200, got %d", m.w)
	}
	if m.h != 50 {
		t.Errorf("Expected height=50, got %d", m.h)
	}
}
