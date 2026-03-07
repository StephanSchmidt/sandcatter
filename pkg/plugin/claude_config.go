package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// installedPluginsFile represents the structure of installed_plugins.json
type installedPluginsFile struct {
	Version int                                  `json:"version"`
	Plugins map[string][]installedPluginEntry    `json:"plugins"`
}

type installedPluginEntry struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	InstalledAt  string `json:"installedAt"`
	LastUpdated  string `json:"lastUpdated"`
	GitCommitSha string `json:"gitCommitSha"`
}

// knownMarketplacesFile represents the structure of known_marketplaces.json
type knownMarketplaceEntry struct {
	Source          marketplaceSource `json:"source"`
	InstallLocation string           `json:"installLocation"`
	LastUpdated     string           `json:"lastUpdated"`
}

type marketplaceSource struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
}

// RewriteClaudeConfig copies Claude plugin config files into devcontainerDir,
// rewriting host home paths to container home paths in the JSON fields that
// contain absolute paths.
func RewriteClaudeConfig(hostHome, containerHome, devcontainerDir string) error {
	pluginsDir := filepath.Join(hostHome, ".claude", "plugins")

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return fmt.Errorf("failed to create devcontainer dir: %w", err)
	}

	// Rewrite installed_plugins.json
	if err := rewriteInstalledPlugins(
		filepath.Join(pluginsDir, "installed_plugins.json"),
		filepath.Join(devcontainerDir, "installed_plugins.json"),
		hostHome, containerHome,
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Rewrite known_marketplaces.json
	if err := rewriteKnownMarketplaces(
		filepath.Join(pluginsDir, "known_marketplaces.json"),
		filepath.Join(devcontainerDir, "known_marketplaces.json"),
		hostHome, containerHome,
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Copy settings.json as-is
	if err := copyFile(
		filepath.Join(hostHome, ".claude", "settings.json"),
		filepath.Join(devcontainerDir, "settings.json"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	return nil
}

func rewriteInstalledPlugins(srcPath, dstPath, hostHome, containerHome string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", srcPath, err)
	}

	var file installedPluginsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to parse %s: %w", srcPath, err)
	}

	for name, entries := range file.Plugins {
		for i, entry := range entries {
			file.Plugins[name][i].InstallPath = rewritePath(entry.InstallPath, hostHome, containerHome)
		}
	}

	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal installed_plugins.json: %w", err)
	}

	return os.WriteFile(dstPath, append(out, '\n'), 0644)
}

func rewriteKnownMarketplaces(srcPath, dstPath, hostHome, containerHome string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", srcPath, err)
	}

	var file map[string]knownMarketplaceEntry
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to parse %s: %w", srcPath, err)
	}

	for name, entry := range file {
		entry.InstallLocation = rewritePath(entry.InstallLocation, hostHome, containerHome)
		file[name] = entry
	}

	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal known_marketplaces.json: %w", err)
	}

	return os.WriteFile(dstPath, append(out, '\n'), 0644)
}

func copyFile(srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", srcPath, err)
	}
	return os.WriteFile(dstPath, data, 0644)
}

// rewritePath replaces the hostHome prefix with containerHome in a path.
func rewritePath(path, hostHome, containerHome string) string {
	if len(path) >= len(hostHome) && path[:len(hostHome)] == hostHome {
		return containerHome + path[len(hostHome):]
	}
	return path
}
