package main

import (
	"strings"
	"testing"
)

func TestCompletionTopLevelCommands(t *testing.T) {
	for name, script := range completionScripts() {
		for _, command := range []string{"config", "download", "place", "verify", "status", "tui", "batch", "dedupe", "clean", "hostcaps", "version", "help", "completion"} {
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
		"download": {"--no-resume", "--summary-json", "--batch-parallel", "--dry-run", "--force", "--no-auth-preflight", "--quant", "--list-quants"},
		"place":    {"--dry-run", "--preset", "--list-presets"},
		"batch":    {"--naming-pattern"},
		"hostcaps": {"--config", "--list", "--clear", "--clear-all", "--json"},
		"tui":      {"--json"},
		"clean":    {"--days", "--dry-run", "--dest", "--include-next-to-dest", "--sidecars"},
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
		case "zsh":
			assertContains(t, name, "config", configSection, "case $words[3]", "validate)", "wizard)", "--strict", "--out")
			assertContains(t, name, "batch", batchSection, "case $words[3]", "import)", "--naming-pattern")
		case "fish":
			assertContains(t, name, "config", configSection, "and __fish_seen_subcommand_from validate", "and __fish_seen_subcommand_from wizard", "-l strict", "-l out")
			assertContains(t, name, "batch", batchSection, "and __fish_seen_subcommand_from import", "-l naming-pattern")
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
