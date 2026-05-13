package recommend

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/jxwalker/modfetch/internal/discovery"
)

type HardwareProfile struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CPU           string `json:"cpu,omitempty"`
	RAMBytes      int64  `json:"ram_bytes,omitempty"`
	VRAMBytes     int64  `json:"vram_bytes,omitempty"`
	UnifiedMemory bool   `json:"unified_memory,omitempty"`
	Source        string `json:"source,omitempty"`
}

type Options struct {
	Query    string
	Task     string
	Provider string
	Limit    int
	Hardware HardwareProfile
	Feedback map[string]Feedback
}

type Feedback struct {
	Selected int
	Skipped  int
	Shown    int
}

type RuntimeHint struct {
	Runtime         string `json:"runtime"`
	Reason          string `json:"reason"`
	PlacementPreset string `json:"placement_preset,omitempty"`
	SetupCommand    string `json:"setup_command,omitempty"`
}

type Recommendation struct {
	Index             int              `json:"index"`
	Provider          string           `json:"provider"`
	ModelID           string           `json:"model_id"`
	Name              string           `json:"name"`
	URI               string           `json:"uri"`
	FileName          string           `json:"file_name,omitempty"`
	FileType          string           `json:"file_type,omitempty"`
	Size              int64            `json:"size,omitempty"`
	Downloads         int64            `json:"downloads,omitempty"`
	Likes             int64            `json:"likes,omitempty"`
	Score             int              `json:"score"`
	Fit               string           `json:"fit"`
	EstimatedRequired int64            `json:"estimated_required_bytes,omitempty"`
	MemoryBudget      int64            `json:"memory_budget_bytes,omitempty"`
	ParameterCount    string           `json:"parameter_count,omitempty"`
	Quantization      string           `json:"quantization,omitempty"`
	RuntimeHints      []RuntimeHint    `json:"runtime_hints,omitempty"`
	Reasons           []string         `json:"reasons,omitempty"`
	DownloadCommand   string           `json:"download_command"`
	Raw               discovery.Result `json:"raw,omitempty"`
}

func DetectHardware(ctx context.Context) HardwareProfile {
	hw := HardwareProfile{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Source: "runtime",
	}
	switch runtime.GOOS {
	case "darwin":
		hw.Source = "sysctl"
		hw.RAMBytes = readDarwinInt(ctx, "hw.memsize")
		hw.CPU = readDarwinString(ctx, "machdep.cpu.brand_string")
		if runtime.GOARCH == "arm64" {
			hw.UnifiedMemory = true
		}
	case "linux":
		hw.Source = "procfs"
		hw.RAMBytes = readLinuxMemTotal()
		hw.CPU = readLinuxCPU()
	}
	return hw
}

func Recommend(ctx context.Context, opts Options) ([]Recommendation, HardwareProfile, error) {
	hw := opts.Hardware
	if hw.OS == "" && hw.Arch == "" && hw.RAMBytes == 0 && hw.VRAMBytes == 0 {
		hw = DetectHardware(ctx)
	}
	query := strings.TrimSpace(opts.Query)
	task := NormalizeTask(opts.Task)
	if query == "" {
		query = DefaultQuery(task)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}
	searchLimit := limit * 3
	if searchLimit < 8 {
		searchLimit = 8
	}
	if searchLimit > 25 {
		searchLimit = 25
	}
	provider := strings.TrimSpace(opts.Provider)
	if provider == "" {
		provider = discovery.ProviderHuggingFace
	}
	results, err := discovery.Search(ctx, discovery.Options{Provider: provider, Query: query, Limit: searchLimit})
	if err != nil {
		return nil, hw, err
	}
	ranked := Rank(results, hw, task)
	ApplyFeedback(ranked, opts.Feedback)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	for i := range ranked {
		ranked[i].Index = i + 1
	}
	return ranked, hw, nil
}

func ApplyFeedback(recs []Recommendation, feedback map[string]Feedback) {
	if len(recs) == 0 || len(feedback) == 0 {
		return
	}
	for i := range recs {
		fb, ok := feedback[FeedbackKey(recs[i].URI)]
		if !ok {
			continue
		}
		delta := fb.Selected*18 - fb.Skipped*10
		if delta > 54 {
			delta = 54
		}
		if delta < -40 {
			delta = -40
		}
		if delta == 0 && fb.Shown > 0 {
			delta = -2
		}
		recs[i].Score += delta
		switch {
		case fb.Selected > 0 && fb.Skipped > 0:
			recs[i].Reasons = append(recs[i].Reasons, fmt.Sprintf("learned from prior selections (%d selected, %d skipped)", fb.Selected, fb.Skipped))
		case fb.Selected > 0:
			recs[i].Reasons = append(recs[i].Reasons, fmt.Sprintf("learned from %d prior selection(s)", fb.Selected))
		case fb.Skipped > 0:
			recs[i].Reasons = append(recs[i].Reasons, fmt.Sprintf("previously skipped %d time(s)", fb.Skipped))
		}
	}
	sort.SliceStable(recs, func(i, j int) bool { return betterRecommendation(recs[i], recs[j]) })
}

func FeedbackKey(uri string) string {
	return strings.TrimSpace(uri)
}

func Rank(results []discovery.Result, hw HardwareProfile, task string) []Recommendation {
	task = NormalizeTask(task)
	out := make([]Recommendation, 0, len(results))
	for _, result := range results {
		rec := scoreResult(result, hw, task)
		if rec.URI == "" {
			continue
		}
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return betterRecommendation(out[i], out[j]) })
	for i := range out {
		out[i].Index = i + 1
	}
	return out
}

func NormalizeTask(task string) string {
	switch strings.ToLower(strings.TrimSpace(task)) {
	case "", "chat", "general", "assistant":
		return "chat"
	case "code", "coding", "programming", "developer":
		return "coding"
	case "embed", "embedding", "embeddings":
		return "embedding"
	case "image", "sd", "stable-diffusion":
		return "image"
	default:
		return strings.ToLower(strings.TrimSpace(task))
	}
}

func betterRecommendation(a, b Recommendation) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Fit != b.Fit {
		return fitRank(a.Fit) > fitRank(b.Fit)
	}
	return a.Downloads > b.Downloads
}

func DefaultQuery(task string) string {
	switch NormalizeTask(task) {
	case "coding":
		return "qwen coder gguf"
	case "embedding":
		return "embedding model"
	case "image":
		return "stable diffusion safetensors"
	default:
		return "llama instruct gguf"
	}
}

func MemoryBudgetBytes(hw HardwareProfile) int64 {
	if hw.VRAMBytes > 0 && !hw.UnifiedMemory {
		return int64(float64(hw.VRAMBytes) * 0.88)
	}
	if hw.RAMBytes <= 0 {
		return 0
	}
	if hw.UnifiedMemory {
		return int64(float64(hw.RAMBytes) * 0.72)
	}
	return int64(float64(hw.RAMBytes) * 0.55)
}

func scoreResult(result discovery.Result, hw HardwareProfile, task string) Recommendation {
	text := strings.Join([]string{result.Name, result.ModelID, result.FileName, result.FilePath, strings.Join(result.Tags, " "), result.Pipeline, result.Description}, " ")
	params := inferParameterCount(text)
	quant := inferQuantization(text)
	required := estimateRequiredBytes(result.Size, params, quant)
	budget := MemoryBudgetBytes(hw)
	fit := fitStatus(required, budget)
	score := 0
	var reasons []string

	switch fit {
	case "excellent":
		score += 35
		reasons = append(reasons, "comfortable memory fit")
	case "good":
		score += 28
		reasons = append(reasons, "good memory fit")
	case "tight":
		score += 12
		reasons = append(reasons, "fits, but leaves limited headroom")
	case "too_large":
		score -= 60
		reasons = append(reasons, "likely too large for this hardware")
	default:
		reasons = append(reasons, "unknown size; fit is estimated from metadata")
	}

	score += taskScore(text, task, &reasons)
	score += formatScore(result.FileType, result.FileName, &reasons)
	if isSplitShard(text) {
		score -= 80
		reasons = append(reasons, "multi-part shard; prefer a single complete artifact for beginner downloads")
	}
	score += quantScore(quant, fit, &reasons)
	score += parameterScore(params, fit, &reasons)
	score += popularityScore(result.Downloads, result.Likes)

	if result.Provider == discovery.ProviderHuggingFace {
		score += 8
	}
	if result.URI == "" {
		score -= 100
	}

	name := strings.TrimSpace(result.Name)
	if name == "" {
		name = result.ModelID
	}
	return Recommendation{
		Provider:          result.Provider,
		ModelID:           result.ModelID,
		Name:              name,
		URI:               result.URI,
		FileName:          firstNonEmpty(result.FileName, pathBase(result.FilePath)),
		FileType:          result.FileType,
		Size:              result.Size,
		Downloads:         result.Downloads,
		Likes:             result.Likes,
		Score:             score,
		Fit:               fit,
		EstimatedRequired: required,
		MemoryBudget:      budget,
		ParameterCount:    params,
		Quantization:      quant,
		RuntimeHints:      runtimeHints(result, task, hw),
		Reasons:           reasons,
		DownloadCommand:   "modfetch download --url " + strconv.Quote(result.URI) + " --profile auto",
		Raw:               result,
	}
}

func runtimeHints(result discovery.Result, task string, hw HardwareProfile) []RuntimeHint {
	ext := strings.ToLower(strings.TrimPrefix(result.FileType, "."))
	if ext == "" {
		name := strings.ToLower(firstNonEmpty(result.FileName, result.FilePath))
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
			ext = strings.TrimPrefix(name[dot:], ".")
		}
	}
	switch ext {
	case "gguf":
		return []RuntimeHint{
			{Runtime: "llama.cpp", Reason: "GGUF is directly runnable by llama.cpp-style runtimes", PlacementPreset: "hf-cache"},
			{Runtime: "Ollama", Reason: "GGUF can be imported with a Modelfile", PlacementPreset: "ollama", SetupCommand: "ollama create NAME -f Modelfile"},
			{Runtime: "LM Studio", Reason: "LM Studio can load local GGUF files"},
		}
	case "safetensors":
		if NormalizeTask(task) == "image" || containsAny(strings.ToLower(strings.Join(result.Tags, " ")), "diffusion", "sdxl", "lora") {
			return []RuntimeHint{
				{Runtime: "ComfyUI", Reason: "safetensors image checkpoints and LoRAs belong under ComfyUI model folders", PlacementPreset: "comfyui"},
				{Runtime: "AUTOMATIC1111/Forge", Reason: "safetensors image artifacts are supported by Stable Diffusion WebUI layouts", PlacementPreset: "automatic1111"},
			}
		}
		hints := []RuntimeHint{
			{Runtime: "Transformers", Reason: "safetensors is the preferred safe weight format for Transformers models", PlacementPreset: "hf-cache"},
			{Runtime: "vLLM", Reason: "many safetensors causal language models can be served by vLLM"},
		}
		if hw.OS == "darwin" && hw.Arch == "arm64" {
			hints = append(hints, RuntimeHint{Runtime: "MLX", Reason: "many Hugging Face safetensors language models can be converted for Apple Silicon with mlx-lm", SetupCommand: "mlx_lm.convert --hf-path REPO --mlx-path OUT"})
		}
		return hints
	case "bin", "pt", "pth":
		return []RuntimeHint{
			{Runtime: "Transformers", Reason: "PyTorch-format weights usually require a Transformers-compatible repo layout", PlacementPreset: "hf-cache"},
		}
	case "onnx":
		return []RuntimeHint{
			{Runtime: "ONNX Runtime", Reason: "ONNX artifacts are designed for ONNX Runtime inference"},
		}
	default:
		return nil
	}
}

func taskScore(text, task string, reasons *[]string) int {
	lower := strings.ToLower(text)
	switch NormalizeTask(task) {
	case "coding":
		if containsAny(lower, "coder", "coding", "code", "starcoder", "deepseek", "qwen") {
			*reasons = append(*reasons, "coding-oriented model signals")
			return 24
		}
		return -4
	case "embedding":
		if containsAny(lower, "embed", "embedding", "bge", "gte", "e5") {
			*reasons = append(*reasons, "embedding model signals")
			return 28
		}
		return -20
	case "image":
		if containsAny(lower, "stable-diffusion", "sdxl", "flux", "diffusion", "checkpoint") {
			*reasons = append(*reasons, "image model signals")
			return 24
		}
		return -12
	default:
		if containsAny(lower, "instruct", "chat", "llama", "qwen", "mistral", "gemma") {
			*reasons = append(*reasons, "general chat/instruct signals")
			return 16
		}
	}
	return 0
}

func formatScore(fileType, fileName string, reasons *[]string) int {
	ext := strings.ToLower(strings.TrimPrefix(fileType, "."))
	name := strings.ToLower(fileName)
	if ext == "" {
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
			ext = strings.TrimPrefix(name[dot:], ".")
		}
	}
	switch ext {
	case "gguf":
		*reasons = append(*reasons, "GGUF is ready for local llama.cpp-style runtimes")
		return 18
	case "safetensors":
		return 8
	case "bin", "pt", "pth":
		return -4
	default:
		return 0
	}
}

func quantScore(quant, fit string, reasons *[]string) int {
	switch strings.ToUpper(quant) {
	case "Q4_K_M", "Q5_K_M":
		*reasons = append(*reasons, "strong quality/size quantization")
		return 18
	case "Q4_0", "Q4_1", "Q5_0", "Q5_1", "Q6_K":
		return 12
	case "Q8_0":
		if fit == "excellent" || fit == "good" {
			return 10
		}
		return -5
	case "Q2_K", "Q3_K_S":
		return -16
	case "FP16", "F16", "BF16":
		if fit == "excellent" {
			return 8
		}
		return -12
	}
	return 0
}

func parameterScore(params, fit string, reasons *[]string) int {
	b := parseBillions(params)
	if b <= 0 || fit == "too_large" {
		return 0
	}
	switch {
	case b >= 30:
		*reasons = append(*reasons, "large parameter count that still fits")
		return 18
	case b >= 13:
		return 14
	case b >= 7:
		return 10
	case b >= 3:
		return 4
	default:
		return 0
	}
}

func popularityScore(downloads, likes int64) int {
	score := 0
	if downloads > 0 {
		score += int(math.Min(18, math.Log10(float64(downloads)+1)*4))
	}
	if likes > 0 {
		score += int(math.Min(8, math.Log10(float64(likes)+1)*2))
	}
	return score
}

func fitStatus(required, budget int64) string {
	if required <= 0 || budget <= 0 {
		return "unknown"
	}
	ratio := float64(required) / float64(budget)
	switch {
	case ratio <= 0.60:
		return "excellent"
	case ratio <= 0.85:
		return "good"
	case ratio <= 1.00:
		return "tight"
	default:
		return "too_large"
	}
}

func estimateRequiredBytes(size int64, params, quant string) int64 {
	if size > 0 {
		return int64(float64(size) * 1.25)
	}
	b := parseBillions(params)
	if b <= 0 {
		return 0
	}
	bits := quantBits(quant)
	if bits <= 0 {
		bits = 8
	}
	weights := b * 1_000_000_000 * (float64(bits) / 8.0)
	return int64(weights*1.20) + 1_000_000_000
}

func quantBits(quant string) int {
	q := strings.ToUpper(quant)
	switch {
	case strings.HasPrefix(q, "Q2"):
		return 2
	case strings.HasPrefix(q, "Q3"):
		return 3
	case strings.HasPrefix(q, "Q4"), q == "4BIT":
		return 4
	case strings.HasPrefix(q, "Q5"):
		return 5
	case strings.HasPrefix(q, "Q6"):
		return 6
	case strings.HasPrefix(q, "Q8"), q == "8BIT":
		return 8
	case q == "FP16" || q == "F16" || q == "BF16":
		return 16
	case q == "FP32" || q == "F32":
		return 32
	default:
		return 0
	}
}

var (
	paramPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*[-_ ]?b\b`)
	quantPattern = regexp.MustCompile(`(?i)\b(q[2-8](?:[_-]k)?(?:[_-][sm])?|q[2-8][_ -]?[01]|fp16|fp32|f16|f32|bf16|4bit|8bit|awq|gptq)\b`)
	shardPattern = regexp.MustCompile(`(?i)(?:^|[-_.])\d{5}-of-\d{5}(?:[-_.]|$)`)
)

func inferParameterCount(text string) string {
	m := paramPattern.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(m[1] + "B"))
}

func inferQuantization(text string) string {
	m := quantPattern.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(m[1]))
}

func isSplitShard(text string) bool {
	return shardPattern.MatchString(text)
}

func parseBillions(params string) float64 {
	params = strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(params)), "B")
	v, err := strconv.ParseFloat(params, 64)
	if err != nil {
		return 0
	}
	return v
}

func fitRank(fit string) int {
	switch fit {
	case "excellent":
		return 4
	case "good":
		return 3
	case "tight":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}

func readDarwinInt(ctx context.Context, key string) int64 {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", key).Output()
	if err != nil {
		return 0
	}
	value, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return value
}

func readDarwinString(ctx context.Context, key string) string {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readLinuxMemTotal() int64 {
	body, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, _ := strconv.ParseInt(fields[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

func readLinuxCPU() string {
	body, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if key, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func pathBase(p string) string {
	p = strings.TrimRight(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func FormatBytes(n int64) string {
	if n <= 0 {
		return "-"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
