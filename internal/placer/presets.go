package placer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
)

type Preset struct {
	Name        string
	Description string
	Apps        map[string]config.AppPlacement
	Mapping     []config.MappingRule
}

var presetCatalog = map[string]Preset{
	"automatic1111": {
		Name:        "automatic1111",
		Description: "AUTOMATIC1111 Stable Diffusion WebUI layout",
		Apps: map[string]config.AppPlacement{
			"automatic1111": {
				Base: "~/stable-diffusion-webui",
				Paths: map[string]string{
					"checkpoints": "models/Stable-diffusion",
					"loras":       "models/Lora",
					"vae":         "models/VAE",
					"controlnet":  "extensions/sd-webui-controlnet/models",
					"embeddings":  "embeddings",
				},
			},
		},
		Mapping: sdMapping("automatic1111"),
	},
	"comfyui": {
		Name:        "comfyui",
		Description: "ComfyUI models directory layout",
		Apps: map[string]config.AppPlacement{
			"comfyui": {
				Base: "~/ComfyUI",
				Paths: map[string]string{
					"checkpoints": "models/checkpoints",
					"loras":       "models/loras",
					"vae":         "models/vae",
					"controlnet":  "models/controlnet",
					"embeddings":  "models/embeddings",
				},
			},
		},
		Mapping: sdMapping("comfyui"),
	},
	"forge": {
		Name:        "forge",
		Description: "Forge Stable Diffusion WebUI layout",
		Apps: map[string]config.AppPlacement{
			"forge": {
				Base: "~/stable-diffusion-webui-forge",
				Paths: map[string]string{
					"checkpoints": "models/Stable-diffusion",
					"loras":       "models/Lora",
					"vae":         "models/VAE",
					"controlnet":  "extensions/sd-webui-controlnet/models",
					"embeddings":  "embeddings",
				},
			},
		},
		Mapping: sdMapping("forge"),
	},
	"hf-cache": {
		Name:        "hf-cache",
		Description: "Generic Hugging Face cache/export directory",
		Apps: map[string]config.AppPlacement{
			"hf-cache": {
				Base: "~/.cache/huggingface/modfetch",
				Paths: map[string]string{
					"models": "models",
				},
			},
		},
		Mapping: []config.MappingRule{
			{Match: "generic", Targets: []config.MappingTarget{{App: "hf-cache", PathKey: "models"}}},
			{Match: "llm.gguf", Targets: []config.MappingTarget{{App: "hf-cache", PathKey: "models"}}},
			{Match: "llm.safetensors", Targets: []config.MappingTarget{{App: "hf-cache", PathKey: "models"}}},
			{Match: "sd.checkpoint", Targets: []config.MappingTarget{{App: "hf-cache", PathKey: "models"}}},
		},
	},
	"ollama": {
		Name:        "ollama",
		Description: "Ollama local models directory for LLM artifacts",
		Apps: map[string]config.AppPlacement{
			"ollama": {
				Base: "~/.ollama",
				Paths: map[string]string{
					"models": "models",
				},
			},
		},
		Mapping: []config.MappingRule{
			{Match: "llm.gguf", Targets: []config.MappingTarget{{App: "ollama", PathKey: "models"}}},
			{Match: "llm.safetensors", Targets: []config.MappingTarget{{App: "ollama", PathKey: "models"}}},
		},
	},
}

func Presets() map[string]Preset {
	return presetCatalog
}

func PresetNames() []string {
	presets := Presets()
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ApplyPresets(cfg *config.Config, names []string) error {
	for _, name := range names {
		if err := ApplyPreset(cfg, name); err != nil {
			return err
		}
	}
	return nil
}

func ApplyPreset(cfg *config.Config, name string) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	preset, ok := Presets()[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return fmt.Errorf("unknown placement preset %q (available: %s)", name, strings.Join(PresetNames(), ", "))
	}
	if cfg.Placement.Apps == nil {
		cfg.Placement.Apps = map[string]config.AppPlacement{}
	}
	for appName, presetApp := range preset.Apps {
		current := cfg.Placement.Apps[appName]
		if strings.TrimSpace(current.Base) == "" {
			current.Base = expandPresetPath(presetApp.Base)
		}
		if current.Paths == nil {
			current.Paths = map[string]string{}
		}
		for key, path := range presetApp.Paths {
			if strings.TrimSpace(current.Paths[key]) == "" {
				current.Paths[key] = path
			}
		}
		cfg.Placement.Apps[appName] = current
	}
	for _, rule := range preset.Mapping {
		mergeMappingRule(&cfg.Placement.Mapping, rule)
	}
	return nil
}

func ParsePresetList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		name := strings.ToLower(strings.TrimSpace(field))
		if name != "" && name != "none" {
			names = append(names, name)
		}
	}
	return names
}

func sdMapping(app string) []config.MappingRule {
	return []config.MappingRule{
		{Match: "sd.checkpoint", Targets: []config.MappingTarget{{App: app, PathKey: "checkpoints"}}},
		{Match: "sd.lora", Targets: []config.MappingTarget{{App: app, PathKey: "loras"}}},
		{Match: "sd.vae", Targets: []config.MappingTarget{{App: app, PathKey: "vae"}}},
		{Match: "sd.controlnet", Targets: []config.MappingTarget{{App: app, PathKey: "controlnet"}}},
		{Match: "sd.embedding", Targets: []config.MappingTarget{{App: app, PathKey: "embeddings"}}},
	}
}

func mergeMappingRule(rules *[]config.MappingRule, presetRule config.MappingRule) {
	for i := range *rules {
		if (*rules)[i].Match == presetRule.Match {
			for _, target := range presetRule.Targets {
				if !hasMappingTarget((*rules)[i].Targets, target) {
					(*rules)[i].Targets = append((*rules)[i].Targets, target)
				}
			}
			return
		}
	}
	*rules = append(*rules, presetRule)
}

func hasMappingTarget(targets []config.MappingTarget, want config.MappingTarget) bool {
	for _, target := range targets {
		if target.App == want.App && target.PathKey == want.PathKey {
			return true
		}
	}
	return false
}

func expandPresetPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
