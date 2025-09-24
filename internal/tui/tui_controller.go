package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TUIController struct {
	model        *TUIModel
	view         *TUIView
	selected     int
	showInfo     bool
	showHelp     bool
	menuOn       bool
	menuSelected int
	filterOn     bool
	filterInput  textinput.Model
	statusFilter []string
	newDL        bool
	newStep      int
	newURLInput  textinput.Model
	newDestInput textinput.Model
	newSHAInput  textinput.Model
}

func NewTUIController(model *TUIModel, view *TUIView) *TUIController {
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter downloads..."

	urlInput := textinput.New()
	urlInput.Placeholder = "https://example.com/model.safetensors"
	urlInput.Focus()

	destInput := textinput.New()
	destInput.Placeholder = "/path/to/destination"

	shaInput := textinput.New()
	shaInput.Placeholder = "SHA256 hash (optional)"

	return &TUIController{
		model:        model,
		view:         view,
		filterInput:  filterInput,
		newURLInput:  urlInput,
		newDestInput: destInput,
		newSHAInput:  shaInput,
		statusFilter: []string{"pending", "downloading", "completed", "failed"},
	}
}

func (c *TUIController) wrapModel() tea.Model {
	return &model{tuiModel: c.model, tuiView: c.view, tuiController: c}
}

func (c *TUIController) Init() tea.Cmd {
	if err := c.model.LoadRows(); err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	return tickCmd()
}

func (c *TUIController) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.view.SetSize(msg.Width, msg.Height)
		return c.wrapModel(), nil

	case tea.KeyMsg:
		return c.handleKeyMsg(msg)

	case tickMsg:
		if err := c.model.LoadRows(); err != nil {
			return c.wrapModel(), func() tea.Msg { return errMsg{err} }
		}
		return c.wrapModel(), tickCmd()

	case dlDoneMsg:
		if err := c.model.LoadRows(); err != nil {
			return c.wrapModel(), func() tea.Msg { return errMsg{err} }
		}
		return c.wrapModel(), nil

	case errMsg:
		return c.wrapModel(), nil
	}

	return c.wrapModel(), nil
}

func (c *TUIController) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if c.showHelp {
		return c.handleHelpKeys(msg)
	}

	if c.menuOn {
		return c.handleMenuKeys(msg)
	}

	if c.newDL {
		return c.handleNewDownloadKeys(msg)
	}

	if c.filterOn {
		return c.handleFilterKeys(msg)
	}

	return c.handleNormalKeys(msg)
}

func (c *TUIController) handleNormalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := c.model.FilteredRows(c.statusFilter)

	switch msg.String() {
	case "q", "ctrl+c":
		return c.wrapModel(), tea.Quit

	case "?":
		c.showHelp = !c.showHelp

	case "up", "k":
		if c.selected > 0 {
			c.selected--
		}

	case "down", "j":
		if c.selected < len(rows)-1 {
			c.selected++
		}

	case "enter":
		c.showInfo = !c.showInfo

	case "n":
		c.newDL = true
		c.newStep = 0
		c.newURLInput.SetValue("")
		c.newDestInput.SetValue("")
		c.newSHAInput.SetValue("")
		c.newURLInput.Focus()

	case "m":
		c.menuOn = true
		c.menuSelected = 0

	case "/":
		c.filterOn = true
		c.filterInput.Focus()

	case "r":
		return c.wrapModel(), func() tea.Msg {
			if err := c.model.LoadRows(); err != nil {
				return errMsg{err}
			}
			return nil
		}
	}

	return c.wrapModel(), nil
}

func (c *TUIController) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q":
		c.showHelp = false
	}
	return c.wrapModel(), nil
}

func (c *TUIController) handleMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		c.menuOn = false

	case "up", "k":
		if c.menuSelected > 0 {
			c.menuSelected--
		}

	case "down", "j":
		if c.menuSelected < 3 {
			c.menuSelected++
		}

	case "enter":
		return c.applyMenuChoice()
	}

	return c.wrapModel(), nil
}

func (c *TUIController) handleNewDownloadKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		c.newDL = false
		return c.wrapModel(), nil

	case "enter":
		switch c.newStep {
		case 0:
			url := strings.TrimSpace(c.newURLInput.Value())
			if url == "" {
				return c.wrapModel(), nil
			}
			c.newStep = 1
			c.newDestInput.SetValue(c.model.DestGuess(url))
			c.newDestInput.Focus()

		case 1:
			dest := strings.TrimSpace(c.newDestInput.Value())
			if dest == "" {
				return c.wrapModel(), nil
			}
			c.newStep = 2
			c.newSHAInput.Focus()

		case 2:
			return c.startNewDownload()
		}
	}

	var cmd tea.Cmd
	switch c.newStep {
	case 0:
		c.newURLInput, cmd = c.newURLInput.Update(msg)
	case 1:
		c.newDestInput, cmd = c.newDestInput.Update(msg)
	case 2:
		c.newSHAInput, cmd = c.newSHAInput.Update(msg)
	}

	return c.wrapModel(), cmd
}

func (c *TUIController) handleFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		c.filterOn = false
		c.filterInput.Blur()

	case "enter":
		c.filterOn = false
		c.filterInput.Blur()
		filter := strings.TrimSpace(c.filterInput.Value())
		if filter == "" {
			c.statusFilter = []string{"pending", "downloading", "completed", "failed"}
		} else {
			c.statusFilter = []string{filter}
		}
	}

	var cmd tea.Cmd
	c.filterInput, cmd = c.filterInput.Update(msg)
	return c.wrapModel(), cmd
}

func (c *TUIController) applyMenuChoice() (tea.Model, tea.Cmd) {
	c.menuOn = false

	switch c.menuSelected {
	case 0:
		return c.wrapModel(), func() tea.Msg {
			if err := c.model.LoadRows(); err != nil {
				return errMsg{err}
			}
			return nil
		}

	case 1:
		return c.wrapModel(), func() tea.Msg {
			return nil
		}

	case 2:
		rows := c.model.FilteredRows(c.statusFilter)
		if c.selected < len(rows) {
			row := rows[c.selected]
			key := row.URL + "|" + row.Dest
			if cancel, exists := c.model.GetRunning()[key]; exists {
				cancel()
			}
		}
	}

	return c.wrapModel(), nil
}

func (c *TUIController) startNewDownload() (tea.Model, tea.Cmd) {
	url := strings.TrimSpace(c.newURLInput.Value())
	dest := strings.TrimSpace(c.newDestInput.Value())
	sha := strings.TrimSpace(c.newSHAInput.Value())

	c.newDL = false

	return c.wrapModel(), func() tea.Msg {
		ctx := context.Background()
		if err := c.model.StartDownload(ctx, url, dest, sha, nil); err != nil {
			return errMsg{err}
		}
		return dlDoneMsg{url: url, dest: dest}
	}
}
