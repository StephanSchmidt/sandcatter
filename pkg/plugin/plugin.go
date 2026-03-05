package plugin

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StephanSchmidt/sandcatter/pkg/builtin"
)

// Plugin represents a sandcutter plugin configuration
type Plugin struct {
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	Description    string            `json:"description"`
	AptPackages    []string          `json:"apt_packages"`
	Fonts          []string          `json:"fonts"`
	LocalePackages []string          `json:"locale_packages"`
	Files          []FileMapping     `json:"files"`
	ComposeEnv     map[string]string `json:"compose_env"`
	ComposeCommand string            `json:"compose_command,omitempty"`
	LocaleSetup    *LocaleConfig     `json:"locale_setup"`

	// Internal fields
	pluginDir string
	isBuiltin bool
	fs        fs.FS // Filesystem for reading plugin files
}

// FileMapping represents a file to copy into the container
type FileMapping struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Chmod       string `json:"chmod"`
}

// LocaleConfig represents locale generation settings
type LocaleConfig struct {
	Locale   string `json:"locale"`
	Generate bool   `json:"generate"`
}

// Load reads and parses a plugin.json file from disk
func Load(pluginPath string) (*Plugin, error) {
	manifestPath := filepath.Join(pluginPath, "plugin.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin manifest: %w", err)
	}

	var plugin Plugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to parse plugin manifest: %w", err)
	}

	plugin.pluginDir = pluginPath
	plugin.isBuiltin = false
	plugin.fs = nil // Use OS filesystem

	if err := plugin.Validate(); err != nil {
		return nil, fmt.Errorf("invalid plugin: %w", err)
	}

	return &plugin, nil
}

// ReadFile reads a file from the plugin (embedded or disk)
func (p *Plugin) ReadFile(relativePath string) ([]byte, error) {
	fullPath := filepath.Join(p.pluginDir, relativePath)

	if p.isBuiltin && p.fs != nil {
		return fs.ReadFile(p.fs, fullPath)
	}

	// Read from disk
	return os.ReadFile(fullPath)
}

// IsBuiltin returns true if this is a built-in plugin
func (p *Plugin) IsBuiltin() bool {
	return p.isBuiltin
}

// Validate checks if the plugin configuration is valid
func (p *Plugin) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	// Validate file mappings
	for _, file := range p.Files {
		if file.Source == "" {
			return fmt.Errorf("file source is required")
		}
		if file.Destination == "" {
			return fmt.Errorf("file destination is required")
		}

		// Check if source file exists
		sourcePath := filepath.Join(p.pluginDir, file.Source)
		if _, err := os.Stat(sourcePath); err != nil {
			return fmt.Errorf("source file %s does not exist: %w", file.Source, err)
		}
	}

	return nil
}

// GetFilePath returns the absolute path to a plugin file
func (p *Plugin) GetFilePath(relativePath string) string {
	return filepath.Join(p.pluginDir, relativePath)
}

// DiscoverPlugins finds all plugins (built-in and user-provided)
func DiscoverPlugins(pluginsDir string) ([]*Plugin, error) {
	pluginMap := make(map[string]*Plugin)

	// Load built-in plugins first
	builtinFS, err := fs.Sub(builtin.Plugins, "plugins")
	if err != nil {
		return nil, fmt.Errorf("failed to access built-in plugins: %w", err)
	}

	err = loadPluginsFromFS(builtinFS, pluginMap, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error loading built-in plugins: %v\n", err)
	}

	// Load user plugins (can override built-in plugins)
	if pluginsDir != "" {
		err = loadPluginsFromDir(pluginsDir, pluginMap, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error loading user plugins: %v\n", err)
		}
	}

	// Convert map to slice
	plugins := make([]*Plugin, 0, len(pluginMap))
	for _, plugin := range pluginMap {
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// loadPluginsFromFS loads plugins from an embed.FS or other fs.FS
func loadPluginsFromFS(pluginFS fs.FS, pluginMap map[string]*Plugin, isBuiltin bool) error {
	entries, err := fs.ReadDir(pluginFS, ".")
	if err != nil {
		return fmt.Errorf("failed to read plugin filesystem: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()
		plugin, err := loadFromFS(pluginFS, pluginName, isBuiltin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping plugin %s: %v\n", pluginName, err)
			continue
		}

		pluginMap[plugin.Name] = plugin
	}

	return nil
}

// loadFromFS loads a single plugin from an fs.FS
func loadFromFS(pluginFS fs.FS, pluginName string, isBuiltin bool) (*Plugin, error) {
	manifestPath := filepath.Join(pluginName, "plugin.json")

	data, err := fs.ReadFile(pluginFS, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin manifest: %w", err)
	}

	var plugin Plugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to parse plugin manifest: %w", err)
	}

	plugin.pluginDir = pluginName
	plugin.isBuiltin = isBuiltin
	plugin.fs = pluginFS

	if err := plugin.validateFS(); err != nil {
		return nil, fmt.Errorf("invalid plugin: %w", err)
	}

	return &plugin, nil
}

// loadPluginsFromDir loads plugins from a directory on disk
func loadPluginsFromDir(pluginsDir string, pluginMap map[string]*Plugin, isBuiltin bool) error {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, that's OK
		}
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(pluginsDir, entry.Name())
		plugin, err := Load(pluginPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping plugin %s: %v\n", entry.Name(), err)
			continue
		}

		// User plugins override built-in plugins
		pluginMap[plugin.Name] = plugin
	}

	return nil
}

// validateFS validates the plugin using fs.FS
func (p *Plugin) validateFS() error {
	if p.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	// Validate file mappings
	for _, file := range p.Files {
		if file.Source == "" {
			return fmt.Errorf("file source is required")
		}
		if file.Destination == "" {
			return fmt.Errorf("file destination is required")
		}

		// Check if source file exists in the filesystem
		sourcePath := filepath.Join(p.pluginDir, file.Source)
		if _, err := fs.Stat(p.fs, sourcePath); err != nil {
			return fmt.Errorf("source file %s does not exist: %w", file.Source, err)
		}
	}

	return nil
}
