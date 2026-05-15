package main

import (
	"fmt"
	neturl "net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type runHelpHint struct {
	Runtime         string `json:"runtime"`
	Command         string `json:"command,omitempty"`
	Note            string `json:"note,omitempty"`
	PlacementPreset string `json:"placement_preset,omitempty"`
}

func runHelpHintsForPath(path string) []runHelpHint {
	if runHelpRemoteURI(path) {
		return []runHelpHint{
			{
				Runtime: "Local runtime",
				Note:    "Remote destinations are not directly runnable; sync or download this artifact to a local filesystem path before launching a local runtime.",
			},
		}
	}
	base := runHelpArtifactName(path)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), "."))
	switch ext {
	case "gguf":
		name := runHelpModelName(base)
		quoted := shellQuote(path)
		return []runHelpHint{
			{
				Runtime: "llama.cpp",
				Command: "llama-cli -m " + quoted + " -p " + shellQuote("Hello from modfetch"),
				Note:    "Runs the GGUF directly from the downloaded path.",
			},
			{
				Runtime:         "Ollama",
				Command:         "printf 'FROM %s\\n' " + quoted + " | ollama create " + name + " -f - && ollama run " + name,
				PlacementPreset: "ollama",
				Note:            "Streams a one-line Modelfile to Ollama without moving the GGUF.",
			},
			{
				Runtime: "LM Studio",
				Note:    "Open LM Studio, choose a local model, and select " + path + ".",
			},
		}
	case "safetensors":
		quoted := shellQuote(path)
		imageHints := []runHelpHint{
			{
				Runtime:         "ComfyUI",
				Command:         "modfetch place --path " + quoted + " --preset comfyui",
				PlacementPreset: "comfyui",
				Note:            "Use this for image checkpoints, LoRAs, and diffusion model assets.",
			},
			{
				Runtime:         "AUTOMATIC1111/Forge",
				Command:         "modfetch place --path " + quoted + " --preset automatic1111",
				PlacementPreset: "automatic1111",
				Note:            "Places the file into a Stable Diffusion WebUI-compatible model folder.",
			},
		}
		textHint := runHelpHint{
			Runtime: "Transformers",
			Note:    "For text models, prefer a full repository snapshot with config and tokenizer files; a single safetensors file may not be runnable alone.",
		}
		if looksLikeImageSafetensors(base) {
			return append(imageHints, textHint)
		}
		return append([]runHelpHint{textHint}, imageHints...)
	case "onnx":
		return []runHelpHint{
			{
				Runtime: "ONNX Runtime",
				Note:    "Load this file with onnxruntime.InferenceSession from your application code.",
			},
		}
	case "bin", "pt", "pth":
		return []runHelpHint{
			{
				Runtime: "Transformers",
				Note:    "PyTorch-format weights usually need the matching repository config and tokenizer files before local inference.",
			},
		}
	default:
		return []runHelpHint{}
	}
}

func runHelpArtifactName(path string) string {
	trimmed := strings.TrimSpace(path)
	if u, err := neturl.Parse(trimmed); err == nil && len(u.Scheme) > 1 && u.Path != "" {
		return filepath.Base(u.Path)
	}
	return filepath.Base(trimmed)
}

func runHelpRemoteURI(path string) bool {
	u, err := neturl.Parse(strings.TrimSpace(path))
	if err != nil || len(u.Scheme) <= 1 {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "s3", "gs", "az", "http", "https":
		return true
	default:
		return false
	}
}

func looksLikeImageSafetensors(name string) bool {
	lower := strings.ToLower(name)
	for _, signal := range []string{"image", "sdxl", "stable-diffusion", "diffusion", "checkpoint", "lora", "controlnet", "flux", "unet", "vae"} {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

func printRunHelp(path string, hints []runHelpHint) {
	fmt.Println()
	fmt.Println("Run it locally:")
	if len(hints) == 0 {
		fmt.Printf("  No runtime guidance is available for %s yet.\n", path)
		return
	}
	for _, hint := range hints {
		label := hint.Runtime
		if hint.PlacementPreset != "" {
			label += " (" + hint.PlacementPreset + ")"
		}
		fmt.Println("  " + label + ":")
		if hint.Command != "" {
			fmt.Println("    " + hint.Command)
		}
		if hint.Note != "" {
			fmt.Println("    " + hint.Note)
		}
	}
}

func runHelpModelName(name string) string {
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	name = strings.ToLower(name)
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		keep := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
		if keep {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-_.")
	out = multiDash.ReplaceAllString(out, "-")
	if out == "" {
		return "modfetch-model"
	}
	return out
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

var multiDash = regexp.MustCompile(`-+`)
