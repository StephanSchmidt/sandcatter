# Creating Sandcatter Plugins

This guide explains how to create custom user plugins for sandcatter.

## Built-in vs User Plugins

Sandcatter includes **built-in plugins** that are embedded in the binary:
- Always available, no installation needed
- Currently includes: `tmux`

You can also create **user plugins** in your local `plugins/` directory:
- Custom configurations for your environment
- Can override built-in plugins by using the same name

## Plugin Structure

Each user plugin lives in its own directory under `plugins/`:

```
plugins/
└── myplugin/
    ├── plugin.json       # Required: Plugin manifest
    └── files/            # Optional: Files to copy into container
        ├── config1.conf
        └── config2.txt
```

## Plugin Manifest (plugin.json)

The `plugin.json` file defines what the plugin does. Here's a complete example:

```json
{
  "name": "myplugin",
  "version": "1.0.0",
  "description": "Short description of what this plugin provides",
  "apt_packages": ["package1", "package2"],
  "fonts": ["fonts-dejavu", "fonts-noto-core"],
  "locale_packages": ["locales"],
  "files": [
    {
      "source": "files/config.conf",
      "destination": "/etc/skel/.config.conf",
      "chmod": "644"
    }
  ],
  "compose_env": {
    "MY_VAR": "value",
    "ANOTHER_VAR": "another-value"
  },
  "locale_setup": {
    "locale": "en_US.UTF-8",
    "generate": true
  }
}
```

## Manifest Fields

### Required Fields

- **name** (string): Unique identifier for the plugin (lowercase, alphanumeric, hyphens/underscores allowed)
- **version** (string): Semantic version (e.g., "1.0.0", "2.1.3")
- **description** (string): Brief description shown in `sandcatter list`

### Optional Fields

#### apt_packages (array of strings)
Debian packages to install via `apt-get install`. These are merged into the existing `RUN apt-get install` command in the Dockerfile.

```json
"apt_packages": ["tmux", "vim", "curl"]
```

**Important:** Sandcatter checks for duplicates, so if a package is already installed, it won't be added again.

#### fonts (array of strings)
Font packages to install. These are treated the same as `apt_packages` but kept separate for clarity.

```json
"fonts": [
  "fonts-dejavu",
  "fonts-liberation",
  "fonts-noto-core",
  "fonts-noto-ui-core"
]
```

Common font packages:
- `fonts-dejavu` - High-quality fonts (Sans, Serif, Mono)
- `fonts-liberation` - Microsoft metric-compatible fonts
- `fonts-noto-core` - Google Noto with extensive Unicode
- `fonts-noto-ui-core` - UI-optimized variants
- `fontconfig` - Font configuration library

#### locale_packages (array of strings)
Packages needed for locale support (usually just `["locales"]`).

```json
"locale_packages": ["locales"]
```

#### files (array of objects)
Files to copy from the plugin directory into the container.

Each file object has:
- **source** (string): Path relative to plugin directory (e.g., "files/tmux.conf")
- **destination** (string): Absolute path in container (e.g., "/etc/skel/.tmux.conf")
- **chmod** (string): Optional file permissions (e.g., "644", "755")

```json
"files": [
  {
    "source": "files/my-config.conf",
    "destination": "/etc/skel/.my-config.conf",
    "chmod": "644"
  },
  {
    "source": "files/executable.sh",
    "destination": "/usr/local/bin/myscript.sh",
    "chmod": "755"
  }
]
```

**Process:**
1. File is copied from plugin to `.devcontainer/`
2. A `COPY` command is added to Dockerfile.app
3. File is then copied into the container at the specified destination

**Tip:** Use `/etc/skel/` for user config files - they'll be automatically copied to new user home directories.

#### compose_env (object)
Environment variables to add to the `app` service in `compose-all.yml`.

```json
"compose_env": {
  "TERM": "xterm-256color",
  "EDITOR": "vim",
  "MY_CUSTOM_VAR": "value"
}
```

Variables are added to the `environment:` section of the app service.

#### locale_setup (object)
Configuration for locale generation.

```json
"locale_setup": {
  "locale": "en_US.UTF-8",
  "generate": true
}
```

Fields:
- **locale** (string): Locale name (e.g., "en_US.UTF-8", "de_DE.UTF-8")
- **generate** (boolean): Whether to run locale-gen

**What it does:**
1. Uncomments the locale in `/etc/locale.gen`
2. Runs `locale-gen`
3. Sets environment variables: `LANG`, `LANGUAGE`, `LC_ALL`

## Example Plugins

### Minimal Plugin

The simplest possible plugin - just installs a package:

```json
{
  "name": "curl",
  "version": "1.0.0",
  "description": "Command-line tool for transferring data",
  "apt_packages": ["curl"]
}
```

### Configuration File Plugin

Plugin that installs a tool and adds its config file:

```json
{
  "name": "vim",
  "version": "1.0.0",
  "description": "Vim text editor with custom configuration",
  "apt_packages": ["vim"],
  "files": [
    {
      "source": "files/vimrc",
      "destination": "/etc/skel/.vimrc",
      "chmod": "644"
    }
  ]
}
```

Directory structure:
```
plugins/vim/
├── plugin.json
└── files/
    └── vimrc
```

### Full-Featured Plugin

The tmux plugin demonstrates all features:

```json
{
  "name": "tmux",
  "version": "1.0.0",
  "description": "Terminal multiplexer with 256-color support",
  "apt_packages": ["tmux"],
  "fonts": [
    "fonts-dejavu",
    "fonts-liberation",
    "fonts-noto-core",
    "fonts-noto-ui-core",
    "fontconfig"
  ],
  "locale_packages": ["locales"],
  "files": [
    {
      "source": "files/tmux.conf",
      "destination": "/etc/skel/.tmux.conf",
      "chmod": "644"
    }
  ],
  "compose_env": {
    "TERM": "xterm-256color"
  },
  "locale_setup": {
    "locale": "en_US.UTF-8",
    "generate": true
  }
}
```

## Best Practices

### 1. Use /etc/skel/ for User Configs

Files in `/etc/skel/` are automatically copied to new user home directories:

```json
"files": [
  {
    "source": "files/bashrc",
    "destination": "/etc/skel/.bashrc",
    "chmod": "644"
  }
]
```

### 2. Include Required Fonts

If your tool has a TUI or uses Unicode characters, include fonts:

```json
"fonts": [
  "fonts-dejavu",
  "fonts-noto-core",
  "fontconfig"
]
```

### 3. Set Up Locales for UTF-8

For proper Unicode support, always include locale setup:

```json
"locale_packages": ["locales"],
"locale_setup": {
  "locale": "en_US.UTF-8",
  "generate": true
}
```

### 4. Document Your Plugin

Add comments to your config files explaining what each setting does. Users will thank you!

### 5. Test Idempotency

Your plugin should be safe to apply multiple times:

```bash
./sandcatter apply /path/to/sandcat myplugin
./sandcatter apply /path/to/sandcat myplugin  # Should not break
```

Sandcatter handles this automatically, but test to be sure.

### 6. Consider Dependencies

If your plugin requires other plugins, document this in the description or README.

### 7. Use Descriptive Names

Good: `neovim`, `zsh-config`, `python-dev-tools`
Bad: `plugin1`, `stuff`, `misc`

## Testing Your Plugin

1. **Validate JSON syntax:**
   ```bash
   cat plugins/myplugin/plugin.json | jq .
   ```

2. **Check plugin loads:**
   ```bash
   ./sandcatter list
   ```

3. **Test with dry-run:**
   ```bash
   ./sandcatter apply /path/to/sandcat myplugin --dry-run
   ```

4. **Apply to test installation:**
   ```bash
   ./sandcatter apply /path/to/test-sandcat myplugin
   ```

5. **Rebuild and verify:**
   ```bash
   cd /path/to/test-sandcat
   docker compose -f .devcontainer/compose-all.yml build
   ```

## Plugin Ideas

Here are some plugin ideas to get you started:

- **neovim** - Modern Vim with LSP support
- **zsh-config** - Zsh with oh-my-zsh
- **python-dev** - Python development tools (pip, venv, poetry)
- **node-dev** - Additional Node.js tools (yarn, pnpm)
- **rust-dev** - Rust toolchain with cargo
- **docker-cli** - Docker CLI tools for container management
- **k8s-tools** - kubectl, helm, k9s
- **git-tools** - tig, lazygit, gh extensions
- **monitoring** - htop, btop, glances
- **network-tools** - netcat, tcpdump, wireshark-cli

## Contributing Plugins

If you create a useful plugin, consider contributing it back:

1. Fork the sandcatter repository
2. Add your plugin to `plugins/`
3. Test thoroughly
4. Submit a pull request with:
   - Plugin files
   - Documentation
   - Example usage

## Plugin Versioning

Follow semantic versioning:
- **1.0.0** - Initial release
- **1.0.1** - Bug fixes, no new features
- **1.1.0** - New features, backward compatible
- **2.0.0** - Breaking changes

## Troubleshooting

### Plugin not showing in list
- Check JSON syntax with `jq`
- Ensure `plugin.json` exists in plugin directory
- Check file permissions

### Files not being copied
- Verify source path is relative to plugin directory
- Check file exists: `ls plugins/myplugin/files/`
- Ensure destination is an absolute path

### Packages not installing
- Test package name: `docker run debian apt-cache search package-name`
- Check for typos in package names
- Verify package exists in Debian repos

### Locale errors
- Ensure `locales` package is in `locale_packages`
- Check locale name is valid: `locale -a` in a Debian container
- Verify `generate: true` is set in `locale_setup`

## Reference: tmux Plugin

The included tmux plugin is the reference implementation. Study it to understand:
- How to structure a complete plugin
- How to copy configuration files
- How to set up fonts and locales
- How to add environment variables

Location: `plugins/tmux/`
