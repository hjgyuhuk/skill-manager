[中文](README_zh.md)

# skillman

A CLI tool for managing agent skills across multiple AI coding tools.

## Supported directories

Scans `skills/` and `skills_disabled/` under each of these:

`~/.aider-desk` · `~/.agents` · `~/.augment` · `~/.bob` · `~/.claude` · `~/.codex` · `~/.codeartsdoer` · `~/.codebuddy` · `~/.codemaker` · `~/.codestudio` · `~/.commandcode` · `~/.continue` · `~/.cortex` · `~/.crush` · `~/.devin` · `~/.factory` · `~/.forge` · `~/.goose` · `~/.junie` · `~/.iflow` · `~/.kilocode` · `~/.kiro` · `~/.kode` · `~/.mcpjam` · `~/.vibe` · `~/.mux` · `~/.openhands` · `~/.pi` · `~/.qoder` · `~/.qwen` · `~/.rovodev` · `~/.roo` · `~/.tabnine/agent` · `~/.trae` · `~/.windsurf` · `~/.zencoder` · `~/.neovate` · `~/.pochi` · `~/.adal`

## Install

build from source:

```bash
git clone https://github.com/hjgyuhuk/skillman.git
cd skillman
go build -o skillman .
mv skillman /usr/local/bin/
```

## Usage

### Install skills

Install skills from a GitHub repository to `~/.agents/skills/`.

```bash
# Install from owner/repo shorthand
skillman install vercel-labs/skills

# Install from full URL
skillman install https://github.com/org/repo

# Install with a specific branch/tag
skillman install vercel-labs/skills@canary

# Install a specific skill
skillman install vercel-labs/skills -s react

# Install to a different agent directory
skillman install vercel-labs/skills -a .claude

# Skip confirmation (auto-overwrite existing)
skillman install vercel-labs/skills -y
```

When multiple skills are found in a repo, an interactive selector is shown:

```
  [x] react
  [ ] vue
  [ ] svelte
  [x] angular
  ↑↓ move  space toggle  a select all  enter confirm
```

### Update skills

Update skills that were installed via `skillman install`. Each installed skill stores its git source in `.skillman.json` for tracking.

```bash
# Check and update all installed skills
skillman update

# Update a specific skill
skillman update react

# Skip confirmation
skillman update -y
```

Example output:

```
Checking vercel-labs/skills... abc12345
  react: update available (abc12345 → def67890)
  vue: up to date
Update 1 skill(s):
  react (abc12345 → def67890)
Proceed? [y/N]
```

The update check uses `git ls-remote` (lightweight, no download) to compare commit SHAs before cloning.

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
| `install` | No confirmation | N/A (interactive selector) |
| `update` | No confirmation | Requires `[y/N]` confirmation |
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
