package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
	"github.com/jxwalker/modfetch/internal/discovery"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/recommend"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
)

type recommendStep int

const (
	recommendStepTask recommendStep = iota
	recommendStepHardware
	recommendStepProvider
	recommendStepRuntime
	recommendStepSize
	recommendStepQuery
	recommendStepResults
)

type recommendChoice struct {
	label       string
	value       string
	description string
	bytes       int64
}

type recommendFlow struct {
	active        bool
	step          recommendStep
	flowID        int64
	input         textinput.Model
	cancel        context.CancelFunc
	taskIndex     int
	hardwareIndex int
	providerIndex int
	runtimeIndex  int
	sizeIndex     int
	selected      int
	query         string
	task          string
	hardware      recommend.HardwareProfile
	hardwareKey   string
	results       []recommend.Recommendation
	inspect       bool
	probeLoading  bool
	probeErr      string
	probeDetails  recommendProbeDetails
	loading       bool
	err           string
}

type recommendResultsMsg struct {
	flowID          int64
	recommendations []recommend.Recommendation
	hardware        recommend.HardwareProfile
	query           string
	task            string
	hardwareKey     string
	warning         string
	err             error
}

type recommendProbeDetails struct {
	URI          string
	ResolvedURL  string
	FinalURL     string
	Filename     string
	Size         int64
	ETag         string
	LastModified string
	AcceptRange  bool
	History      state.TransferHistoryRow
	HasHistory   bool
}

type recommendInspectProbeMsg struct {
	flowID   int64
	selected int
	uri      string
	details  recommendProbeDetails
	err      error
}

type recommendLocalInspection struct {
	Destination         string
	ArtifactType        string
	PlacementPreset     string
	PlacementConfigured bool
	PlacementNote       string
}

func (m *Model) startRecommendFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	input := textinput.New()
	input.Placeholder = "Optional search, e.g. llama 8b gguf"
	input.CharLimit = 256
	m.recommendFlow = recommendFlow{
		active:   true,
		step:     recommendStepTask,
		flowID:   time.Now().UnixNano(),
		input:    input,
		hardware: recommend.DetectHardware(ctx),
	}
}

func (m *Model) updateRecommendFlow(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	if s == "esc" {
		m.cancelRecommendBackground()
		m.recommendFlow.active = false
		m.recommendFlow.input.Blur()
		return m, nil
	}
	if m.recommendFlow.loading {
		return m, nil
	}
	if m.recommendFlow.step == recommendStepQuery {
		switch s {
		case "enter", "ctrl+j":
			m.recommendFlow.query = strings.TrimSpace(m.recommendFlow.input.Value())
			m.recommendFlow.input.Blur()
			m.recommendFlow.step = recommendStepResults
			m.recommendFlow.loading = true
			m.recommendFlow.err = ""
			m.recommendFlow.results = nil
			return m, m.recommendSearchCmd()
		case "shift+tab":
			m.recommendFlow.step = recommendStepSize
			m.recommendFlow.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.recommendFlow.input, cmd = m.recommendFlow.input.Update(msg)
		return m, cmd
	}
	if m.recommendFlow.step == recommendStepResults {
		switch s {
		case "j", "down":
			m.moveRecommendSelection(1)
			return m, nil
		case "k", "up":
			m.moveRecommendSelection(-1)
			return m, nil
		case "left", "shift+tab":
			m.cancelRecommendBackground()
			m.clearRecommendProbe()
			m.recommendFlow.step = recommendStepQuery
			m.recommendFlow.input.Focus()
			return m, nil
		case "i":
			m.recommendFlow.inspect = !m.recommendFlow.inspect
			return m, nil
		case "p", "P":
			return m, m.startRecommendProbe()
		case "enter", "ctrl+j":
			return m, m.startRecommendedDownload()
		}
		return m, nil
	}
	switch s {
	case "j", "down":
		m.adjustRecommendChoice(1)
		return m, nil
	case "k", "up":
		m.adjustRecommendChoice(-1)
		return m, nil
	case "left", "ctrl+h":
		m.previousRecommendStep()
		return m, nil
	case "enter", "ctrl+j":
		m.nextRecommendStep()
		return m, nil
	}
	return m, nil
}

func (m *Model) moveRecommendSelection(delta int) {
	if len(m.recommendFlow.results) == 0 {
		return
	}
	prev := m.recommendFlow.selected
	m.recommendFlow.selected += delta
	if m.recommendFlow.selected < 0 {
		m.recommendFlow.selected = 0
	}
	if m.recommendFlow.selected >= len(m.recommendFlow.results) {
		m.recommendFlow.selected = len(m.recommendFlow.results) - 1
	}
	if m.recommendFlow.selected != prev {
		m.cancelRecommendBackground()
		m.clearRecommendProbe()
	}
}

func (m *Model) cancelRecommendBackground() {
	if m.recommendFlow.cancel != nil {
		m.recommendFlow.cancel()
		m.recommendFlow.cancel = nil
	}
}

func (m *Model) clearRecommendProbe() {
	m.recommendFlow.probeLoading = false
	m.recommendFlow.probeErr = ""
	m.recommendFlow.probeDetails = recommendProbeDetails{}
}

func (m *Model) nextRecommendStep() {
	switch m.recommendFlow.step {
	case recommendStepTask:
		m.recommendFlow.step = recommendStepHardware
	case recommendStepHardware:
		m.recommendFlow.step = recommendStepProvider
	case recommendStepProvider:
		m.recommendFlow.step = recommendStepRuntime
	case recommendStepRuntime:
		m.recommendFlow.step = recommendStepSize
	case recommendStepSize:
		m.recommendFlow.step = recommendStepQuery
		m.recommendFlow.input.Focus()
	}
}

func (m *Model) previousRecommendStep() {
	switch m.recommendFlow.step {
	case recommendStepHardware:
		m.recommendFlow.step = recommendStepTask
	case recommendStepProvider:
		m.recommendFlow.step = recommendStepHardware
	case recommendStepRuntime:
		m.recommendFlow.step = recommendStepProvider
	case recommendStepSize:
		m.recommendFlow.step = recommendStepRuntime
	case recommendStepResults:
		m.recommendFlow.step = recommendStepQuery
		m.recommendFlow.input.Focus()
	}
}

func (m *Model) adjustRecommendChoice(delta int) {
	var idx *int
	var count int
	switch m.recommendFlow.step {
	case recommendStepTask:
		idx = &m.recommendFlow.taskIndex
		count = len(recommendTaskChoices())
	case recommendStepHardware:
		idx = &m.recommendFlow.hardwareIndex
		count = len(m.recommendHardwareChoices())
	case recommendStepProvider:
		idx = &m.recommendFlow.providerIndex
		count = len(recommendProviderChoices())
	case recommendStepRuntime:
		idx = &m.recommendFlow.runtimeIndex
		count = len(recommendRuntimeChoices())
	case recommendStepSize:
		idx = &m.recommendFlow.sizeIndex
		count = len(recommendSizeChoices())
	}
	if idx == nil || count == 0 {
		return
	}
	*idx += delta
	if *idx < 0 {
		*idx = count - 1
	}
	if *idx >= count {
		*idx = 0
	}
}

func (m *Model) startRecommendProbe() tea.Cmd {
	if len(m.recommendFlow.results) == 0 {
		return nil
	}
	if m.recommendFlow.selected < 0 || m.recommendFlow.selected >= len(m.recommendFlow.results) {
		m.recommendFlow.selected = 0
	}
	m.cancelRecommendBackground()
	m.recommendFlow.inspect = true
	m.recommendFlow.probeLoading = true
	m.recommendFlow.probeErr = ""
	m.recommendFlow.probeDetails = recommendProbeDetails{}
	return m.recommendInspectProbeCmd()
}

func (m *Model) recommendSearchCmd() tea.Cmd {
	task := m.selectedRecommendTask()
	provider := m.selectedRecommendProvider()
	runtimeTarget := m.selectedRecommendRuntime()
	sizeLimit := m.selectedRecommendSizeLimit()
	query := strings.TrimSpace(m.recommendFlow.query)
	hardware := m.selectedRecommendHardware()
	flowID := m.recommendFlow.flowID
	st := m.st
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	m.recommendFlow.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		effectiveTask := recommend.NormalizeTask(task)
		effectiveQuery := query
		if effectiveQuery == "" {
			effectiveQuery = recommend.DefaultQuery(effectiveTask)
		}
		hardwareKey := recommend.HardwareKey(hardware)
		var feedback map[string]recommend.Feedback
		if st != nil {
			var err error
			feedback, err = recommend.FeedbackFromHistory(st, effectiveTask, effectiveQuery, hardwareKey)
			if err != nil {
				return recommendResultsMsg{flowID: flowID, err: fmt.Errorf("load recommendation history: %w", err)}
			}
		}
		recs, hw, err := recommend.Recommend(ctx, recommend.Options{
			Query:    effectiveQuery,
			Task:     effectiveTask,
			Provider: provider,
			Limit:    20,
			Hardware: hardware,
			Feedback: feedback,
		})
		if err != nil {
			return recommendResultsMsg{flowID: flowID, err: err}
		}
		recs = filterRecommendResults(recs, runtimeTarget, sizeLimit)
		if len(recs) > 8 {
			recs = recs[:8]
		}
		for i := range recs {
			recs[i].Index = i + 1
		}
		warning := ""
		if st != nil && len(recs) > 0 {
			if err := recommend.RecordHistory(st, effectiveTask, effectiveQuery, hardwareKey, recs, "shown", 0); err != nil {
				warning = "record shown recommendations: " + err.Error()
			}
		}
		return recommendResultsMsg{
			flowID:          flowID,
			recommendations: recs,
			hardware:        hw,
			query:           effectiveQuery,
			task:            effectiveTask,
			hardwareKey:     hardwareKey,
			warning:         warning,
		}
	}
}

func (m *Model) recommendInspectProbeCmd() tea.Cmd {
	if m.recommendFlow.selected < 0 || m.recommendFlow.selected >= len(m.recommendFlow.results) {
		return nil
	}
	rec := m.recommendFlow.results[m.recommendFlow.selected]
	flowID := m.recommendFlow.flowID
	selected := m.recommendFlow.selected
	cfg := m.cfg
	st := m.st
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	m.recommendFlow.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		details := recommendProbeDetails{
			URI:      rec.URI,
			Filename: firstNonEmptyString(rec.FileName, rec.Raw.FileName, rec.Raw.FilePath),
			Size:     rec.Size,
		}
		resolvedURL := rec.URI
		var headers map[string]string
		if recommendURINeedsResolve(rec.URI) {
			res, err := resolver.Resolve(ctx, rec.URI, cfg)
			if err != nil {
				return recommendInspectProbeMsg{flowID: flowID, selected: selected, uri: rec.URI, err: err}
			}
			resolvedURL = res.URL
			headers = res.Headers
			details.ResolvedURL = res.URL
			details.Filename = firstNonEmptyString(details.Filename, res.SuggestedFilename, res.FileName)
		} else {
			details.ResolvedURL = rec.URI
		}
		meta, err := downloader.ProbeURL(ctx, cfg, resolvedURL, headers)
		if err != nil {
			return recommendInspectProbeMsg{flowID: flowID, selected: selected, uri: rec.URI, err: err}
		}
		details.FinalURL = meta.FinalURL
		details.Filename = firstNonEmptyString(meta.Filename, details.Filename)
		if meta.Size > 0 {
			details.Size = meta.Size
		}
		details.ETag = meta.ETag
		details.LastModified = meta.LastModified
		details.AcceptRange = meta.AcceptRange
		if st != nil {
			host := downloader.HostFromURLForHistory(firstNonEmptyString(meta.FinalURL, resolvedURL))
			if host != "" {
				if row, ok, hErr := st.BestTransferHistory(host, "modfetch"); hErr == nil && ok {
					details.History = row
					details.HasHistory = true
				}
			}
		}
		return recommendInspectProbeMsg{flowID: flowID, selected: selected, uri: rec.URI, details: details}
	}
}

func recommendURINeedsResolve(uri string) bool {
	uri = strings.ToLower(strings.TrimSpace(uri))
	return strings.HasPrefix(uri, "hf://") ||
		strings.HasPrefix(uri, "huggingface://") ||
		strings.HasPrefix(uri, "civitai://") ||
		strings.HasPrefix(uri, "starter://")
}

func filterRecommendResults(recs []recommend.Recommendation, runtimeTarget string, sizeLimit int64) []recommend.Recommendation {
	runtimeTarget = strings.ToLower(strings.TrimSpace(runtimeTarget))
	out := make([]recommend.Recommendation, 0, len(recs))
	for _, rec := range recs {
		if sizeLimit > 0 && rec.Size > 0 && rec.Size > sizeLimit {
			continue
		}
		if runtimeTarget != "" && runtimeTarget != "any" && !recommendationMatchesRuntime(rec, runtimeTarget) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func recommendationMatchesRuntime(rec recommend.Recommendation, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, hint := range rec.RuntimeHints {
		runtimeName := strings.ToLower(strings.TrimSpace(hint.Runtime))
		preset := strings.ToLower(strings.TrimSpace(hint.PlacementPreset))
		if runtimeName == target || preset == target {
			return true
		}
		if target == "automatic1111" && strings.Contains(runtimeName, "automatic1111") {
			return true
		}
		if target == "lm studio" && runtimeName == "lm studio" {
			return true
		}
	}
	return false
}

func recommendPlacementPreset(rec recommend.Recommendation, target string) string {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" || target == "any" {
		return ""
	}
	for _, hint := range rec.RuntimeHints {
		runtimeName := strings.ToLower(strings.TrimSpace(hint.Runtime))
		preset := strings.ToLower(strings.TrimSpace(hint.PlacementPreset))
		if preset == "" {
			continue
		}
		if runtimeName == target || preset == target {
			return preset
		}
		if target == "automatic1111" && strings.Contains(runtimeName, "automatic1111") {
			return preset
		}
	}
	return ""
}

func (m *Model) startRecommendedDownload() tea.Cmd {
	m.cancelRecommendBackground()
	if m.st == nil {
		m.addToast("recommend: state database unavailable")
		return nil
	}
	if len(m.recommendFlow.results) == 0 {
		return nil
	}
	if m.recommendFlow.selected < 0 || m.recommendFlow.selected >= len(m.recommendFlow.results) {
		m.recommendFlow.selected = 0
	}
	rec := m.recommendFlow.results[m.recommendFlow.selected]
	task := recommend.NormalizeTask(m.recommendFlow.task)
	query := strings.TrimSpace(m.recommendFlow.query)
	if query == "" {
		query = recommend.DefaultQuery(task)
	}
	hardwareKey := strings.TrimSpace(m.recommendFlow.hardwareKey)
	if hardwareKey == "" {
		hardwareKey = recommend.HardwareKey(m.recommendFlow.hardware)
	}
	if err := recommend.RecordHistory(m.st, task, query, hardwareKey, m.recommendFlow.results, "selected", rec.Index); err != nil {
		m.addToast("warning: recommendation history failed: " + err.Error())
	} else if err := recommend.RecordHistory(m.st, task, query, hardwareKey, m.recommendFlow.results, "skipped", rec.Index); err != nil {
		m.addToast("warning: recommendation history failed: " + err.Error())
	}
	inspection := m.recommendLocalInspection(rec)
	artifactType := inspection.ArtifactType
	dest := inspection.Destination
	if inspection.PlacementPreset != "" && !inspection.PlacementConfigured {
		m.addToast("placement target not configured; saving to download root")
	}
	if err := m.preflightDest(dest); err != nil {
		m.addToast("dest not writable: " + err.Error())
		return nil
	}
	if err := m.st.UpsertDownload(state.DownloadRow{URL: rec.URI, Dest: dest, Status: "pending"}); err != nil {
		m.addToast("recommend start failed: " + err.Error())
		return nil
	}
	m.ensureDownloadMaps()
	key := rec.URI + "|" + dest
	if artifactType != "" {
		m.placeType[key] = artifactType
	}
	cmd := m.startDownloadCmd(rec.URI, dest, false, artifactType)
	m.addToast("started recommendation: " + truncateMiddle(filepath.Base(dest), 40))
	m.recommendFlow.active = false
	m.recommendFlow.input.Blur()
	return cmd
}

func (m *Model) recommendLocalInspection(rec recommend.Recommendation) recommendLocalInspection {
	task := recommend.NormalizeTask(m.recommendFlow.task)
	if task == "" {
		task = recommend.NormalizeTask(m.selectedRecommendTask())
	}
	runtimeTarget := m.selectedRecommendRuntime()
	artifactType := recommendArtifactType(rec, task, runtimeTarget)
	dest := m.computeDefaultDest(rec.URI)
	out := recommendLocalInspection{
		Destination:  dest,
		ArtifactType: artifactType,
	}
	preset := recommendPlacementPreset(rec, runtimeTarget)
	if preset == "" || artifactType == "" {
		out.PlacementNote = "no selected placement target; download root will be used"
		return out
	}
	out.PlacementPreset = preset
	if targetDest, ok := m.recommendPlacementDestination(rec.URI, artifactType, preset); ok {
		out.Destination = targetDest
		out.PlacementConfigured = true
		out.PlacementNote = "configured placement target"
		return out
	}
	out.PlacementNote = "placement target is not configured; download root will be used"
	return out
}

func (m *Model) recommendPlacementDestination(urlStr, artifactType, preset string) (string, bool) {
	if m.cfg == nil {
		return "", false
	}
	preset = strings.ToLower(strings.TrimSpace(preset))
	artifactType = strings.TrimSpace(artifactType)
	if preset == "" || artifactType == "" {
		return "", false
	}
	base := m.recommendDestinationBase(urlStr)
	for _, rule := range m.cfg.Placement.Mapping {
		if strings.TrimSpace(rule.Match) != artifactType {
			continue
		}
		for _, target := range rule.Targets {
			appName := strings.ToLower(strings.TrimSpace(target.App))
			if appName != preset {
				continue
			}
			app, ok := m.cfg.Placement.Apps[target.App]
			if !ok {
				return "", false
			}
			rel, ok := app.Paths[target.PathKey]
			if !ok {
				return "", false
			}
			return filepath.Join(app.Base, rel, base), true
		}
	}
	return "", false
}

func (m *Model) recommendDestinationBase(urlStr string) string {
	base := filepath.Base(m.computeDefaultDest(urlStr))
	if strings.TrimSpace(base) == "" || base == "." || base == string(filepath.Separator) {
		base = filepath.Base(m.immediateDestCandidate(urlStr))
	}
	if strings.TrimSpace(base) == "" || base == "." || base == string(filepath.Separator) {
		return "download"
	}
	return base
}

func (m *Model) ensureDownloadMaps() {
	if m.running == nil {
		m.running = map[string]context.CancelFunc{}
	}
	if m.placeType == nil {
		m.placeType = map[string]string{}
	}
	if m.autoPlace == nil {
		m.autoPlace = map[string]bool{}
	}
}

func recommendArtifactType(rec recommend.Recommendation, task, target string) string {
	ext := strings.ToLower(strings.TrimPrefix(rec.FileType, "."))
	if ext == "" {
		name := strings.ToLower(firstNonEmptyString(rec.FileName, rec.Raw.FileName, rec.Raw.FilePath))
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
			ext = strings.TrimPrefix(name[dot:], ".")
		}
	}
	switch ext {
	case "gguf":
		return "llm.gguf"
	case "safetensors":
		if recommend.NormalizeTask(task) == "image" || target == "comfyui" || target == "automatic1111" {
			return "sd.checkpoint"
		}
		return "llm.safetensors"
	case "bin", "pt", "pth":
		return "generic"
	default:
		return "generic"
	}
}

func (m *Model) selectedRecommendTask() string {
	return selectedRecommendChoiceValue(recommendTaskChoices(), m.recommendFlow.taskIndex, "chat")
}

func (m *Model) selectedRecommendProvider() string {
	return selectedRecommendChoiceValue(recommendProviderChoices(), m.recommendFlow.providerIndex, discovery.ProviderAll)
}

func (m *Model) selectedRecommendRuntime() string {
	return selectedRecommendChoiceValue(recommendRuntimeChoices(), m.recommendFlow.runtimeIndex, "any")
}

func (m *Model) selectedRecommendSizeLimit() int64 {
	choices := recommendSizeChoices()
	if m.recommendFlow.sizeIndex < 0 || m.recommendFlow.sizeIndex >= len(choices) {
		return 0
	}
	return choices[m.recommendFlow.sizeIndex].bytes
}

func (m *Model) selectedRecommendHardware() recommend.HardwareProfile {
	choices := m.recommendHardwareChoices()
	if m.recommendFlow.hardwareIndex <= 0 || m.recommendFlow.hardwareIndex >= len(choices) {
		return m.recommendFlow.hardware
	}
	selected := choices[m.recommendFlow.hardwareIndex]
	hw := m.recommendFlow.hardware
	hw.RAMBytes = selected.bytes
	hw.VRAMBytes = 0
	hw.UnifiedMemory = hw.OS == "darwin" && hw.Arch == "arm64"
	hw.Source = "tui-override"
	return hw
}

func selectedRecommendChoiceValue(choices []recommendChoice, index int, fallback string) string {
	if index < 0 || index >= len(choices) {
		return fallback
	}
	return choices[index].value
}

func recommendTaskChoices() []recommendChoice {
	return []recommendChoice{
		{label: "Chat / assistant", value: "chat", description: "general local assistant and writing"},
		{label: "Coding", value: "coding", description: "code generation, refactoring, and agent work"},
		{label: "Embeddings", value: "embedding", description: "search, RAG, and semantic indexes"},
		{label: "Image generation", value: "image", description: "Stable Diffusion, SDXL, Flux-style checkpoints"},
	}
}

func recommendProviderChoices() []recommendChoice {
	return []recommendChoice{
		{label: "All providers", value: discovery.ProviderAll, description: "search Hugging Face, CivitAI, and ModelScope"},
		{label: "Hugging Face", value: discovery.ProviderHuggingFace, description: "best for GGUF and general ML artifacts"},
		{label: "CivitAI", value: discovery.ProviderCivitAI, description: "best for Stable Diffusion artifacts"},
		{label: "ModelScope", value: discovery.ProviderModelScope, description: "alternate model catalog"},
	}
}

func recommendRuntimeChoices() []recommendChoice {
	return []recommendChoice{
		{label: "Any runtime", value: "any", description: "show every compatible result"},
		{label: "llama.cpp", value: "llama.cpp", description: "local GGUF runtimes and servers"},
		{label: "Ollama", value: "ollama", description: "GGUF import into Ollama models"},
		{label: "LM Studio", value: "lm studio", description: "desktop GGUF model loading"},
		{label: "MLX", value: "mlx", description: "Apple Silicon safetensors workflows"},
		{label: "ComfyUI", value: "comfyui", description: "image checkpoints and LoRAs"},
		{label: "AUTOMATIC1111/Forge", value: "automatic1111", description: "Stable Diffusion WebUI layouts"},
	}
}

func recommendSizeChoices() []recommendChoice {
	return []recommendChoice{
		{label: "Any size", value: "any", description: "rank by hardware fit only"},
		{label: "Up to 4 GiB", value: "4", description: "small first download", bytes: 4 << 30},
		{label: "Up to 8 GiB", value: "8", description: "typical 7B Q4/Q5 models", bytes: 8 << 30},
		{label: "Up to 16 GiB", value: "16", description: "larger 14B-ish local models", bytes: 16 << 30},
		{label: "Up to 32 GiB", value: "32", description: "larger local models and image checkpoints", bytes: 32 << 30},
		{label: "Up to 64 GiB", value: "64", description: "large unified-memory systems", bytes: 64 << 30},
	}
}

func (m *Model) recommendHardwareChoices() []recommendChoice {
	hw := m.recommendFlow.hardware
	detected := "Detected"
	var parts []string
	if hw.OS != "" || hw.Arch != "" {
		parts = append(parts, strings.Trim(strings.ToLower(hw.OS+"/"+hw.Arch), "/"))
	}
	if hw.RAMBytes > 0 {
		parts = append(parts, "RAM "+humanize.Bytes(uint64(hw.RAMBytes)))
	}
	if hw.VRAMBytes > 0 {
		parts = append(parts, "VRAM "+humanize.Bytes(uint64(hw.VRAMBytes)))
	}
	if hw.UnifiedMemory {
		parts = append(parts, "unified")
	}
	if len(parts) > 0 {
		detected += ": " + strings.Join(parts, ", ")
	}
	return []recommendChoice{
		{label: detected, value: "detected", description: "use this machine's detected hardware"},
		{label: "16 GiB RAM", value: "16", description: "override for smaller machines", bytes: 16 << 30},
		{label: "32 GiB RAM", value: "32", description: "mainstream local model box", bytes: 32 << 30},
		{label: "64 GiB RAM", value: "64", description: "larger model workstation", bytes: 64 << 30},
		{label: "128 GiB RAM", value: "128", description: "large unified-memory Mac or server", bytes: 128 << 30},
	}
}

func (m *Model) renderRecommendFlow() string {
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Recommend Models") + "\n")
	sb.WriteString(m.th.label.Render(m.renderRecommendSummary()) + "\n\n")
	if m.recommendFlow.loading {
		sb.WriteString(m.th.label.Render("Searching providers and applying local history...") + "\n")
		sb.WriteString(m.th.label.Render("Esc closes the panel and cancels the in-flight request."))
		return sb.String()
	}
	if m.recommendFlow.err != "" {
		sb.WriteString(m.th.bad.Render("Error: "+m.recommendFlow.err) + "\n\n")
	}
	switch m.recommendFlow.step {
	case recommendStepTask:
		sb.WriteString(m.th.label.Render("Choose the job this model should do.") + "\n\n")
		sb.WriteString(m.renderRecommendChoices(recommendTaskChoices(), m.recommendFlow.taskIndex))
	case recommendStepHardware:
		sb.WriteString(m.th.label.Render("Choose the hardware budget to rank against.") + "\n\n")
		sb.WriteString(m.renderRecommendChoices(m.recommendHardwareChoices(), m.recommendFlow.hardwareIndex))
	case recommendStepProvider:
		sb.WriteString(m.th.label.Render("Choose where to search.") + "\n\n")
		sb.WriteString(m.renderRecommendChoices(recommendProviderChoices(), m.recommendFlow.providerIndex))
	case recommendStepRuntime:
		sb.WriteString(m.th.label.Render("Choose a runtime or placement target.") + "\n\n")
		sb.WriteString(m.renderRecommendChoices(recommendRuntimeChoices(), m.recommendFlow.runtimeIndex))
	case recommendStepSize:
		sb.WriteString(m.th.label.Render("Choose the maximum file size.") + "\n\n")
		sb.WriteString(m.renderRecommendChoices(recommendSizeChoices(), m.recommendFlow.sizeIndex))
	case recommendStepQuery:
		sb.WriteString(m.th.label.Render("Search terms are optional; empty uses the task default.") + "\n")
		sb.WriteString(m.th.label.Render("Default: "+recommend.DefaultQuery(m.selectedRecommendTask())) + "\n\n")
		sb.WriteString(m.recommendFlow.input.View())
	case recommendStepResults:
		sb.WriteString(m.renderRecommendResults())
	}
	backHint := "Shift+Tab/Left"
	if m.recommendFlow.step == recommendStepQuery {
		backHint = "Shift+Tab"
	}
	sb.WriteString("\n")
	if m.recommendFlow.step == recommendStepResults {
		sb.WriteString(m.th.label.Render("j/k: choose  •  i: details  •  p: probe  •  Enter: start  •  "+backHint+": back  •  Esc: close") + "\n")
	} else {
		sb.WriteString(m.th.label.Render("j/k: choose  •  Enter: continue/start  •  "+backHint+": back  •  Esc: close") + "\n")
	}
	return sb.String()
}

func (m *Model) renderRecommendChoices(choices []recommendChoice, selected int) string {
	var sb strings.Builder
	for i, choice := range choices {
		prefix := "  "
		if i == selected {
			prefix = "▸ "
		}
		line := fmt.Sprintf("%s%-24s %s", prefix, choice.label, choice.description)
		if i == selected {
			sb.WriteString(m.th.ok.Render(line) + "\n")
		} else {
			sb.WriteString(m.th.label.Render(line) + "\n")
		}
	}
	return sb.String()
}

func (m *Model) renderRecommendResults() string {
	var sb strings.Builder
	if len(m.recommendFlow.results) == 0 {
		sb.WriteString(m.th.label.Render("No recommendations matched the selected filters.") + "\n")
		sb.WriteString(m.th.label.Render("Go back and loosen the runtime, size, or provider filter.") + "\n")
		return sb.String()
	}
	sb.WriteString(m.th.label.Render("Select a model to start the normal resumable download path. Press i for details or p to probe.") + "\n\n")
	for i, rec := range m.recommendFlow.results {
		prefix := "  "
		if i == m.recommendFlow.selected {
			prefix = "▸ "
		}
		size := "-"
		if rec.Size > 0 {
			size = humanize.Bytes(uint64(rec.Size))
		}
		meta := []string{rec.Provider, "fit=" + rec.Fit, "score=" + fmt.Sprint(rec.Score), "size=" + size}
		if rec.ParameterCount != "" {
			meta = append(meta, rec.ParameterCount)
		}
		if rec.Quantization != "" {
			meta = append(meta, rec.Quantization)
		}
		line := fmt.Sprintf("%s%d. %s", prefix, rec.Index, truncateMiddle(rec.Name, 44))
		detail := "    " + strings.Join(meta, " • ")
		if i == m.recommendFlow.selected {
			sb.WriteString(m.th.ok.Render(line) + "\n")
			sb.WriteString(m.th.label.Render(detail) + "\n")
			if len(rec.RuntimeHints) > 0 {
				sb.WriteString(m.th.label.Render("    runtimes: "+renderRuntimeHints(rec.RuntimeHints)) + "\n")
			}
			if len(rec.Reasons) > 0 {
				sb.WriteString(m.th.label.Render("    why: "+truncateMiddle(strings.Join(rec.Reasons, "; "), 96)) + "\n")
			}
			sb.WriteString(m.th.label.Render("    uri: "+truncateMiddle(rec.URI, 96)) + "\n")
		} else {
			sb.WriteString(m.th.label.Render(line+"  "+strings.Join(meta, " • ")) + "\n")
		}
	}
	if m.recommendFlow.inspect {
		selected := m.recommendFlow.selected
		if selected < 0 || selected >= len(m.recommendFlow.results) {
			selected = 0
		}
		sb.WriteString(m.renderRecommendInspection(m.recommendFlow.results[selected]))
	}
	return sb.String()
}

func (m *Model) renderRecommendInspection(rec recommend.Recommendation) string {
	local := m.recommendLocalInspection(rec)
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(m.th.head.Render("Details") + "\n")
	sb.WriteString(m.th.label.Render("  model: "+firstNonEmptyString(rec.Name, rec.ModelID)) + "\n")
	sb.WriteString(m.th.label.Render("  file: "+firstNonEmptyString(rec.FileName, rec.Raw.FileName, rec.Raw.FilePath, "-")) + "\n")
	sb.WriteString(m.th.label.Render("  provider: "+rec.Provider+"  model-id: "+firstNonEmptyString(rec.ModelID, "-")) + "\n")
	sb.WriteString(m.th.label.Render("  score: "+fmt.Sprint(rec.Score)+"  fit: "+rec.Fit+"  size: "+recommendBytes(rec.Size)) + "\n")
	if rec.EstimatedRequired > 0 || rec.MemoryBudget > 0 {
		sb.WriteString(m.th.label.Render("  memory: needs "+recommendBytes(rec.EstimatedRequired)+" of "+recommendBytes(rec.MemoryBudget)+" budget") + "\n")
	}
	if rec.ParameterCount != "" || rec.Quantization != "" {
		sb.WriteString(m.th.label.Render("  model shape: params="+firstNonEmptyString(rec.ParameterCount, "-")+" quant="+firstNonEmptyString(rec.Quantization, "-")) + "\n")
	}
	if rec.Downloads > 0 || rec.Likes > 0 {
		sb.WriteString(m.th.label.Render(fmt.Sprintf("  popularity: downloads=%d likes=%d", rec.Downloads, rec.Likes)) + "\n")
	}
	if len(rec.Reasons) > 0 {
		sb.WriteString(m.th.label.Render("  rationale:") + "\n")
		for _, reason := range rec.Reasons {
			sb.WriteString(m.th.label.Render("    - "+reason) + "\n")
		}
	}
	if len(rec.RuntimeHints) > 0 {
		sb.WriteString(m.th.label.Render("  runtime setup:") + "\n")
		for _, hint := range rec.RuntimeHints {
			line := "    - " + hint.Runtime
			if hint.PlacementPreset != "" {
				line += " -> " + hint.PlacementPreset
			}
			if hint.Reason != "" {
				line += ": " + hint.Reason
			}
			sb.WriteString(m.th.label.Render(line) + "\n")
			if cmd := recommendRuntimeSetupCommand(hint, local.Destination); cmd != "" {
				sb.WriteString(m.th.label.Render("      setup: "+cmd) + "\n")
			}
		}
	}
	sb.WriteString(m.th.label.Render("  placement: "+local.PlacementNote) + "\n")
	if local.PlacementPreset != "" {
		sb.WriteString(m.th.label.Render("    preset: "+local.PlacementPreset+"  artifact: "+local.ArtifactType) + "\n")
	}
	sb.WriteString(m.th.label.Render("  dry-run transfer:") + "\n")
	sb.WriteString(m.th.label.Render("    dest: "+truncateMiddle(local.Destination, 100)) + "\n")
	sb.WriteString(m.th.label.Render("    "+m.recommendTransferSettings()) + "\n")
	if m.recommendFlow.probeLoading {
		sb.WriteString(m.th.label.Render("    live metadata: probing selected URL...") + "\n")
	} else if m.recommendFlow.probeErr != "" {
		sb.WriteString(m.th.bad.Render("    live metadata failed: "+m.recommendFlow.probeErr) + "\n")
	} else if m.recommendFlow.probeDetails.URI == rec.URI {
		sb.WriteString(m.renderRecommendProbeDetails(m.recommendFlow.probeDetails))
	} else {
		sb.WriteString(m.th.label.Render("    live metadata: press p to resolve URL, check size, range support, and history") + "\n")
	}
	return sb.String()
}

func (m *Model) renderRecommendProbeDetails(details recommendProbeDetails) string {
	var sb strings.Builder
	sb.WriteString(m.th.label.Render("    resolved: "+truncateMiddle(firstNonEmptyString(details.ResolvedURL, details.URI), 100)) + "\n")
	if details.FinalURL != "" && details.FinalURL != details.ResolvedURL {
		sb.WriteString(m.th.label.Render("    final: "+truncateMiddle(details.FinalURL, 100)) + "\n")
	}
	sb.WriteString(m.th.label.Render("    remote size: "+recommendBytes(details.Size)+"  ranges: "+yesNo(details.AcceptRange)) + "\n")
	if details.Filename != "" {
		sb.WriteString(m.th.label.Render("    server filename: "+details.Filename) + "\n")
	}
	if details.ETag != "" || details.LastModified != "" {
		sb.WriteString(m.th.label.Render("    validators: etag="+firstNonEmptyString(details.ETag, "-")+" last-modified="+firstNonEmptyString(details.LastModified, "-")) + "\n")
	}
	if details.HasHistory {
		sb.WriteString(m.th.label.Render(fmt.Sprintf("    prior host speed: %s/s over %d sample(s), connections=%d chunk=%dMiB status=%s",
			humanize.Bytes(uint64(details.History.AvgBPS)), details.History.Samples, details.History.Connections, details.History.ChunkSizeMB, details.History.LastStatus)) + "\n")
	}
	return sb.String()
}

func recommendRuntimeSetupCommand(hint recommend.RuntimeHint, dest string) string {
	if strings.TrimSpace(hint.SetupCommand) != "" {
		return hint.SetupCommand
	}
	switch strings.ToLower(strings.TrimSpace(hint.Runtime)) {
	case "llama.cpp":
		return "llama-cli -m " + strconv.Quote(dest) + " -p \"Hello\""
	case "lm studio":
		return "Open LM Studio and add " + dest
	default:
		return ""
	}
}

func (m *Model) recommendTransferSettings() string {
	if m.cfg == nil {
		return "transfer: config unavailable"
	}
	connections := "default"
	if m.cfg.Concurrency.PerFileChunks > 0 {
		connections = fmt.Sprintf("%d", m.cfg.Concurrency.PerFileChunks)
	}
	chunk := "auto"
	if m.cfg.Concurrency.ChunkSizeMB > 0 {
		chunk = fmt.Sprintf("%d MiB", m.cfg.Concurrency.ChunkSizeMB)
	}
	return "transfer: connections=" + connections + " chunk=" + chunk + " profile=auto/adaptive"
}

func recommendBytes(size int64) string {
	if size <= 0 {
		return "unknown"
	}
	return humanize.Bytes(uint64(size))
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func renderRuntimeHints(hints []recommend.RuntimeHint) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		value := hint.Runtime
		if hint.PlacementPreset != "" {
			value += " -> " + hint.PlacementPreset
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "; ")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (m *Model) renderRecommendSummary() string {
	step := "task"
	switch m.recommendFlow.step {
	case recommendStepHardware:
		step = "hardware"
	case recommendStepProvider:
		step = "provider"
	case recommendStepRuntime:
		step = "runtime"
	case recommendStepSize:
		step = "size"
	case recommendStepQuery:
		step = "query"
	case recommendStepResults:
		step = "results"
	}
	query := strings.TrimSpace(m.recommendFlow.query)
	if query == "" {
		query = recommend.DefaultQuery(m.selectedRecommendTask())
	}
	size := "any"
	if b := m.selectedRecommendSizeLimit(); b > 0 {
		size = humanize.Bytes(uint64(b))
	}
	return fmt.Sprintf("Step: %s • task=%s • source=%s • runtime=%s • max-size=%s • query=%s",
		step, m.selectedRecommendTask(), m.selectedRecommendProvider(), m.selectedRecommendRuntime(), size, query)
}
