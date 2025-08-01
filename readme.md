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

1. Historical Baseline Comparison
   - Distinguishes between "normal bad" vs "newly bad" performance
   - Enables regression detection after deployments
   - Compare current performance against 7-day, 30-day moving averages

2. Streaming Real-time Analysis
    - Enable immediate alerts for GC storms
    - Catch transient issues that don't show up in daily log analysis
    - Allow proactive intervention before user impact

3. Predictive Analytics (Prevents Outages)
    - Forecast when applications will hit memory limits
    - Predict when GC pauses will exceed SLA thresholds
    - Enable proactive scaling decisions

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

| **Problem Category** | **Specific Issue** | **Primary Metrics** | **Thresholds** | **Additional Context** |
|---------------------|-------------------|-------------------|---------------|----------------------|
| **Memory Leaks** | Memory Leak Pattern | • Heap growth trend (linear regression)<br>• Old region growth slope<br>• Consecutive growth cycles | • Growth rate >5% per collection<br>• >15 consecutive growth cycles | • `HeapAfter` trend analysis<br>• Old generation region count |
| **Evacuation Issues** | Evacuation Failures | • `EvacuationFailure` count<br>• Evacuation failure rate<br>• `ObjectCopyTime` average | • Failure rate >2%<br>• Object copy >50ms | • `ToSpaceExhausted` events<br>• `EvacuationFailureTime` |
| | Poor Evacuation Efficiency | • Eden regions before/after<br>• Evacuation efficiency ratio | • Efficiency <80% | • Region evacuation patterns |
| **Pause Time Issues** | Pause Target Violations | • `PauseTargetMissRate`<br>• `PauseTimeVariance`<br>• Pause percentiles (P95, P99) | • Miss rate >20%<br>• High variance (>50%) | • Individual phase timing |
| | Phase-Specific Delays | • `ObjectCopyTime`<br>• `ExtRootScanTime`<br>• `TerminationTime`<br>• `ReferenceProcessingTime` | • Object Copy >50ms<br>• Root Scan >30ms<br>• Termination >10ms<br>• Ref Processing >20ms | • Worker thread utilization |
| **Mixed Collections** | Missing Mixed Collections | • Mixed collection count<br>• Mixed-to-young ratio | • Ratio <5% | • `MixedGCCount` vs `YoungGCCount` |
| | Inefficient Mixed Collections | • Mixed collection efficiency<br>• Regions reclaimed vs selected | • Efficiency <30% | • Old generation cleanup rate |
| **Concurrent Marking** | Marking Falling Behind | • `ConcurrentMarkingKeepup` flag<br>• Allocation rate vs marking speed | • Cannot keep up indicator | • Mixed collection frequency |
| | Long Concurrent Phases | • `ConcurrentDuration` per phase<br>• Phase-specific thresholds | • Mark >5s<br>• Scan Root Regions >100ms<br>• Create Live Data >2s | • Individual concurrent phase timing |
| **Memory Pressure** | High Heap Utilization | • `AvgHeapUtil`<br>• Region utilization patterns | • >80% warning<br>• >90% critical | • `HeapUsedRegions`/`HeapTotalRegions` |
| | Region Exhaustion | • Region utilization rate<br>• Fragmentation analysis | • >85% utilization<br>• High fragmentation | • Region distribution patterns |
| **Allocation Issues** | High Allocation Rate | • `AllocationRate` (MB/s)<br>• Allocation bursts | • >1000 MB/s warning<br>• >5000 MB/s critical | • Heap growth between collections |
| | Allocation Bursts | • Allocation spikes count<br>• Irregular patterns | • >10% irregular intervals | • Statistical spike detection |
| | Premature Promotion | • Young generation efficiency<br>• Promotion rate | • Efficiency <70%<br>• Promotion >20% | • Survivor overflow analysis |
| **Worker Thread Issues** | Work Imbalance | • `TerminationTime`<br>• Worker utilization variance | • Termination >10ms | • Work distribution analysis |
| | Thread Saturation | • `WorkersUsed`/`WorkersAvailable`<br>• Worker efficiency | • >95% utilization | • Thread utilization patterns |
| | Underutilization | • Worker thread usage ratio | • <50% utilization | • Thread scaling issues |
| **Reference Processing** | Ref Processing Bottleneck | • `ReferenceProcessingTime`<br>• Reference processing frequency | • >20ms average<br>• >50ms critical | • Weak/Soft reference patterns |
| **GC Thrashing** | Excessive GC Frequency | • GC frequency (collections/sec)<br>• Short interval count | • >5 collections/sec<br>• >30% short intervals | • Collection interval analysis |
| **Full GC Issues** | Full GC Events | • `FullGCCount`<br>• Full GC rate per hour | • Any Full GC is critical | • G1 fallback indicators |
| **Region Sizing** | Poor Region Utilization | • Region size vs allocation patterns<br>• Region efficiency metrics | • Very high/low utilization | • Region size optimization |
| **Metaspace Issues** | Metaspace Pressure | • Metaspace-triggered collections<br>• Metaspace growth rate | • Any metaspace collections | • ClassLoader leak indicators |

## **Key Derived Metrics:**

| **Calculated Metric** | **Formula** | **Used For** |
|---------------------|-------------|--------------|
| `Throughput` | `(TotalRuntime - TotalGCTime) / TotalRuntime * 100` | Overall performance |
| `AllocationRate` | `HeapGrowth / TimeBetweenCollections` | Allocation pressure |
| `PromotionRate` | `OldGenGrowth / YoungGenSize` | Object lifecycle |
| `GCFrequency` | `CollectionCount / TotalTime` | GC thrashing |
| `PauseVariance` | `StdDev(PauseTimes) / Mean(PauseTimes)` | Pause predictability |
| `RegionUtilization` | `UsedRegions / TotalRegions` | Memory efficiency |

## **Critical Thresholds Summary:**

- **🔴 Critical**: Evacuation failures >5%, Full GC events, Throughput <80%, Pause variance >100%
- **⚠️ Warning**: Throughput <90%, Allocation rate >1GB/s, Heap utilization >80%, Phase timing beyond targets
- **✅ Good**: Throughput >95%, Low pause variance, Efficient mixed collections, No evacuation failures

internal/
├── common/
│   ├── types.go              # Universal types (Duration, MemorySize, etc.)
│   ├── interfaces.go         # Common interfaces across all analyzers
│   └── utils.go              # Universal utilities
├── gc/
│   ├── common/
│   │   ├── interfaces.go     # GC-specific interfaces
│   │   └── types.go          # GC shared types
│   ├── g1gc/
│   │   ├── parser.go
│   │   ├── metrics.go
│   │   ├── analysis.go
│   │   └── types.go
│   ├── zgc/
│   │   ├── parser.go
│   │   ├── metrics.go  
│   │   ├── analysis.go
│   │   └── types.go
│   └── parallel/             # Future: Parallel GC
├── heap/
│   ├── common/
│   │   ├── interfaces.go     # Heap analysis interfaces
│   │   └── types.go          # Heap shared types  
│   ├── hprof/
│   │   ├── parser.go         # .hprof file parsing
│   │   ├── metrics.go        # Object stats, leak detection
│   │   ├── analysis.go       # Memory leak patterns
│   │   └── types.go
│   ├── jfr/
│   │   ├── parser.go         # JFR heap events parsing
│   │   ├── metrics.go
│   │   ├── analysis.go
│   │   └── types.go
│   └── mat/                  # Future: Eclipse MAT integration
├── thread/
│   ├── common/
│   │   ├── interfaces.go     # Thread analysis interfaces
│   │   └── types.go          # Thread shared types
│   ├── jstack/
│   │   ├── parser.go         # jstack output parsing
│   │   ├── metrics.go        # Deadlock, contention analysis
│   │   ├── analysis.go       # Thread patterns
│   │   └── types.go
│   ├── jfr/
│   │   ├── parser.go         # JFR thread events
│   │   ├── metrics.go
│   │   ├── analysis.go
│   │   └── types.go
│   └── flight/               # Future: Flight Recorder integration
├── output/
│   ├── common/
│   │   ├── colors.go         # Universal color schemes
│   │   ├── tables.go         # Table formatting utilities
│   │   ├── charts.go         # ASCII chart utilities
│   │   └── utils.go          # Common formatting functions
│   ├── cli/
│   │   ├── common/
│   │   │   ├── layout.go     # CLI layout utilities
│   │   │   └── templates.go  # CLI templates
│   │   ├── gc/
│   │   │   ├── g1gc.go       # G1GC CLI formatter
│   │   │   └── zgc.go        # ZGC CLI formatter
│   │   ├── heap/
│   │   │   ├── hprof.go      # Heap dump CLI formatter
│   │   │   └── jfr.go        # JFR heap CLI formatter
│   │   └── thread/
│   │       ├── jstack.go     # Thread dump CLI formatter
│   │       └── jfr.go        # JFR thread CLI formatter
│   ├── tui/
│   │   ├── app.go            # Main TUI coordinator
│   │   ├── common/
│   │   │   ├── navigation.go # Common TUI navigation
│   │   │   ├── styles.go     # TUI styles
│   │   │   └── components.go # Reusable TUI components
│   │   ├── gc/
│   │   │   ├── g1gc/
│   │   │   │   ├── dashboard.go
│   │   │   │   ├── metrics.go
│   │   │   │   └── issues.go
│   │   │   └── zgc/
│   │   │       ├── dashboard.go
│   │   │       ├── metrics.go
│   │   │       └── issues.go
│   │   ├── heap/
│   │   │   ├── hprof/
│   │   │   │   ├── overview.go   # Object distribution
│   │   │   │   ├── leaks.go      # Memory leak analysis
│   │   │   │   └── objects.go    # Top objects view
│   │   │   └── jfr/
│   │   │       ├── allocation.go # Allocation tracking
│   │   │       └── timeline.go   # Heap over time
│   │   └── thread/
│   │       ├── jstack/
│   │       │   ├── overview.go   # Thread states overview
│   │       │   ├── deadlock.go   # Deadlock analysis
│   │       │   └── contention.go # Lock contention
│   │       └── jfr/
│   │           ├── activity.go   # Thread activity
│   │           └── blocking.go   # Blocking events
│   └── html/
│       ├── common/
│       │   ├── templates.go      # HTML templates
│       │   ├── assets.go         # CSS/JS assets  
│       │   └── charts.go         # Chart.js integration
│       ├── gc/
│       │   ├── g1gc.go
│       │   └── zgc.go
│       ├── heap/
│       │   ├── hprof.go
│       │   └── jfr.go
│       └── thread/
│           ├── jstack.go
│           └── jfr.go
└── detector/
    ├── file.go                   # Auto-detect file types
    ├── gc.go                     # Auto-detect GC type  
    └── format.go                 # Auto-detect dump formats

```bash
jdiag gc analyze gc.log -o cli|tui|html
jdiag heap analyze heap.hprof -o cli|tui|html
jdiag thread analyze thread.dump -o cli|tui|html  

# analyze/compare/watch/export 
jdiag jfr analyze recording.jfr
jdiag gc watch gc.log
jdiag jfr watch recording.jfr

```

Use sensible pause time target default
    - 10ms: realtime, trading, gaming
    - 50ms: web, api, microservices
    - 100ms: enterprise, application
    - 200ms: batch, analytics, etl
