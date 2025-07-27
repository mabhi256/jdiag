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

## Performance Tuning

- *Focus*: Optimizing throughput and latency
- *Key Metrics*: Pause times, allocation rates, GC frequency, throughput percentage
- *Parsing Needs*: Extract pause times, heap before/after, duration, GC type
- *Analysis*: Percentile calculations, trend analysis, recommendations engine
- *Output*: "Your 95th percentile pause time is 15ms, target is <10ms"

## Issue Diagnosis

- *Focus*: Root cause analysis of GC problems
- *Key Metrics*: Evacuation failures, Full GCs, allocation spikes, humongous allocations
- *Parsing Needs*: Detailed phase breakdowns, failure patterns, concurrent cycle analysis
- *Analysis*: Anomaly detection, correlation analysis, pattern matching
- *Output*: "Detected evacuation failure at 06:54:42, caused by humongous allocations"

## Capacity Planning

- *Focus*: Right-sizing heap and predicting future needs
- *Key Metrics*: Heap utilization trends, allocation rates, region usage patterns
- *Parsing Needs*: Heap region data, metaspace usage, allocation patterns over time
- *Analysis*: Trend projection, utilization modeling, scenario planning
- *Output*: "Based on allocation trends, recommend increasing heap to 1GB by Q2"

## Todo

```bash
// avgBytesPerEvent = 250  // Typical GC log line size
// Small files (<10MB): Load everything
// Medium files (10MB-100MB): Chunked processing
// Large files (>100MB): Streaming with summary or sampling
Events: make([]GCEvent, 0, min(estimateFromFileSize(filename), 10000)),

// For huge logs, offer different analysis modes
jdiag gc analyze huge.log --mode=summary   // Just key metrics
jdiag gc analyze huge.log --mode=sample    // Analyze every 10th event
jdiag gc analyze huge.log --mode=recent    // Last 1000 events only
jdiag gc analyze huge.log --mode=streaming // Process in chunks
```
