package configwizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jxwalker/modfetch/internal/config"
)

type Wizard struct {
	inputs []textinput.Model
	focus  int
	done   bool
	out    *config.Config
}

func New(defaults *config.Config) *Wizard {
	w := &Wizard{}
	fields := make([]textinput.Model, 0, 9)
	mk := func(ph, val string) textinput.Model {
		ti := textinput.New()
		ti.Prompt = "> "
		ti.Placeholder = ph
		ti.SetValue(val)
		ti.CharLimit = 256
		return ti
	}
	// data_root, download_root, placement_mode
	dr := "~/modfetch/data"
	dd := "~/modfetch/downloads"
	pm := "symlink"
	if defaults != nil {
		if defaults.General.DataRoot != "" {
			dr = defaults.General.DataRoot
		}
		if defaults.General.DownloadRoot != "" {
			dd = defaults.General.DownloadRoot
		}
		if defaults.General.PlacementMode != "" {
			pm = defaults.General.PlacementMode
		}
	}
	fields = append(fields, mk("general.data_root", dr))
	fields = append(fields, mk("general.download_root", dd))
	fields = append(fields, mk("general.placement_mode (symlink|hardlink|copy)", pm))
	// hf enabled, token env
	hfEn := "true"
	hfTok := "HF_TOKEN"
	// civitai enabled, token env
	cvEn := "true"
	cvTok := "CIVITAI_TOKEN"
	if defaults != nil {
		if defaults.Sources.HuggingFace.Enabled {
			hfEn = "true"
		} else {
			hfEn = "false"
		}
		if defaults.Sources.HuggingFace.TokenEnv != "" {
			hfTok = defaults.Sources.HuggingFace.TokenEnv
		}
		if defaults.Sources.CivitAI.Enabled {
			cvEn = "true"
		} else {
			cvEn = "false"
		}
		if defaults.Sources.CivitAI.TokenEnv != "" {
			cvTok = defaults.Sources.CivitAI.TokenEnv
		}
	}
	fields = append(fields, mk("sources.huggingface.enabled (true|false)", hfEn))
	fields = append(fields, mk("sources.huggingface.token_env", hfTok))
	fields = append(fields, mk("sources.civitai.enabled (true|false)", cvEn))
	fields = append(fields, mk("sources.civitai.token_env", cvTok))
	// chunk params
	csm := 8
	pfc := 4
	if defaults != nil {
		if defaults.Concurrency.ChunkSizeMB > 0 {
			csm = defaults.Concurrency.ChunkSizeMB
		}
		if defaults.Concurrency.PerFileChunks > 0 {
			pfc = defaults.Concurrency.PerFileChunks
		}
	}
	fields = append(fields, mk("concurrency.chunk_size_mb", fmt.Sprint(csm)))
	fields = append(fields, mk("concurrency.per_file_chunks", fmt.Sprint(pfc)))

	w.inputs = fields
	w.focus = 0
	w.inputs[0].Focus()
	return w
}

func (w *Wizard) Init() tea.Cmd { return nil }

func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "ctrl+c", "q":
			w.done = true
			return w, tea.Quit
		case "tab", "shift+tab", "enter", "up", "down":
			i := w.focus
			if m.String() == "enter" {
				if i == len(w.inputs)-1 {
					w.done = true
					w.out = w.buildConfig()
					return w, tea.Quit
				}
			}
			if m.String() == "up" || m.String() == "shift+tab" {
				w.focus--
				if w.focus < 0 {
					w.focus = 0
				}
			} else {
				w.focus++
				if w.focus >= len(w.inputs) {
					w.focus = len(w.inputs) - 1
				}
			}
			for j := range w.inputs {
				if j == w.focus {
					w.inputs[j].Focus()
				} else {
					w.inputs[j].Blur()
				}
			}
		}
	}
	// Update all inputs
	cmds := make([]tea.Cmd, len(w.inputs))
	for i := range w.inputs {
		w.inputs[i], cmds[i] = w.inputs[i].Update(msg)
	}
	return w, tea.Batch(cmds...)
}

func (w *Wizard) View() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("modfetch config wizard") + "\n")
	b.WriteString("Fill in fields. Tab/Shift-Tab to navigate, Enter to submit. q to quit.\n\n")
	labels := []string{
		"general.data_root",
		"general.download_root",
		"general.placement_mode",
		"sources.huggingface.enabled",
		"sources.huggingface.token_env",
		"sources.civitai.enabled",
		"sources.civitai.token_env",
		"concurrency.chunk_size_mb",
		"concurrency.per_file_chunks",
	}
	for i, input := range w.inputs {
		marker := " "
		if i == w.focus {
			marker = ">"
		}
		b.WriteString(fmt.Sprintf("%s %-32s %s\n", marker, labels[i]+":", input.View()))
	}
	if w.done {
		b.WriteString("\nDone. Saving...\n")
	}
	return b.String()
}

func (w *Wizard) buildConfig() *config.Config {
	get := func(i int) string { return strings.TrimSpace(w.inputs[i].Value()) }
	parseBool := func(s string) bool {
		return strings.EqualFold(s, "true") || s == "1" || strings.EqualFold(s, "y") || strings.EqualFold(s, "yes")
	}
	parseInt := func(s string, def int) int {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n >= 0 {
			return n
		}
		return def
	}
	o := &config.Config{Version: 1}
	o.General.DataRoot = get(0)
	o.General.DownloadRoot = get(1)
	pm := strings.ToLower(get(2))
	if pm != "symlink" && pm != "hardlink" && pm != "copy" {
		pm = "symlink"
	}
	o.General.PlacementMode = pm
	o.Sources.HuggingFace.Enabled = parseBool(get(3))
	o.Sources.HuggingFace.TokenEnv = get(4)
	o.Sources.CivitAI.Enabled = parseBool(get(5))
	o.Sources.CivitAI.TokenEnv = get(6)
	o.Concurrency.ChunkSizeMB = parseInt(get(7), 8)
	o.Concurrency.PerFileChunks = parseInt(get(8), 4)
	return o
}

func (w *Wizard) Config() *config.Config { return w.out }
