package classifier

import (
	"path/filepath"
	"strings"
)

// Artifact types
// sd.checkpoint, sd.lora, sd.vae, sd.controlnet, sd.embedding,
// llm.gguf, llm.safetensors, generic

func Detect(filePath string) string {
	name := strings.ToLower(filepath.Base(filePath))
	ext := strings.ToLower(filepath.Ext(name))
	switch ext := ext; ext {
	case ".gguf":
		return "llm.gguf"
	case ".ckpt":
		return "sd.checkpoint"
	case ".safetensors":
		// heuristics based on name
		if strings.Contains(name, "lora") || strings.Contains(name, "lycoris") || strings.Contains(name, "locon") {
			return "sd.lora"
		}
		if strings.Contains(name, "vae") {
			return "sd.vae"
		}
		if strings.Contains(name, "controlnet") {
			return "sd.controlnet"
		}
		// Ambiguous: default to sd.checkpoint for SD ecosystem, can be overridden
		return "sd.checkpoint"
	case ".pt":
		if strings.Contains(name, "embed") || strings.Contains(name, "embedding") { return "sd.embedding" }
		return "generic"
	default:
		return "generic"
	}
}

