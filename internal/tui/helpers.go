package tui

import (
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

// Theme and styling helpers

func defaultTheme() Theme {
	b := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	return Theme{
		border:      b.BorderForeground(lipgloss.Color("63")),
		title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		label:       lipgloss.NewStyle().Faint(true),
		tabActive:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219")),
		tabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		row:         lipgloss.NewStyle(),
		rowSelected: lipgloss.NewStyle().Bold(true),
		head:        lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true),
		footer:      lipgloss.NewStyle().Faint(true),
		ok:          lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		bad:         lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	}
}

func themePresets() []Theme {
	dark := defaultTheme()
	light := Theme{
		border:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("240")),
		title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")),
		label:       lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		tabActive:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("162")),
		tabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		row:         lipgloss.NewStyle(),
		rowSelected: lipgloss.NewStyle().Bold(true),
		head:        lipgloss.NewStyle().Foreground(lipgloss.Color("162")).Bold(true),
		footer:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		ok:          lipgloss.NewStyle().Foreground(lipgloss.Color("22")),
		bad:         lipgloss.NewStyle().Foreground(lipgloss.Color("160")),
	}
	return []Theme{dark, light}
}

func themeIndexByName(name string) int {
	presets := themePresets()
	names := []string{"dark", "light"}
	for i, n := range names {
		if strings.EqualFold(n, name) {
			return i % len(presets)
		}
	}
	return 0
}

// String utilities

func truncateMiddle(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 7 {
		return s[:max]
	}
	left := (max - 3) / 2
	right := max - 3 - left
	return s[:left] + "..." + s[len(s)-right:]
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	pfx := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, pfx) {
			pfx = pfx[:len(pfx)-1]
			if pfx == "" {
				return ""
			}
		}
	}
	return pfx
}

// Download row utilities

func keyFor(r state.DownloadRow) string { return r.URL + "|" + r.Dest }

func hostOf(urlStr string) string {
	u, err := neturl.Parse(urlStr)
	if err != nil || u.Host == "" {
		return "(unknown)"
	}
	return u.Host
}

// Time and ETA utilities

func etaSeconds(cur, total int64, rate float64) float64 {
	if rate <= 0 {
		return 0
	}
	rem := float64(total - cur)
	return rem / rate
}

// File system utilities

func tryWrite(dir string) error {
	if dir == "" {
		return fmt.Errorf("empty dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".modfetch-write-test")
	defer func() { _ = os.Remove(tmp) }()
	return os.WriteFile(tmp, []byte("test"), 0o644)
}

func openInFileManager(p string, reveal bool) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if reveal {
			cmd = exec.Command("open", "-R", p)
		} else {
			cmd = exec.Command("open", p)
		}
	case "windows":
		if reveal {
			cmd = exec.Command("explorer.exe", "/select,", p)
		} else {
			cmd = exec.Command("explorer.exe", filepath.Dir(p))
		}
	case "linux", "freebsd", "openbsd":
		// Try xdg-open
		dir := p
		if reveal {
			dir = filepath.Dir(p)
		}
		cmd = exec.Command("xdg-open", dir)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func copyToClipboard(s string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux", "freebsd", "openbsd":
		// Try xclip, then xsel, then wl-copy (Wayland)
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			return fmt.Errorf("no clipboard tool found (tried xclip, xsel, wl-copy)")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := in.Write([]byte(s)); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

// CivitAI utilities

func mapCivitFileType(civType, fileName string) string {
	// CivitAI's "type" field is not always reliable; use file extension as hint
	ext := strings.ToLower(filepath.Ext(fileName))
	lower := strings.ToLower(civType)

	if strings.Contains(lower, "lora") {
		return "LoRA"
	}
	if strings.Contains(lower, "checkpoint") || strings.Contains(lower, "model") {
		if ext == ".safetensors" || ext == ".ckpt" {
			return "Checkpoint"
		}
	}
	if strings.Contains(lower, "vae") {
		return "VAE"
	}
	if strings.Contains(lower, "embedding") || strings.Contains(lower, "textualinversion") {
		return "Embedding"
	}
	// Fallback
	return "Model"
}

// Config utilities

func computeTypesFromConfig(cfg *config.Config) []string {
	seen := make(map[string]bool)
	var types []string
	for _, rule := range cfg.Placement.Mapping {
		for _, tgt := range rule.Targets {
			if tgt.PathKey != "" && !seen[tgt.PathKey] {
				types = append(types, tgt.PathKey)
				seen[tgt.PathKey] = true
			}
		}
	}
	return types
}
