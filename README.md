# Sandcatter

A Go tool for adding functionality plugins to devcontainer setups — supports both [Sandcat](https://github.com/VirtusLab/sandcat) and [Anthropic Claude Code](https://github.com/anthropics/claude-code) devcontainers.

## Quick Example

Add tmux with full color support and a custom config to your devcontainer in one command:

```bash
$ sandcatter apply . tmux

✓ Merged 3 apt packages into Dockerfile
✓ Added locale configuration
✓ Copied tmux.conf → .devcontainer/tmux.conf
✓ Added TERM=xterm-256color to compose environment
✓ Plugin 'tmux' applied successfully

# That's it — rebuild your container and tmux is ready to go.
```

No manual Dockerfile editing. No hunting for the right apt packages. No figuring out where to insert `COPY` lines. Sandcatter handles the plumbing — you just pick the plugins.

## What is Sandcatter?

Sandcatter allows you to enhance development containers by applying pre-configured plugins that modify the Dockerfile and (optionally) docker-compose configuration. Each plugin can install packages, configure fonts and locales, copy configuration files, and set environment variables.

### Supported Container Types

| | Sandcat | Anthropic Claude Code |
|---|---|---|
| Dockerfile | `.devcontainer/Dockerfile.app` | `.devcontainer/Dockerfile` |
| Compose file | `.devcontainer/compose-all.yml` | None (devcontainer.json only) |
| Auto-detected | ✓ | ✓ |

Sandcatter auto-detects which type of container it's working with — no configuration needed.

## Installation

```bash
# One-line install (Linux/macOS)
curl -sSfL https://raw.githubusercontent.com/StephanSchmidt/sandcatter/main/install.sh | bash

# Or build from source
go build -o sandcatter .
```

You can set a custom install directory with `INSTALL_DIR`:

```bash
curl -sSfL https://raw.githubusercontent.com/StephanSchmidt/sandcatter/main/install.sh | INSTALL_DIR=~/.local/bin bash
```

## Usage

### List Available Plugins

```bash
sandcatter list
```

### Scan Installed Plugins

```bash
sandcatter scan /path/to/devcontainer
```

### Apply Plugins

```bash
# Apply single plugin
sandcatter apply /path/to/devcontainer tmux

# Apply multiple plugins
sandcatter apply /path/to/devcontainer tmux neovim

# Dry run (preview changes without applying)
sandcatter apply /path/to/devcontainer tmux --dry-run
```

### After Applying Plugins

Rebuild your container to apply the changes. For sandcat installations:

```bash
cd /path/to/sandcat-install
docker compose -f .devcontainer/compose-all.yml build
```

For Anthropic devcontainers (or any devcontainer.json-based setup), rebuild via your IDE or the devcontainer CLI.

## Available Plugins

Sandcatter includes built-in plugins that are embedded in the binary, and supports user-created plugins.

### Built-in Plugins

| Plugin | Description |
|--------|-------------|
| **tmux** | Terminal multiplexer with 256-color/true color support, mouse integration, custom keybindings, and fonts. Config: `.devcontainer/tmux.conf` |
| **claude-tools** | Installs Homebrew and CLI tools (yq, ripgrep, mdq, fd, fzf) for Claude Code containers. Sets up PATH for linuxbrew. |
| **golsp** | Go language server ([gopls](https://pkg.go.dev/golang.org/x/tools/gopls)) for [Claude Code LSP support](https://www.amazingcto.com/lsp-in-claude/). Installs Go via GVM and sets up gopls. |

### User Plugins

You can create your own plugins in the `plugins/` directory. See [PLUGINS.md](PLUGINS.md) for details on creating custom plugins.

User plugins can override built-in plugins by using the same name.

## Creating Custom Plugins

See [PLUGINS.md](PLUGINS.md) for the full guide on creating custom plugins, including manifest fields, examples, best practices, and troubleshooting.

## How It Works

Sandcatter modifies your devcontainer setup by:

1. **Dockerfile modifications** (Dockerfile.app or Dockerfile):
   - Merges apt packages into existing `RUN apt-get install` commands
   - Adds locale generation commands
   - Adds `RUN`, `COPY`, and `ENV` commands for plugin configuration
   - Inserts commands before `ENTRYPOINT` (sandcat) or before the final `USER` line (Anthropic), keeping them in root context

2. **Compose file modifications** (when present):
   - Adds environment variables to the app service
   - Sets compose command if specified by the plugin

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
- **Validation:** Checks that target directory contains a recognized devcontainer Dockerfile
- **Dry run:** Preview changes before applying with `--dry-run`
- **Idempotency markers:** Prevents duplicate entries
- **Non-destructive merging:** Adds to existing configuration, doesn't replace

## Project Structure

```
sandcatter/
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

## License

MIT

## Contributing

Contributions welcome! Please submit issues and pull requests on GitHub.
