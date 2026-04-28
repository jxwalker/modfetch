package classifier

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jxwalker/modfetch/internal/config"
)

// Artifact types
// sd.checkpoint, sd.lora, sd.vae, sd.controlnet, sd.embedding,
// llm.gguf, llm.safetensors, generic

type Result struct {
	Type       string
	Confidence string
	Reason     string
}

// Detect attempts to determine the artifact type of a file.
// Custom rules from the config are consulted before built-ins.
func Detect(cfg *config.Config, filePath string) string {
	return Analyze(cfg, filePath).Type
}

func Analyze(cfg *config.Config, filePath string) Result {
	name := strings.ToLower(filepath.Base(filePath))

	// Custom rules first
	if cfg != nil {
		for _, r := range cfg.Classifier.Rules {
			re, err := regexp.Compile(r.Regex)
			if err != nil {
				continue
			}
			if re.MatchString(name) {
				return Result{Type: r.Type, Confidence: "high", Reason: "matched classifier rule " + r.Regex}
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".gguf":
		return Result{Type: "llm.gguf", Confidence: "high", Reason: "file extension .gguf"}
	case ".ckpt":
		return Result{Type: "sd.checkpoint", Confidence: "high", Reason: "file extension .ckpt"}
	case ".safetensors":
		// heuristics based on name
		if strings.Contains(name, "lora") || strings.Contains(name, "lycoris") || strings.Contains(name, "locon") {
			return Result{Type: "sd.lora", Confidence: "medium", Reason: "safetensors filename suggests LoRA"}
		}
		if strings.Contains(name, "vae") {
			return Result{Type: "sd.vae", Confidence: "medium", Reason: "safetensors filename suggests VAE"}
		}
		if strings.Contains(name, "controlnet") {
			return Result{Type: "sd.controlnet", Confidence: "medium", Reason: "safetensors filename suggests ControlNet"}
		}
		// Ambiguous: default to sd.checkpoint for SD ecosystem, can be overridden
		return Result{Type: "sd.checkpoint", Confidence: "low", Reason: "ambiguous .safetensors default"}
	case ".pt":
		if strings.Contains(name, "embed") || strings.Contains(name, "embedding") {
			return Result{Type: "sd.embedding", Confidence: "medium", Reason: "filename suggests embedding"}
		}
		return Result{Type: "generic", Confidence: "low", Reason: "unrecognized .pt filename"}
	default:
		if t := detectMagic(filePath); t != "" {
			return Result{Type: t, Confidence: "medium", Reason: "file header matched " + t}
		}
		return Result{Type: "generic", Confidence: "low", Reason: "no extension or header rule matched"}
	}
}

func detectMagic(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 8)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return ""
	}
	if n >= 4 && (buf[0] == 'G' && buf[1] == 'G' && buf[2] == 'U' && buf[3] == 'F' ||
		strings.EqualFold(string(buf[:4]), "GGUF")) {
		return "llm.gguf"
	}
	if n >= 2 && buf[0] == 0x80 && buf[1] == 0x04 {
		return "sd.checkpoint"
	}
	if n >= 8 {
		headerLen := binary.LittleEndian.Uint64(buf[:8])
		if headerLen > 0 && headerLen < 1<<20 {
			header := make([]byte, headerLen)
			if _, err := io.ReadFull(f, header); err == nil && json.Valid(header) {
				return "sd.checkpoint"
			}
		}
	}
	return ""
}
