package tui

import tea "github.com/charmbracelet/bubbletea"

type stateChangedMsg struct{}

func (m *Model) stateEventsCmd() tea.Cmd {
	ch := m.stateEvents
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		if _, ok := <-ch; !ok {
			return nil
		}
		return stateChangedMsg{}
	}
}
