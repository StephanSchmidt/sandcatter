package dockerfile

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestAddEnvironmentVariablesNewSection(t *testing.T) {
	input := `name: test-project

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile.app
    network_mode: "service:wg-client"
    volumes:
      - ..:/workspaces/test:cached
    command: sleep infinity`

	fs := afero.NewMemMapFs()
	path := "/compose.yml"
	afero.WriteFile(fs, path, []byte(input), 0644)

	cf, err := LoadComposeFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load compose file: %v", err)
	}

	envVars := map[string]string{
		"TERM":   "xterm-256color",
		"EDITOR": "vim",
	}

	err = cf.AddEnvironmentVariables(envVars)
	if err != nil {
		t.Fatalf("AddEnvironmentVariables failed: %v", err)
	}

	got := cf.GetContent()

	// Check that environment section was created
	if !strings.Contains(got, "environment:") {
		t.Error("Expected environment section to be created")
	}

	// Check that variables were added
	if !strings.Contains(got, "TERM=xterm-256color") {
		t.Error("Expected TERM variable to be added")
	}
	if !strings.Contains(got, "EDITOR=vim") {
		t.Error("Expected EDITOR variable to be added")
	}

	// Check that environment section is before network_mode
	envIdx := strings.Index(got, "environment:")
	networkIdx := strings.Index(got, "network_mode:")
	if envIdx > networkIdx {
		t.Error("environment section should be before network_mode")
	}
}

func TestAddEnvironmentVariablesExistingSection(t *testing.T) {
	input := `name: test-project

services:
  app:
    build:
      context: .
    environment:
      - EXISTING_VAR=value
    network_mode: "service:wg-client"`

	fs := afero.NewMemMapFs()
	path := "/compose.yml"
	afero.WriteFile(fs, path, []byte(input), 0644)

	cf, err := LoadComposeFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load compose file: %v", err)
	}

	envVars := map[string]string{
		"TERM": "xterm-256color",
	}

	err = cf.AddEnvironmentVariables(envVars)
	if err != nil {
		t.Fatalf("AddEnvironmentVariables failed: %v", err)
	}

	got := cf.GetContent()

	// Check that new variable was added
	if !strings.Contains(got, "TERM=xterm-256color") {
		t.Error("Expected TERM variable to be added")
	}

	// Check that existing variable is still there
	if !strings.Contains(got, "EXISTING_VAR=value") {
		t.Error("Expected existing variable to be preserved")
	}

	// Count environment section occurrences (should be exactly 1)
	count := strings.Count(got, "environment:")
	if count != 1 {
		t.Errorf("Expected exactly 1 environment section, got %d", count)
	}
}

func TestAddEnvironmentVariablesIdempotency(t *testing.T) {
	input := `name: test-project

services:
  app:
    build:
      context: .
    environment:
      - TERM=xterm-256color
    network_mode: "service:wg-client"`

	fs := afero.NewMemMapFs()
	path := "/compose.yml"
	afero.WriteFile(fs, path, []byte(input), 0644)

	cf, err := LoadComposeFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load compose file: %v", err)
	}

	before := cf.GetContent()

	envVars := map[string]string{
		"TERM": "xterm-256color",
	}

	err = cf.AddEnvironmentVariables(envVars)
	if err != nil {
		t.Fatalf("AddEnvironmentVariables failed: %v", err)
	}

	after := cf.GetContent()

	if before != after {
		t.Error("AddEnvironmentVariables should be idempotent (no changes when variable already exists)")
	}
}

func TestAddEnvironmentVariablesNoAppService(t *testing.T) {
	input := `name: test-project

services:
  worker:
    build:
      context: .`

	fs := afero.NewMemMapFs()
	path := "/compose.yml"
	afero.WriteFile(fs, path, []byte(input), 0644)

	cf, err := LoadComposeFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load compose file: %v", err)
	}

	envVars := map[string]string{
		"TERM": "xterm-256color",
	}

	err = cf.AddEnvironmentVariables(envVars)
	if err == nil {
		t.Fatal("Expected error when app service is not found")
	}

	if !strings.Contains(err.Error(), "app service") {
		t.Errorf("Expected error about missing app service, got: %v", err)
	}
}

func TestAddEnvironmentVariablesEmpty(t *testing.T) {
	input := `name: test-project

services:
  app:
    build:
      context: .`

	fs := afero.NewMemMapFs()
	path := "/compose.yml"
	afero.WriteFile(fs, path, []byte(input), 0644)

	cf, err := LoadComposeFs(fs, path)
	if err != nil {
		t.Fatalf("Failed to load compose file: %v", err)
	}

	before := cf.GetContent()

	// Empty map should not change anything
	err = cf.AddEnvironmentVariables(map[string]string{})
	if err != nil {
		t.Fatalf("AddEnvironmentVariables failed: %v", err)
	}

	after := cf.GetContent()

	if before != after {
		t.Error("AddEnvironmentVariables with empty map should not change the file")
	}
}
