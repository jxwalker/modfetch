package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/jxwalker/modfetch/internal/scanner"
	"github.com/jxwalker/modfetch/internal/state"
)

// Library view rendering and interaction

func (m *Model) refreshLibraryData() {
	if m.st == nil {
		return
	}

	// Build filters based on current library filter state
	filters := state.MetadataFilters{
		Source:    m.libraryFilterSource,
		ModelType: m.libraryFilterType,
		Favorite:  m.libraryShowFavorites,
		OrderBy:   "updated_at", // Most recently updated first
		Limit:     1000,         // Reasonable limit
	}

	var rows []state.ModelMetadata
	var err error

	if m.librarySearch != "" {
		// If searching, use search function
		rows, err = m.st.SearchMetadata(m.librarySearch)
	} else {
		// Otherwise use filtered list
		rows, err = m.st.ListMetadata(filters)
	}

	if err != nil {
		if m.log != nil {
			m.log.Errorf("failed to load library data: %v", err)
		}
		return
	}

	m.libraryRows = rows
	m.libraryNeedsRefresh = false

	// Reset selection if out of bounds
	if m.librarySelected >= len(m.libraryRows) {
		m.librarySelected = 0
	}
}

// renderLibrary renders the library view showing downloaded models with metadata

func (m *Model) renderLibrary() string {
	if m.libraryViewingDetail && m.libraryDetailModel != nil {
		return m.renderLibraryDetail()
	}

	var sb strings.Builder

	// Header with filter status
	header := m.th.head.Render("Model Library")
	if m.librarySearch != "" {
		header += m.th.label.Render(fmt.Sprintf(" • Search: %q", m.librarySearch))
	}
	if m.libraryFilterType != "" {
		header += m.th.label.Render(fmt.Sprintf(" • Type: %s", m.libraryFilterType))
	}
	if m.libraryFilterSource != "" {
		header += m.th.label.Render(fmt.Sprintf(" • Source: %s", m.libraryFilterSource))
	}
	if m.libraryShowFavorites {
		header += m.th.ok.Render(" • ★ Favorites")
	}
	sb.WriteString(header + "\n\n")

	if len(m.libraryRows) == 0 {
		sb.WriteString(m.th.label.Render("No models found in library.\n"))
		sb.WriteString(m.th.label.Render("Download some models to see them here!\n\n"))
		sb.WriteString(m.th.footer.Render("Press 1-4 to view downloads, 5 or L for library"))
		return sb.String()
	}

	// Calculate available height for list
	headerLines := 3 // Header + filter info + blank line
	footerLines := 3 // Help text
	availableHeight := m.h - headerLines - footerLines
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Calculate pagination
	start := m.librarySelected
	if start > len(m.libraryRows)-availableHeight {
		start = len(m.libraryRows) - availableHeight
	}
	if start < 0 {
		start = 0
	}
	end := start + availableHeight
	if end > len(m.libraryRows) {
		end = len(m.libraryRows)
	}

	// Render model list
	for i := start; i < end; i++ {
		model := m.libraryRows[i]
		style := m.th.row
		cursor := "  "

		if i == m.librarySelected {
			style = m.th.rowSelected
			cursor = "▶ "
		}

		// Format: [★] ModelName (Type) • Size • Quantization • Source
		line := cursor

		// Favorite indicator
		if model.Favorite {
			line += m.th.ok.Render("★ ")
		}

		// Model name (truncate if needed)
		name := model.ModelName
		if name == "" {
			name = model.ModelID
		}
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		line += style.Bold(true).Render(name)

		// Model type
		if model.ModelType != "" {
			line += m.th.label.Render(fmt.Sprintf(" (%s)", model.ModelType))
		}

		// File size
		if model.FileSize > 0 {
			line += m.th.label.Render(fmt.Sprintf(" • %s", humanize.Bytes(uint64(model.FileSize))))
		}

		// Quantization
		if model.Quantization != "" {
			line += m.th.label.Render(fmt.Sprintf(" • %s", model.Quantization))
		}

		// Source
		sourceColor := m.th.label
		if model.Source == "huggingface" {
			sourceColor = m.th.ok
		} else if model.Source == "civitai" {
			sourceColor = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
		}
		line += sourceColor.Render(fmt.Sprintf(" • %s", model.Source))

		sb.WriteString(style.Render(line) + "\n")
	}

	// Footer with help
	sb.WriteString("\n")
	sb.WriteString(m.th.footer.Render(fmt.Sprintf("Showing %d-%d of %d models | ↑↓ navigate • Enter view details • / search • F filter • Q quit",
		start+1, end, len(m.libraryRows))))

	return sb.String()
}

// renderLibraryDetail renders detailed view of a single model

func (m *Model) renderLibraryDetail() string {
	if m.libraryDetailModel == nil {
		return "No model selected"
	}

	model := m.libraryDetailModel
	var sb strings.Builder

	// Title
	title := model.ModelName
	if title == "" {
		title = model.ModelID
	}
	sb.WriteString(m.th.head.Render(title) + "\n\n")

	// Basic info section
	sb.WriteString(m.th.label.Render("Type: "))
	sb.WriteString(model.ModelType + "\n")

	if model.Version != "" {
		sb.WriteString(m.th.label.Render("Version: "))
		sb.WriteString(model.Version + "\n")
	}

	if model.Source != "" {
		sb.WriteString(m.th.label.Render("Source: "))
		sb.WriteString(model.Source + "\n")
	}

	if model.Author != "" {
		sb.WriteString(m.th.label.Render("Author: "))
		sb.WriteString(model.Author + "\n")
	}

	if model.License != "" {
		sb.WriteString(m.th.label.Render("License: "))
		sb.WriteString(model.License + "\n")
	}

	sb.WriteString("\n")

	// Model specs
	if model.Quantization != "" || model.Architecture != "" || model.ParameterCount != "" {
		sb.WriteString(m.th.head.Render("Specifications") + "\n")

		if model.Architecture != "" {
			sb.WriteString(m.th.label.Render("Architecture: "))
			sb.WriteString(model.Architecture + "\n")
		}

		if model.ParameterCount != "" {
			sb.WriteString(m.th.label.Render("Parameters: "))
			sb.WriteString(model.ParameterCount + "\n")
		}

		if model.Quantization != "" {
			sb.WriteString(m.th.label.Render("Quantization: "))
			sb.WriteString(model.Quantization + "\n")
		}

		if model.BaseModel != "" {
			sb.WriteString(m.th.label.Render("Base Model: "))
			sb.WriteString(model.BaseModel + "\n")
		}

		sb.WriteString("\n")
	}

	// File info
	sb.WriteString(m.th.head.Render("File Information") + "\n")
	if model.FileSize > 0 {
		sb.WriteString(m.th.label.Render("Size: "))
		sb.WriteString(humanize.Bytes(uint64(model.FileSize)) + "\n")
	}
	if model.FileFormat != "" {
		sb.WriteString(m.th.label.Render("Format: "))
		sb.WriteString(model.FileFormat + "\n")
	}
	if model.Dest != "" {
		sb.WriteString(m.th.label.Render("Location: "))
		sb.WriteString(model.Dest + "\n")
	}
	sb.WriteString("\n")

	// Description
	if model.Description != "" {
		sb.WriteString(m.th.head.Render("Description") + "\n")
		// Wrap description text
		desc := model.Description
		if len(desc) > 500 {
			desc = desc[:497] + "..."
		}
		sb.WriteString(desc + "\n\n")
	}

	// Tags
	if len(model.Tags) > 0 {
		sb.WriteString(m.th.head.Render("Tags") + "\n")
		sb.WriteString(strings.Join(model.Tags, ", ") + "\n\n")
	}

	// Usage stats
	if model.TimesUsed > 0 || model.DownloadCount > 0 {
		sb.WriteString(m.th.head.Render("Usage Statistics") + "\n")
		if model.DownloadCount > 0 {
			sb.WriteString(m.th.label.Render("Downloads: "))
			sb.WriteString(fmt.Sprintf("%d\n", model.DownloadCount))
		}
		if model.TimesUsed > 0 {
			sb.WriteString(m.th.label.Render("Times Used: "))
			sb.WriteString(fmt.Sprintf("%d\n", model.TimesUsed))
		}
		if model.LastUsed != nil {
			sb.WriteString(m.th.label.Render("Last Used: "))
			sb.WriteString(model.LastUsed.Format("2006-01-02 15:04") + "\n")
		}
		sb.WriteString("\n")
	}

	// User data
	if model.UserRating > 0 || model.Favorite || model.UserNotes != "" {
		sb.WriteString(m.th.head.Render("User Data") + "\n")
		if model.UserRating > 0 {
			stars := strings.Repeat("★", model.UserRating) + strings.Repeat("☆", 5-model.UserRating)
			sb.WriteString(m.th.label.Render("Rating: "))
			sb.WriteString(m.th.ok.Render(stars) + "\n")
		}
		if model.Favorite {
			sb.WriteString(m.th.ok.Render("★ Favorite\n"))
		}
		if model.UserNotes != "" {
			sb.WriteString(m.th.label.Render("Notes: "))
			sb.WriteString(model.UserNotes + "\n")
		}
		sb.WriteString("\n")
	}

	// Links
	if model.RepoURL != "" || model.HomepageURL != "" {
		sb.WriteString(m.th.head.Render("Links") + "\n")
		if model.HomepageURL != "" {
			sb.WriteString(m.th.label.Render("Homepage: "))
			sb.WriteString(model.HomepageURL + "\n")
		}
		if model.RepoURL != "" {
			sb.WriteString(m.th.label.Render("Repository: "))
			sb.WriteString(model.RepoURL + "\n")
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString(m.th.footer.Render("Press Esc to go back • F to toggle favorite • Q to quit"))

	return sb.String()
}

// renderSettings displays current configuration

func (m *Model) updateLibrarySearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	switch s {
	case "esc":
		m.librarySearchActive = false
		m.librarySearchInput.Blur()
		m.librarySearch = ""
		m.libraryNeedsRefresh = true
		m.refreshLibraryData()
		return m, nil
	case "enter", "ctrl+j":
		m.librarySearchActive = false
		m.librarySearchInput.Blur()
		m.librarySearch = strings.TrimSpace(m.librarySearchInput.Value())
		m.libraryNeedsRefresh = true
		m.refreshLibraryData()
		return m, nil
	}

	var cmd tea.Cmd
	m.librarySearchInput, cmd = m.librarySearchInput.Update(msg)
	return m, cmd
}

// scanDirectoriesCmd initiates a directory scan for models

func (m *Model) scanDirectoriesCmd() tea.Cmd {
	return func() tea.Msg {
		if m.st == nil || m.cfg == nil {
			return scanCompleteMsg{err: fmt.Errorf("database or config not available")}
		}

		// Get directories to scan from config
		dirs := []string{}

		// Add download root
		if m.cfg.General.DownloadRoot != "" {
			dirs = append(dirs, m.cfg.General.DownloadRoot)
		}

		// Add placement app base directories and subdirectories
		for _, app := range m.cfg.Placement.Apps {
			if app.Base != "" {
				dirs = append(dirs, app.Base)
			}
			for _, path := range app.Paths {
				if path != "" {
					fullPath := filepath.Join(app.Base, path)
					dirs = append(dirs, fullPath)
				}
			}
		}

		if len(dirs) == 0 {
			return scanCompleteMsg{err: fmt.Errorf("no directories configured to scan")}
		}

		// Perform scan
		scanner := scanner.NewScanner(m.st)
		result, err := scanner.ScanDirectories(dirs)

		if err != nil {
			return scanCompleteMsg{err: err, result: result}
		}

		return scanCompleteMsg{result: result}
	}
}

// scanCompleteMsg is sent when directory scan completes
type scanCompleteMsg struct {
	result *scanner.ScanResult
	err    error
}
