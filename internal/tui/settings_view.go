package tui

import (
	"fmt"
	"os"
	"strings"
)

// Settings view rendering

// updateTokenEnvStatus checks if API tokens are set in environment variables
func (m *Model) updateTokenEnvStatus() {
	// Presence only (set/unset); rejection is updated upon errors in dlDoneMsg
	if m.cfg.Sources.HuggingFace.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
		if env == "" {
			env = "HF_TOKEN"
		}
		m.hfTokenSet = strings.TrimSpace(os.Getenv(env)) != ""
	} else {
		m.hfTokenSet = false
	}
	if m.cfg.Sources.CivitAI.Enabled {
		env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
		if env == "" {
			env = "CIVITAI_TOKEN"
		}
		m.civTokenSet = strings.TrimSpace(os.Getenv(env)) != ""
	} else {
		m.civTokenSet = false
	}
}

func (m *Model) renderAuthStatus() string {
	var sb strings.Builder
	if m.cfg.Sources.HuggingFace.Enabled {
		env := m.cfg.Sources.HuggingFace.TokenEnv
		if env == "" {
			env = "HF_TOKEN"
		}
		sb.WriteString("HF ")
		if m.hfTokenSet {
			if m.hfTokenRejected {
				sb.WriteString(m.th.bad.Render("✗"))
			} else {
				sb.WriteString(m.th.ok.Render("✓"))
			}
		} else {
			sb.WriteString(m.th.label.Render("-"))
		}
		sb.WriteString(" ")
	}
	if m.cfg.Sources.CivitAI.Enabled {
		env := m.cfg.Sources.CivitAI.TokenEnv
		if env == "" {
			env = "CIVITAI_TOKEN"
		}
		sb.WriteString("Civ ")
		if m.civTokenSet {
			if m.civTokenRejected {
				sb.WriteString(m.th.bad.Render("✗"))
			} else {
				sb.WriteString(m.th.ok.Render("✓"))
			}
		} else {
			sb.WriteString(m.th.label.Render("-"))
		}
	}
	return sb.String()
}

// renderSettings displays current configuration
func (m *Model) renderSettings() string {
	var sb strings.Builder

	sb.WriteString(m.th.head.Render("Settings & Configuration") + "\n\n")

	// Section: General Paths
	sb.WriteString(m.th.head.Render("Directory Paths") + "\n")
	sb.WriteString(m.th.label.Render("Data Root: "))
	sb.WriteString(m.cfg.General.DataRoot + "\n")
	sb.WriteString(m.th.label.Render("Download Root: "))
	sb.WriteString(m.cfg.General.DownloadRoot + "\n")
	if m.cfg.General.PartialsRoot != "" {
		sb.WriteString(m.th.label.Render("Partials Root: "))
		sb.WriteString(m.cfg.General.PartialsRoot + "\n")
	}
	sb.WriteString(m.th.label.Render("Placement Mode: "))
	sb.WriteString(m.cfg.General.PlacementMode + "\n")
	sb.WriteString("\n")

	// Section: Token Status
	sb.WriteString(m.th.head.Render("API Token Status") + "\n")

	// HuggingFace token
	if m.cfg.Sources.HuggingFace.Enabled {
		env := m.cfg.Sources.HuggingFace.TokenEnv
		if env == "" {
			env = "HF_TOKEN"
		}
		sb.WriteString(m.th.label.Render("HuggingFace (" + env + "): "))
		if m.hfTokenSet {
			if m.hfTokenRejected {
				sb.WriteString(m.th.err.Render("✗ Set but rejected by API") + "\n")
			} else {
				sb.WriteString(m.th.ok.Render("✓ Set") + "\n")
			}
		} else {
			sb.WriteString(m.th.label.Render("Not set") + "\n")
		}
	} else {
		sb.WriteString(m.th.label.Render("HuggingFace: "))
		sb.WriteString("Disabled\n")
	}

	// CivitAI token
	if m.cfg.Sources.CivitAI.Enabled {
		env := m.cfg.Sources.CivitAI.TokenEnv
		if env == "" {
			env = "CIVITAI_TOKEN"
		}
		sb.WriteString(m.th.label.Render("CivitAI (" + env + "): "))
		if m.civTokenSet {
			if m.civTokenRejected {
				sb.WriteString(m.th.err.Render("✗ Set but rejected by API") + "\n")
			} else {
				sb.WriteString(m.th.ok.Render("✓ Set") + "\n")
			}
		} else {
			sb.WriteString(m.th.label.Render("Not set") + "\n")
		}
	} else {
		sb.WriteString(m.th.label.Render("CivitAI: "))
		sb.WriteString("Disabled\n")
	}
	sb.WriteString("\n")

	// Section: Placement Rules
	sb.WriteString(m.th.head.Render("Placement Rules") + "\n")
	if len(m.cfg.Placement.Apps) > 0 {
		for name, app := range m.cfg.Placement.Apps {
			sb.WriteString(m.th.label.Render("  " + name + ": "))
			sb.WriteString(app.Base + "\n")
			if len(app.Paths) > 0 {
				for key, path := range app.Paths {
					sb.WriteString(m.th.label.Render("    " + key + ": "))
					sb.WriteString(path + "\n")
				}
			}
		}
	} else {
		sb.WriteString("  No placement apps configured\n")
	}
	sb.WriteString("\n")

	// Section: Download Settings
	sb.WriteString(m.th.head.Render("Download Settings") + "\n")
	sb.WriteString(m.th.label.Render("Timeout: "))
	sb.WriteString(fmt.Sprintf("%d seconds\n", m.cfg.Network.TimeoutSeconds))
	sb.WriteString(m.th.label.Render("Max Redirects: "))
	sb.WriteString(fmt.Sprintf("%d\n", m.cfg.Network.MaxRedirects))
	sb.WriteString(m.th.label.Render("Chunk Size: "))
	sb.WriteString(fmt.Sprintf("%d MB\n", m.cfg.Concurrency.ChunkSizeMB))
	sb.WriteString(m.th.label.Render("Per-File Chunks: "))
	sb.WriteString(fmt.Sprintf("%d\n", m.cfg.Concurrency.PerFileChunks))
	sb.WriteString(m.th.label.Render("Global Files: "))
	sb.WriteString(fmt.Sprintf("%d\n", m.cfg.Concurrency.GlobalFiles))
	sb.WriteString(m.th.label.Render("Stage Partials: "))
	if m.cfg.General.StagePartials {
		sb.WriteString("Yes\n")
	} else {
		sb.WriteString("No\n")
	}
	sb.WriteString("\n")

	// Section: UI Preferences
	sb.WriteString(m.th.head.Render("UI Preferences") + "\n")
	sb.WriteString(m.th.label.Render("Theme: "))
	if m.cfg.UI.Theme != "" {
		sb.WriteString(m.cfg.UI.Theme + "\n")
	} else {
		sb.WriteString("default\n")
	}
	sb.WriteString(m.th.label.Render("Column Mode: "))
	if m.columnMode != "" {
		sb.WriteString(m.columnMode + "\n")
	} else {
		sb.WriteString("dest\n")
	}
	sb.WriteString(m.th.label.Render("Compact View: "))
	if m.isCompact() {
		sb.WriteString("Yes\n")
	} else {
		sb.WriteString("No\n")
	}
	sb.WriteString(m.th.label.Render("Refresh Rate: "))
	if m.cfg.UI.RefreshHz > 0 {
		sb.WriteString(fmt.Sprintf("%d Hz\n", m.cfg.UI.RefreshHz))
	} else {
		sb.WriteString("1 Hz (default)\n")
	}
	sb.WriteString("\n")

	// Section: Validation
	sb.WriteString(m.th.head.Render("Validation Settings") + "\n")
	sb.WriteString(m.th.label.Render("Require SHA256: "))
	if m.cfg.Validation.RequireSHA256 {
		sb.WriteString("Yes\n")
	} else {
		sb.WriteString("No\n")
	}
	sb.WriteString(m.th.label.Render("Accept MD5/SHA1: "))
	if m.cfg.Validation.AcceptMD5SHA1IfProvided {
		sb.WriteString("Yes\n")
	} else {
		sb.WriteString("No\n")
	}
	sb.WriteString(m.th.label.Render("Safetensors Deep Verify: "))
	if m.cfg.Validation.SafetensorsDeepVerifyAfterDownload {
		sb.WriteString("Yes\n")
	} else {
		sb.WriteString("No\n")
	}
	sb.WriteString("\n")

	// Footer note
	sb.WriteString(m.th.footer.Render("To edit settings, modify the YAML config file directly and restart"))

	return sb.String()
}
