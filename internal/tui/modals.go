package tui

import (
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/resolver"
)

// Modal dialogs and input handlers

func (m *Model) updateNewJob(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Handle quantization selection mode separately
	if m.newQuantSelecting {
		switch s {
		case "esc":
			// Cancel quantization selection, go back to type selection
			m.newQuantSelecting = false
			return m, nil
		case "j", "down":
			// Navigate down in quantization list
			if m.newSelectedQuant < len(m.newAvailableQuants)-1 {
				m.newSelectedQuant++
			}
			return m, nil
		case "k", "up":
			// Navigate up in quantization list
			if m.newSelectedQuant > 0 {
				m.newSelectedQuant--
			}
			return m, nil
		case "enter", "ctrl+j":
			// Confirm quantization selection and update URL with selected file
			if m.newSelectedQuant >= 0 && m.newSelectedQuant < len(m.newAvailableQuants) {
				selected := m.newAvailableQuants[m.newSelectedQuant]

				// Validate and sanitize FilePath
				filePath := strings.TrimSpace(selected.FilePath)
				filePath = strings.TrimPrefix(filePath, "/")
				if filePath == "" {
					m.addToast("error: invalid quantization file path")
					return m, nil
				}

				// Update URL to point to the selected file
				// Parse the current hf:// URL and update the path
				if strings.HasPrefix(m.newURL, "hf://") {
					// Format: hf://owner/repo[/path]?rev=...&quant=...
					// We need to update it to hf://owner/repo/selected.FilePath?rev=...
					// Note: We strip out quant= parameter since we're specifying the exact file
					parts := strings.Split(strings.TrimPrefix(m.newURL, "hf://"), "?")
					pathPart := parts[0]

					// Preserve only rev parameter, drop quant parameter
					revParam := ""
					if len(parts) > 1 {
						if q, err := neturl.ParseQuery(parts[1]); err == nil {
							if rev := q.Get("rev"); rev != "" {
								revParam = "?rev=" + neturl.QueryEscape(rev)
							}
						}
					}

					// Split path into owner/repo[/oldpath]
					pathSegments := strings.Split(pathPart, "/")
					if len(pathSegments) >= 2 {
						owner := pathSegments[0]
						repo := pathSegments[1]
						// Rebuild with selected file path (no quant param needed)
						m.newURL = "hf://" + owner + "/" + repo + "/" + filePath + revParam
					}
				}
			}

			m.newQuantSelecting = false
			m.newInput.SetValue("")
			m.newInput.Placeholder = "Artifact type (optional, e.g. sd.checkpoint)"
			m.newStep = 2
			return m, nil
		}
		return m, nil
	}

	switch s {
	case "esc":
		m.newJob = false
		return m, nil
	case "tab":
		if m.newStep == 4 {
			cur := strings.TrimSpace(m.newInput.Value())
			comp := m.completePath(cur)
			if strings.TrimSpace(comp) != "" {
				m.newInput.SetValue(comp)
				m.newAutoSuggested = comp
			}
			return m, nil
		}
		return m, nil
	case "x", "X":
		if m.newStep == 4 && strings.TrimSpace(m.newSuggestExt) != "" {
			cur := strings.TrimSpace(m.newInput.Value())
			if filepath.Ext(cur) == "" {
				m.newInput.SetValue(cur + m.newSuggestExt)
				m.newSuggestExt = ""
			}
		}
		return m, nil
	case "enter", "ctrl+j":
		val := strings.TrimSpace(m.newInput.Value())
		switch m.newStep {
		case 1:
			if val == "" {
				return m, nil
			}
			m.newURL = val
			if norm, ok := m.normalizeURLForNote(val); ok {
				m.newNormNote = norm
			} else {
				m.newNormNote = ""
			}
			m.newInput.SetValue("")
			m.newInput.Placeholder = "Artifact type (optional, e.g. sd.checkpoint)"
			m.newStep = 2
			return m, m.resolveMetaCmd(m.newURL)
		case 2:
			m.newType = val
			m.newInput.SetValue("")
			m.newInput.Placeholder = "Auto place after download? y/n (default n)"
			m.newStep = 3
			return m, nil
		case 3:
			v := strings.ToLower(strings.TrimSpace(val))
			m.newAutoPlace = v == "y" || v == "yes" || v == "true" || v == "1"
			var cand string
			if m.newAutoPlace {
				cand = m.computePlacementSuggestionImmediate(m.newURL, m.newType)
				m.newAutoSuggested = cand
				m.newInput.SetValue(cand)
				m.newInput.Placeholder = "Destination path (Enter to accept)"
				m.newStep = 4
				if filepath.Ext(cand) == "" {
					m.newSuggestExt = m.inferExt()
				} else {
					m.newSuggestExt = ""
				}
				return m, m.suggestPlacementCmd(m.newURL, m.newType)
			}
			cand = m.immediateDestCandidate(m.newURL)
			m.newAutoSuggested = cand
			m.newInput.SetValue(cand)
			m.newInput.Placeholder = "Destination path (Enter to accept)"
			m.newStep = 4
			if filepath.Ext(cand) == "" {
				m.newSuggestExt = m.inferExt()
			} else {
				m.newSuggestExt = ""
			}
			return m, m.suggestDestCmd(m.newURL)
		case 4:
			urlStr := m.newURL
			dest := strings.TrimSpace(val)
			if dest == "" {
				if m.newAutoPlace {
					dest = m.computePlacementSuggestionImmediate(urlStr, m.newType)
				} else {
					dest = m.computeDefaultDest(urlStr)
				}
			}
			if err := m.preflightDest(dest); err != nil {
				m.addToast("dest not writable: " + err.Error())
				return m, nil
			}
			_ = m.st.UpsertDownload(state.DownloadRow{URL: urlStr, Dest: dest, Status: "pending"})
			// Set placement data before spawning goroutine
			key := urlStr + "|" + dest
			if strings.TrimSpace(m.newType) != "" {
				m.placeType[key] = m.newType
			}
			if m.newAutoPlace {
				m.autoPlace[key] = true
			}
			// Capture placement data to pass (avoid concurrent map access)
			autoPlace := m.autoPlace[key]
			placeType := m.placeType[key]
			cmd := m.startDownloadCmd(urlStr, dest, autoPlace, placeType)
			m.addToast("started: " + truncateMiddle(dest, 40))
			m.newJob = false
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.newInput, cmd = m.newInput.Update(msg)
	return m, cmd
}

func (m *Model) updateBatchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc":
		m.batchMode = false
		return m, nil
	case "enter", "ctrl+j":
		path := strings.TrimSpace(m.batchInput.Value())
		var cmd tea.Cmd
		if path != "" {
			cmd = m.importBatchFile(path)
		}
		m.batchMode = false
		return m, cmd
	}
	var cmd tea.Cmd
	m.batchInput, cmd = m.batchInput.Update(msg)
	return m, cmd
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "enter", "ctrl+j":
		m.filterOn = false
		m.filterInput.Blur()
		return m, nil
	case "esc":
		m.filterOn = false
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m *Model) renderNewJobModal() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("New Download") + "\n")
	// Token status inline
	sb.WriteString(m.th.label.Render(m.renderAuthStatus()) + "\n\n")
	steps := []string{
		"1) URL/URI",
		"2) Artifact type (optional)",
		"3) Auto place after download (y/n)",
		"4) Destination path",
	}
	sb.WriteString(m.th.label.Render(strings.Join(steps, " • ")) + "\n\n")
	// Step-specific helper text
	switch m.newStep {
	case 1:
		sb.WriteString(m.th.label.Render("Enter an HTTP URL or a resolver URI (hf://owner/repo/path?rev=..., civitai://model/ID?version=...).") + "\n")
	case 2:
		// Show available types from config mapping
		if len(m.configTypes) > 0 {
			sb.WriteString(m.th.label.Render("Types from your config: "+strings.Join(m.configTypes, ", ")) + "\n")
		}
		sb.WriteString(m.th.label.Render("Type helps choose placement directories. Leave blank to skip placement or if unsure.") + "\n")
	case 3:
		sb.WriteString(m.th.label.Render("y: Place into mapped app directories after download • n: Save only to download_root.") + "\n")
	case 4:
		if m.newAutoPlace {
			cand := strings.TrimSpace(m.newAutoSuggested)
			if cand != "" {
				sb.WriteString(m.th.label.Render("Will place into: "+cand) + "\n")
			}
			sb.WriteString(m.th.label.Render("Edit the destination to override.") + "\n")
		} else {
			sb.WriteString(m.th.label.Render("Choose where to save the file. You can place later via 'place'.") + "\n")
		}
		// If we can infer an extension and current input lacks one, suggest adding via X
		curVal := strings.TrimSpace(m.newInput.Value())
		if strings.TrimSpace(m.newSuggestExt) != "" && filepath.Ext(curVal) == "" {
			sb.WriteString(m.th.label.Render("Suggestion: append " + m.newSuggestExt + " (press X)\n"))
		}
	}
	cur := ""
	if m.newStep == 1 {
		cur = "Enter URL or resolver URI"
	}
	if m.newStep == 2 {
		cur = "Artifact type (optional)"
	}
	if m.newStep == 3 {
		cur = "Auto place? y/n"
	}
	if m.newStep == 4 {
		cur = "Destination path (Tab to complete)"
	}
	sb.WriteString(m.th.label.Render(cur) + "\n")
	// Show normalization note once URL entered
	if m.newStep >= 2 && strings.TrimSpace(m.newNormNote) != "" {
		sb.WriteString(m.th.label.Render(m.newNormNote) + "\n")
	}
	// Show detected type/name if available when choosing type
	if m.newStep == 2 && strings.TrimSpace(m.newTypeDetected) != "" {
		from := m.newTypeSource
		if from == "" {
			from = "heuristic"
		}
		sb.WriteString(m.th.label.Render(fmt.Sprintf("Detected: %s (%s)", m.newTypeDetected, from)) + "\n")
	}
	sb.WriteString(m.newInput.View())
	return sb.String()
}

// renderQuantizationSelection renders the quantization selection UI

func (m *Model) renderQuantizationSelection() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Select Quantization") + "\n\n")

	// Extract repo info from URL if available
	repoInfo := ""
	if strings.HasPrefix(m.newURL, "hf://") {
		parts := strings.Split(strings.TrimPrefix(m.newURL, "hf://"), "/")
		if len(parts) >= 2 {
			repoInfo = parts[0] + "/" + parts[1]
		}
	}
	if repoInfo != "" {
		sb.WriteString(m.th.label.Render("Repository: "+repoInfo) + "\n\n")
	}

	sb.WriteString(m.th.label.Render("Available quantizations (j/k to navigate, Enter to select):") + "\n\n")

	// Render each quantization option
	for i, q := range m.newAvailableQuants {
		prefix := "  "

		// Highlight selected item
		if i == m.newSelectedQuant {
			prefix = "▸ "
		}

		// Format size
		sizeStr := humanize.Bytes(uint64(q.Size))

		// Build line: "  Q4_K_M  (4.1 GB)  - gguf"
		line := fmt.Sprintf("%s%-10s  (%7s)  - %s", prefix, q.Name, sizeStr, q.FileType)

		if i == m.newSelectedQuant {
			sb.WriteString(m.th.ok.Render(line) + "\n")
		} else {
			sb.WriteString(m.th.label.Render(line) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(m.th.label.Render("j/k: navigate  •  Enter: select  •  Esc: cancel") + "\n")

	return sb.String()
}

func (m *Model) renderBatchModal() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Batch Import") + "\n")
	sb.WriteString(m.th.label.Render("Enter path to text file with one URL per line (tokens: dest=..., type=..., place=true, mode=copy|symlink|hardlink)") + "\n\n")
	sb.WriteString(m.batchInput.View())
	return sb.String()
}

// Preflight: ensure destination's parent is writable

func (m *Model) preflightDest(dest string) error {
	d := strings.TrimSpace(dest)
	if d == "" {
		return fmt.Errorf("empty dest")
	}
	dir := filepath.Dir(d)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return tryWrite(dir)
}

// Path completion for destination

func (m *Model) completePath(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return s
	}
	// Expand ~ and env for lookup
	h, _ := os.UserHomeDir()
	exp := os.ExpandEnv(s)
	if strings.HasPrefix(exp, "~") {
		exp = strings.Replace(exp, "~", h, 1)
	}
	dir := exp
	prefix := ""
	if fi, err := os.Stat(exp); err == nil && fi.IsDir() {
		// complete inside dir
	} else {
		dir = filepath.Dir(exp)
		prefix = filepath.Base(exp)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return s
	}
	matches := make([]string, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			if e.IsDir() {
				name += "/"
			}
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		return s
	}
	// If single match, replace basename
	var base string
	if len(matches) == 1 {
		base = matches[0]
	} else {
		// longest common prefix
		base = longestCommonPrefix(matches)
	}
	out := filepath.Join(dir, base)
	// compress home to ~ for display
	if h != "" && strings.HasPrefix(out, h+string(os.PathSeparator)) {
		out = "~" + strings.TrimPrefix(out, h)
	}
	return out
}

// Immediate (non-blocking) dest candidate based on URL only

func (m *Model) immediateDestCandidate(urlStr string) string {
	s := strings.TrimSpace(urlStr)
	if s == "" {
		return filepath.Join(m.cfg.General.DownloadRoot, "download")
	}
	// If it's a resolver URI, guess based on path base
	if strings.HasPrefix(s, "hf://") || strings.HasPrefix(s, "civitai://") {
		if u, err := neturl.Parse(s); err == nil {
			base := filepath.Base(strings.Trim(u.Path, "/"))
			if base == "" {
				base = "download"
			}
			return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
		}
	}
	// HTTP(S)
	if u, err := neturl.Parse(s); err == nil {
		h := strings.ToLower(u.Hostname())
		if hostIs(h, "huggingface.co") {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 1 {
				if len(parts) >= 5 && parts[2] == "blob" {
					base := filepath.Base(strings.Join(parts[4:], "/"))
					if base != "" {
						return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
					}
				}
				return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(filepath.Base(u.Path)))
			}
		}
		if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
			// Use model ID as placeholder
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 {
				id := parts[1]
				if id != "" {
					return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(id))
				}
			}
		}
		// Fallback to path base
		base := util.URLPathBase(s)
		if base == "" || base == "/" || base == "." {
			base = "download"
		}
		return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
	}
	// raw fallback
	base := util.URLPathBase(s)
	if base == "" || base == "/" || base == "." {
		base = "download"
	}
	return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
}

// Background suggestion using resolver (may perform network I/O)

func (m *Model) suggestDestCmd(urlStr string) tea.Cmd {
	return func() tea.Msg {
		d := m.computeDefaultDest(urlStr)
		// Improve default name for CivitAI direct download endpoints by probing filename via HEAD
		if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
			if u, err := neturl.Parse(urlStr); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/api/download/") {
					if name := m.headFilename(urlStr); strings.TrimSpace(name) != "" {
						name = util.SafeFileName(name)
						d = filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
		}
		return destSuggestMsg{url: urlStr, dest: d}
	}
}

// headFilename performs a HEAD request to retrieve a filename from Content-Disposition.

func (m *Model) headFilename(u string) string {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return ""
	}
	// Add CivitAI token if configured
	if m.cfg != nil && m.cfg.Sources.CivitAI.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
		if env == "" {
			env = "CIVITAI_TOKEN"
		}
		if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	lcd := strings.ToLower(cd)
	// RFC 5987 filename*
	if idx := strings.Index(lcd, "filename*="); idx >= 0 {
		v := cd[idx+len("filename*="):]
		// expect UTF-8''... optionally
		if strings.HasPrefix(strings.ToLower(v), "utf-8''") {
			v = v[len("utf-8''"):]
		}
		if semi := strings.IndexByte(v, ';'); semi >= 0 {
			v = v[:semi]
		}
		name, _ := neturl.QueryUnescape(strings.Trim(v, "\"' "))
		return name
	}
	// Simple filename=
	if idx := strings.Index(lcd, "filename="); idx >= 0 {
		v := cd[idx+len("filename="):]
		if semi := strings.IndexByte(v, ';'); semi >= 0 {
			v = v[:semi]
		}
		name := strings.Trim(v, "\"' ")
		return name
	}
	return ""
}

// Immediate placement suggestion using artifact type and quick filename

func (m *Model) computePlacementSuggestionImmediate(urlStr, atype string) string {
	atype = strings.TrimSpace(atype)
	if atype == "" {
		return m.immediateDestCandidate(urlStr)
	}
	dirs, err := placer.ComputeTargets(m.cfg, atype)
	if err != nil || len(dirs) == 0 {
		return m.immediateDestCandidate(urlStr)
	}
	base := filepath.Base(m.immediateDestCandidate(urlStr))
	if strings.TrimSpace(base) == "" {
		base = "download"
	}
	return filepath.Join(dirs[0], util.SafeFileName(base))
}

func (m *Model) inferExt() string {
	// Prefer explicit user-provided type
	t := strings.TrimSpace(m.newType)
	if t == "" {
		t = strings.TrimSpace(m.newTypeDetected)
	}
	switch strings.ToLower(t) {
	case "sd.checkpoint", "sd.lora", "sd.vae", "sd.controlnet":
		return ".safetensors"
	case "llm.gguf":
		return ".gguf"
	case "sd.embedding":
		// ambiguous, skip to avoid wrong ext
		return ""
	}
	return ""
}

// Background placement suggestion: resolve filename then join with placement target

func (m *Model) suggestPlacementCmd(urlStr, atype string) tea.Cmd {
	return func() tea.Msg {
		atype = strings.TrimSpace(atype)
		if atype == "" {
			return destSuggestMsg{url: urlStr, dest: m.computeDefaultDest(urlStr)}
		}
		dirs, err := placer.ComputeTargets(m.cfg, atype)
		if err != nil || len(dirs) == 0 {
			return destSuggestMsg{url: urlStr, dest: m.computeDefaultDest(urlStr)}
		}
		// Derive a good filename via resolver-backed default dest
		cand := m.computeDefaultDest(urlStr)
		base := filepath.Base(cand)
		if strings.TrimSpace(base) == "" {
			base = "download"
		}
		d := filepath.Join(dirs[0], util.SafeFileName(base))
		return destSuggestMsg{url: urlStr, dest: d}
	}
}

// Resolve metadata (suggested filename, civitai file type, and quantizations) in background

func (m *Model) resolveMetaCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		s := strings.TrimSpace(raw)
		if s == "" {
			return metaMsg{url: raw}
		}

		// Use centralized URL normalization and resolution
		normalized := NormalizeURL(s)

		// Only resolve URIs (hf:// or civitai://)
		if strings.HasPrefix(normalized, "hf://") || strings.HasPrefix(normalized, "civitai://") {
			res, err := resolver.Resolve(context.Background(), normalized, m.cfg)
			if err == nil {
				return metaMsg{
					url:           raw,
					fileName:      res.FileName,
					suggested:     res.SuggestedFilename,
					civType:       res.FileType,
					quants:        res.AvailableQuantizations,
					selectedQuant: res.SelectedQuantization,
				}
			}
		}
		return metaMsg{url: raw}
	}
}

func (m *Model) normalizeURLForNote(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return "", false
	}
	u, err := neturl.Parse(s)
	if err != nil {
		return "", false
	}
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
				civ += "?version=" + ver
			}
			return "Normalized to " + civ, true
		}
	}
	if hostIs(h, "huggingface.co") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			owner := parts[0]
			repo := parts[1]
			rev := parts[3]
			filePath := strings.Join(parts[4:], "/")
			hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
			return "Normalized to " + hf, true
		}
	}
	return "", false
}

// Token/env helpers

func (m *Model) computeDefaultDest(urlStr string) string {
	ctx := context.Background()
	u := strings.TrimSpace(urlStr)
	if strings.HasPrefix(u, "hf://") || strings.HasPrefix(u, "civitai://") {
		if res, err := resolver.Resolve(ctx, u, m.cfg); err == nil {
			name := res.SuggestedFilename
			if strings.TrimSpace(name) == "" {
				name = util.URLPathBase(res.URL)
			}
			name = util.SafeFileName(name)
			return filepath.Join(m.cfg.General.DownloadRoot, name)
		}
	}
	// translate civitai model page URLs and Hugging Face blob pages
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		if pu, err := neturl.Parse(u); err == nil {
			h := strings.ToLower(pu.Hostname())
			// For direct CivitAI download endpoints, try HEAD for a better filename first
			if hostIs(h, "civitai.com") && strings.HasPrefix(pu.Path, "/api/download/") {
				if name := m.headFilename(u); strings.TrimSpace(name) != "" {
					name = util.SafeFileName(name)
					return filepath.Join(m.cfg.General.DownloadRoot, name)
				}
			}
			if hostIs(h, "civitai.com") && strings.HasPrefix(pu.Path, "/models/") {
				parts := strings.Split(strings.Trim(pu.Path, "/"), "/")
				if len(parts) >= 2 {
					modelID := parts[1]
					q := pu.Query()
					ver := q.Get("modelVersionId")
					if ver == "" {
						ver = q.Get("version")
					}
					civ := "civitai://model/" + modelID
					if strings.TrimSpace(ver) != "" {
						civ += "?version=" + neturl.QueryEscape(ver)
					}
					if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil {
						name := res.SuggestedFilename
						if strings.TrimSpace(name) == "" {
							name = filepath.Base(res.URL)
						}
						name = util.SafeFileName(name)
						return filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
			if hostIs(h, "huggingface.co") {
				parts := strings.Split(strings.Trim(pu.Path, "/"), "/")
				if len(parts) >= 5 && parts[2] == "blob" {
					owner := parts[0]
					repo := parts[1]
					rev := parts[3]
					filePath := strings.Join(parts[4:], "/")
					hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
					if res, err := resolver.Resolve(ctx, hf, m.cfg); err == nil {
						name := util.URLPathBase(res.URL)
						name = util.SafeFileName(name)
						return filepath.Join(m.cfg.General.DownloadRoot, name)
					}
				}
			}
		}
	}
	base := util.URLPathBase(u)
	if base == "" || base == "/" || base == "." {
		base = "download"
	}
	return filepath.Join(m.cfg.General.DownloadRoot, util.SafeFileName(base))
}
