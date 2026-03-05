package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/StephanSchmidt/sandcatter/pkg/dockerfile"
)

// Applier handles applying plugins to a sandcat installation
type Applier struct {
	TargetDir      string
	DockerfilePath string
	ComposePath    string
	PluginsDir     string
}

// NewApplier creates a new plugin applier
func NewApplier(targetDir, pluginsDir string) (*Applier, error) {
	// Validate target directory
	dockerfilePath := filepath.Join(targetDir, ".devcontainer", "Dockerfile.app")
	composePath := filepath.Join(targetDir, ".devcontainer", "compose-all.yml")

	if _, err := os.Stat(dockerfilePath); err != nil {
		return nil, fmt.Errorf("target directory does not appear to be a sandcat installation (missing %s)", dockerfilePath)
	}

	if _, err := os.Stat(composePath); err != nil {
		return nil, fmt.Errorf("target directory does not appear to be a sandcat installation (missing %s)", composePath)
	}

	return &Applier{
		TargetDir:      targetDir,
		DockerfilePath: dockerfilePath,
		ComposePath:    composePath,
		PluginsDir:     pluginsDir,
	}, nil
}

// detectComposeCommand returns "docker compose" if the plugin is available,
// otherwise falls back to "docker-compose". Returns an error if neither is found.
func detectComposeCommand() (string, error) {
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		return "docker compose", nil
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return "docker-compose", nil
	}
	return "", fmt.Errorf("docker compose is not installed; install Docker Compose (https://docs.docker.com/compose/install/) and try again")
}

// Apply applies a plugin to the target sandcat installation
func (a *Applier) Apply(plugin *Plugin, dryRun bool) error {
	fmt.Printf("Applying plugin: %s v%s\n", plugin.Name, plugin.Version)
	fmt.Printf("  %s\n\n", plugin.Description)

	// Load Dockerfile
	df, err := dockerfile.Load(a.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to load Dockerfile: %w", err)
	}

	// Load compose file
	compose, err := dockerfile.LoadCompose(a.ComposePath)
	if err != nil {
		return fmt.Errorf("failed to load compose file: %w", err)
	}

	// Create backups
	if !dryRun {
		fmt.Println("Creating backups...")
		if err := df.Backup(); err != nil {
			return fmt.Errorf("failed to backup Dockerfile: %w", err)
		}
		if err := compose.Backup(); err != nil {
			return fmt.Errorf("failed to backup compose file: %w", err)
		}
	}

	// Apply locale setup FIRST (before adding packages)
	// This is important because locale setup inserts after the apt command,
	// and we want it to be separate from the package markers
	if plugin.LocaleSetup != nil && plugin.LocaleSetup.Generate {
		fmt.Printf("Setting up locale: %s\n", plugin.LocaleSetup.Locale)
		if err := df.AddLocaleSetup(plugin.LocaleSetup.Locale); err != nil {
			return fmt.Errorf("failed to add locale setup: %w", err)
		}
	}

	// Apply apt packages
	allPackages := append([]string{}, plugin.LocalePackages...)
	allPackages = append(allPackages, plugin.Fonts...)
	allPackages = append(allPackages, plugin.AptPackages...)

	if len(allPackages) > 0 {
		fmt.Printf("Adding apt packages: %v\n", allPackages)
		if err := df.AddAptPackages(allPackages, plugin.Name); err != nil {
			return fmt.Errorf("failed to add apt packages: %w", err)
		}
	}

	// Apply run commands
	if len(plugin.RunCommands) > 0 {
		fmt.Printf("Adding RUN commands: %d command(s)\n", len(plugin.RunCommands))
		if err := df.AddRunCommands(plugin.RunCommands, plugin.Name); err != nil {
			return fmt.Errorf("failed to add RUN commands: %w", err)
		}
	}

	// Apply Dockerfile ENV variables
	if len(plugin.DockerEnv) > 0 {
		fmt.Printf("Adding Dockerfile ENV: %v\n", plugin.DockerEnv)
		if err := df.AddDockerEnv(plugin.DockerEnv, plugin.Name); err != nil {
			return fmt.Errorf("failed to add Dockerfile ENV: %w", err)
		}
	}

	// Copy plugin files to target
	devcontainerDir := filepath.Join(a.TargetDir, ".devcontainer")
	for _, file := range plugin.Files {
		targetFileName := filepath.Base(file.Source)
		targetPath := filepath.Join(devcontainerDir, targetFileName)

		fmt.Printf("Copying file: %s -> %s\n", file.Source, targetFileName)

		if !dryRun {
			// Use plugin.ReadFile to support both embedded and disk files
			data, err := plugin.ReadFile(file.Source)
			if err != nil {
				return fmt.Errorf("failed to read source file %s: %w", file.Source, err)
			}

			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write target file %s: %w", targetPath, err)
			}
		}

		// Add COPY command to Dockerfile
		relativePath := targetFileName
		fmt.Printf("Adding COPY command to Dockerfile: %s -> %s\n", relativePath, file.Destination)
		if err := df.AddCopyCommand(relativePath, file.Destination, file.Chmod); err != nil {
			return fmt.Errorf("failed to add COPY command: %w", err)
		}
	}

	// Apply environment variables to compose file
	if len(plugin.ComposeEnv) > 0 {
		fmt.Printf("Adding environment variables: %v\n", plugin.ComposeEnv)
		if err := compose.AddEnvironmentVariables(plugin.ComposeEnv); err != nil {
			return fmt.Errorf("failed to add environment variables: %w", err)
		}
	}

	// Apply compose command
	if plugin.ComposeCommand != "" {
		fmt.Printf("Setting compose command: %s\n", plugin.ComposeCommand)
		if err := compose.SetComposeCommand(plugin.ComposeCommand); err != nil {
			return fmt.Errorf("failed to set compose command: %w", err)
		}
	}

	// Save changes
	if dryRun {
		fmt.Println("\n--- DRY RUN: Dockerfile.app changes ---")
		fmt.Println(df.GetContent())
		fmt.Println("\n--- DRY RUN: compose-all.yml changes ---")
		fmt.Println(compose.GetContent())
		fmt.Println("\n(No changes were actually written)")
	} else {
		fmt.Println("\nSaving changes...")
		if err := df.Save(); err != nil {
			return fmt.Errorf("failed to save Dockerfile: %w", err)
		}
		if err := compose.Save(); err != nil {
			return fmt.Errorf("failed to save compose file: %w", err)
		}

		fmt.Println("\n✓ Plugin applied successfully!")
		fmt.Println("\nBackups saved with .backup extension")
		composeCmd, err := detectComposeCommand()
		if err != nil {
			return err
		}
		fmt.Println("\nRun:")
		fmt.Printf("  cd %s && %s -f .devcontainer/compose-all.yml run --rm --build app\n", a.TargetDir, composeCmd)
	}

	return nil
}
