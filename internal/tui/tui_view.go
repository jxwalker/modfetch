package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/jxwalker/modfetch/internal/state"
)

// TUIView handles all rendering logic and UI presentation for the TUI.
type TUIView struct {
	styles uiStyles
	prog   progress.Model
	width  int
	height int
}

type uiStyles struct {
	header lipgloss.Style
	row    lipgloss.Style
	sel    lipgloss.Style
	border lipgloss.Style
	title  lipgloss.Style
	footer lipgloss.Style
}

// NewTUIView creates a new TUIView instance with default styling and progress bar.
func NewTUIView() *TUIView {
	p := progress.New(progress.WithDefaultGradient())
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("63"))
	styles := uiStyles{
		header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		row:    lipgloss.NewStyle(),
		sel:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
		border: border,
		title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		footer: lipgloss.NewStyle().Faint(true),
	}
	return &TUIView{
		styles: styles,
		prog:   p,
	}
}

// SetSize updates the view dimensions for responsive rendering.
func (v *TUIView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// View renders the complete TUI interface based on the current model and controller state.
func (v *TUIView) View(model *TUIModel, controller *TUIController) string {
	if v.width == 0 {
		return "Loading..."
	}

	var parts []string

	title := v.styles.title.Render("ModFetch Downloads • TUI v1 (refactored MVC)")
	titleBar := v.styles.border.Width(v.width - 2).Render(title)
	parts = append(parts, titleBar)

	if controller.showHelp {
		help := v.styles.border.Width(v.width - 2).Render(v.helpView())
		parts = append(parts, help)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	if controller.menuOn {
		menu := v.styles.border.Width(v.width - 2).Render(v.menuView(controller))
		parts = append(parts, menu)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	if controller.newDL {
		modal := v.styles.border.Width(v.width - 4).Render(v.renderNewDownloadModal(controller))
		parts = append(parts, modal)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	table := v.styles.border.Width(v.width - 2).Render(v.renderTable(model, controller))
	parts = append(parts, table)

	rows := model.FilteredRows(controller.statusFilter)
	if controller.showInfo && controller.selected < len(rows) {
		details := v.styles.border.Width(v.width - 2).Render(v.renderDetails(rows[controller.selected]))
		parts = append(parts, details)
	}

	commands := v.styles.border.Width(v.width - 2).Render(v.renderCommandsBar())
	parts = append(parts, commands)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (v *TUIView) renderHeader() string {
	return v.styles.header.Render("ModFetch Downloads")
}

func (v *TUIView) renderTable(model *TUIModel, controller *TUIController) string {
	var b strings.Builder

	rows := model.FilteredRows(controller.statusFilter)
	if len(rows) == 0 {
		return "No downloads found."
	}

	for i, row := range rows {
		style := v.styles.row
		if i == controller.selected {
			style = v.styles.sel
		}

		line := fmt.Sprintf("%-40s %s",
			truncate(row.URL, 40),
			row.Status)

		if row.Status == "downloading" || row.Status == "running" || row.Status == "pending" {
			current, total, _ := model.ProgressFor(row.URL, row.Dest)
			if total > 0 && current > 0 {
				pct := float64(current) / float64(total)
				line += fmt.Sprintf(" %s/%s (%.1f%%)",
					humanize.Bytes(uint64(current)),
					humanize.Bytes(uint64(total)),
					pct*100)
			}
		}

		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (v *TUIView) renderDetails(row state.DownloadRow) string {
	var b strings.Builder
	b.WriteString("Details:\n")
	b.WriteString(fmt.Sprintf("URL: %s\n", row.URL))
	b.WriteString(fmt.Sprintf("Dest: %s\n", row.Dest))
	b.WriteString(fmt.Sprintf("Status: %s\n", row.Status))
	if row.ExpectedSHA256 != "" {
		b.WriteString(fmt.Sprintf("SHA256: %s\n", row.ExpectedSHA256))
	}
	return b.String()
}

func (v *TUIView) helpView() string {
	help := `ModFetch TUI Help (v1 - Refactored MVC)

Navigation:
  ↑/k       Move up
  ↓/j       Move down
  enter     Toggle details
  n         New download
  m         Menu
  /         Filter
  r         Refresh
  q         Quit
  ?         Toggle help

Menu Options:
  r         Refresh downloads
  c         Clear completed
  x         Cancel selected download`
	return help
}

func (v *TUIView) menuView(controller *TUIController) string {
	var b strings.Builder
	b.WriteString("Menu:\n")

	options := []string{
		"r - Refresh downloads",
		"c - Clear completed",
		"x - Cancel selected download",
	}

	for i, opt := range options {
		style := v.styles.row
		if i == controller.menuSelected {
			style = v.styles.sel
		}
		b.WriteString(style.Render(opt))
		b.WriteString("\n")
	}

	return b.String()
}

func (v *TUIView) renderNewDownloadModal(controller *TUIController) string {
	var b strings.Builder
	b.WriteString("New Download\n\n")

	steps := []string{"1) URL/URI", "2) Artifact Type", "3) Auto Place", "4) Destination"}
	b.WriteString(strings.Join(steps, " • ") + "\n\n")

	switch controller.newStep {
	case 1:
		b.WriteString("Enter URL or resolver URI:\n")
	case 2:
		b.WriteString("Enter artifact type (optional):\n")
	case 3:
		b.WriteString("Auto place after download? y/n (default n):\n")
	case 4:
		b.WriteString("Enter destination path:\n")
	}

	b.WriteString(controller.newInput.View())
	b.WriteString("\n\nPress Esc to cancel, Enter to continue")
	return b.String()
}

func (v *TUIView) renderCommandsBar() string {
	return "↑/↓/j/k nav • enter details • n new • m menu • / filter • r refresh • ? help • q quit"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
