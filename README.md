# Sandcutter

A Go tool for adding functionality plugins to existing [Sandcat](https://github.com/VirtusLab/sandcat) installations.

## What is Sandcutter?

Sandcutter allows you to enhance Sandcat development containers by applying pre-configured plugins that modify the Dockerfile and docker-compose configuration. Each plugin can install packages, configure fonts and locales, copy configuration files, and set environment variables.

## Installation

```bash
# Build from source
go build -o sandcutter .

# Or install globally
go install .
```

## Usage

### List Available Plugins

```bash
./sandcutter list
```

### Apply Plugins

```bash
# Apply single plugin
./sandcutter apply /path/to/sandcat-install tmux

# Apply multiple plugins
./sandcutter apply /path/to/sandcat-install tmux neovim

# Dry run (preview changes without applying)
./sandcutter apply /path/to/sandcat-install tmux --dry-run
```

### After Applying Plugins

Rebuild your container to apply the changes:

```bash
cd /path/to/sandcat-install
docker compose -f .devcontainer/compose-all.yml build
```

## Available Plugins

Sandcutter includes built-in plugins that are embedded in the binary, and supports user-created plugins.

### Built-in Plugins

#### tmux (built-in)

Terminal multiplexer with 256-color support, mouse integration, and custom keybindings.

**Features:**
- Installs tmux and required fonts (DejaVu, Liberation, Noto)
- Configures UTF-8 locale support
- Sets up 256-color and true color terminal support
- Custom keybindings for intuitive pane splitting
- Mouse support for scrolling and pane selection
- Status bar with custom styling
- 10,000 line scrollback buffer

**Configuration File:** `.devcontainer/tmux.conf`

### User Plugins

You can create your own plugins in the `plugins/` directory. See [PLUGINS.md](PLUGINS.md) for details on creating custom plugins.

User plugins can override built-in plugins by using the same name.

## Creating Custom Plugins

Plugins are stored in the `plugins/` directory. Each plugin has:

```
plugins/
└── myplugin/
    ├── plugin.json       # Plugin manifest
    └── files/            # Files to copy into container
        └── config.conf
```

### Plugin Manifest (plugin.json)

```json
{
  "name": "myplugin",
  "version": "1.0.0",
  "description": "My custom plugin",
  "apt_packages": ["package1", "package2"],
  "fonts": ["fonts-dejavu"],
  "locale_packages": ["locales"],
  "files": [
    {
      "source": "files/config.conf",
      "destination": "/etc/skel/.config.conf",
      "chmod": "644"
    }
  ],
  "compose_env": {
    "MY_VAR": "value"
  },
  "locale_setup": {
    "locale": "en_US.UTF-8",
    "generate": true
  }
}
```

### Plugin Fields

- **name**: Plugin identifier (required)
- **version**: Semantic version (required)
- **description**: Human-readable description (required)
- **apt_packages**: Debian packages to install
- **fonts**: Font packages to install
- **locale_packages**: Locale packages (usually `["locales"]`)
- **files**: Files to copy from plugin to container
  - **source**: Path relative to plugin directory
  - **destination**: Absolute path in container
  - **chmod**: File permissions (e.g., "644", "755")
- **compose_env**: Environment variables to add to docker-compose.yml
- **locale_setup**: Locale configuration
  - **locale**: Locale name (e.g., "en_US.UTF-8")
  - **generate**: Whether to run locale-gen

## How It Works

Sandcutter modifies your sandcat installation by:

1. **Dockerfile.app modifications:**
   - Merges apt packages into existing `RUN apt-get install` commands
   - Adds locale generation commands
   - Adds `COPY` commands for configuration files

2. **compose-all.yml modifications:**
   - Adds environment variables to the app service

3. **File copying:**
   - Copies plugin files to `.devcontainer/` directory
   - These files are then copied into the container via Dockerfile

4. **Idempotency:**
   - Uses markers to track what's been added
   - Running the same plugin twice won't duplicate entries

5. **Backups:**
   - Creates `.backup` files before making changes
   - You can restore from backups if needed

## Safety Features

- **Backups:** Original files are saved with `.backup` extension
- **Validation:** Checks that target directory is a valid sandcat installation
- **Dry run:** Preview changes before applying with `--dry-run`
- **Idempotency markers:** Prevents duplicate entries
- **Non-destructive merging:** Adds to existing configuration, doesn't replace

## Project Structure

```
sandcutter/
├── main.go                           # CLI entry point
├── go.mod                            # Go module definition
├── pkg/
│   ├── builtin/                      # Built-in plugins (embedded)
│   │   ├── embed.go                 # Embed directive
│   │   └── plugins/
│   │       └── tmux/                # Built-in tmux plugin
│   │           ├── plugin.json
│   │           └── files/
│   │               └── tmux.conf
│   ├── plugin/
│   │   ├── plugin.go                # Plugin loading and parsing
│   │   └── applier.go               # Plugin application logic
│   └── dockerfile/
│       ├── modifier.go              # Dockerfile modification
│       └── compose.go               # Compose file modification
└── plugins/                          # User plugins directory
    └── .gitkeep                     # Keeps directory tracked
```

## Development

### Running Tests

```bash
# Run all unit tests
make test

# Run tests with coverage
make test-coverage

# Run integration tests
make integration-test

# Run all checks (fmt, vet, test)
make check
```

### Building

```bash
# Build the binary
make build

# Or use go directly
go build -o sandcutter .
```

### Available Make Targets

Run `make help` to see all available targets:

- `make build` - Build the binary
- `make test` - Run all unit tests
- `make test-coverage` - Generate coverage report
- `make integration-test` - Run end-to-end tests with fresh sandcat
- `make clean` - Clean build artifacts
- `make install` - Install to $GOPATH/bin
- `make check` - Run fmt, vet, and tests
- `make fmt` - Format code
- `make vet` - Run go vet

### Adding a New Plugin

1. Create plugin directory: `mkdir -p plugins/myplugin/files`
2. Create `plugin.json` manifest
3. Add configuration files to `files/` directory
4. Test with: `./sandcutter apply ../test-sandcat myplugin --dry-run`

## Reference Implementation

The built-in tmux plugin serves as the reference implementation for how plugins should work.

## License

MIT

## Contributing

Contributions welcome! Please submit issues and pull requests on GitHub.
