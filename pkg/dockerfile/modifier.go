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

	// Build the lines to insert with markers
	var linesToInsert []string

	// Add start marker
	startMarkerLine := fmt.Sprintf("%s# sandcutter:plugin:%s:start", indent, pluginName)
	linesToInsert = append(linesToInsert, startMarkerLine)

	// Insert the new packages
	for i, pkg := range newPackages {
		suffix := " \\"
		if i == len(newPackages)-1 {
			// Last package needs backslash before end marker
			suffix = " \\"
		}
		linesToInsert = append(linesToInsert, fmt.Sprintf("%s%s%s", indent, pkg, suffix))
	}

	// Add end marker
	endMarker := fmt.Sprintf("%s# sandcutter:plugin:%s:end", indent, pluginName)

	// Check if next line needs a backslash
	nextLine := strings.TrimSpace(d.Lines[insertIdx])
	if !strings.HasPrefix(nextLine, "&&") {
		// Remove backslash from end marker line
		if len(linesToInsert) > 1 {
			lastPkg := linesToInsert[len(linesToInsert)-1]
			linesToInsert[len(linesToInsert)-1] = strings.TrimSuffix(lastPkg, " \\")
		}
	}

	linesToInsert = append(linesToInsert, endMarker)

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

// GetContent returns the Dockerfile content as a string
func (d *Dockerfile) GetContent() string {
	return strings.Join(d.Lines, "\n") + "\n"
}
