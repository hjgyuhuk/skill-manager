[中文](README_zh.md)

# skillman

A CLI tool for managing agent skills across multiple AI coding tools.

## Supported directories

Scans `skills/` and `skills_disabled/` under each of these:

`~/.aider-desk` · `~/.agents` · `~/.augment` · `~/.bob` · `~/.claude` · `~/.codex` · `~/.codeartsdoer` · `~/.codebuddy` · `~/.codemaker` · `~/.codestudio` · `~/.commandcode` · `~/.continue` · `~/.cortex` · `~/.crush` · `~/.devin` · `~/.factory` · `~/.forge` · `~/.goose` · `~/.junie` · `~/.iflow` · `~/.kilocode` · `~/.kiro` · `~/.kode` · `~/.mcpjam` · `~/.vibe` · `~/.mux` · `~/.openhands` · `~/.pi` · `~/.qoder` · `~/.qwen` · `~/.rovodev` · `~/.roo` · `~/.tabnine/agent` · `~/.trae` · `~/.windsurf` · `~/.zencoder` · `~/.neovate` · `~/.pochi` · `~/.adal`

## Install

build from source:

```bash
git clone https://github.com/<your-username>/skillman.git
cd skillman
go build -o skillman .
mv skillman /usr/local/bin/
```

## Usage

### List skills

```bash
# Show all skills (enabled and disabled)
skillman list

# Show only enabled skills
skillman list --enabled

# Show only disabled skills
skillman list --disabled
```

Output:

```
Enabled:
  ✓ d3js
  ✓ threejs  (.agents, .claude)

Disabled:
  ✗ pixijs-events

Total: 3
```

When a skill exists in multiple directories, the source directories are shown in parentheses.

### Disable skills

Move skills from `skills/` to `skills_disabled/` in their respective directories.

```bash
skillman disable threejs-animation
skillman disable "threejs*"
skillman disable threejs-animation threejs-fundamentals
```

### Enable skills

Move skills from `skills_disabled/` back to `skills/`.

```bash
skillman enable threejs-animation
skillman enable "threejs*"
```

### Uninstall skills

Permanently delete skill directories. Always requires confirmation.

```bash
skillman uninstall threejs-animation
skillman uninstall "threejs*"
```

## Glob pattern matching

Commands support glob patterns (`*`, `?`, `[...]`) via `filepath.Match`:

| Pattern | Matches |
|---------|---------|
| `threejs*` | `threejs`, `threejs-animation`, `threejs-fundamentals`, ... |
| `pixijs-??` | `pixijs-math` (not `pixijs-events`) |
| `[abc]*` | `alpha`, `beta`, `gamma`, ... |

## Confirmation rules

| Command | Single exact name | Glob pattern / Multiple args |
|---------|-------------------|------------------------------|
| `disable` | No confirmation | Requires `[y/N]` confirmation |
| `enable` | No confirmation | Requires `[y/N]` confirmation |
| `uninstall` | Requires confirmation | Requires `[y/N]` confirmation |

## Zsh tip

Zsh errors on unmatched globs by default. Add this to `~/.zshrc`:

```zsh
setopt no_nomatch
```

This lets you write `skillman disable threejs*` without quotes.

## How it works

skillman scans all supported directories in parallel. Each directory has its own `skills/` (enabled) and `skills_disabled/` (disabled) folders. Operations like disable/enable/uninstall automatically find which directory a skill belongs to and operate on it in place.

## License

MIT
