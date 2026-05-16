package main

import (
	"strings"
	"testing"
)

func TestCompletionTopLevelCommands(t *testing.T) {
	for name, script := range completionScripts() {
		for _, command := range []string{"config", "download", "bench", "discover", "get", "recommend", "pack", "starter", "snapshot", "place", "verify", "status", "tui", "library", "batch", "dedupe", "clean", "hostcaps", "version", "help", "completion"} {
			want := completionCommandToken(name, command)
			if !strings.Contains(script, want) {
				t.Fatalf("%s completion missing top-level command %q", name, command)
			}
		}
	}
}

func TestCompletionCurrentFlags(t *testing.T) {
	tests := map[string][]string{
		"config":   {"--strict", "--out"},
		"download": {"--no-resume", "--summary-json", "--batch-parallel", "--profile", "--connections", "--chunk-size-mb", "--dry-run", "--run-help", "--force", "--no-auth-preflight", "--quant", "--list-quants"},
		"bench":    {"--url", "--tools", "--duration", "--profile", "--connections", "--chunk-size-mb", "--keep", "--history"},
		"discover": {"--provider", "--limit", "--select", "--summary-json", "--dry-run", "--run-help"},
		"get": {
			"--provider", "--query", "--limit", "--select",
			"--small", "--medium", "--large", "--size",
			"--download", "--dry-run", "--run-help", "--dest", "--place",
			"--summary-json", "--quiet", "--no-resume",
			"--ram-gb", "--vram-gb", "--unified-memory",
			"--starter-id", "--no-learn",
		},
		"recommend": {"--provider", "--task", "--ram-gb", "--vram-gb", "--unified-memory", "--download", "--select", "--dry-run", "--run-help", "--history", "--history-limit", "--no-learn"},
		"pack":      {"--id", "--output", "--format", "--dest-dir", "--dry-run", "--batch-parallel", "--profile"},
		"starter":   {"--id", "--summary-json", "--dry-run", "--run-help"},
		"snapshot":  {"--include", "--exclude", "--rev", "--output", "--format", "--dest-dir", "--max-files", "--download", "--dry-run", "--batch-parallel", "--profile"},
		"place":     {"--dry-run", "--preset", "--list-presets"},
		"batch":     {"--naming-pattern"},
		"hostcaps":  {"--config", "--list", "--clear", "--clear-all", "--json"},
		"tui":       {"--json", "--snapshot"},
		"clean":     {"--days", "--dry-run", "--dest", "--include-next-to-dest", "--sidecars"},
		"library":   {"--target"},
	}

	for name, script := range completionScripts() {
		for command, flags := range tests {
			section := completionCommandSection(t, name, script, command)
			for _, flag := range flags {
				want := completionFlagToken(name, flag)
				if !strings.Contains(section, want) {
					t.Fatalf("%s completion for %s missing %q", name, command, want)
				}
			}
		}
	}
}

func TestCompletionDropsStaleTUIFlags(t *testing.T) {
	for name, script := range completionScripts() {
		section := completionCommandSection(t, name, script, "tui")
		for _, stale := range []string{"--v1", "--v2"} {
			if strings.Contains(section, stale) {
				t.Fatalf("%s completion still advertises stale TUI flag %q", name, stale)
			}
		}
	}
}

func TestCompletionNestedSubcommandFlags(t *testing.T) {
	for name, script := range completionScripts() {
		configSection := completionCommandSection(t, name, script, "config")
		batchSection := completionCommandSection(t, name, script, "batch")

		switch name {
		case "bash":
			assertContains(t, name, "config", configSection, "case ${words[2]}", "validate)", "wizard)", "--strict", "--out")
			assertContains(t, name, "batch", batchSection, "case ${words[2]}", "import)", "--naming-pattern")
			assertContains(t, name, "discover", completionCommandSection(t, name, script, "discover"), "search", "download", "--provider", "--select")
			assertContains(t, name, "library", completionCommandSection(t, name, script, "library"), "sync", "push", "pull", "--target", "--dry-run", "--token-env")
			assertContains(t, name, "pack", completionCommandSection(t, name, script, "pack"), "list", "show", "export", "download", "llm-smoke")
			assertContains(t, name, "starter", completionCommandSection(t, name, script, "starter"), "list", "show", "download", "gpt2-config")
			assertContains(t, name, "snapshot", completionCommandSection(t, name, script, "snapshot"), "--include", "--download", "--max-files")
		case "zsh":
			assertContains(t, name, "config", configSection, "case $words[3]", "validate)", "wizard)", "--strict", "--out")
			assertContains(t, name, "batch", batchSection, "case $words[3]", "import)", "--naming-pattern")
			assertContains(t, name, "discover", completionCommandSection(t, name, script, "discover"), "search", "download", "--provider", "--select")
			assertContains(t, name, "library", completionCommandSection(t, name, script, "library"), "sync", "push", "pull", "--target", "--dry-run", "--token-env")
			assertContains(t, name, "pack", completionCommandSection(t, name, script, "pack"), "list", "show", "export", "download", "llm-smoke")
			assertContains(t, name, "starter", completionCommandSection(t, name, script, "starter"), "list", "show", "download", "gpt2-config")
			assertContains(t, name, "snapshot", completionCommandSection(t, name, script, "snapshot"), "--include", "--download", "--max-files")
		case "fish":
			assertContains(t, name, "config", configSection, "and __fish_seen_subcommand_from validate", "and __fish_seen_subcommand_from wizard", "-l strict", "-l out")
			assertContains(t, name, "batch", batchSection, "and __fish_seen_subcommand_from import", "-l naming-pattern")
			assertContains(t, name, "discover", completionCommandSection(t, name, script, "discover"), "search", "download", "-l provider", "-l select")
			assertContains(t, name, "library", completionCommandSection(t, name, script, "library"), "sync", "push", "pull", "-l target", "-l dry-run", "-l token-env")
			assertContains(t, name, "pack", completionCommandSection(t, name, script, "pack"), "llm-smoke", "-l id", "-l batch-parallel")
			assertContains(t, name, "starter", completionCommandSection(t, name, script, "starter"), "gpt2-config", "-l id", "-l dry-run")
			assertContains(t, name, "snapshot", completionCommandSection(t, name, script, "snapshot"), "-l include", "-l download", "-l max-files")
		}
	}
}

func completionScripts() map[string]string {
	return map[string]string{
		"bash": bashCompletion,
		"zsh":  zshCompletion,
		"fish": fishCompletion,
	}
}

func completionCommandToken(shell, command string) string {
	if shell == "fish" {
		return `-a "` + command + `"`
	}
	return command
}

func completionFlagToken(shell, flag string) string {
	if shell == "fish" {
		return "-l " + strings.TrimPrefix(flag, "--")
	}
	return flag
}

func completionCommandSection(t *testing.T, shell, script, command string) string {
	t.Helper()

	if shell == "fish" {
		return completionFishCommandSection(t, script, command)
	}

	prefix := "        "
	if shell == "zsh" {
		prefix = "    "
	}

	lines := strings.Split(script, "\n")
	for i, line := range lines {
		if line != prefix+command+")" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if isCompletionCaseLabel(lines[j], prefix) {
				return strings.Join(lines[i+1:j], "\n")
			}
		}
		return strings.Join(lines[i+1:], "\n")
	}

	t.Fatalf("%s completion missing section for %s", shell, command)
	return ""
}

func isCompletionCaseLabel(line, prefix string) bool {
	if !strings.HasPrefix(line, prefix) {
		return false
	}
	if len(line) <= len(prefix) || line[len(prefix)] == ' ' || line[len(prefix)] == '\t' {
		return false
	}
	return strings.HasSuffix(strings.TrimSpace(line), ")")
}

func completionFishCommandSection(t *testing.T, script, command string) string {
	t.Helper()

	lines := strings.Split(script, "\n")
	var matches []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "__fish_seen_subcommand_from "+command) {
			matches = append(matches, line)
			continue
		}
		if !strings.HasPrefix(line, "for cmd in ") {
			continue
		}
		commands := strings.Fields(strings.TrimPrefix(line, "for cmd in "))
		if !containsString(commands, command) {
			continue
		}
		for j := i + 1; j < len(lines) && lines[j] != "end"; j++ {
			matches = append(matches, lines[j])
		}
	}
	if len(matches) == 0 {
		t.Fatalf("fish completion missing section for %s", command)
	}
	return strings.Join(matches, "\n")
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertContains(t *testing.T, shell, command, section string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(section, value) {
			t.Fatalf("%s completion for %s missing %q", shell, command, value)
		}
	}
}
