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
    # sandcatter:plugin:test-plugin:start \
    tmux \
    curl \
    # sandcatter:plugin:test-plugin:end \
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
        # sandcatter:plugin:tmux:start \
        tmux \
        fonts-dejavu \
        # sandcatter:plugin:tmux:end \
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
    # sandcatter:plugin:test-plugin:start \
    curl \
    # sandcatter:plugin:test-plugin:end \
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
	if !strings.Contains(firstResult, "# sandcatter:plugin:test-plugin:start") {
		t.Error("Expected start marker to be present")
	}
	if !strings.Contains(firstResult, "# sandcatter:plugin:test-plugin:end") {
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
	if !strings.Contains(result, "# sandcatter:plugin:plugin-one:start") {
		t.Error("Expected plugin-one start marker to be present")
	}
	if !strings.Contains(result, "# sandcatter:plugin:plugin-one:end") {
		t.Error("Expected plugin-one end marker to be present")
	}
	if !strings.Contains(result, "# sandcatter:plugin:plugin-two:start") {
		t.Error("Expected plugin-two start marker to be present")
	}
	if !strings.Contains(result, "# sandcatter:plugin:plugin-two:end") {
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
	if !strings.Contains(got, "sandcatter:managed") {
		t.Error("Expected sandcatter:managed marker")
	}
}

func TestAddLocaleSetupIdempotency(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y locales \
    && rm -rf /var/lib/apt/lists/*

# Configure locale for UTF-8 support (sandcatter:managed)
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

func TestAddUserRunCommands(t *testing.T) {
	input := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER vscode
RUN mise use -g node@lts
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	err = df.AddUserRunCommands([]string{"bash -c 'curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash && source /home/vscode/.gvm/scripts/gvm && gvm install go1.25.7 -B && gvm use go1.25.7 --default && go install golang.org/x/tools/gopls@latest'"}, "golsp")
	if err != nil {
		t.Fatalf("AddUserRunCommands failed: %v", err)
	}

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update && apt-get install -y vim && rm -rf /var/lib/apt/lists/*
USER vscode
RUN mise use -g node@lts

# sandcatter:run:golsp:start
RUN bash -c 'curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash && source /home/vscode/.gvm/scripts/gvm && gvm install go1.25.7 -B && gvm use go1.25.7 --default && go install golang.org/x/tools/gopls@latest'
# sandcatter:run:golsp:end
USER root
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("AddUserRunCommands() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestAddUserRunCommandsIdempotency(t *testing.T) {
	input := `FROM debian
USER vscode
RUN mise use -g node@lts
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	commands := []string{"bash -c 'curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash && source /home/vscode/.gvm/scripts/gvm && gvm install go1.25.7 -B && gvm use go1.25.7 --default'"}

	err = df.AddUserRunCommands(commands, "golsp")
	if err != nil {
		t.Fatalf("AddUserRunCommands failed: %v", err)
	}
	firstResult := df.GetContent()

	err = df.AddUserRunCommands(commands, "golsp")
	if err != nil {
		t.Fatalf("AddUserRunCommands failed on second run: %v", err)
	}
	secondResult := df.GetContent()

	if firstResult != secondResult {
		t.Errorf("AddUserRunCommands not idempotent:\nFirst:\n%s\nSecond:\n%s", firstResult, secondResult)
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
# sandcatter:run:test-plugin:start
RUN echo hello
RUN echo world
# sandcatter:run:test-plugin:end
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
# sandcatter:file:/etc/skel/.tmux.conf
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
# sandcatter:run:test-plugin:start
RUN echo hello
RUN echo world
# sandcatter:run:test-plugin:end
# sandcatter:file:/etc/skel/.tmux.conf
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
# sandcatter:run:claude-tools:start
RUN /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
# sandcatter:run:claude-tools:end
# sandcatter:file:/etc/skel/.tmux.conf
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
	if !strings.Contains(got, "# sandcatter:env:claude-tools:start") {
		t.Error("Expected env start marker to be present")
	}
	if !strings.Contains(got, "ENV PATH=/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:$PATH") {
		t.Error("Expected ENV PATH instruction to be present")
	}
	if !strings.Contains(got, "# sandcatter:env:claude-tools:end") {
		t.Error("Expected env end marker to be present")
	}

	// ENV should be after run:end and before file marker
	runEndIdx := strings.Index(got, "# sandcatter:run:claude-tools:end")
	envStartIdx := strings.Index(got, "# sandcatter:env:claude-tools:start")
	fileIdx := strings.Index(got, "# sandcatter:file:")
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
# sandcatter:run:claude-tools:start
RUN echo hello
# sandcatter:run:claude-tools:end
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
        # sandcatter:plugin:tmux:start \
        tmux \
        # sandcatter:plugin:tmux:end \
        # sandcatter:plugin:claude-tools:start \
        build-essential \
        # sandcatter:plugin:claude-tools:end \
    && rm -rf /var/lib/apt/lists/*
USER root
# sandcatter:run:tmux:start
RUN echo "tmux setup"
# sandcatter:run:tmux:end
# sandcatter:run:claude-tools:start
RUN curl -fsSL https://example.com/install.sh | bash
# sandcatter:run:claude-tools:end
# sandcatter:env:claude-tools:start
ENV PATH=/home/linuxbrew/.linuxbrew/bin:$PATH
# sandcatter:env:claude-tools:end
# sandcatter:file:/etc/skel/.tmux.conf
COPY --chmod=644 tmux.conf /etc/skel/.tmux.conf
# sandcatter:file:/usr/local/bin/setup.sh
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
        # sandcatter:plugin:tmux:start \
        tmux \
        # sandcatter:plugin:tmux:end \
        # sandcatter:plugin:claude-tools:start \
        build-essential \
        # sandcatter:plugin:claude-tools:end \
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
        # sandcatter:plugin:claude-tools:start \
        build-essential \
        # sandcatter:plugin:claude-tools:end \
    && rm -rf /var/lib/apt/lists/*
`

	if got != want {
		t.Errorf("RemovePluginPackages() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}

	// Verify claude-tools is still there
	if !strings.Contains(got, "# sandcatter:plugin:claude-tools:start") {
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
        # sandcatter:plugin:tmux:start \
        tmux \
        # sandcatter:plugin:tmux:end \
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
# sandcatter:run:test-plugin:start
RUN echo hello
RUN echo world
# sandcatter:run:test-plugin:end
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
# sandcatter:run:claude-tools:start
RUN echo hello
# sandcatter:run:claude-tools:end
# sandcatter:env:claude-tools:start
ENV PATH=/home/linuxbrew/.linuxbrew/bin:$PATH
# sandcatter:env:claude-tools:end
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
# sandcatter:run:claude-tools:start
RUN echo hello
# sandcatter:run:claude-tools:end
ENTRYPOINT ["/bin/sh"]
`

	if got != want {
		t.Errorf("RemoveDockerEnv() mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRemoveCopyCommand(t *testing.T) {
	input := `FROM debian
USER root
# sandcatter:file:/etc/skel/.tmux.conf
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

func TestRemovePluginPackagesNonExistent(t *testing.T) {
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

	before := df.GetContent()
	df.RemovePluginPackages("nonexistent")
	after := df.GetContent()

	if before != after {
		t.Errorf("Removing non-existent plugin should not change content:\nBefore:\n%s\nAfter:\n%s", before, after)
	}
}

func TestRemovePluginPackagesFirstBlock(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
        # sandcatter:plugin:first:start \
        pkg-a \
        # sandcatter:plugin:first:end \
        # sandcatter:plugin:second:start \
        pkg-b \
        # sandcatter:plugin:second:end \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemovePluginPackages("first")

	got := df.GetContent()

	if strings.Contains(got, "pkg-a") {
		t.Error("Expected pkg-a to be removed")
	}
	if !strings.Contains(got, "pkg-b") {
		t.Error("Expected pkg-b to remain")
	}
	if !strings.Contains(got, "# sandcatter:plugin:second:start") {
		t.Error("Expected second plugin block to remain")
	}
}

func TestRemovePluginPackagesAllBlocks(t *testing.T) {
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
        # sandcatter:plugin:first:start \
        pkg-a \
        # sandcatter:plugin:first:end \
        # sandcatter:plugin:second:start \
        pkg-b \
        # sandcatter:plugin:second:end \
    && rm -rf /var/lib/apt/lists/*`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemovePluginPackages("first")
	df.RemovePluginPackages("second")

	got := df.GetContent()

	want := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        vim \
    && rm -rf /var/lib/apt/lists/*
`

	if got != want {
		t.Errorf("RemovePluginPackages all blocks mismatch:\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRemoveRunCommandsNonExistent(t *testing.T) {
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

	before := df.GetContent()
	df.RemoveRunCommands("nonexistent")
	after := df.GetContent()

	if before != after {
		t.Errorf("Removing non-existent run commands should not change content")
	}
}

func TestRemoveRunCommandsKeepsOtherPlugins(t *testing.T) {
	input := `FROM debian
USER root
# sandcatter:run:plugin-a:start
RUN echo a
# sandcatter:run:plugin-a:end
# sandcatter:run:plugin-b:start
RUN echo b
# sandcatter:run:plugin-b:end
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveRunCommands("plugin-a")

	got := df.GetContent()

	if strings.Contains(got, "echo a") {
		t.Error("Expected plugin-a run command to be removed")
	}
	if !strings.Contains(got, "echo b") {
		t.Error("Expected plugin-b run command to remain")
	}
	if !strings.Contains(got, "# sandcatter:run:plugin-b:start") {
		t.Error("Expected plugin-b markers to remain")
	}
}

func TestRemoveDockerEnvNonExistent(t *testing.T) {
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

	before := df.GetContent()
	df.RemoveDockerEnv("nonexistent")
	after := df.GetContent()

	if before != after {
		t.Errorf("Removing non-existent env should not change content")
	}
}

func TestRemoveDockerEnvKeepsOtherPlugins(t *testing.T) {
	input := `FROM debian
USER root
# sandcatter:env:plugin-a:start
ENV FOO=bar
# sandcatter:env:plugin-a:end
# sandcatter:env:plugin-b:start
ENV BAZ=qux
# sandcatter:env:plugin-b:end
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveDockerEnv("plugin-a")

	got := df.GetContent()

	if strings.Contains(got, "FOO=bar") {
		t.Error("Expected plugin-a env to be removed")
	}
	if !strings.Contains(got, "BAZ=qux") {
		t.Error("Expected plugin-b env to remain")
	}
}

func TestRemoveCopyCommandNonExistent(t *testing.T) {
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

	before := df.GetContent()
	df.RemoveCopyCommand("/nonexistent/path")
	after := df.GetContent()

	if before != after {
		t.Errorf("Removing non-existent copy command should not change content")
	}
}

func TestRemoveCopyCommandMarkerOnly(t *testing.T) {
	// Edge case: marker exists but next line is not a COPY command
	input := `FROM debian
USER root
# sandcatter:file:/etc/config
ENV FOO=bar
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveCopyCommand("/etc/config")

	got := df.GetContent()

	// Marker should be removed, but ENV line should remain
	if strings.Contains(got, "sandcatter:file:/etc/config") {
		t.Error("Expected marker to be removed")
	}
	if !strings.Contains(got, "ENV FOO=bar") {
		t.Error("Expected ENV line to be preserved when it's not a COPY")
	}
}

func TestRemoveCopyCommandMultipleFiles(t *testing.T) {
	input := `FROM debian
USER root
# sandcatter:file:/etc/config-a
COPY --chmod=644 config-a /etc/config-a
# sandcatter:file:/etc/config-b
COPY --chmod=755 config-b /etc/config-b
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	df.RemoveCopyCommand("/etc/config-a")

	got := df.GetContent()

	if strings.Contains(got, "config-a") {
		t.Error("Expected config-a COPY to be removed")
	}
	if !strings.Contains(got, "config-b") {
		t.Error("Expected config-b COPY to remain")
	}
}

func TestRemovePluginRoundTrip(t *testing.T) {
	// Apply packages, run commands, and env — then remove; result should match original.
	// Note: COPY round-trip is tested separately; AddCopyCommand inserts marker+COPY
	// as a single line element which RemoveCopyCommand handles independently.
	input := `FROM debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends vim \
    && rm -rf /var/lib/apt/lists/*
USER root
ENTRYPOINT ["/bin/sh"]`

	fs := afero.NewMemMapFs()
	path := "/Dockerfile"
	afero.WriteFile(fs, path, []byte(input), 0644)

	df, err := LoadFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load Dockerfile: %v", err)
	}

	original := df.GetContent()

	// Apply
	err = df.AddAptPackages([]string{"tmux", "curl"}, "test-plugin")
	if err != nil {
		t.Fatalf("AddAptPackages failed: %v", err)
	}
	err = df.AddRunCommands([]string{"echo hello"}, "test-plugin")
	if err != nil {
		t.Fatalf("AddRunCommands failed: %v", err)
	}
	envVars := map[string]string{"PATH": "/custom:$PATH"}
	err = df.AddDockerEnv(envVars, "test-plugin")
	if err != nil {
		t.Fatalf("AddDockerEnv failed: %v", err)
	}

	// Verify something was added
	applied := df.GetContent()
	if applied == original {
		t.Fatal("Expected content to change after apply")
	}

	// Remove
	df.RemovePluginPackages("test-plugin")
	df.RemoveRunCommands("test-plugin")
	df.RemoveDockerEnv("test-plugin")

	restored := df.GetContent()

	if restored != original {
		t.Errorf("Round-trip failed — content differs after remove:\nOriginal:\n%s\nRestored:\n%s", original, restored)
	}
}

func TestAddCopyCommandIdempotency(t *testing.T) {
	input := `FROM debian
USER root
# sandcatter:file:/etc/skel/.tmux.conf
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
# sandcatter:run:test-plugin:start
RUN echo hello
# sandcatter:run:test-plugin:end
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
	if !strings.Contains(got, "# sandcatter:env:test-plugin:start") {
		t.Error("Expected env start marker to be present")
	}

	envIdx := strings.Index(got, "# sandcatter:env:test-plugin:start")
	userIdx := strings.LastIndex(got, "USER node")
	if envIdx > userIdx {
		t.Error("ENV block should be before final USER line")
	}
}
