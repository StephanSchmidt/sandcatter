package plugin

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/StephanSchmidt/sandcatter/pkg/dockerfile"
)

// Applier handles applying plugins to a devcontainer setup
type Applier struct {
	TargetDir      string
	DockerfilePath string
	ComposePath    string
	PluginsDir     string
}

// NewApplier creates a new plugin applier
func NewApplier(targetDir, pluginsDir string) (*Applier, error) {
	devcontainerDir := filepath.Join(targetDir, ".devcontainer")

	// Auto-detect Dockerfile
	dockerfilePath := ""
	for _, name := range []string{"Dockerfile.app", "Dockerfile"} {
		candidate := filepath.Join(devcontainerDir, name)
		if _, err := os.Stat(candidate); err == nil {
			dockerfilePath = candidate
			break
		}
	}
	if dockerfilePath == "" {
		return nil, fmt.Errorf("no Dockerfile found in %s (tried Dockerfile.app, Dockerfile)", devcontainerDir)
	}

	// Auto-detect compose file (optional)
	composePath := ""
	for _, name := range []string{"compose-all.yml", "docker-compose.yml", "compose.yml"} {
		candidate := filepath.Join(devcontainerDir, name)
		if _, err := os.Stat(candidate); err == nil {
			composePath = candidate
			break
		}
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

// Backup creates backups of the Dockerfile and compose file
func (a *Applier) Backup() error {
	df, err := dockerfile.Load(a.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to load Dockerfile: %w", err)
	}
	if err := df.Backup(); err != nil {
		return fmt.Errorf("failed to backup Dockerfile: %w", err)
	}

	if a.ComposePath != "" {
		compose, err := dockerfile.LoadCompose(a.ComposePath)
		if err != nil {
			return fmt.Errorf("failed to load compose file: %w", err)
		}
		if err := compose.Backup(); err != nil {
			return fmt.Errorf("failed to backup compose file: %w", err)
		}
	}

	return nil
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

	// Load compose file (optional)
	var compose *dockerfile.ComposeFile
	if a.ComposePath != "" {
		compose, err = dockerfile.LoadCompose(a.ComposePath)
		if err != nil {
			return fmt.Errorf("failed to load compose file: %w", err)
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
		if plugin.RunAs == "user" {
			if err := df.AddUserRunCommands(plugin.RunCommands, plugin.Name); err != nil {
				return fmt.Errorf("failed to add user RUN commands: %w", err)
			}
		} else {
			if err := df.AddRunCommands(plugin.RunCommands, plugin.Name); err != nil {
				return fmt.Errorf("failed to add RUN commands: %w", err)
			}
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
	if compose != nil && len(plugin.ComposeEnv) > 0 {
		fmt.Printf("Adding environment variables: %v\n", plugin.ComposeEnv)
		if err := compose.AddEnvironmentVariables(plugin.ComposeEnv); err != nil {
			return fmt.Errorf("failed to add environment variables: %w", err)
		}
	}

	// Apply compose volumes
	if compose != nil && len(plugin.ComposeVolumes) > 0 {
		fmt.Printf("Adding volumes: %v\n", plugin.ComposeVolumes)
		if err := compose.AddVolumes(plugin.ComposeVolumes); err != nil {
			return fmt.Errorf("failed to add volumes: %w", err)
		}
	}

	// Rewrite Claude plugin config files for container paths
	if hasClaudeVolumes(plugin) && !dryRun {
		hostHome, _ := os.UserHomeDir()
		containerHome := "/home/vscode"
		if err := RewriteClaudeConfig(hostHome, containerHome, devcontainerDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to rewrite Claude config: %v\n", err)
		}
	}

	// Apply compose command
	if compose != nil && plugin.ComposeCommand != "" {
		fmt.Printf("Setting compose command: %s\n", plugin.ComposeCommand)
		if err := compose.SetComposeCommand(plugin.ComposeCommand); err != nil {
			return fmt.Errorf("failed to set compose command: %w", err)
		}
	}

	// Save changes
	if dryRun {
		dockerfileName := filepath.Base(a.DockerfilePath)
		fmt.Printf("\n--- DRY RUN: %s changes ---\n", dockerfileName)
		fmt.Println(df.GetContent())
		if compose != nil {
			composeName := filepath.Base(a.ComposePath)
			fmt.Printf("\n--- DRY RUN: %s changes ---\n", composeName)
			fmt.Println(compose.GetContent())
		}
		fmt.Println("\n(No changes were actually written)")
	} else {
		fmt.Println("\nSaving changes...")
		if err := df.Save(); err != nil {
			return fmt.Errorf("failed to save Dockerfile: %w", err)
		}
		if compose != nil {
			if err := compose.Save(); err != nil {
				return fmt.Errorf("failed to save compose file: %w", err)
			}
		}

		fmt.Println("\n✓ Plugin applied successfully!")
		if plugin.PostInstallMessage != "" {
			fmt.Println("\nPost-install instructions:")
			fmt.Println(plugin.PostInstallMessage)
		}
		fmt.Println("\nBackups saved with .backup extension")
		if a.ComposePath != "" {
			composeCmd, err := detectComposeCommand()
			if err != nil {
				return err
			}
			composeName := filepath.Base(a.ComposePath)
			fmt.Println("\nRun:")
			if a.TargetDir != "." {
				fmt.Printf("  cd %s && %s -f .devcontainer/%s run --rm --build app\n", a.TargetDir, composeCmd, composeName)
			} else {
				fmt.Printf("  %s -f .devcontainer/%s run --rm --build app\n", composeCmd, composeName)
			}

			if compose != nil && hasHomeVolume(compose) {
				a.promptDeleteHomeVolume(composeCmd, composeName)
			}
		}
	}

	return nil
}

// Remove removes a plugin from the target devcontainer setup
func (a *Applier) Remove(plugin *Plugin, dryRun bool) error {
	fmt.Printf("Removing plugin: %s v%s\n", plugin.Name, plugin.Version)

	// Load Dockerfile
	df, err := dockerfile.Load(a.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to load Dockerfile: %w", err)
	}

	// Load compose file (optional)
	var compose *dockerfile.ComposeFile
	if a.ComposePath != "" {
		compose, err = dockerfile.LoadCompose(a.ComposePath)
		if err != nil {
			return fmt.Errorf("failed to load compose file: %w", err)
		}
	}

	// Remove plugin packages
	df.RemovePluginPackages(plugin.Name)
	fmt.Println("Removed plugin packages")

	// Remove run commands
	df.RemoveRunCommands(plugin.Name)
	fmt.Println("Removed RUN commands")

	// Remove Dockerfile ENV
	df.RemoveDockerEnv(plugin.Name)
	fmt.Println("Removed Dockerfile ENV")

	// Remove file COPY commands and physical files
	devcontainerDir := filepath.Join(a.TargetDir, ".devcontainer")
	for _, file := range plugin.Files {
		df.RemoveCopyCommand(file.Destination)
		fmt.Printf("Removed COPY command for %s\n", file.Destination)

		if !dryRun {
			targetFileName := filepath.Base(file.Source)
			targetPath := filepath.Join(devcontainerDir, targetFileName)
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: could not remove file %s: %v\n", targetPath, err)
			} else if err == nil {
				fmt.Printf("Deleted file: %s\n", targetPath)
			}
		}
	}

	// Remove compose environment variables
	if compose != nil && len(plugin.ComposeEnv) > 0 {
		if err := compose.RemoveEnvironmentVariables(plugin.ComposeEnv); err != nil {
			return fmt.Errorf("failed to remove environment variables: %w", err)
		}
		fmt.Printf("Removed compose environment variables: %s\n", strings.Join(mapKeys(plugin.ComposeEnv), ", "))
	}

	// Remove compose volumes
	if compose != nil && len(plugin.ComposeVolumes) > 0 {
		if err := compose.RemoveVolumes(plugin.ComposeVolumes); err != nil {
			return fmt.Errorf("failed to remove volumes: %w", err)
		}
		fmt.Printf("Removed compose volumes: %v\n", plugin.ComposeVolumes)
	}

	// Remove rewritten Claude config files
	if hasClaudeVolumes(plugin) && !dryRun {
		devcontainerDir := filepath.Join(a.TargetDir, ".devcontainer")
		for _, name := range []string{"installed_plugins.json", "known_marketplaces.json", "settings.json"} {
			p := filepath.Join(devcontainerDir, name)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: could not remove %s: %v\n", p, err)
			} else if err == nil {
				fmt.Printf("Deleted file: %s\n", p)
			}
		}
	}

	// Remove compose command
	if compose != nil && plugin.ComposeCommand != "" {
		if err := compose.RemoveComposeCommand(); err != nil {
			return fmt.Errorf("failed to remove compose command: %w", err)
		}
		fmt.Println("Removed compose command")
	}

	// Save changes
	if dryRun {
		dockerfileName := filepath.Base(a.DockerfilePath)
		fmt.Printf("\n--- DRY RUN: %s changes ---\n", dockerfileName)
		fmt.Println(df.GetContent())
		if compose != nil {
			composeName := filepath.Base(a.ComposePath)
			fmt.Printf("\n--- DRY RUN: %s changes ---\n", composeName)
			fmt.Println(compose.GetContent())
		}
		fmt.Println("\n(No changes were actually written)")
	} else {
		fmt.Println("\nSaving changes...")
		if err := df.Save(); err != nil {
			return fmt.Errorf("failed to save Dockerfile: %w", err)
		}
		if compose != nil {
			if err := compose.Save(); err != nil {
				return fmt.Errorf("failed to save compose file: %w", err)
			}
		}
		fmt.Println("\n✓ Plugin removed successfully!")
	}

	return nil
}

// hasHomeVolume checks whether the compose file mounts a named volume on /home/vscode.
func hasHomeVolume(compose *dockerfile.ComposeFile) bool {
	for _, line := range strings.Split(compose.GetContent(), "\n") {
		trimmed := strings.TrimSpace(line)
		// Match lines like "- app-home:/home/vscode"
		if strings.HasPrefix(trimmed, "- ") {
			entry := strings.TrimPrefix(trimmed, "- ")
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) == 2 {
				src := parts[0]
				dst := strings.Split(parts[1], ":")[0] // strip :ro etc.
				if dst == "/home/vscode" && !strings.HasPrefix(src, "/") && !strings.HasPrefix(src, ".") && !strings.HasPrefix(src, "~") && !strings.HasPrefix(src, "$") {
					return true
				}
			}
		}
	}
	return false
}

// promptDeleteHomeVolume asks the user whether to remove the home volume
// so that a rebuilt image can populate it with newly installed tools.
func (a *Applier) promptDeleteHomeVolume(composeCmd, composeName string) {
	fmt.Println("\nThe compose file uses a persistent home volume (app-home).")
	fmt.Println("Docker keeps old volume contents across rebuilds, so new tools")
	fmt.Println("installed by this plugin won't be visible until the volume is deleted.")
	fmt.Printf("\nDelete it now?  %s -f .devcontainer/%s down -v  [y/N] ", composeCmd, composeName)

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "y" || answer == "yes" {
		args := strings.Fields(composeCmd)
		args = append(args, "-f", filepath.Join(".devcontainer", composeName), "down", "-v")

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = a.TargetDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove volumes: %v\n", err)
		} else {
			fmt.Println("Volumes removed.")
		}
	}
}

// hasClaudeVolumes returns true if any compose volume mounts into a .claude path.
func hasClaudeVolumes(plugin *Plugin) bool {
	for _, v := range plugin.ComposeVolumes {
		if strings.Contains(v, ".claude") {
			return true
		}
	}
	return false
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
