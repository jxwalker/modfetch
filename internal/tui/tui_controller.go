package tui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/resolver"
)

type TUIController struct {
	model        *TUIModel
	view         *TUIView
	teaModel     tea.Model // Reference to the main tea.Model instance
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
	newInput     textinput.Model
	newURL       string
	newType      string
	newAutoPlace bool
	newDest      string
}

func NewTUIController(model *TUIModel, view *TUIView) *TUIController {
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter downloads..."

	newInput := textinput.New()
	newInput.Placeholder = "Enter URL or resolver URI"

	return &TUIController{
		model:        model,
		view:         view,
		filterInput:  filterInput,
		newInput:     newInput,
		statusFilter: []string{"pending", "downloading", "completed", "failed"},
	}
}

func (c *TUIController) SetModel(m tea.Model) {
	c.teaModel = m
}

func (c *TUIController) wrapModel() tea.Model {
	fmt.Printf("DEBUG: wrapModel called, newStep=%d, newDL=%t\n", c.newStep, c.newDL)
	return c.teaModel
}

func (c *TUIController) Init() tea.Cmd {
	if err := c.model.LoadRows(); err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	return tickCmd()
}

func (c *TUIController) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	fmt.Printf("DEBUG: TUIController.Update received message type: %T\n", msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.view.SetSize(msg.Width, msg.Height)
		return c.wrapModel(), nil

	case tea.KeyMsg:
		fmt.Printf("DEBUG: Processing KeyMsg: %s\n", msg.String())
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

	case metaMsg:
		fmt.Printf("DEBUG: Processing metaMsg: %+v, newDL=%t, newStep=%d\n", msg, c.newDL, c.newStep)
		if c.newDL && c.newStep == 2 {
			if msg.suggested != "" {
				c.newInput.SetValue(msg.suggested)
			} else {
				c.newInput.SetValue(c.model.DestGuess(c.newURL))
			}
		}
		return c.wrapModel(), nil

	case errMsg:
		fmt.Printf("DEBUG: Processing errMsg: %v\n", msg.err)
		return c.wrapModel(), nil
	}

	fmt.Printf("DEBUG: Unhandled message type: %T\n", msg)
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
		c.newStep = 1
		c.newInput.SetValue("")
		c.newInput.Placeholder = "Enter URL or resolver URI"
		c.newInput.Focus()

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
		if c.menuSelected < 2 {
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

	case "enter", "ctrl+j":
		val := strings.TrimSpace(c.newInput.Value())
		switch c.newStep {
		case 1:
			if val == "" {
				return c.wrapModel(), nil
			}
			c.newURL = val
			c.newInput.SetValue("")
			c.newInput.Placeholder = "Artifact type (optional, e.g. sd.checkpoint)"
			c.newStep = 2
			return c.wrapModel(), c.resolveMetaCmd(val)

		case 2:
			c.newType = val
			c.newInput.SetValue("")
			c.newInput.Placeholder = "Auto place after download? y/n (default n)"
			c.newStep = 3
			return c.wrapModel(), nil

		case 3:
			v := strings.ToLower(strings.TrimSpace(val))
			c.newAutoPlace = v == "y" || v == "yes" || v == "true" || v == "1"
			var cand string
			if c.newAutoPlace {
				cand = c.model.DestGuess(c.newURL)
			} else {
				cand = c.model.DestGuess(c.newURL)
			}
			c.newInput.SetValue(cand)
			c.newInput.Placeholder = "Destination path (Enter to accept)"
			c.newStep = 4
			return c.wrapModel(), nil

		case 4:
			if val == "" {
				val = c.model.DestGuess(c.newURL)
			}
			c.newDest = val
			return c.startNewDownload()
		}
		return c.wrapModel(), nil

	default:
		var cmd tea.Cmd
		c.newInput, cmd = c.newInput.Update(msg)
		return c.wrapModel(), cmd
	}
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
	url := c.newURL
	dest := c.newDest

	c.newDL = false

	return c.wrapModel(), func() tea.Msg {
		ctx := context.Background()
		if err := c.model.StartDownload(ctx, url, dest, "", nil); err != nil {
			return errMsg{err}
		}
		return dlDoneMsg{url: url, dest: dest}
	}
}

func (c *TUIController) resolveMetaCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		s := strings.TrimSpace(raw)
		if s == "" {
			return metaMsg{url: raw}
		}
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
			if u, err := url.Parse(s); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 2 {
						modelID := parts[1]
						q := u.Query()
						ver := q.Get("modelVersionId")
						if ver == "" {
							ver = q.Get("version")
						}
						civ := "civitai://model/" + modelID
						if strings.TrimSpace(ver) != "" {
							civ += "?version=" + url.QueryEscape(ver)
						}
						s = civ
					}
				}
			}
		}
		if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
			if res, err := resolver.Resolve(context.Background(), s, c.model.cfg); err == nil {
				return metaMsg{url: raw, fileName: res.FileName, suggested: res.SuggestedFilename, civType: res.FileType}
			}
		}
		return metaMsg{url: raw}
	}
}

func hostIs(hostname, target string) bool {
	return hostname == target || strings.HasSuffix(hostname, "."+target)
}
