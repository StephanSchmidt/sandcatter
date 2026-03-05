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
    # sandcutter:plugin:test-plugin:start
    tmux \
    curl \
    # sandcutter:plugin:test-plugin:end
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
        # sandcutter:plugin:tmux:start
        tmux \
        fonts-dejavu \
        # sandcutter:plugin:tmux:end
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
    # sandcutter:plugin:test-plugin:start
    curl \
    # sandcutter:plugin:test-plugin:end
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
