# Tracemesh

**AI-agent-aware task management for Git workflows.**

Tracemesh (`tm`) is a low-friction CLI that tracks work tasks in Git branches and automatically injects context protocols into AI coding agent instruction files. It ensures your AI assistant always knows what task you're working on.

## Features

- **Git-native task tracking** — Tasks are tied to branches, with automatic sync on checkout
- **AI agent integration** — Injects context protocols into Cursor, Claude, Windsurf, Copilot, Cline, OpenCode, and Antigravity instruction files
- **Post-checkout hook** — Automatically synchronizes active task when switching branches
- **Task lifecycle** — Start, finish, switch, and archive tasks with simple commands
- **Implementation logging** — Timestamped notes for tracking decisions and changes
- **Config file locking** — Prevents corruption from concurrent operations
- **Resource limits** — Protected against excessively large files

## Installation

### From Source

```bash
git clone https://github.com/Varun136/trace-mesh-cli.git
cd tracemesh
make install
```

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/Varun136/trace-mesh-cli/releases) page.

### Requirements

- Go 1.22 or later (for building from source)
- Git 2.20 or later

## Quick Start

```bash
# Initialize Tracemesh in your Git repository
tm init

# Start a new task
tm start "Implement user authentication"

# Check current status
tm status

# Add a note to the active task
tm note "Decided to use JWT over session cookies"

# View the active task
tm show

# List all tasks
tm list

# View task history for current branch
tm history

# Finish and archive the active task
tm finish
```

## Commands

| Command | Description |
|---------|-------------|
| `tm init` | Initialize Tracemesh in the current Git repository |
| `tm start [title]` | Create and activate a new task |
| `tm status` | Show current branch and active task |
| `tm show` | Display the active task's full content |
| `tm list` | List all tasks (active and archived) |
| `tm history` | Show task history for the current branch |
| `tm switch [id]` | Switch to a different active task |
| `tm sync` | Synchronize active task with current branch |
| `tm finish` | Archive the active task |
| `tm note [text]` | Append a timestamped note to the active task |
| `tm add [agent]` | Add Tracemesh instructions to an agent's rule file |
| `tm tm-prompt` | Print the Tracemesh context protocol |
| `tm --version` | Show version information |

## AI Agent Integration

Tracemesh automatically detects and injects context protocols into:

| Agent | File |
|-------|------|
| Cursor | `.cursorrules` |
| Claude | `CLAUDE.md` |
| Windsurf | `.windsurfrules` |
| Copilot | `.github/copilot-instructions.md` |
| Cline | `.clinerules` |
| OpenCode | `AGENTS.md` |
| Antigravity | `GEMINI.md` or `.agents/rule[s]/tracemesh.md` |

## Configuration

Tracemesh stores its state in `.tracemesh/` within your repository. `tm init` automatically adds `.tracemesh/` to the repository's `.gitignore` so task state remains available when switching branches without being committed.

If no supported AI-agent instruction file is found during initialization, Tracemesh asks whether to create `AGENTS.md` with the protocol (the default) or print the protocol for manual installation:

```
.tracemesh/
├── config.json      # Branch-to-task mappings and agent configuration
├── active.md        # Symlink to the currently active task
├── tasks/           # Active task files
└── archive/         # Archived (finished) task files
```

## Building

```bash
# Build the binary
make build

# Run tests
make test

# Build for all platforms
make build-all

# Clean build artifacts
make clean
```

## License

[MIT](LICENSE)
