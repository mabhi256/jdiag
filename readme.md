# jdiag - Java Diagnostics Tool

A comprehensive tool for analyzing Java applications and logs.

- sample taken from <https://github.com/microsoft/gctoolkit-testdata/tree/main/gctoolkit-gclogs>

## Installation

### Quick Install (Recommended)

```bash
go install github.com/myprofile/jdiag@latest

jdiag gc analyze app.log
# 🔧 First run detected, setting up jdiag...
# ✅ Shell completions installed automatically
# 💡 Restart your shell to enable tab completion
```

### Alternative Installation Methods

#### From Source

```bash
git clone https://github.com/myprofile/jdiag.git
cd jdiag
go build -o jdiag
./jdiag install
```

## Usage

### GC Log Analysis

```bash
# Validate a GC log file
jdiag gc validate app.log

# Analyze with CLI output (default)
jdiag gc analyze app.log

# Analyze with HTML output
jdiag gc analyze app.log -o html

# Analyze with TUI (Terminal UI)
jdiag gc analyze app.log -o tui
```

### Shell Completion

Completions are installed automatically on first use!

```bash
# After restarting your shell, enjoy tab completion:
jdiag gc <TAB>                    # Shows: analyze, validate
jdiag gc analyze <TAB>            # Shows .log files
jdiag gc analyze app.log -o <TAB> # Shows: cli, tui, html
```

If auto-setup doesn't work, run `jdiag install` for manual setup.

## Commands

- `jdiag gc analyze` - Analyze GC log files
- `jdiag gc validate` - Validate GC log files  
- `jdiag install` - Install shell completions and verify setup
- `jdiag version` - Show version information

### How Auto-Setup Works

On first run, jdiag:

1. Detects your shell (bash/zsh/fish/powershell)
2. Installs completion files in user-local directories (safe, no sudo needed)
3. Creates a marker file to avoid repeated setup attempts
4. Falls back gracefully if installation fails

Completion files are installed to:

- **Bash**: `~/.local/share/bash-completion/completions/jdiag`
- **Zsh**: `~/.zsh/completions/_jdiag`
- **Fish**: `~/.config/fish/completions/jdiag.fish`  
- **PowerShell**: `~/jdiag_completion.ps1`

## Publish this build

```bash
git tag v1.0.0
git push origin v1.0.0

# Test the build without releasing
goreleaser build --snapshot --clean

# Test the full release process without publishing
goreleaser release --snapshot --clean

# Release using the tag
goreleaser release --clean
```
