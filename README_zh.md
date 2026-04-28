[English](README.md)

# skillman

管理多个 AI 编程工具 skills 的 CLI 工具。

## 支持的目录

扫描以下目录下的 `skills/` 和 `skills_disabled/`：

`~/.aider-desk` · `~/.agents` · `~/.augment` · `~/.bob` · `~/.claude` · `~/.codex` · `~/.codeartsdoer` · `~/.codebuddy` · `~/.codemaker` · `~/.codestudio` · `~/.commandcode` · `~/.continue` · `~/.cortex` · `~/.crush` · `~/.devin` · `~/.factory` · `~/.forge` · `~/.goose` · `~/.junie` · `~/.iflow` · `~/.kilocode` · `~/.kiro` · `~/.kode` · `~/.mcpjam` · `~/.vibe` · `~/.mux` · `~/.openhands` · `~/.pi` · `~/.qoder` · `~/.qwen` · `~/.rovodev` · `~/.roo` · `~/.tabnine/agent` · `~/.trae` · `~/.windsurf` · `~/.zencoder` · `~/.neovate` · `~/.pochi` · `~/.adal`

## 安装
从源码编译：

```bash
git clone https://github.com/<your-username>/skillman.git
cd skillman
go build -o skillman .
mv skillman /usr/local/bin/
```

## 使用

### 列出 skills

```bash
# 显示全部（已启用 + 已禁用）
skillman list

# 只显示已启用的
skillman list --enabled

# 只显示已禁用的
skillman list --disabled
```

输出示例：

```
Enabled:
  ✓ d3js
  ✓ threejs  (.agents, .claude)

Disabled:
  ✗ pixijs-events

Total: 3
```

当同一个 skill 存在于多个目录时，括号中会显示来源目录。

### 禁用 skills

将 skill 从对应目录的 `skills/` 移到 `skills_disabled/`。

```bash
skillman disable threejs-animation
skillman disable "threejs*"
skillman disable threejs-animation threejs-fundamentals
```

### 启用 skills

将 skill 从 `skills_disabled/` 移回 `skills/`。

```bash
skillman enable threejs-animation
skillman enable "threejs*"
```

### 卸载 skills

永久删除 skill 目录。始终需要确认。

```bash
skillman uninstall threejs-animation
skillman uninstall "threejs*"
```

## 通配符匹配

命令通过 `filepath.Match` 支持 glob 模式（`*`、`?`、`[...]`）：

| 模式 | 匹配结果 |
|------|---------|
| `threejs*` | `threejs`、`threejs-animation`、`threejs-fundamentals`、... |
| `pixijs-??` | `pixijs-math`（不匹配 `pixijs-events`） |
| `[abc]*` | `alpha`、`beta`、`gamma`、... |

## 确认规则

| 命令 | 单个精确名称 | 通配符 / 多个参数 |
|------|-------------|-------------------|
| `disable` | 无需确认 | 需要 `[y/N]` 确认 |
| `enable` | 无需确认 | 需要 `[y/N]` 确认 |
| `uninstall` | 需要确认 | 需要 `[y/N]` 确认 |

## Zsh 提示

Zsh 默认在通配符无匹配时报错。在 `~/.zshrc` 中添加：

```zsh
setopt no_nomatch
```

这样就可以不加引号直接写 `skillman disable threejs*`。

## 工作原理

skillman 并行扫描所有支持的目录。每个目录有独立的 `skills/`（启用）和 `skills_disabled/`（禁用）文件夹。禁用/启用/卸载操作会自动查找 skill 所在的目录，并在对应目录中执行。

## License

MIT
