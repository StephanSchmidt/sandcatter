package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteInstalledPlugins(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "installed_plugins.json")
	dstPath := filepath.Join(dir, "out_installed_plugins.json")

	input := installedPluginsFile{
		Version: 2,
		Plugins: map[string][]installedPluginEntry{
			"gopls@claude-code-lsps": {
				{
					Scope:        "user",
					InstallPath:  "/home/stephan/.claude/plugins/cache/claude-code-lsps/gopls/1.0.0",
					Version:      "1.0.0",
					InstalledAt:  "2026-03-04T04:45:00.192Z",
					LastUpdated:  "2026-03-04T04:45:00.192Z",
					GitCommitSha: "abc123",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(input, "", "  ")
	os.WriteFile(srcPath, data, 0644)

	err := rewriteInstalledPlugins(srcPath, dstPath, "/home/stephan", "/home/vscode")
	if err != nil {
		t.Fatalf("rewriteInstalledPlugins failed: %v", err)
	}

	outData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var result installedPluginsFile
	if err := json.Unmarshal(outData, &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	entries := result.Plugins["gopls@claude-code-lsps"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	want := "/home/vscode/.claude/plugins/cache/claude-code-lsps/gopls/1.0.0"
	if entries[0].InstallPath != want {
		t.Errorf("installPath = %q, want %q", entries[0].InstallPath, want)
	}

	// Other fields preserved
	if entries[0].Version != "1.0.0" {
		t.Error("version field was lost")
	}
	if entries[0].GitCommitSha != "abc123" {
		t.Error("gitCommitSha field was lost")
	}
}

func TestRewriteKnownMarketplaces(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "known_marketplaces.json")
	dstPath := filepath.Join(dir, "out_known_marketplaces.json")

	input := map[string]knownMarketplaceEntry{
		"claude-code-lsps": {
			Source: marketplaceSource{
				Source: "github",
				Repo:   "boostvolt/claude-code-lsps",
			},
			InstallLocation: "/home/stephan/.claude/plugins/marketplaces/claude-code-lsps",
			LastUpdated:     "2026-03-05T21:10:18.108Z",
		},
	}
	data, _ := json.MarshalIndent(input, "", "  ")
	os.WriteFile(srcPath, data, 0644)

	err := rewriteKnownMarketplaces(srcPath, dstPath, "/home/stephan", "/home/vscode")
	if err != nil {
		t.Fatalf("rewriteKnownMarketplaces failed: %v", err)
	}

	outData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var result map[string]knownMarketplaceEntry
	if err := json.Unmarshal(outData, &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	entry := result["claude-code-lsps"]
	want := "/home/vscode/.claude/plugins/marketplaces/claude-code-lsps"
	if entry.InstallLocation != want {
		t.Errorf("installLocation = %q, want %q", entry.InstallLocation, want)
	}

	if entry.Source.Repo != "boostvolt/claude-code-lsps" {
		t.Error("source.repo was lost")
	}
}

func TestRewriteMissingFiles(t *testing.T) {
	dir := t.TempDir()

	// Should not error when source files don't exist
	err := rewriteInstalledPlugins(
		filepath.Join(dir, "nonexistent.json"),
		filepath.Join(dir, "out.json"),
		"/home/stephan", "/home/vscode",
	)
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}

	err = rewriteKnownMarketplaces(
		filepath.Join(dir, "nonexistent.json"),
		filepath.Join(dir, "out.json"),
		"/home/stephan", "/home/vscode",
	)
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
}

func TestRewriteIdempotent(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "installed_plugins.json")
	dstPath := filepath.Join(dir, "out.json")

	input := installedPluginsFile{
		Version: 2,
		Plugins: map[string][]installedPluginEntry{
			"test": {
				{
					InstallPath: "/home/stephan/.claude/plugins/test",
					Version:     "1.0.0",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(input, "", "  ")
	os.WriteFile(srcPath, data, 0644)

	// Run twice
	rewriteInstalledPlugins(srcPath, dstPath, "/home/stephan", "/home/vscode")
	first, _ := os.ReadFile(dstPath)

	rewriteInstalledPlugins(srcPath, dstPath, "/home/stephan", "/home/vscode")
	second, _ := os.ReadFile(dstPath)

	if string(first) != string(second) {
		t.Errorf("rewrite not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestRewriteClaudeConfigCopiesSettings(t *testing.T) {
	// Create a fake home directory structure
	dir := t.TempDir()
	hostHome := filepath.Join(dir, "hosthome")
	devcontainerDir := filepath.Join(dir, "devcontainer")

	os.MkdirAll(filepath.Join(hostHome, ".claude", "plugins"), 0755)

	settingsContent := `{"enabledPlugins": {"gopls": true}}`
	os.WriteFile(filepath.Join(hostHome, ".claude", "settings.json"), []byte(settingsContent), 0644)

	err := RewriteClaudeConfig(hostHome, "/home/vscode", devcontainerDir)
	if err != nil {
		t.Fatalf("RewriteClaudeConfig failed: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(devcontainerDir, "settings.json"))
	if err != nil {
		t.Fatalf("failed to read copied settings.json: %v", err)
	}
	if string(out) != settingsContent {
		t.Errorf("settings.json content = %q, want %q", string(out), settingsContent)
	}
}

func TestRewritePath(t *testing.T) {
	tests := []struct {
		path, hostHome, containerHome, want string
	}{
		{"/home/stephan/.claude/plugins", "/home/stephan", "/home/vscode", "/home/vscode/.claude/plugins"},
		{"/other/path", "/home/stephan", "/home/vscode", "/other/path"},
		{"", "/home/stephan", "/home/vscode", ""},
		{"/home/stephan", "/home/stephan", "/home/vscode", "/home/vscode"},
	}
	for _, tt := range tests {
		got := rewritePath(tt.path, tt.hostHome, tt.containerHome)
		if got != tt.want {
			t.Errorf("rewritePath(%q, %q, %q) = %q, want %q", tt.path, tt.hostHome, tt.containerHome, got, tt.want)
		}
	}
}
