package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

// setupTestSettings creates a test model with various config settings
func setupTestSettings(t *testing.T, cfg *config.Config) (*Model, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if cfg == nil {
		cfg = &config.Config{
			General: config.General{
				DataRoot:      tmpDir,
				DownloadRoot:  filepath.Join(tmpDir, "downloads"),
				PartialsRoot:  filepath.Join(tmpDir, "partials"),
				PlacementMode: "auto",
				StagePartials: true,
			},
			Network: config.Network{
				TimeoutSeconds: 300,
				MaxRedirects:   5,
			},
			Concurrency: config.Concurrency{
				ChunkSizeMB:   64,
				PerFileChunks: 4,
				GlobalFiles:   3,
			},
			UI: config.UIOptions{
				Theme:     "dark",
				RefreshHz: 10,
			},
			Validation: config.Validation{
				RequireSHA256:                      false,
				AcceptMD5SHA1IfProvided:            true,
				SafetensorsDeepVerifyAfterDownload: true,
			},
			Sources: config.Sources{
				HuggingFace: config.SourceWithToken{
					Enabled:  true,
					TokenEnv: "HF_TOKEN",
				},
				CivitAI: config.SourceWithToken{
					Enabled:  true,
					TokenEnv: "CIVITAI_TOKEN",
				},
			},
			Placement: config.Placement{
				Apps: map[string]config.AppPlacement{
					"comfyui": {
						Base: "/opt/comfyui",
						Paths: map[string]string{
							"checkpoint": "models/checkpoints",
							"lora":       "models/loras",
						},
					},
				},
			},
		}
	}

	model := New(cfg, db, "test-version").(*Model)
	model.w = 120
	model.h = 40
	model.activeTab = 6 // Settings tab

	cleanup := func() {
		if err := db.SQL.Close(); err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	}

	return model, cleanup
}

func TestSettings_RenderBasic(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	output := model.renderSettings()

	// Check main header
	if !strings.Contains(output, "Settings & Configuration") {
		t.Error("Settings view should contain main header")
	}

	// Check all sections are present
	sections := []string{
		"Directory Paths",
		"API Token Status",
		"Placement Rules",
		"Download Settings",
		"UI Preferences",
		"Validation Settings",
	}

	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("Settings view should contain section: %s", section)
		}
	}
}

func TestSettings_DirectoryPaths(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	output := model.renderSettings()

	// Check directory paths are shown
	if !strings.Contains(output, "Data Root:") {
		t.Error("Should show Data Root path")
	}

	if !strings.Contains(output, "Download Root:") {
		t.Error("Should show Download Root path")
	}

	if !strings.Contains(output, "Partials Root:") {
		t.Error("Should show Partials Root path")
	}

	if !strings.Contains(output, "Placement Mode:") {
		t.Error("Should show Placement Mode")
	}

	// Check actual values are displayed
	if !strings.Contains(output, model.cfg.General.DataRoot) {
		t.Error("Should display actual data root path")
	}

	if !strings.Contains(output, "auto") {
		t.Error("Should display placement mode value")
	}
}

func TestSettings_TokenStatus_NotSet(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Ensure tokens are not set
	_ = os.Unsetenv("HF_TOKEN")
	_ = os.Unsetenv("CIVITAI_TOKEN")

	model.updateTokenEnvStatus()
	output := model.renderSettings()

	// Check HuggingFace token status
	if !strings.Contains(output, "HuggingFace (HF_TOKEN):") {
		t.Error("Should show HuggingFace token config")
	}

	if !strings.Contains(output, "Not set") {
		t.Error("Should indicate tokens are not set")
	}

	// Check CivitAI token status
	if !strings.Contains(output, "CivitAI (CIVITAI_TOKEN):") {
		t.Error("Should show CivitAI token config")
	}
}

func TestSettings_TokenStatus_Set(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Set tokens in environment
	_ = os.Setenv("HF_TOKEN", "test-hf-token")
	_ = os.Setenv("CIVITAI_TOKEN", "test-civ-token")
	defer func() {
		_ = os.Unsetenv("HF_TOKEN")
		_ = os.Unsetenv("CIVITAI_TOKEN")
	}()

	model.updateTokenEnvStatus()
	output := model.renderSettings()

	// Should show tokens are set
	if !strings.Contains(output, "✓ Set") {
		t.Error("Should show token is set with checkmark")
	}

	// Verify internal state
	if !model.hfTokenSet {
		t.Error("hfTokenSet should be true when HF_TOKEN is set")
	}

	if !model.civTokenSet {
		t.Error("civTokenSet should be true when CIVITAI_TOKEN is set")
	}
}

func TestSettings_TokenStatus_Rejected(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Set tokens
	_ = os.Setenv("HF_TOKEN", "invalid-token")
	_ = os.Setenv("CIVITAI_TOKEN", "invalid-token")
	defer func() {
		_ = os.Unsetenv("HF_TOKEN")
		_ = os.Unsetenv("CIVITAI_TOKEN")
	}()

	model.updateTokenEnvStatus()

	// Simulate token rejection
	model.hfRejected = true
	model.civRejected = true

	output := model.renderSettings()

	// Should show rejection status
	if !strings.Contains(output, "✗ Set but rejected by API") {
		t.Error("Should show token rejection message")
	}
}

func TestSettings_TokenStatus_Disabled(t *testing.T) {
	cfg := &config.Config{
		General: config.General{
			DataRoot:     os.TempDir(),
			DownloadRoot: os.TempDir(),
		},
		Sources: config.Sources{
			HuggingFace: config.SourceWithToken{
				Enabled: false,
			},
			CivitAI: config.SourceWithToken{
				Enabled: false,
			},
		},
	}

	model, cleanup := setupTestSettings(t, cfg)
	defer cleanup()

	model.updateTokenEnvStatus()
	output := model.renderSettings()

	// Should show disabled status
	if !strings.Contains(output, "Disabled") {
		t.Error("Should show 'Disabled' for disabled sources")
	}

	// Verify internal state
	if model.hfTokenSet {
		t.Error("hfTokenSet should be false when HuggingFace is disabled")
	}

	if model.civTokenSet {
		t.Error("civTokenSet should be false when CivitAI is disabled")
	}
}

func TestSettings_RenderAuthStatus_Compact(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Test: No tokens set
	_ = os.Unsetenv("HF_TOKEN")
	_ = os.Unsetenv("CIVITAI_TOKEN")
	model.updateTokenEnvStatus()

	output := model.renderAuthStatus()
	if !strings.Contains(output, "HF") {
		t.Error("Auth status should show 'HF' label")
	}
	if !strings.Contains(output, "Civ") {
		t.Error("Auth status should show 'Civ' label")
	}
	if !strings.Contains(output, "-") {
		t.Error("Auth status should show '-' for unset tokens")
	}

	// Test: Tokens set
	_ = os.Setenv("HF_TOKEN", "test")
	_ = os.Setenv("CIVITAI_TOKEN", "test")
	defer func() {
		_ = os.Unsetenv("HF_TOKEN")
		_ = os.Unsetenv("CIVITAI_TOKEN")
	}()
	model.updateTokenEnvStatus()

	output = model.renderAuthStatus()
	if !strings.Contains(output, "✓") {
		t.Error("Auth status should show '✓' for set tokens")
	}

	// Test: Tokens rejected
	model.hfRejected = true
	output = model.renderAuthStatus()
	if !strings.Contains(output, "✗") {
		t.Error("Auth status should show '✗' for rejected tokens")
	}
}

func TestSettings_PlacementRules(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	output := model.renderSettings()

	// Check placement apps are displayed
	if !strings.Contains(output, "Placement Rules") {
		t.Error("Should show Placement Rules section")
	}

	if !strings.Contains(output, "comfyui") {
		t.Error("Should show configured app 'comfyui'")
	}

	if !strings.Contains(output, "/opt/comfyui") {
		t.Error("Should show app base path")
	}

	if !strings.Contains(output, "checkpoint") {
		t.Error("Should show checkpoint path config")
	}

	if !strings.Contains(output, "lora") {
		t.Error("Should show lora path config")
	}
}

func TestSettings_PlacementRules_Empty(t *testing.T) {
	cfg := &config.Config{
		General: config.General{
			DataRoot:     os.TempDir(),
			DownloadRoot: os.TempDir(),
		},
		Placement: config.Placement{
			Apps: map[string]config.AppPlacement{},
		},
	}

	model, cleanup := setupTestSettings(t, cfg)
	defer cleanup()

	output := model.renderSettings()

	// Should show message about no placement apps
	if !strings.Contains(output, "No placement apps configured") {
		t.Error("Should show message when no placement apps configured")
	}
}

func TestSettings_DownloadSettings(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	output := model.renderSettings()

	// Check download settings are displayed
	fields := []string{
		"Timeout:",
		"Max Redirects:",
		"Chunk Size:",
		"Per-File Chunks:",
		"Global Files:",
		"Stage Partials:",
	}

	for _, field := range fields {
		if !strings.Contains(output, field) {
			t.Errorf("Should show download setting: %s", field)
		}
	}

	// Check actual values
	if !strings.Contains(output, "300 seconds") {
		t.Error("Should show timeout value")
	}

	if !strings.Contains(output, "64 MB") {
		t.Error("Should show chunk size")
	}

	// Check boolean rendering
	if model.cfg.General.StagePartials {
		if !strings.Contains(output, "Stage Partials: Yes") {
			t.Error("Should show 'Yes' for StagePartials when true")
		}
	} else {
		if !strings.Contains(output, "Stage Partials: No") {
			t.Error("Should show 'No' for StagePartials when false")
		}
	}
}

func TestSettings_UIPreferences(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()
	model.columnMode = "dest"

	output := model.renderSettings()

	// Check UI preferences are displayed
	if !strings.Contains(output, "UI Preferences") {
		t.Error("Should show UI Preferences section")
	}

	if !strings.Contains(output, "Theme:") {
		t.Error("Should show theme setting")
	}

	if !strings.Contains(output, "dark") {
		t.Error("Should show theme value")
	}

	if !strings.Contains(output, "Column Mode:") {
		t.Error("Should show column mode")
	}

	if !strings.Contains(output, "dest") {
		t.Error("Should show column mode value")
	}

	if !strings.Contains(output, "Compact View:") {
		t.Error("Should show compact view setting")
	}

	if !strings.Contains(output, "Refresh Rate:") {
		t.Error("Should show refresh rate")
	}

	if !strings.Contains(output, "10 Hz") {
		t.Error("Should show refresh rate value")
	}
}

func TestSettings_UIPreferences_Defaults(t *testing.T) {
	cfg := &config.Config{
		General: config.General{
			DataRoot:     os.TempDir(),
			DownloadRoot: os.TempDir(),
		},
		UI: config.UIOptions{
			Theme:     "",
			RefreshHz: 0,
		},
	}

	model, cleanup := setupTestSettings(t, cfg)
	defer cleanup()
	model.columnMode = ""

	output := model.renderSettings()

	// Should show defaults
	if !strings.Contains(output, "default") {
		t.Error("Should show 'default' when theme is empty")
	}

	if !strings.Contains(output, "1 Hz (default)") {
		t.Error("Should show default refresh rate when not configured")
	}

	if !strings.Contains(output, "dest") {
		t.Error("Should show default column mode")
	}
}

func TestSettings_ValidationSettings(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	output := model.renderSettings()

	// Check validation settings are displayed
	if !strings.Contains(output, "Validation Settings") {
		t.Error("Should show Validation Settings section")
	}

	fields := []string{
		"Require SHA256:",
		"Accept MD5/SHA1:",
		"Safetensors Deep Verify:",
	}

	for _, field := range fields {
		if !strings.Contains(output, field) {
			t.Errorf("Should show validation setting: %s", field)
		}
	}
}

func TestSettings_ValidationSettings_BooleanRendering(t *testing.T) {
	tests := []struct {
		name                      string
		requireSHA256             bool
		acceptMD5                 bool
		safetensorsVerify         bool
		expectedSHA256            string
		expectedMD5               string
		expectedSafetensorsVerify string
	}{
		{
			name:                      "all true",
			requireSHA256:             true,
			acceptMD5:                 true,
			safetensorsVerify:         true,
			expectedSHA256:            "Require SHA256: Yes",
			expectedMD5:               "Accept MD5/SHA1: Yes",
			expectedSafetensorsVerify: "Safetensors Deep Verify: Yes",
		},
		{
			name:                      "all false",
			requireSHA256:             false,
			acceptMD5:                 false,
			safetensorsVerify:         false,
			expectedSHA256:            "Require SHA256: No",
			expectedMD5:               "Accept MD5/SHA1: No",
			expectedSafetensorsVerify: "Safetensors Deep Verify: No",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				General: config.General{
					DataRoot:     os.TempDir(),
					DownloadRoot: os.TempDir(),
				},
				Validation: config.Validation{
					RequireSHA256:                      tt.requireSHA256,
					AcceptMD5SHA1IfProvided:            tt.acceptMD5,
					SafetensorsDeepVerifyAfterDownload: tt.safetensorsVerify,
				},
			}

			model, cleanup := setupTestSettings(t, cfg)
			defer cleanup()

			output := model.renderSettings()

			if !strings.Contains(output, tt.expectedSHA256) {
				t.Errorf("Expected %q in output", tt.expectedSHA256)
			}

			if !strings.Contains(output, tt.expectedMD5) {
				t.Errorf("Expected %q in output", tt.expectedMD5)
			}

			if !strings.Contains(output, tt.expectedSafetensorsVerify) {
				t.Errorf("Expected %q in output", tt.expectedSafetensorsVerify)
			}
		})
	}
}

func TestSettings_Footer(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	output := model.renderSettings()

	// Check footer message
	if !strings.Contains(output, "To edit settings") {
		t.Error("Should show footer message about editing config")
	}

	if !strings.Contains(output, "YAML config file") {
		t.Error("Should mention YAML config file")
	}
}

func TestSettings_CustomTokenEnv(t *testing.T) {
	cfg := &config.Config{
		General: config.General{
			DataRoot:     os.TempDir(),
			DownloadRoot: os.TempDir(),
		},
		Sources: config.Sources{
			HuggingFace: config.SourceWithToken{
				Enabled:  true,
				TokenEnv: "CUSTOM_HF_TOKEN",
			},
			CivitAI: config.SourceWithToken{
				Enabled:  true,
				TokenEnv: "CUSTOM_CIV_TOKEN",
			},
		},
	}

	model, cleanup := setupTestSettings(t, cfg)
	defer cleanup()

	// Set custom token env vars
	_ = os.Setenv("CUSTOM_HF_TOKEN", "test")
	_ = os.Setenv("CUSTOM_CIV_TOKEN", "test")
	defer func() {
		_ = os.Unsetenv("CUSTOM_HF_TOKEN")
		_ = os.Unsetenv("CUSTOM_CIV_TOKEN")
	}()

	model.updateTokenEnvStatus()
	output := model.renderSettings()

	// Should show custom env var names
	if !strings.Contains(output, "CUSTOM_HF_TOKEN") {
		t.Error("Should show custom HuggingFace token env var")
	}

	if !strings.Contains(output, "CUSTOM_CIV_TOKEN") {
		t.Error("Should show custom CivitAI token env var")
	}

	// Verify tokens are detected
	if !model.hfTokenSet {
		t.Error("Should detect custom HF token")
	}

	if !model.civTokenSet {
		t.Error("Should detect custom CivitAI token")
	}
}

func TestSettings_CompactViewToggle(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Test compact mode off
	model.compactToggle()
	output := model.renderSettings()
	if !strings.Contains(output, "Compact View: Yes") {
		t.Error("Should show 'Yes' when compact mode is on")
	}

	// Test compact mode on
	model.compactToggle()
	output = model.renderSettings()
	if !strings.Contains(output, "Compact View: No") {
		t.Error("Should show 'No' when compact mode is off")
	}
}

func TestSettings_UpdateTokenEnvStatus_EmptyEnvValue(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Set token to empty string (should be treated as not set)
	_ = os.Setenv("HF_TOKEN", "")
	_ = os.Setenv("CIVITAI_TOKEN", "   ") // Whitespace only
	defer func() {
		_ = os.Unsetenv("HF_TOKEN")
		_ = os.Unsetenv("CIVITAI_TOKEN")
	}()

	model.updateTokenEnvStatus()

	// Empty/whitespace tokens should not count as set
	if model.hfTokenSet {
		t.Error("Empty token should not count as set")
	}

	if model.civTokenSet {
		t.Error("Whitespace-only token should not count as set")
	}
}

func TestSettings_NoThemeRendering(t *testing.T) {
	model, cleanup := setupTestSettings(t, nil)
	defer cleanup()

	// Render should not panic even without theme
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Rendering settings should not panic: %v", r)
		}
	}()

	_ = model.renderSettings()
	_ = model.renderAuthStatus()
}
