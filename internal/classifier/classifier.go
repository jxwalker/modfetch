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

// Detect attempts to determine the artifact type of a file.
// Custom rules from the config are consulted before built-ins.
func Detect(cfg *config.Config, filePath string) string {
	name := strings.ToLower(filepath.Base(filePath))

	// Custom rules first
	if cfg != nil {
		for _, r := range cfg.Classifier.Rules {
			re, err := regexp.Compile(r.Regex)
			if err != nil {
				continue
			}
			if re.MatchString(name) {
				return r.Type
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
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
		if strings.Contains(name, "embed") || strings.Contains(name, "embedding") {
			return "sd.embedding"
		}
		return "generic"
	default:
		if t := detectMagic(filePath); t != "" {
			return t
		}
		return "generic"
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
