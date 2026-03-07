package plugin

import (
	"strings"
	"testing"

	"github.com/StephanSchmidt/sandcatter/pkg/dockerfile"
	"github.com/spf13/afero"
)

// setupTestFiles creates a minimal devcontainer setup on the given filesystem
// and returns the Dockerfile and ComposeFile objects.
func setupTestFiles(t *testing.T, fs afero.Fs, dockerfileContent, composeContent string) (*dockerfile.Dockerfile, *dockerfile.ComposeFile) {
	t.Helper()

	dfPath := "/project/.devcontainer/Dockerfile"
	fs.MkdirAll("/project/.devcontainer", 0755)
	afero.WriteFile(fs, dfPath, []byte(dockerfileContent), 0644)

	df, err := dockerfile.LoadFs(fs, dfPath)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	var cf *dockerfile.ComposeFile
	if composeContent != "" {
		cfPath := "/project/.devcontainer/compose.yml"
		afero.WriteFile(fs, cfPath, []byte(composeContent), 0644)
		cf, err = dockerfile.LoadComposeFs(fs, cfPath)
		if err != nil {
			t.Fatalf("Failed to load compose file: %v", err)
		}
	}

	return df, cf
}

// applyPlugin simulates what Applier.Apply does using the given df/cf objects.
func applyPlugin(t *testing.T, df *dockerfile.Dockerfile, cf *dockerfile.ComposeFile, p *Plugin) {
	t.Helper()

	allPackages := append([]string{}, p.LocalePackages...)
	allPackages = append(allPackages, p.Fonts...)
	allPackages = append(allPackages, p.AptPackages...)

	if len(allPackages) > 0 {
		if err := df.AddAptPackages(allPackages, p.Name); err != nil {
			t.Fatalf("AddAptPackages failed: %v", err)
		}
	}
	if len(p.RunCommands) > 0 {
		if p.RunAs == "user" {
			if err := df.AddUserRunCommands(p.RunCommands, p.Name); err != nil {
				t.Fatalf("AddUserRunCommands failed: %v", err)
			}
		} else {
			if err := df.AddRunCommands(p.RunCommands, p.Name); err != nil {
				t.Fatalf("AddRunCommands failed: %v", err)
			}
		}
	}
	if len(p.DockerEnv) > 0 {
		if err := df.AddDockerEnv(p.DockerEnv, p.Name); err != nil {
			t.Fatalf("AddDockerEnv failed: %v", err)
		}
	}
	if cf != nil && len(p.ComposeEnv) > 0 {
		if err := cf.AddEnvironmentVariables(p.ComposeEnv); err != nil {
			t.Fatalf("AddEnvironmentVariables failed: %v", err)
		}
	}
	if cf != nil && len(p.ComposeVolumes) > 0 {
		if err := cf.AddVolumes(p.ComposeVolumes); err != nil {
			t.Fatalf("AddVolumes failed: %v", err)
		}
	}
	if cf != nil && p.ComposeCommand != "" {
		if err := cf.SetComposeCommand(p.ComposeCommand); err != nil {
			t.Fatalf("SetComposeCommand failed: %v", err)
		}
	}
}

// removePlugin simulates what Applier.Remove does using the given df/cf objects.
func removePlugin(t *testing.T, df *dockerfile.Dockerfile, cf *dockerfile.ComposeFile, p *Plugin) {
	t.Helper()

	df.RemovePluginPackages(p.Name)
	df.RemoveRunCommands(p.Name)
	df.RemoveDockerEnv(p.Name)

	for _, file := range p.Files {
		df.RemoveCopyCommand(file.Destination)
	}

	if cf != nil && len(p.ComposeEnv) > 0 {
		if err := cf.RemoveEnvironmentVariables(p.ComposeEnv); err != nil {
			t.Fatalf("RemoveEnvironmentVariables failed: %v", err)
		}
	}
	if cf != nil && len(p.ComposeVolumes) > 0 {
		if err := cf.RemoveVolumes(p.ComposeVolumes); err != nil {
			t.Fatalf("RemoveVolumes failed: %v", err)
		}
	}
	if cf != nil && p.ComposeCommand != "" {
		if err := cf.RemoveComposeCommand(); err != nil {
			t.Fatalf("RemoveComposeCommand failed: %v", err)
		}
	}
}

func TestRemoveFullPlugin(t *testing.T) {
	dockerfileContent := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*
USER root
ENTRYPOINT ["/bin/sh"]`

	composeContent := `name: test-project

services:
  app:
    build:
      context: .
    volumes:
      - ..:/workspaces/test:cached
    command: sleep infinity`

	fs := afero.NewMemMapFs()
	df, cf := setupTestFiles(t, fs, dockerfileContent, composeContent)

	plugin := &Plugin{
		Name:        "test-plugin",
		Version:     "1.0.0",
		AptPackages: []string{"tmux", "curl"},
		RunCommands: []string{"echo setup"},
		DockerEnv:   map[string]string{"PATH": "/custom/bin:$PATH"},
		ComposeEnv:  map[string]string{"TERM": "xterm-256color"},
		ComposeVolumes: []string{"${HOME}/.config:/home/vscode/.config"},
		ComposeCommand: "tmux",
	}

	originalDf := df.GetContent()
	originalCf := cf.GetContent()

	// Apply
	applyPlugin(t, df, cf, plugin)

	// Verify apply changed things
	if df.GetContent() == originalDf {
		t.Fatal("Expected Dockerfile to change after apply")
	}
	if cf.GetContent() == originalCf {
		t.Fatal("Expected compose file to change after apply")
	}

	// Remove
	removePlugin(t, df, cf, plugin)

	restoredDf := df.GetContent()
	restoredCf := cf.GetContent()

	if restoredDf != originalDf {
		t.Errorf("Dockerfile not restored after remove:\nOriginal:\n%s\nRestored:\n%s", originalDf, restoredDf)
	}

	// Compose: check key elements are restored (env section header may linger)
	if strings.Contains(restoredCf, "TERM") {
		t.Error("Expected TERM env var to be removed from compose")
	}
	if strings.Contains(restoredCf, ".config") {
		t.Error("Expected .config volume to be removed from compose")
	}
	if !strings.Contains(restoredCf, "..:/workspaces/test:cached") {
		t.Error("Expected original volume to remain in compose")
	}
}

func TestRemoveOneOfTwoPlugins(t *testing.T) {
	dockerfileContent := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*
USER root
ENTRYPOINT ["/bin/sh"]`

	composeContent := `name: test-project

services:
  app:
    build:
      context: .
    volumes:
      - ..:/workspaces/test:cached
    command: sleep infinity`

	fs := afero.NewMemMapFs()
	df, cf := setupTestFiles(t, fs, dockerfileContent, composeContent)

	pluginA := &Plugin{
		Name:        "plugin-a",
		Version:     "1.0.0",
		AptPackages: []string{"tmux"},
		RunCommands: []string{"echo setup-a"},
		DockerEnv:   map[string]string{"FOO": "bar"},
		ComposeEnv:  map[string]string{"TERM": "xterm-256color"},
	}

	pluginB := &Plugin{
		Name:        "plugin-b",
		Version:     "1.0.0",
		AptPackages: []string{"curl"},
		RunCommands: []string{"echo setup-b"},
		DockerEnv:   map[string]string{"BAZ": "qux"},
		ComposeEnv:  map[string]string{"EDITOR": "vim"},
	}

	// Apply both
	applyPlugin(t, df, cf, pluginA)
	applyPlugin(t, df, cf, pluginB)

	// Remove only plugin-a
	removePlugin(t, df, cf, pluginA)

	dfContent := df.GetContent()
	cfContent := cf.GetContent()

	// plugin-a should be gone
	if strings.Contains(dfContent, "sandcatter:plugin:plugin-a") {
		t.Error("Expected plugin-a package markers to be removed")
	}
	if strings.Contains(dfContent, "sandcatter:run:plugin-a") {
		t.Error("Expected plugin-a run markers to be removed")
	}
	if strings.Contains(dfContent, "sandcatter:env:plugin-a") {
		t.Error("Expected plugin-a env markers to be removed")
	}
	if strings.Contains(cfContent, "TERM") {
		t.Error("Expected plugin-a compose env to be removed")
	}

	// plugin-b should remain
	if !strings.Contains(dfContent, "sandcatter:plugin:plugin-b") {
		t.Error("Expected plugin-b package markers to remain")
	}
	if !strings.Contains(dfContent, "curl") {
		t.Error("Expected curl package to remain")
	}
	if !strings.Contains(dfContent, "sandcatter:run:plugin-b") {
		t.Error("Expected plugin-b run markers to remain")
	}
	if !strings.Contains(dfContent, "echo setup-b") {
		t.Error("Expected plugin-b run command to remain")
	}
	if !strings.Contains(dfContent, "BAZ=qux") {
		t.Error("Expected plugin-b env to remain")
	}
	if !strings.Contains(cfContent, "EDITOR=vim") {
		t.Error("Expected plugin-b compose env to remain")
	}
}

func TestRemovePluginDockerfileOnly(t *testing.T) {
	// Test removal when there is no compose file
	dockerfileContent := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	df, _ := setupTestFiles(t, fs, dockerfileContent, "")

	plugin := &Plugin{
		Name:        "test-plugin",
		Version:     "1.0.0",
		AptPackages: []string{"tmux"},
		RunCommands: []string{"echo hello"},
	}

	originalDf := df.GetContent()

	applyPlugin(t, df, nil, plugin)
	removePlugin(t, df, nil, plugin)

	if df.GetContent() != originalDf {
		t.Errorf("Dockerfile not restored after remove without compose:\nOriginal:\n%s\nRestored:\n%s", originalDf, df.GetContent())
	}
}

func TestRemovePluginSaveToFs(t *testing.T) {
	// Test that removal persists when saved to afero filesystem
	dockerfileContent := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	dfPath := "/project/.devcontainer/Dockerfile"
	fs.MkdirAll("/project/.devcontainer", 0755)
	afero.WriteFile(fs, dfPath, []byte(dockerfileContent), 0644)

	df, err := dockerfile.LoadFs(fs, dfPath)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	plugin := &Plugin{
		Name:        "test-plugin",
		Version:     "1.0.0",
		AptPackages: []string{"tmux"},
		RunCommands: []string{"echo hello"},
		DockerEnv:   map[string]string{"FOO": "bar"},
	}

	applyPlugin(t, df, nil, plugin)

	// Save applied state
	if err := df.Save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Reload from filesystem
	df2, err := dockerfile.LoadFs(fs, dfPath)
	if err != nil {
		t.Fatalf("Failed to reload Dockerfile: %v", err)
	}

	// Remove and save
	removePlugin(t, df2, nil, plugin)
	if err := df2.Save(); err != nil {
		t.Fatalf("Failed to save after remove: %v", err)
	}

	// Reload and verify
	df3, err := dockerfile.LoadFs(fs, dfPath)
	if err != nil {
		t.Fatalf("Failed to reload Dockerfile after remove: %v", err)
	}

	content := df3.GetContent()
	if strings.Contains(content, "sandcatter:plugin:test-plugin") {
		t.Error("Expected plugin markers to be gone after save/reload")
	}
	if strings.Contains(content, "sandcatter:run:test-plugin") {
		t.Error("Expected run markers to be gone after save/reload")
	}
	if strings.Contains(content, "sandcatter:env:test-plugin") {
		t.Error("Expected env markers to be gone after save/reload")
	}
	if !strings.Contains(content, "vim") {
		t.Error("Expected original vim package to remain")
	}
}
