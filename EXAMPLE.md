# Sandcutter Example Usage

This document demonstrates how to use sandcutter to add tmux support to a fresh sandcat installation.

## Prerequisites

- A working sandcat installation (see [VirtusLab/sandcat](https://github.com/VirtusLab/sandcat))
- Go 1.21+ installed

## Step 1: Build Sandcutter

```bash
cd /path/to/sandcutter
go build -o sandcutter .
```

## Step 2: List Available Plugins

```bash
./sandcutter list
```

Output:
```
Available plugins (1):

  tmux (v1.0.0)
    Terminal multiplexer with 256-color support, mouse integration, and custom keybindings
    Packages: tmux
    Fonts: fonts-dejavu, fonts-liberation, fonts-noto-core, fonts-noto-ui-core, fontconfig
```

## Step 3: Preview Changes (Dry Run)

Before applying changes, it's a good idea to preview what will be modified:

```bash
./sandcutter apply /path/to/your/sandcat-install tmux --dry-run
```

This will show you:
- Which packages will be added to Dockerfile.app
- Which files will be copied
- Changes to compose-all.yml
- The complete modified Dockerfile and compose file contents

## Step 4: Apply the Plugin

Once you're satisfied with the preview:

```bash
./sandcutter apply /path/to/your/sandcat-install tmux
```

Output:
```
Applying plugin: tmux v1.0.0
  Terminal multiplexer with 256-color support, mouse integration, and custom keybindings

Creating backups...
Adding apt packages: [locales fonts-dejavu fonts-liberation fonts-noto-core fonts-noto-ui-core fontconfig tmux]
Setting up locale: en_US.UTF-8
Copying file: files/tmux.conf -> tmux.conf
Adding COPY command to Dockerfile: tmux.conf -> /etc/skel/.tmux.conf
Adding environment variables: map[TERM:xterm-256color]

Saving changes...

✓ Plugin applied successfully!

Next steps:
  1. Review the changes in /path/to/your/sandcat-install/.devcontainer/Dockerfile.app
  2. Review the changes in /path/to/your/sandcat-install/.devcontainer/compose-all.yml
  3. Rebuild your container:
     cd /path/to/your/sandcat-install
     docker compose -f .devcontainer/compose-all.yml build

Backups saved with .backup extension
```

## Step 5: Review Changes

Check what was modified:

```bash
cd /path/to/your/sandcat-install/.devcontainer

# View Dockerfile changes
diff Dockerfile.app.backup Dockerfile.app

# View compose file changes
diff compose-all.yml.backup compose-all.yml
```

## Step 6: Rebuild Container

Build the container with the new configuration:

```bash
cd /path/to/your/sandcat-install
docker compose -f .devcontainer/compose-all.yml build
```

## Step 7: Start Your Container

```bash
docker compose -f .devcontainer/compose-all.yml up -d
```

Or if you have a launch script:

```bash
./claudecat.sh  # or whatever your entry script is named
```

## What Gets Added?

### Dockerfile.app Modifications

1. **APT Packages:** Adds tmux and font packages to the existing `apt-get install` command
2. **Locale Setup:** Generates en_US.UTF-8 locale and sets environment variables
3. **Configuration File:** Copies tmux.conf to /etc/skel/.tmux.conf

### compose-all.yml Modifications

1. **Environment Variable:** Adds `TERM=xterm-256color` to the app service

### Files Copied

1. **tmux.conf:** Placed in `.devcontainer/tmux.conf` (then copied into container)

## Verifying Installation

Once your container is running, you can verify tmux is working:

```bash
# Inside the container
tmux

# Check terminal colors
echo $TERM
# Should output: xterm-256color

# Check tmux terminal
tmux info | grep default-terminal
# Should show: tmux-256color

# Test custom keybindings
# Press Ctrl+B then | to split vertically
# Press Ctrl+B then - to split horizontally
```

## Idempotency

Sandcutter is idempotent - you can run the same plugin multiple times:

```bash
./sandcutter apply /path/to/sandcat tmux
./sandcutter apply /path/to/sandcat tmux  # Won't duplicate entries
```

The tool uses markers and checks to ensure packages and configurations aren't duplicated.

## Restoring from Backup

If you need to undo changes:

```bash
cd /path/to/your/sandcat-install/.devcontainer

# Restore original files
mv Dockerfile.app.backup Dockerfile.app
mv compose-all.yml.backup compose-all.yml

# Remove copied tmux.conf
rm tmux.conf

# Rebuild
docker compose -f compose-all.yml build
```

## Multiple Plugins

You can apply multiple plugins in one command:

```bash
./sandcutter apply /path/to/sandcat tmux neovim zsh-config
```

Each plugin will be applied in sequence.

## What's in the tmux Plugin?

The built-in tmux plugin includes:

- **256-color and true color support**
- **Mouse support** for scrolling and pane selection
- **10,000 line scrollback buffer**
- **Custom status bar** with date/time
- **Intuitive keybindings:**
  - `Ctrl+B |` - Split pane vertically
  - `Ctrl+B -` - Split pane horizontally
  - `Ctrl+B r` - Reload tmux config
- **Clipboard integration**
- **Vim/editor-friendly settings** (reduced escape time, focus events)

All font packages needed for proper Unicode rendering in Claude Code are also installed.
