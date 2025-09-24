package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

type model struct {
	tuiModel      *TUIModel
	tuiView       *TUIView
	tuiController *TUIController
}

type tickMsg time.Time

type dlDoneMsg struct {
	url, dest string
}

type errMsg struct{ err error }

type metaMsg struct {
	url       string
	fileName  string
	suggested string
	civType   string
}

// New creates a new TUI model that implements the tea.Model interface.
// It orchestrates the MVC components: TUIModel, TUIView, and TUIController.
func New(cfg *config.Config, st *state.DB) tea.Model {
	tuiModel := NewTUIModel(cfg, st)
	tuiView := NewTUIView()
	tuiController := NewTUIController(tuiModel, tuiView)

	m := &model{
		tuiModel:      tuiModel,
		tuiView:       tuiView,
		tuiController: tuiController,
	}

	tuiController.SetModel(m)

	return m
}

func (m *model) Init() tea.Cmd {
	return m.tuiController.Init()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.tuiController.Update(msg)
}

func (m *model) View() string {
	return m.tuiView.View(m.tuiModel, m.tuiController)
}

func tickCmd() tea.Cmd {
	d := time.Second
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}
