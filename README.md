# dcell

Development context manager combining:
- **Git/JJ worktrees** - Isolated working copies
- **Docker environments** - Port-mapped, isolated services
- **AI sessions** - Context-aware AI assistant integration

## Installation

```bash
# Build from source
git clone https://github.com/heelune/dcell
cd dcell
go build -o dcell ./cmd/dcell
mv dcell ~/.local/bin/
```

## Quick Start

```bash
# Create a new context
dcell create feature-x --from main

# List contexts
dcell list

# Switch to a context
dcell switch feature-x
cd ../feature-x

# Start AI assistant
dcell ai

# Remove when done
dcell remove feature-x
```

## Features

### VCS Support
- **Jujutsu (jj)** - Native workspace support
- **Git** - Worktree support with automatic fallback

### Docker Integration
- Automatic port allocation (prevents conflicts)
- Auto-generated `docker-compose.dcell.yml`
- Per-context `.env.dcell` with database URLs

### AI Session Management
- Per-context session storage
- `context.md`, `todo.md`, `decisions.md` auto-created
- Claude Code and Kimi CLI support

## Configuration

### Global config: `~/.config/dcell/config.toml`
```toml
[vcs]
prefer = "jj"  # "jj" or "git"

[docker]
port_base = 3000
port_step = 10
services = ["app", "db", "redis"]

[ai]
default = "claude"
```

### Project config: `.dcell/config.toml`
Project-specific overrides.

## Commands

| Command | Description |
|---------|-------------|
| `create <name>` | Create new context |
| `switch <name>` | Switch to context |
| `list` | List all contexts |
| `remove <name>` | Remove context |
| `ai [name]` | Start AI assistant |

## License

MIT
