package dockerfile

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/afero"
)

// ComposeFile represents a docker-compose.yml file
type ComposeFile struct {
	Lines    []string
	FilePath string
	fs       afero.Fs
}

// LoadCompose reads a docker-compose.yml file
func LoadCompose(path string) (*ComposeFile, error) {
	return LoadComposeFs(AppFs, path)
}

// LoadComposeFs reads a docker-compose.yml file from a filesystem
func LoadComposeFs(fs afero.Fs, path string) (*ComposeFile, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open compose file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	return &ComposeFile{
		Lines:    lines,
		FilePath: path,
		fs:       fs,
	}, nil
}

// Save writes the compose file back to disk
func (c *ComposeFile) Save() error {
	return c.SaveAs(c.FilePath)
}

// SaveAs writes the compose file to a specific path
func (c *ComposeFile) SaveAs(path string) error {
	content := strings.Join(c.Lines, "\n") + "\n"
	return afero.WriteFile(c.fs, path, []byte(content), 0644)
}

// Backup creates a backup of the compose file
func (c *ComposeFile) Backup() error {
	backupPath := c.FilePath + ".backup"
	return c.SaveAs(backupPath)
}

// AddEnvironmentVariables adds environment variables to the app service
func (c *ComposeFile) AddEnvironmentVariables(envVars map[string]string) error {
	if len(envVars) == 0 {
		return nil
	}

	// Find the app service and its environment section
	inAppService := false
	inEnvironment := false
	envSectionIdx := -1
	appServiceIdx := -1
	envIndent := ""

	for i, line := range c.Lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're in the app service
		if strings.HasPrefix(trimmed, "app:") {
			inAppService = true
			appServiceIdx = i
			continue
		}

		// Check if we've left the app service (new service or end of services)
		if inAppService && len(trimmed) > 0 && !strings.HasPrefix(line, " ") && strings.Contains(trimmed, ":") {
			inAppService = false
		}

		// Look for environment section within app service
		if inAppService && strings.HasPrefix(trimmed, "environment:") {
			inEnvironment = true
			envSectionIdx = i
			// Determine indent for env vars (should be more than "environment:")
			indent := len(line) - len(strings.TrimLeft(line, " "))
			envIndent = strings.Repeat(" ", indent+2)
			continue
		}

		// Check if we've left the environment section
		if inEnvironment && len(trimmed) > 0 {
			currentIndent := len(line) - len(strings.TrimLeft(line, " "))
			envBaseIndent := len(envIndent) - 2
			if currentIndent <= envBaseIndent {
				// We've moved to a new section
				break
			}
		}
	}

	if appServiceIdx == -1 {
		return fmt.Errorf("could not find app service in compose file")
	}

	// If no environment section exists, create one
	if envSectionIdx == -1 {
		// Find where to insert the environment section (after build: section or at start of app service)
		insertIdx := appServiceIdx + 1

		// Look for a good insertion point (after build: or after working_dir:)
		for i := appServiceIdx + 1; i < len(c.Lines); i++ {
			line := c.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Stop if we hit another top-level key in app service
			if len(trimmed) > 0 && !strings.HasPrefix(line, " ") && strings.Contains(trimmed, ":") {
				break
			}

			// If we find network_mode, volumes, command, etc., insert before them
			if strings.HasPrefix(trimmed, "network_mode:") ||
				strings.HasPrefix(trimmed, "volumes:") ||
				strings.HasPrefix(trimmed, "command:") ||
				strings.HasPrefix(trimmed, "depends_on:") {
				insertIdx = i
				break
			}

			// After build or working_dir, that's a good spot
			if strings.HasPrefix(trimmed, "dockerfile:") || strings.HasPrefix(trimmed, "working_dir:") {
				insertIdx = i + 1
			}
		}

		// Add environment section
		baseIndent := "    " // 4 spaces for app service keys
		envIndent = "      " // 6 spaces for env var entries

		newLines := []string{baseIndent + "environment:"}
		c.Lines = append(c.Lines[:insertIdx], append(newLines, c.Lines[insertIdx:]...)...)
		envSectionIdx = insertIdx
	}

	// Find where to insert (at the end of the environment section)
	insertIdx := envSectionIdx + 1
	for i := envSectionIdx + 1; i < len(c.Lines); i++ {
		line := c.Lines[i]
		trimmed := strings.TrimSpace(line)

		if len(trimmed) == 0 {
			break
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		envBaseIndent := len(envIndent) - 2
		if currentIndent <= envBaseIndent {
			break
		}

		insertIdx = i + 1
	}

	// Add new environment variables
	var newLines []string
	for key, value := range envVars {
		// Check if variable already exists
		exists := false
		for _, line := range c.Lines {
			if strings.Contains(line, fmt.Sprintf("%s:", key)) || strings.Contains(line, fmt.Sprintf("- %s=", key)) {
				exists = true
				break
			}
		}

		if !exists {
			newLines = append(newLines, fmt.Sprintf("%s- %s=%s", envIndent, key, value))
		}
	}

	if len(newLines) > 0 {
		// Insert new environment variables
		c.Lines = append(c.Lines[:insertIdx], append(newLines, c.Lines[insertIdx:]...)...)
	}

	return nil
}

// GetContent returns the compose file content as a string
func (c *ComposeFile) GetContent() string {
	return strings.Join(c.Lines, "\n") + "\n"
}
