package dockerfile

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/afero"
)

const (
	SandcutterMarker = "# sandcutter:managed"
)

var (
	// AppFs is the filesystem abstraction, can be replaced with afero.NewMemMapFs() for testing
	AppFs = afero.NewOsFs()
)

// Dockerfile represents a parsed Dockerfile
type Dockerfile struct {
	Lines    []string
	FilePath string
	fs       afero.Fs
}

// Load reads a Dockerfile from disk
func Load(path string) (*Dockerfile, error) {
	return LoadFs(AppFs, path)
}

// LoadFs reads a Dockerfile from a filesystem
func LoadFs(fs afero.Fs, path string) (*Dockerfile, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Dockerfile: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	return &Dockerfile{
		Lines:    lines,
		FilePath: path,
		fs:       fs,
	}, nil
}

// Save writes the Dockerfile back to disk
func (d *Dockerfile) Save() error {
	return d.SaveAs(d.FilePath)
}

// SaveAs writes the Dockerfile to a specific path
func (d *Dockerfile) SaveAs(path string) error {
	content := strings.Join(d.Lines, "\n") + "\n"
	return afero.WriteFile(d.fs, path, []byte(content), 0644)
}

// Backup creates a backup of the Dockerfile
func (d *Dockerfile) Backup() error {
	backupPath := d.FilePath + ".backup"
	return d.SaveAs(backupPath)
}

// HasPackage checks if an apt package is already installed
func (d *Dockerfile) HasPackage(pkg string) bool {
	inAptInstall := false
	for _, line := range d.Lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "apt-get install") {
			inAptInstall = true
		}

		if inAptInstall {
			// Check if this line contains the package as a whole word
			words := strings.Fields(trimmed)
			for _, word := range words {
				// Remove trailing backslash
				word = strings.TrimSuffix(word, "\\")
				if word == pkg {
					return true
				}
			}

			// End of apt-get install command
			if !strings.HasSuffix(trimmed, "\\") && !strings.HasSuffix(trimmed, "&&") {
				inAptInstall = false
			}
		}
	}
	return false
}

// AddAptPackages adds packages to the existing apt-get install command
func (d *Dockerfile) AddAptPackages(packages []string, pluginName string) error {
	// Find the apt-get install line
	aptLineIdx := -1
	for i, line := range d.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "apt-get install") {
			aptLineIdx = i
			break
		}
	}

	if aptLineIdx == -1 {
		return fmt.Errorf("could not find apt-get install line in Dockerfile")
	}

	// Check if this plugin's packages are already added
	startMarker := fmt.Sprintf("# sandcutter:plugin:%s:start", pluginName)
	for _, line := range d.Lines {
		if strings.Contains(line, startMarker) {
			// Plugin packages already added
			return nil
		}
	}

	// Filter out packages that are already installed
	var newPackages []string
	for _, pkg := range packages {
		if !d.HasPackage(pkg) {
			newPackages = append(newPackages, pkg)
		}
	}

	if len(newPackages) == 0 {
		// All packages already installed
		return nil
	}

	// Find the package installation lines (between "apt-get install" and cleanup)
	// We want to insert before the cleanup line (e.g., "&& rm -rf /var/lib/apt/lists/*")
	insertIdx := -1

	for i := aptLineIdx; i < len(d.Lines); i++ {
		line := strings.TrimSpace(d.Lines[i])

		// Found the cleanup line - insert packages before this
		if strings.Contains(line, "rm -rf /var/lib/apt/lists") {
			insertIdx = i
			break
		}

		// If line doesn't end with \, this is the end of the RUN command
		if !strings.HasSuffix(line, "\\") {
			insertIdx = i
			break
		}
	}

	if insertIdx == -1 {
		return fmt.Errorf("could not find insertion point in apt-get install command")
	}

	// Determine the indent by looking at the line before insertIdx
	indent := "        " // default 8 spaces
	if insertIdx > 0 {
		prevLine := d.Lines[insertIdx-1]
		leadingSpaces := len(prevLine) - len(strings.TrimLeft(prevLine, " "))
		indent = strings.Repeat(" ", leadingSpaces)
	}

	// Check if the previous line ends with a backslash
	prevLine := d.Lines[insertIdx-1]
	prevTrimmed := strings.TrimSpace(prevLine)
	needsBackslash := !strings.HasSuffix(prevTrimmed, "\\")

	// Add backslash to previous line if needed
	if needsBackslash {
		d.Lines[insertIdx-1] = strings.TrimRight(d.Lines[insertIdx-1], " ") + " \\"
	}

	// Check if the next line continues the RUN command (e.g. && or another plugin block)
	nextLine := strings.TrimSpace(d.Lines[insertIdx])
	needsContinuation := strings.HasPrefix(nextLine, "&&") ||
		strings.HasPrefix(nextLine, "# sandcutter:plugin:")

	// Build the lines to insert with markers
	var linesToInsert []string

	// Add start marker (always needs continuation since packages follow)
	startMarkerLine := fmt.Sprintf("%s# sandcutter:plugin:%s:start \\", indent, pluginName)
	linesToInsert = append(linesToInsert, startMarkerLine)

	// Insert the new packages (all get backslash continuation)
	for _, pkg := range newPackages {
		linesToInsert = append(linesToInsert, fmt.Sprintf("%s%s \\", indent, pkg))
	}

	// Add end marker
	if needsContinuation {
		linesToInsert = append(linesToInsert, fmt.Sprintf("%s# sandcutter:plugin:%s:end \\", indent, pluginName))
	} else {
		linesToInsert = append(linesToInsert, fmt.Sprintf("%s# sandcutter:plugin:%s:end", indent, pluginName))
	}

	// Insert all lines at once
	d.Lines = append(d.Lines[:insertIdx], append(linesToInsert, d.Lines[insertIdx:]...)...)

	return nil
}

// AddLocaleSetup adds locale generation commands if not present
func (d *Dockerfile) AddLocaleSetup(locale string) error {
	// Check if locale setup already exists
	for _, line := range d.Lines {
		if strings.Contains(line, "locale-gen") {
			return nil // Already configured
		}
	}

	// Find where to insert (after apt-get install locales)
	insertIdx := -1
	for i, line := range d.Lines {
		if strings.Contains(line, "apt-get install") {
			// Find the end of the apt command
			for j := i; j < len(d.Lines); j++ {
				trimmed := strings.TrimSpace(d.Lines[j])
				if !strings.HasSuffix(trimmed, "\\") {
					insertIdx = j + 1
					break
				}
			}
			break
		}
	}

	if insertIdx == -1 {
		return fmt.Errorf("could not find insertion point for locale setup")
	}

	// Add locale configuration
	localeLines := []string{
		"",
		"# Configure locale for UTF-8 support (sandcutter:managed)",
		fmt.Sprintf("RUN sed -i '/%s/s/^# //g' /etc/locale.gen \\", strings.Replace(locale, ".", "\\.", -1)),
		"    && locale-gen",
		fmt.Sprintf("ENV LANG=%s LANGUAGE=%s LC_ALL=%s", locale, strings.Split(locale, ".")[0]+":en", locale),
	}

	// Insert at the found position
	d.Lines = append(d.Lines[:insertIdx], append(localeLines, d.Lines[insertIdx:]...)...)

	return nil
}

// AddRunCommands adds RUN commands before file COPY markers and ENTRYPOINT
func (d *Dockerfile) AddRunCommands(commands []string, pluginName string) error {
	startMarker := fmt.Sprintf("# sandcutter:run:%s:start", pluginName)
	endMarker := fmt.Sprintf("# sandcutter:run:%s:end", pluginName)

	// Check if already exists (idempotent)
	for _, line := range d.Lines {
		if strings.Contains(line, startMarker) {
			return nil
		}
	}

	// Find insertion point: before the first sandcutter:file marker before ENTRYPOINT,
	// or before ENTRYPOINT if no file markers exist
	insertIdx := len(d.Lines)
	for i, line := range d.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ENTRYPOINT") {
			insertIdx = i
			break
		}
	}

	// Scan backward from ENTRYPOINT to find the first sandcutter:file marker block
	for i := insertIdx - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(d.Lines[i])
		if strings.HasPrefix(trimmed, "# sandcutter:file:") {
			insertIdx = i
			continue
		}
		if strings.HasPrefix(trimmed, "COPY") || trimmed == "" {
			continue
		}
		break
	}

	// Build lines to insert
	var linesToInsert []string
	linesToInsert = append(linesToInsert, startMarker)
	for _, cmd := range commands {
		linesToInsert = append(linesToInsert, fmt.Sprintf("RUN %s", cmd))
	}
	linesToInsert = append(linesToInsert, endMarker)

	// Insert
	d.Lines = append(d.Lines[:insertIdx], append(linesToInsert, d.Lines[insertIdx:]...)...)

	return nil
}

// AddCopyCommand adds a COPY command for a file
func (d *Dockerfile) AddCopyCommand(source, destination, chmod string) error {
	marker := fmt.Sprintf("# sandcutter:file:%s", destination)

	// Check if already exists
	for _, line := range d.Lines {
		if strings.Contains(line, marker) {
			return nil // Already added
		}
	}

	// Find insertion point (before ENTRYPOINT or at the end)
	insertIdx := len(d.Lines)
	for i, line := range d.Lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ENTRYPOINT") {
			insertIdx = i
			break
		}
	}

	// Build COPY command
	var copyCmd string
	if chmod != "" {
		copyCmd = fmt.Sprintf("%s\nCOPY --chmod=%s %s %s", marker, chmod, source, destination)
	} else {
		copyCmd = fmt.Sprintf("%s\nCOPY %s %s", marker, source, destination)
	}

	// Insert the command
	d.Lines = append(d.Lines[:insertIdx], append([]string{copyCmd}, d.Lines[insertIdx:]...)...)

	return nil
}

// AddDockerEnv adds ENV instructions to the Dockerfile
func (d *Dockerfile) AddDockerEnv(envVars map[string]string, pluginName string) error {
	startMarker := fmt.Sprintf("# sandcutter:env:%s:start", pluginName)
	endMarker := fmt.Sprintf("# sandcutter:env:%s:end", pluginName)

	// Check if already exists (idempotent)
	for _, line := range d.Lines {
		if strings.Contains(line, startMarker) {
			return nil
		}
	}

	// Preferred insertion point: right after the run block end marker for this plugin
	runEndMarker := fmt.Sprintf("# sandcutter:run:%s:end", pluginName)
	insertIdx := -1
	for i, line := range d.Lines {
		if strings.TrimSpace(line) == runEndMarker {
			insertIdx = i + 1
			break
		}
	}

	// Fallback: same logic as AddRunCommands — before COPY markers / ENTRYPOINT
	if insertIdx == -1 {
		insertIdx = len(d.Lines)
		for i, line := range d.Lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "ENTRYPOINT") {
				insertIdx = i
				break
			}
		}

		// Scan backward from ENTRYPOINT to find sandcutter:file marker blocks
		for i := insertIdx - 1; i >= 0; i-- {
			trimmed := strings.TrimSpace(d.Lines[i])
			if strings.HasPrefix(trimmed, "# sandcutter:file:") {
				insertIdx = i
				continue
			}
			if strings.HasPrefix(trimmed, "COPY") || trimmed == "" {
				continue
			}
			break
		}
	}

	// Build lines to insert
	var linesToInsert []string
	linesToInsert = append(linesToInsert, startMarker)
	for key, value := range envVars {
		linesToInsert = append(linesToInsert, fmt.Sprintf("ENV %s=%s", key, value))
	}
	linesToInsert = append(linesToInsert, endMarker)

	// Insert
	d.Lines = append(d.Lines[:insertIdx], append(linesToInsert, d.Lines[insertIdx:]...)...)

	return nil
}

// InstalledPlugin describes a plugin detected in a Dockerfile
type InstalledPlugin struct {
	Name     string
	Packages bool
	Run      bool
	Env      bool
}

// ScanResult holds the results of scanning a Dockerfile for sandcutter markers
type ScanResult struct {
	Plugins []InstalledPlugin
	Files   []string
}

// ScanPlugins scans the Dockerfile for sandcutter markers and returns installed plugins
func (d *Dockerfile) ScanPlugins() ScanResult {
	plugins := make(map[string]*InstalledPlugin)
	var files []string

	getOrCreate := func(name string) *InstalledPlugin {
		if p, ok := plugins[name]; ok {
			return p
		}
		p := &InstalledPlugin{Name: name}
		plugins[name] = p
		return p
	}

	for _, line := range d.Lines {
		trimmed := strings.TrimSpace(line)
		// Strip trailing backslash for inline markers within apt blocks
		trimmed = strings.TrimSuffix(trimmed, " \\")

		if strings.HasPrefix(trimmed, "# sandcutter:plugin:") && strings.HasSuffix(trimmed, ":start") {
			name := trimmed[len("# sandcutter:plugin:") : len(trimmed)-len(":start")]
			getOrCreate(name).Packages = true
		} else if strings.HasPrefix(trimmed, "# sandcutter:run:") && strings.HasSuffix(trimmed, ":start") {
			name := trimmed[len("# sandcutter:run:") : len(trimmed)-len(":start")]
			getOrCreate(name).Run = true
		} else if strings.HasPrefix(trimmed, "# sandcutter:env:") && strings.HasSuffix(trimmed, ":start") {
			name := trimmed[len("# sandcutter:env:") : len(trimmed)-len(":start")]
			getOrCreate(name).Env = true
		} else if strings.HasPrefix(trimmed, "# sandcutter:file:") {
			path := trimmed[len("# sandcutter:file:"):]
			files = append(files, path)
		}
	}

	var result []InstalledPlugin
	for _, p := range plugins {
		result = append(result, *p)
	}

	return ScanResult{Plugins: result, Files: files}
}

// GetContent returns the Dockerfile content as a string
func (d *Dockerfile) GetContent() string {
	return strings.Join(d.Lines, "\n") + "\n"
}
