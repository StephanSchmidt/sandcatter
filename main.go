package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StephanSchmidt/sandcatter/pkg/plugin"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "list":
		if err := listPlugins(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "apply":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: target directory required")
			fmt.Fprintln(os.Stderr, "Usage: sandcutter apply <target-dir> [plugin-names...]")
			os.Exit(1)
		}
		targetDir := os.Args[2]
		pluginNames := os.Args[3:]

		dryRun := false
		for i, arg := range pluginNames {
			if arg == "--dry-run" {
				dryRun = true
				pluginNames = append(pluginNames[:i], pluginNames[i+1:]...)
				break
			}
		}

		if err := applyPlugins(targetDir, pluginNames, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Printf("sandcutter v%s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("sandcutter - Add functionality plugins to sandcat installations")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sandcutter list                           List available plugins")
	fmt.Println("  sandcutter apply <target> [plugins...]    Apply plugins to sandcat installation")
	fmt.Println("  sandcutter version                        Show version information")
	fmt.Println("  sandcutter help                           Show this help message")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --dry-run                                 Show changes without applying them")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sandcutter list")
	fmt.Println("  sandcutter apply ../my-sandcat tmux")
	fmt.Println("  sandcutter apply ../my-sandcat tmux neovim --dry-run")
}

func getPluginsDir() (string, error) {
	// Get the directory where the sandcutter binary is located
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	exeDir := filepath.Dir(exe)

	// Try plugins/ relative to executable
	pluginsDir := filepath.Join(exeDir, "plugins")
	if _, err := os.Stat(pluginsDir); err == nil {
		return pluginsDir, nil
	}

	// Try plugins/ in current directory (for development)
	pluginsDir, err = filepath.Abs("plugins")
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	if _, err := os.Stat(pluginsDir); err == nil {
		return pluginsDir, nil
	}

	return "", fmt.Errorf("plugins directory not found")
}

func listPlugins() error {
	// Try to find user plugins directory, but don't fail if it doesn't exist
	pluginsDir, _ := getPluginsDir()

	plugins, err := plugin.DiscoverPlugins(pluginsDir)
	if err != nil {
		return err
	}

	if len(plugins) == 0 {
		fmt.Println("No plugins found")
		return nil
	}

	fmt.Printf("Available plugins (%d):\n\n", len(plugins))

	for _, p := range plugins {
		pluginType := "user"
		if p.IsBuiltin() {
			pluginType = "built-in"
		}
		fmt.Printf("  %s (v%s) [%s]\n", p.Name, p.Version, pluginType)
		fmt.Printf("    %s\n", p.Description)

		if len(p.AptPackages) > 0 {
			fmt.Printf("    Packages: %s\n", strings.Join(p.AptPackages, ", "))
		}
		if len(p.Fonts) > 0 {
			fmt.Printf("    Fonts: %s\n", strings.Join(p.Fonts, ", "))
		}
		fmt.Println()
	}

	return nil
}

func applyPlugins(targetDir string, pluginNames []string, dryRun bool) error {
	pluginsDir, _ := getPluginsDir() // Don't fail if not found

	// If no plugin names specified, try to read from sandcutter.yaml
	if len(pluginNames) == 0 {
		return fmt.Errorf("no plugins specified (sandcutter.yaml support coming soon)")
	}

	// Create applier
	applier, err := plugin.NewApplier(targetDir, pluginsDir)
	if err != nil {
		return err
	}

	// Discover all plugins (built-in and user)
	allPlugins, err := plugin.DiscoverPlugins(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	// Create a map for quick lookup
	pluginMap := make(map[string]*plugin.Plugin)
	for _, p := range allPlugins {
		pluginMap[p.Name] = p
	}

	// Load and apply each requested plugin
	for _, pluginName := range pluginNames {
		p, found := pluginMap[pluginName]
		if !found {
			return fmt.Errorf("plugin %s not found", pluginName)
		}

		if err := applier.Apply(p, dryRun); err != nil {
			return fmt.Errorf("failed to apply plugin %s: %w", pluginName, err)
		}

		fmt.Println()
	}

	return nil
}
