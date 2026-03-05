package dockerfile

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestAddAptPackages(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		packages   []string
		pluginName string
		want       string
	}{
		{
			name:       "add packages to simple apt command",
			pluginName: "test-plugin",
			input: `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*`,
			packages: []string{"tmux", "curl"},
			want: `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    # sandcutter:plugin:test-plugin:start \
    tmux \
    curl \
    # sandcutter:plugin:test-plugin:end \
    && rm -rf /var/lib/apt/lists/*`,
		},
		{
			name:       "add packages to multi-line apt command",
			pluginName: "tmux",
			input: `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        gh gosu jq vim \
    && rm -rf /var/lib/apt/lists/*`,
			packages: []string{"tmux", "fonts-dejavu"},
			want: `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        gh gosu jq vim \
        # sandcutter:plugin:tmux:start \
        tmux \
        fonts-dejavu \
        # sandcutter:plugin:tmux:end \
    && rm -rf /var/lib/apt/lists/*`,
		},
		{
			name:       "skip already installed packages",
			pluginName: "test-plugin",
			input: `FROM debian
RUN apt-get update \
    && apt-get install -y vim tmux \
    && rm -rf /var/lib/apt/lists/*`,
			packages: []string{"tmux", "curl"},
			want: `FROM debian
RUN apt-get update \
    && apt-get install -y vim tmux \
    # sandcutter:plugin:test-plugin:start \
    curl \
    # sandcutter:plugin:test-plugin:end \
    && rm -rf /var/lib/apt/lists/*`,
		},
		{
			name:       "no changes when all packages already installed",
			pluginName: "test-plugin",
			input: `FROM debian
RUN apt-get update \
    && apt-get install -y vim tmux curl \
    && rm -rf /var/lib/apt/lists/*`,
			packages: []string{"tmux", "curl"},
			want: `FROM debian
RUN apt-get update \
    && apt-get install -y vim tmux curl \
    && rm -rf /var/lib/apt/lists/*`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem
			fs := afero.NewMemMapFs()
			path := "/Dockerfile"
			afero.WriteFile(fs, path, []byte(tt.input), 0644)

			// Load Dockerfile
			df, err := LoadFs(fs, path)
			if err != nil {
				t.Fatalf("Failed to load Dockerfile: %v", err)
			}

			// Add packages
			pluginName := tt.pluginName
			if pluginName == "" {
				pluginName = "test-plugin"
			}
			err = df.AddAptPackages(tt.packages, pluginName)
			if err != nil {
				t.Fatalf("AddAptPackages failed: %v", err)
			}

			// Get result
			got := df.GetContent()
			want := tt.want + "\n"

			if got != want {
				t.Errorf("AddAptPackages() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
			}
		})
	}
}

func TestAddAptPackagesIdempotency(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	// Apply packages first time
	packages := []string{"tmux", "curl"}
	err = df.AddAptPackages(packages, "test-plugin")
	if err != nil {
		t.Fatalf("AddAptPackages failed: %v", err)
	}

	firstResult := df.GetContent()

	// Apply same packages again with same plugin name
	err = df.AddAptPackages(packages, "test-plugin")
	if err != nil {
		t.Fatalf("AddAptPackages failed on second run: %v", err)
	}

	secondResult := df.GetContent()

	// Should be identical - no duplication
	if firstResult != secondResult {
		t.Errorf("AddAptPackages not idempotent:\nFirst:\n%s\nSecond:\n%s", firstResult, secondResult)
	}

	// Verify markers are present
	if !strings.Contains(firstResult, "# sandcutter:plugin:test-plugin:start") {
		t.Error("Expected start marker to be present")
	}
	if !strings.Contains(firstResult, "# sandcutter:plugin:test-plugin:end") {
		t.Error("Expected end marker to be present")
	}
}

func TestAddAptPackagesMultiplePlugins(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	// Apply first plugin
	err = df.AddAptPackages([]string{"tmux"}, "plugin-one")
	if err != nil {
		t.Fatalf("AddAptPackages failed for plugin-one: %v", err)
	}

	// Apply second plugin
	err = df.AddAptPackages([]string{"curl"}, "plugin-two")
	if err != nil {
		t.Fatalf("AddAptPackages failed for plugin-two: %v", err)
	}

	result := df.GetContent()

	// Verify both sets of markers are present
	if !strings.Contains(result, "# sandcutter:plugin:plugin-one:start") {
		t.Error("Expected plugin-one start marker to be present")
	}
	if !strings.Contains(result, "# sandcutter:plugin:plugin-one:end") {
		t.Error("Expected plugin-one end marker to be present")
	}
	if !strings.Contains(result, "# sandcutter:plugin:plugin-two:start") {
		t.Error("Expected plugin-two start marker to be present")
	}
	if !strings.Contains(result, "# sandcutter:plugin:plugin-two:end") {
		t.Error("Expected plugin-two end marker to be present")
	}

	// Verify both packages are present
	if !strings.Contains(result, "tmux") {
		t.Error("Expected tmux package to be present")
	}
	if !strings.Contains(result, "curl") {
		t.Error("Expected curl package to be present")
	}
}

func TestHasPackage(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        gh gosu jq vim \
        fonts-dejavu tmux \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	tests := []struct {
		pkg  string
		want bool
	}{
		{"vim", true},
		{"tmux", true},
		{"fonts-dejavu", true},
		{"curl", false},
		{"vi", false}, // Should not match "vim"
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			got := df.HasPackage(tt.pkg)
			if got != tt.want {
				t.Errorf("HasPackage(%q) = %v, want %v", tt.pkg, got, tt.want)
			}
		})
	}
}

func TestAddLocaleSetup(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y locales \
    && rm -rf /var/lib/apt/lists/*

USER vscode`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddLocaleSetup("en_US.UTF-8")
	if err != nil {
		t.Fatalf("AddLocaleSetup failed: %v", err)
	}

	got := df.GetContent()

	// Check that locale commands were added
	if !strings.Contains(got, "locale-gen") {
		t.Error("Expected locale-gen command to be added")
	}
	if !strings.Contains(got, "LANG=en_US.UTF-8") {
		t.Error("Expected LANG environment variable to be set")
	}
	if !strings.Contains(got, "sandcutter:managed") {
		t.Error("Expected sandcutter:managed marker")
	}
}

func TestAddLocaleSetupIdempotency(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y locales \
    && rm -rf /var/lib/apt/lists/*

# Configure locale for UTF-8 support (sandcutter:managed)
RUN sed -i '/en_US\.UTF-8/s/^# //g' /etc/locale.gen \
    && locale-gen
ENV LANG=en_US.UTF-8 LANGUAGE=en_US:en LC_ALL=en_US.UTF-8

USER vscode`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	before := df.GetContent()

	err = df.AddLocaleSetup("en_US.UTF-8")
	if err != nil {
		t.Fatalf("AddLocaleSetup failed: %v", err)
	}

	after := df.GetContent()

	if before != after {
		t.Error("AddLocaleSetup should be idempotent (no changes when already configured)")
	}
}

func TestAddCopyCommand(t *testing.T) {
	input := `FROM debian
USER vscode
RUN echo "setup"

USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddCopyCommand("tmux.conf", "/etc/skel/.tmux.conf", "644")
	if err != nil {
		t.Fatalf("AddCopyCommand failed: %v", err)
	}

	got := df.GetContent()

	// Should be inserted before ENTRYPOINT
	if !strings.Contains(got, "COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf") {
		t.Error("Expected COPY command to be added")
	}

	// Check it's before ENTRYPOINT
	copyIdx := strings.Index(got, "COPY --chmod=644")
	entrypointIdx := strings.Index(got, "ENTRYPOINT")
	if copyIdx > entrypointIdx {
		t.Error("COPY command should be before ENTRYPOINT")
	}
}

func TestAddRunCommands(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddRunCommands([]string{"echo hello", "echo world"}, "test-plugin")
	if err != nil {
		t.Fatalf("AddRunCommands failed: %v", err)
	}

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER root
# sandcutter:run:test-plugin:start
RUN echo hello
RUN echo world
# sandcutter:run:test-plugin:end
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("AddRunCommands() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestAddRunCommandsIdempotency(t *testing.T) {
	input := `FROM debian
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	commands := []string{"echo hello", "echo world"}

	err = df.AddRunCommands(commands, "test-plugin")
	if err != nil {
		t.Fatalf("AddRunCommands failed: %v", err)
	}
	firstResult := df.GetContent()

	err = df.AddRunCommands(commands, "test-plugin")
	if err != nil {
		t.Fatalf("AddRunCommands failed on second run: %v", err)
	}
	secondResult := df.GetContent()

	if firstResult != secondResult {
		t.Errorf("AddRunCommands not idempotent:\nFirst:\n%s\nSecond:\n%s", firstResult, secondResult)
	}
}

func TestAddRunCommandsBeforeCopy(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER root
# sandcutter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddRunCommands([]string{"echo hello", "echo world"}, "test-plugin")
	if err != nil {
		t.Fatalf("AddRunCommands failed: %v", err)
	}

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER root
# sandcutter:run:test-plugin:start
RUN echo hello
RUN echo world
# sandcutter:run:test-plugin:end
# sandcutter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("AddRunCommandsBeforeCopy() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestAddDockerEnv(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER root
# sandcutter:run:claude-tools:start
RUN /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
# sandcutter:run:claude-tools:end
# sandcutter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	envVars := map[string]string{
		"PATH": "/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:$PATH",
	}
	err = df.AddDockerEnv(envVars, "claude-tools")
	if err != nil {
		t.Fatalf("AddDockerEnv failed: %v", err)
	}

	got := df.GetContent()

	// ENV block should appear after the run block end marker
	if !strings.Contains(got, "# sandcutter:env:claude-tools:start") {
		t.Error("Expected env start marker to be present")
	}
	if !strings.Contains(got, "ENV PATH=/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:$PATH") {
		t.Error("Expected ENV PATH instruction to be present")
	}
	if !strings.Contains(got, "# sandcutter:env:claude-tools:end") {
		t.Error("Expected env end marker to be present")
	}

	// ENV should be after run:end and before file marker
	runEndIdx := strings.Index(got, "# sandcutter:run:claude-tools:end")
	envStartIdx := strings.Index(got, "# sandcutter:env:claude-tools:start")
	fileIdx := strings.Index(got, "# sandcutter:file:")
	if envStartIdx < runEndIdx {
		t.Error("ENV block should be after run block")
	}
	if envStartIdx > fileIdx {
		t.Error("ENV block should be before file COPY markers")
	}
}

func TestAddDockerEnvIdempotency(t *testing.T) {
	input := `FROM debian
USER root
# sandcutter:run:claude-tools:start
RUN echo hello
# sandcutter:run:claude-tools:end
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	envVars := map[string]string{
		"PATH": "/home/linuxbrew/.linuxbrew/bin:$PATH",
	}

	err = df.AddDockerEnv(envVars, "claude-tools")
	if err != nil {
		t.Fatalf("AddDockerEnv failed: %v", err)
	}
	firstResult := df.GetContent()

	err = df.AddDockerEnv(envVars, "claude-tools")
	if err != nil {
		t.Fatalf("AddDockerEnv failed on second run: %v", err)
	}
	secondResult := df.GetContent()

	if firstResult != secondResult {
		t.Errorf("AddDockerEnv not idempotent:\nFirst:\n%s\nSecond:\n%s", firstResult, secondResult)
	}
}

func TestScanPlugins(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
        # sandcutter:plugin:tmux:start \
        tmux \
        # sandcutter:plugin:tmux:end \
        # sandcutter:plugin:claude-tools:start \
        build-essential \
        # sandcutter:plugin:claude-tools:end \
    && rm -rf /var/lib/apt/lists/*
USER root
# sandcutter:run:tmux:start
RUN echo "tmux setup"
# sandcutter:run:tmux:end
# sandcutter:run:claude-tools:start
RUN curl -fsSL https://example.com/install.sh | bash
# sandcutter:run:claude-tools:end
# sandcutter:env:claude-tools:start
ENV PATH=/home/linuxbrew/.linuxbrew/bin:$PATH
# sandcutter:env:claude-tools:end
# sandcutter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
# sandcutter:file:/usr/local/bin/setup.sh
COPY --chmod=755 setup.sh /usr/local/bin/setup.sh
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	result := df.ScanPlugins()

	// Should find 2 plugins
	if len(result.Plugins) != 2 {
		t.Fatalf("Expected 2 plugins, got %d", len(result.Plugins))
	}

	// Build a map for easier lookup
	pluginMap := make(map[string]InstalledPlugin)
	for _, p := range result.Plugins {
		pluginMap[p.Name] = p
	}

	// Check tmux plugin
	tmux, ok := pluginMap["tmux"]
	if !ok {
		t.Fatal("Expected tmux plugin to be found")
	}
	if !tmux.Packages {
		t.Error("Expected tmux to have packages")
	}
	if !tmux.Run {
		t.Error("Expected tmux to have run commands")
	}
	if tmux.Env {
		t.Error("Expected tmux to NOT have env vars")
	}

	// Check claude-tools plugin
	ct, ok := pluginMap["claude-tools"]
	if !ok {
		t.Fatal("Expected claude-tools plugin to be found")
	}
	if !ct.Packages {
		t.Error("Expected claude-tools to have packages")
	}
	if !ct.Run {
		t.Error("Expected claude-tools to have run commands")
	}
	if !ct.Env {
		t.Error("Expected claude-tools to have env vars")
	}

	// Check files
	if len(result.Files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(result.Files))
	}
	expectedFiles := map[string]bool{
		"/etc/skel/.tmux.conf":      false,
		"/usr/local/bin/setup.sh":   false,
	}
	for _, f := range result.Files {
		if _, ok := expectedFiles[f]; !ok {
			t.Errorf("Unexpected file: %s", f)
		}
		expectedFiles[f] = true
	}
	for f, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file not found: %s", f)
		}
	}
}

func TestRemovePluginPackages(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
        # sandcutter:plugin:tmux:start \
        tmux \
        # sandcutter:plugin:tmux:end \
        # sandcutter:plugin:claude-tools:start \
        build-essential \
        # sandcutter:plugin:claude-tools:end \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	// Remove tmux, keep claude-tools
	df.RemovePluginPackages("tmux")

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
        # sandcutter:plugin:claude-tools:start \
        build-essential \
        # sandcutter:plugin:claude-tools:end \
    && rm -rf /var/lib/apt/lists/*
`

	if got != want {
		t.Errorf("RemovePluginPackages() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}

	// Verify claude-tools is still there
	if !strings.Contains(got, "# sandcutter:plugin:claude-tools:start") {
		t.Error("Expected claude-tools plugin to remain")
	}
	if !strings.Contains(got, "build-essential") {
		t.Error("Expected build-essential package to remain")
	}
}

func TestRemovePluginPackagesLastBlock(t *testing.T) {
	// When the removed block is the last one before &&
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
        # sandcutter:plugin:tmux:start \
        tmux \
        # sandcutter:plugin:tmux:end \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemovePluginPackages("tmux")

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
    && rm -rf /var/lib/apt/lists/*
`

	if got != want {
		t.Errorf("RemovePluginPackagesLastBlock() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRemoveRunCommands(t *testing.T) {
	input := `FROM debian
USER root
# sandcutter:run:test-plugin:start
RUN echo hello
RUN echo world
# sandcutter:run:test-plugin:end
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveRunCommands("test-plugin")

	got := df.GetContent()

	want := `FROM debian
USER root
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("RemoveRunCommands() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRemoveDockerEnv(t *testing.T) {
	input := `FROM debian
USER root
# sandcutter:run:claude-tools:start
RUN echo hello
# sandcutter:run:claude-tools:end
# sandcutter:env:claude-tools:start
ENV PATH=/home/linuxbrew/.linuxbrew/bin:$PATH
# sandcutter:env:claude-tools:end
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveDockerEnv("claude-tools")

	got := df.GetContent()

	want := `FROM debian
USER root
# sandcutter:run:claude-tools:start
RUN echo hello
# sandcutter:run:claude-tools:end
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("RemoveDockerEnv() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRemoveCopyCommand(t *testing.T) {
	input := `FROM debian
USER root
# sandcutter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveCopyCommand("/etc/skel/.tmux.conf")

	got := df.GetContent()

	want := `FROM debian
USER root
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("RemoveCopyCommand() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestAddCopyCommandIdempotency(t *testing.T) {
	input := `FROM debian
USER root
# sandcutter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	before := df.GetContent()

	err = df.AddCopyCommand("tmux.conf", "/etc/skel/.tmux.conf", "644")
	if err != nil {
		t.Fatalf("AddCopyCommand failed: %v", err)
	}

	after := df.GetContent()

	if before != after {
		t.Error("AddCopyCommand should be idempotent (no changes when already added)")
	}
}

func TestAddRunCommandsNoEntrypoint(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER node`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddRunCommands([]string{"echo hello"}, "test-plugin")
	if err != nil {
		t.Fatalf("AddRunCommands failed: %v", err)
	}

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
# sandcutter:run:test-plugin:start
RUN echo hello
# sandcutter:run:test-plugin:end
USER node
`

	if got != want {
		t.Errorf("AddRunCommandsNoEntrypoint() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestAddCopyCommandNoEntrypoint(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER node`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddCopyCommand("tmux.conf", "/etc/skel/.tmux.conf", "644")
	if err != nil {
		t.Fatalf("AddCopyCommand failed: %v", err)
	}

	got := df.GetContent()

	// COPY should be inserted before USER node
	if !strings.Contains(got, "COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf") {
		t.Error("Expected COPY command to be added")
	}

	copyIdx := strings.Index(got, "COPY --chmod=644")
	userIdx := strings.LastIndex(got, "USER node")
	if copyIdx > userIdx {
		t.Error("COPY command should be before final USER line")
	}
}

func TestAddDockerEnvNoEntrypoint(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER node`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	envVars := map[string]string{
		"PATH": "/custom/bin:$PATH",
	}
	err = df.AddDockerEnv(envVars, "test-plugin")
	if err != nil {
		t.Fatalf("AddDockerEnv failed: %v", err)
	}

	got := df.GetContent()

	// ENV should be inserted before USER node
	if !strings.Contains(got, "# sandcutter:env:test-plugin:start") {
		t.Error("Expected env start marker to be present")
	}

	envIdx := strings.Index(got, "# sandcutter:env:test-plugin:start")
	userIdx := strings.LastIndex(got, "USER node")
	if envIdx > userIdx {
		t.Error("ENV block should be before final USER line")
	}
}
