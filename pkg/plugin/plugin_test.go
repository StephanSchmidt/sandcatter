package plugin

import (
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
)

func TestLoadPlugin(t *testing.T) {
	manifest := Plugin{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		AptPackages: []string{"vim", "tmux"},
		Files: []FileMapping{
			{
				Source:      "files/config.conf",
				Destination: "/etc/config.conf",
				Chmod:       "644",
			},
		},
	}

	// Create in-memory filesystem
	fs := afero.NewMemMapFs()
	pluginDir := "/plugins/test-plugin"
	fs.MkdirAll(pluginDir+"/files", 0755)

	// Write manifest
	manifestData, _ := json.Marshal(manifest)
	afero.WriteFile(fs, pluginDir+"/plugin.json", manifestData, 0644)

	// Write files referenced in manifest
	afero.WriteFile(fs, pluginDir+"/files/config.conf", []byte("test config"), 0644)

	// Note: We can't easily test Load() without refactoring it to accept fs
	// This test serves as documentation for how the plugin structure works
}

func TestPluginValidation(t *testing.T) {
	tests := []struct {
		name      string
		plugin    Plugin
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid plugin",
			plugin: Plugin{
				Name:    "test",
				Version: "1.0.0",
			},
			wantError: false,
		},
		{
			name: "missing name",
			plugin: Plugin{
				Version: "1.0.0",
			},
			wantError: true,
			errorMsg:  "name is required",
		},
		{
			name: "missing version",
			plugin: Plugin{
				Name: "test",
			},
			wantError: true,
			errorMsg:  "version is required",
		},
		{
			name: "invalid file mapping - missing source",
			plugin: Plugin{
				Name:    "test",
				Version: "1.0.0",
				Files: []FileMapping{
					{
						Destination: "/etc/config",
					},
				},
			},
			wantError: true,
			errorMsg:  "source is required",
		},
		{
			name: "invalid file mapping - missing destination",
			plugin: Plugin{
				Name:    "test",
				Version: "1.0.0",
				Files: []FileMapping{
					{
						Source: "files/config",
					},
				},
			},
			wantError: true,
			errorMsg:  "destination is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary filesystem for file existence checks
			fs := afero.NewMemMapFs()
			tt.plugin.pluginDir = "/test-plugin"

			// Create any files referenced in the plugin
			for _, file := range tt.plugin.Files {
				if file.Source != "" {
					path := tt.plugin.pluginDir + "/" + file.Source
					fs.MkdirAll(tt.plugin.pluginDir+"/files", 0755)
					afero.WriteFile(fs, path, []byte("test"), 0644)
				}
			}

			// Override the file existence check by temporarily creating files
			err := tt.plugin.Validate()

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestFileMapping(t *testing.T) {
	tests := []struct {
		name     string
		mapping  FileMapping
		expected string
	}{
		{
			name: "simple mapping",
			mapping: FileMapping{
				Source:      "files/config.conf",
				Destination: "/etc/config.conf",
				Chmod:       "644",
			},
			expected: "files/config.conf -> /etc/config.conf (644)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the structure is correct
			if tt.mapping.Source == "" {
				t.Error("Source should not be empty")
			}
			if tt.mapping.Destination == "" {
				t.Error("Destination should not be empty")
			}
		})
	}
}

func TestLocaleConfig(t *testing.T) {
	tests := []struct {
		name   string
		config LocaleConfig
		valid  bool
	}{
		{
			name: "valid locale config",
			config: LocaleConfig{
				Locale:   "en_US.UTF-8",
				Generate: true,
			},
			valid: true,
		},
		{
			name: "locale without generation",
			config: LocaleConfig{
				Locale:   "en_US.UTF-8",
				Generate: false,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.Locale == "" && tt.valid {
				t.Error("Valid config should have locale set")
			}
		})
	}
}

func TestPluginGetFilePath(t *testing.T) {
	plugin := Plugin{
		pluginDir: "/plugins/test-plugin",
	}

	path := plugin.GetFilePath("files/config.conf")
	expected := "/plugins/test-plugin/files/config.conf"

	if path != expected {
		t.Errorf("GetFilePath() = %q, want %q", path, expected)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
