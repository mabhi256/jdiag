package gc

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	TimestampLayout = "2006-01-02T15:04:05.000-0700"

	// GC Types
	GCTypeYoung      = "Young"
	GCTypeMixed      = "Mixed"
	GCTypeFull       = "Full"
	GCTypeConcurrent = "Concurrent Mark Cycle"

	// Parsing states
	StateNormal = iota
	StateConfigComplete
)

var (
	// [2025-07-27T06:54:55.176-0400]
	timestampPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{4})\]`)
	// gcIDPattern      = regexp.MustCompile(`GC\((\d+)\)`)

	// ==== Configuration patterns (only used initially) ====

	// Version: 21.0.8+9-Ubuntu-0ubuntu124.04.1 (release)
	versionPattern = regexp.MustCompile(`\[gc,init\]\s+Version:\s+([^\s(]+)`)

	// Heap region size: 1M
	heapRegionPattern = regexp.MustCompile(`\[gc,init\]\s+Heap Region Size:\s+(\d+[KMGT])`)

	// Maximum heap size: 256M
	heapMaxPattern = regexp.MustCompile(`\[gc,init\]\s+Heap Max Capacity:\s+(\d+[KMGT])`)

	// ==== Main GC event patterns ====

	// before->after pattern for memory measurements
	beforeAfter = `(\d+[KMGT])->(\d+[KMGT])\((\d+[KMGT])\)`

	// GC(0) Pause Young (Normal) (G1 Evacuation Pause) 9M->2M(16M) 5.326ms
	gcSummaryPattern = regexp.MustCompile(`GC\((\d+)\)\s+Pause\s+(.+?)\s+` + beforeAfter + `\s+([\d.]+)ms`)

	// GC(0) User=0.00s Sys=0.00s Real=0.01s
	gcCPUPattern = regexp.MustCompile(`GC\((\d+)\)\s+User=([\d.]+)s\s+Sys=([\d.]+)s\s+Real=([\d.]+)s`)

	// ==== Concurrent cycle patterns ====

	// Concurrent Cycle
	// Concurrent Mark Cycle
	concurrentCycleStartPattern = regexp.MustCompile(`GC\((\d+)\)\s+Concurrent (?:Mark )?Cycle$`)

	// Concurrent Cycle 89.437ms
	// Concurrent Mark Cycle 125.683ms
	concurrentCycleEndPattern = regexp.MustCompile(`GC\((\d+)\)\s+Concurrent (?:Mark )?Cycle\s+([\d.]+)ms$`)
	concurrentAbortPattern    = regexp.MustCompile(`GC\((\d+)\)\s+Concurrent Mark Abort`)

	// Pause Remark 211M->211M(256M) 21.685ms
	pauseRemarkPattern = regexp.MustCompile(`GC\((\d+)\)\s+Pause Remark\s+` + beforeAfter + `\s+([\d.]+)ms`)

	// Pause Cleanup 223M->213M(256M) 0.271ms
	pauseCleanupPattern = regexp.MustCompile(`GC\((\d+)\)\s+Pause Cleanup\s+` + beforeAfter + `\s+([\d.]+)ms`)

	// ==== Region and memory patterns ====

	// Eden regions: 50->0(50)
	// Survivor regions: 2->3(8)
	// Old regions: 100->105(200) or just 100->105 without the (200)
	// Humongous regions: 5->3(50)
	regionSummaryPattern = regexp.MustCompile(`(Eden|Survivor|Old|Humongous|Archive) regions:\s+(\d+)->(\d+)(?:\((\d+)\))?`)

	// garbage-first heap   total 975872K, used 587987K
	heapSummaryPattern = regexp.MustCompile(`garbage-first heap\s+total\s+(\d+)K,\s+used\s+(\d+)K`)

	// Metaspace       used 16279K, capacity 17210K, committed 17408K, reserved 1064960K
	// class space    used 1773K, capacity 1988K, committed 2048K, reserved 1048576K
	metaspacePattern = regexp.MustCompile(`(Metaspace|class space)\s+used\s+(\d+)K,\s+capacity\s+(\d+)K,\s+committed\s+(\d+)K,\s+reserved\s+(\d+)K`)

	// Metaspace: 138K(320K)->138K(320K) NonClass: 130K(192K)->130K(192K) Class: 8K(128K)->8K(128K)
	metaspaceBeforeAfterPattern = regexp.MustCompile(`(Metaspace|NonClass|Class):\s+(\d+)K\((\d+)K\)->(\d+)K\((\d+)K\)`)

	// ==== Worker timing patterns ====
	counter           = `(\d+)`
	workerSummaryReal = `Min:\s*([\d.]+),\s*Avg:\s*([\d.]+),\s*Max:\s*([\d.]+),\s*Diff:\s*([\d.]+),\s*Sum:\s*([\d.]+),\s*Workers:\s*(\d+)`

	// Using 8 workers of 8 for evacuation
	// Using 4 workers of 8 to rebuild remembered set
	workerUsageRegex = regexp.MustCompile(`Using ` + counter + ` workers of ` + counter + ` (for evacuation|to rebuild remembered set)`)

	// Ext Root Scanning (ms):            Min:  0.1, Avg:  0.2, Max:  0.4, Diff:  0.3, Sum:  2.1, Workers: 8
	// Update RS (ms):                    Min:  0.0, Avg:  0.1, Max:  0.2, Diff:  0.2, Sum:  0.8, Workers: 8
	// Object Copy (ms):                  Min:  0.5, Avg:  1.2, Max:  2.1, Diff:  1.6, Sum:  9.6, Workers: 8
	evacuationPhaseRegex = regexp.MustCompile(`(Ext Root Scanning|Update RS|Scan RS|Code Root Scanning|Object Copy|Termination|GC Worker Other|GC Worker Total) \(ms\):\s+` + workerSummaryReal)

	// Code Roots Fixup: 0.1ms
	// Reference Processing: 2.5ms
	// Clear Card Table: 0.3ms
	// Free Collection Set: 0.8ms
	postEvacuatePhaseRegex = regexp.MustCompile(`(Code Roots Fixup|Preserve CM Refs|Reference Processing|Clear Card Table|Evacuation Failure|Reference Enqueuing|Merge Per-Thread State|Code Roots Purge|Redirty Cards|Clear Claimed Marks|Free Collection Set|Humongous Reclaim|Expand Heap After Collection):\s+([\d.]+)ms`)
)

type ParseError struct {
	Line    string
	LineNum int
	Err     error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse error at line %d: %v", e.LineNum, e.Err)
}

type LineParser interface {
	CanParse(line string, context *ParseContext) bool
	Parse(line string, context *ParseContext) error
}

type ParseContext struct {
	Events       []*GCEvent
	Analysis     *GCAnalysis
	ActiveEvents map[int]*GCEvent
	Concurrent   map[int]*GCEvent
	// CreatedEvents map[int]*GCEvent
	State      int
	LineNumber int
}

func NewParseContext() *ParseContext {
	return &ParseContext{
		Events:       make([]*GCEvent, 0),
		Analysis:     &GCAnalysis{},
		ActiveEvents: make(map[int]*GCEvent),
		Concurrent:   make(map[int]*GCEvent),
		// CreatedEvents: make(map[int]*GCEvent),
		State: StateNormal,
	}
}

func extractTimestamp(line string, context *ParseContext) {
	if matches := timestampPattern.FindStringSubmatch(line); len(matches) >= 2 {
		if timestamp, err := time.Parse(TimestampLayout, matches[1]); err == nil {
			context.Analysis.EndTime = timestamp
		}
	}
}

// Handles JVM configuration (only processes config once)
type ConfigurationParser struct {
	configComplete bool
}

func NewConfigurationParser() *ConfigurationParser {
	return &ConfigurationParser{}
}

func (cp *ConfigurationParser) CanParse(line string, context *ParseContext) bool {
	if cp.configComplete || context.State == StateConfigComplete {
		return false
	}
	return strings.Contains(line, "[gc,init]")
}

func (cp *ConfigurationParser) Parse(line string, context *ParseContext) error {
	if matches := versionPattern.FindStringSubmatch(line); len(matches) > 1 {
		context.Analysis.JVMVersion = matches[1]
		return nil
	}

	if matches := heapRegionPattern.FindStringSubmatch(line); len(matches) > 1 {
		size, err := ParseMemorySize(matches[1])
		if err != nil {
			return fmt.Errorf("invalid heap region size: %v", err)
		}
		context.Analysis.HeapRegionSize = size
		return nil
	}

	if matches := heapMaxPattern.FindStringSubmatch(line); len(matches) > 1 {
		size, err := ParseMemorySize(matches[1])
		if err != nil {
			return fmt.Errorf("invalid heap max size: %v", err)
		}
		context.Analysis.HeapMax = size

		// Mark configuration as complete - no more config lines expected
		cp.configComplete = true
		context.State = StateConfigComplete
		return nil
	}

	return nil
}

type GCEventParser struct{}

func NewGCEventParser() *GCEventParser {
	return &GCEventParser{}
}

func (gp *GCEventParser) CanParse(line string, context *ParseContext) bool {
	return gcSummaryPattern.MatchString(line)
}

func (gp *GCEventParser) Parse(line string, context *ParseContext) error {
	matches := gcSummaryPattern.FindStringSubmatch(line)
	if len(matches) < 7 {
		return nil
	}

	gcID, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("invalid GC ID: %v", err)
	}

	if _, exists := context.Concurrent[gcID]; exists {
		return nil
	}

	event := gp.getOrCreateEvent(gcID, context)
	return gp.populateEvent(event, matches, context)
}

func (gp *GCEventParser) getOrCreateEvent(gcID int, context *ParseContext) *GCEvent {
	// // First check if event already exists anywhere
	// if event, exists := context.CreatedEvents[gcID]; exists {
	// 	return event // Return existing event (could be concurrent or active)
	// }

	event := &GCEvent{
		ID:         gcID,
		Timestamp:  context.Analysis.EndTime,
		RegionSize: context.Analysis.HeapRegionSize,
	}

	context.ActiveEvents[gcID] = event
	// context.CreatedEvents[gcID] = event
	context.Events = append(context.Events, event)
	return event
}

func (gp *GCEventParser) populateEvent(event *GCEvent, matches []string, context *ParseContext) error {
	// Parse type information
	typeInfo := NewGCTypeParser().Parse(matches[2])
	event.Type = typeInfo.Type
	event.Subtype = typeInfo.Subtype
	event.Cause = typeInfo.Cause

	// Parse memory sizes
	heapBefore, err := ParseMemorySize(matches[3])
	if err != nil {
		return fmt.Errorf("invalid heap before size: %v", err)
	}

	heapAfter, err := ParseMemorySize(matches[4])
	if err != nil {
		return fmt.Errorf("invalid heap after size: %v", err)
	}

	heapTotal, err := ParseMemorySize(matches[5])
	if err != nil {
		return fmt.Errorf("invalid heap total size: %v", err)
	}

	duration, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}

	event.HeapBefore = heapBefore
	event.HeapAfter = heapAfter
	event.HeapTotal = heapTotal
	event.Duration = time.Duration(duration * float64(time.Millisecond))

	return nil
}

// ConcurrentCycleParser handles concurrent GC cycle events
type ConcurrentCycleParser struct{}

func NewConcurrentCycleParser() *ConcurrentCycleParser {
	return &ConcurrentCycleParser{}
}

func (ccp *ConcurrentCycleParser) CanParse(line string, context *ParseContext) bool {
	return concurrentCycleStartPattern.MatchString(line) ||
		concurrentCycleEndPattern.MatchString(line) ||
		concurrentAbortPattern.MatchString(line) ||
		pauseRemarkPattern.MatchString(line) ||
		pauseCleanupPattern.MatchString(line)
}

func (ccp *ConcurrentCycleParser) Parse(line string, context *ParseContext) error {
	// Handle concurrent cycle start
	if matches := concurrentCycleStartPattern.FindStringSubmatch(line); len(matches) >= 2 {
		return ccp.handleCycleStart(matches, context)
	}

	// Handle concurrent abort
	if matches := concurrentAbortPattern.FindStringSubmatch(line); len(matches) >= 2 {
		return ccp.handleConcurrentAbort(matches, context)
	}

	// Handle concurrent cycle end
	if matches := concurrentCycleEndPattern.FindStringSubmatch(line); len(matches) >= 3 {
		return ccp.handleCycleEnd(matches, context)
	}

	// // Handle pause remark - update existing concurrent cycle
	// if matches := pauseRemarkPattern.FindStringSubmatch(line); len(matches) >= 6 {
	// 	return ccp.handlePauseRemark(matches, context)
	// }

	// // Handle pause cleanup - update existing concurrent cycle
	// if matches := pauseCleanupPattern.FindStringSubmatch(line); len(matches) >= 6 {
	// 	return ccp.handlePauseCleanup(matches, context)
	// }

	return nil
}

func (ccp *ConcurrentCycleParser) handleCycleStart(matches []string, context *ParseContext) error {
	gcID, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("invalid GC ID: %v", err)
	}

	// Check if concurrent cycle already exists - don't re-add
	if _, exists := context.Concurrent[gcID]; exists {
		return nil
	}

	// // Check if ANY event with this ID already exists
	// if event, exists := context.CreatedEvents[gcID]; exists {
	// 	// Convert existing event to concurrent type
	// 	fmt.Println("Concurrent Event with id exists: ", gcID, event.Type)
	// 	event.Type = GCTypeConcurrent
	// 	context.Concurrent[gcID] = event
	// 	// Remove from ActiveEvents if it was there
	// 	delete(context.ActiveEvents, gcID)
	// 	return nil
	// }

	event := &GCEvent{
		ID:         gcID,
		Type:       GCTypeConcurrent,
		Timestamp:  context.Analysis.EndTime,
		RegionSize: context.Analysis.HeapRegionSize,
	}

	context.Concurrent[gcID] = event
	// context.CreatedEvents[gcID] = event
	context.Events = append(context.Events, event)
	return nil
}

func (ccp *ConcurrentCycleParser) handleCycleEnd(matches []string, context *ParseContext) error {
	gcID, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("invalid GC ID: %v", err)
	}

	duration, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}

	if event, exists := context.Concurrent[gcID]; exists {
		event.ConcurrentDuration = time.Duration(duration * float64(time.Millisecond))
		delete(context.Concurrent, gcID)
	}

	return nil
}

func (ccp *ConcurrentCycleParser) handleConcurrentAbort(matches []string, context *ParseContext) error {
	gcID, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("invalid GC ID: %v", err)
	}

	if event, exists := context.Concurrent[gcID]; exists {
		event.Type = "Concurrent Mark Abort"
		event.ConcurrentMarkAborted = true
	}

	return nil
}

// func (ccp *ConcurrentCycleParser) handlePauseRemark(matches []string, context *ParseContext) error {
// 	gcID, err := strconv.Atoi(matches[1])
// 	if err != nil {
// 		return fmt.Errorf("invalid GC ID: %v", err)
// 	}

// 	duration, err := strconv.ParseFloat(matches[5], 64)
// 	if err != nil {
// 		return fmt.Errorf("invalid duration: %v", err)
// 	}

// 	// Update existing concurrent cycle instead of creating new event
// 	if event, exists := context.Concurrent[gcID]; exists {
// 		// Add remark duration to total concurrent duration
// 		remarkDuration := time.Duration(duration * float64(time.Millisecond))
// 		event.ConcurrentDuration += remarkDuration
// 	}

// 	return nil
// }

// func (ccp *ConcurrentCycleParser) handlePauseCleanup(matches []string, context *ParseContext) error {
// 	gcID, err := strconv.Atoi(matches[1])
// 	if err != nil {
// 		return fmt.Errorf("invalid GC ID: %v", err)
// 	}

// 	duration, err := strconv.ParseFloat(matches[5], 64)
// 	if err != nil {
// 		return fmt.Errorf("invalid duration: %v", err)
// 	}

// 	// Update existing concurrent cycle instead of creating new event
// 	if event, exists := context.Concurrent[gcID]; exists {
// 		// Add cleanup duration to total concurrent duration
// 		cleanupDuration := time.Duration(duration * float64(time.Millisecond))
// 		event.ConcurrentDuration += cleanupDuration
// 	}

// 	return nil
// }

// RegionDetailsParser handles region and memory information
type RegionDetailsParser struct{}

func NewRegionDetailsParser() *RegionDetailsParser {
	return &RegionDetailsParser{}
}

func (rdp *RegionDetailsParser) CanParse(line string, context *ParseContext) bool {
	return regionSummaryPattern.MatchString(line) ||
		heapSummaryPattern.MatchString(line) ||
		metaspacePattern.MatchString(line) ||
		metaspaceBeforeAfterPattern.MatchString(line)
}

func (rdp *RegionDetailsParser) Parse(line string, context *ParseContext) error {
	// Parse region summary transitions
	if matches := regionSummaryPattern.FindStringSubmatch(line); len(matches) >= 4 {
		return rdp.parseRegionSummary(matches, context)
	}

	// Parse heap summary
	if matches := heapSummaryPattern.FindStringSubmatch(line); len(matches) >= 3 {
		return rdp.parseHeapSummary(matches, context)
	}

	// Parse metaspace information
	if matches := metaspacePattern.FindStringSubmatch(line); len(matches) >= 6 {
		return rdp.parseMetaspaceInfo(matches, context)
	}

	// Parse metaspace before/after format
	if matches := metaspaceBeforeAfterPattern.FindStringSubmatch(line); len(matches) >= 6 {
		return rdp.parseMetaspaceBeforeAfter(matches, context)
	}

	return nil
}

func (rdp *RegionDetailsParser) parseRegionSummary(matches []string, context *ParseContext) error {
	// Find the most recent event to attach region info to
	if len(context.Events) == 0 {
		return nil
	}

	event := context.Events[len(context.Events)-1]
	regionType := matches[1]
	regionsBefore, _ := strconv.Atoi(matches[2])
	regionsAfter, _ := strconv.Atoi(matches[3])
	var regionsTarget int
	if len(matches) > 4 && matches[4] != "" {
		regionsTarget, _ = strconv.Atoi(matches[4])
	}

	switch regionType {
	case "Eden":
		event.EdenRegionsBefore = regionsBefore
		event.EdenRegionsAfter = regionsAfter
		event.EdenRegionsTarget = regionsTarget
		if event.RegionSize > 0 {
			event.EdenMemoryBefore = MemorySize(regionsBefore) * event.RegionSize
			event.EdenMemoryAfter = MemorySize(regionsAfter) * event.RegionSize
		}
	case "Survivor":
		event.SurvivorRegionsBefore = regionsBefore
		event.SurvivorRegionsAfter = regionsAfter
		event.SurvivorRegionsTarget = regionsTarget
		if event.RegionSize > 0 {
			event.SurvivorMemoryBefore = MemorySize(regionsBefore) * event.RegionSize
			event.SurvivorMemoryAfter = MemorySize(regionsAfter) * event.RegionSize
		}
	case "Old":
		event.OldRegionsBefore = regionsBefore
		event.OldRegionsAfter = regionsAfter
		if event.RegionSize > 0 {
			event.OldMemoryBefore = MemorySize(regionsBefore) * event.RegionSize
			event.OldMemoryAfter = MemorySize(regionsAfter) * event.RegionSize
		}
	case "Humongous":
		event.HumongousRegionsBefore = regionsBefore
		event.HumongousRegionsAfter = regionsAfter
		if event.RegionSize > 0 {
			event.HumongousMemoryBefore = MemorySize(regionsBefore) * event.RegionSize
			event.HumongousMemoryAfter = MemorySize(regionsAfter) * event.RegionSize
		}
	}

	return nil
}

func (rdp *RegionDetailsParser) parseHeapSummary(matches []string, context *ParseContext) error {
	if len(context.Events) == 0 {
		return nil
	}

	event := context.Events[len(context.Events)-1]
	totalMemory, _ := ParseMemorySize(matches[1] + "K")
	usedMemory, _ := ParseMemorySize(matches[2] + "K")

	if event.RegionSize > 0 {
		totalRegions := int(totalMemory.Bytes() / event.RegionSize.Bytes())
		usedRegions := int(usedMemory.Bytes() / event.RegionSize.Bytes())

		event.HeapTotalRegions = totalRegions
		event.HeapUsedRegionsAfter = usedRegions
	}

	return nil
}

func (rdp *RegionDetailsParser) parseMetaspaceInfo(matches []string, context *ParseContext) error {
	if len(context.Events) == 0 {
		return nil
	}

	event := context.Events[len(context.Events)-1]
	spaceType := matches[1]
	used, _ := ParseMemorySize(matches[2] + "K")
	capacity, _ := ParseMemorySize(matches[3] + "K")
	committed, _ := ParseMemorySize(matches[4] + "K")
	reserved, _ := ParseMemorySize(matches[5] + "K")

	switch spaceType {
	case "Metaspace":
		event.MetaspaceUsedAfter = used
		event.MetaspaceCapacityAfter = capacity
		event.MetaspaceCommittedAfter = committed
		event.MetaspaceReserved = reserved
	case "class space":
		event.ClassSpaceUsedAfter = used
		event.ClassSpaceCapacityAfter = capacity
		event.ClassSpaceReserved = reserved
	}

	return nil
}

func (rdp *RegionDetailsParser) parseMetaspaceBeforeAfter(matches []string, context *ParseContext) error {
	if len(context.Events) == 0 {
		return nil
	}

	event := context.Events[len(context.Events)-1]
	spaceType := matches[1]
	usedBefore, _ := ParseMemorySize(matches[2] + "K")
	committedBefore, _ := ParseMemorySize(matches[3] + "K")
	usedAfter, _ := ParseMemorySize(matches[4] + "K")
	committedAfter, _ := ParseMemorySize(matches[5] + "K")

	switch spaceType {
	case "Metaspace":
		event.MetaspaceUsedBefore = usedBefore
		event.MetaspaceCommittedBefore = committedBefore
		event.MetaspaceUsedAfter = usedAfter
		event.MetaspaceCommittedAfter = committedAfter
		event.MetaspaceCapacityBefore = committedBefore
		event.MetaspaceCapacityAfter = committedAfter
	case "Class":
		event.ClassSpaceUsedBefore = usedBefore
		event.ClassSpaceCapacityBefore = committedBefore
		event.ClassSpaceUsedAfter = usedAfter
		event.ClassSpaceCapacityAfter = committedAfter
	}

	return nil
}

// WorkerTimingParser handles worker thread timing information - updated to use correct patterns
type WorkerTimingParser struct{}

func NewWorkerTimingParser() *WorkerTimingParser {
	return &WorkerTimingParser{}
}

func (wtp *WorkerTimingParser) CanParse(line string, context *ParseContext) bool {
	return workerUsageRegex.MatchString(line) ||
		evacuationPhaseRegex.MatchString(line) ||
		postEvacuatePhaseRegex.MatchString(line)
}

func (wtp *WorkerTimingParser) Parse(line string, context *ParseContext) error {
	if len(context.Events) == 0 {
		return nil
	}

	event := context.Events[len(context.Events)-1]

	// Parse worker usage: "Using 8 workers of 8 for evacuation"
	if matches := workerUsageRegex.FindStringSubmatch(line); len(matches) >= 4 {
		workersUsed, _ := strconv.Atoi(matches[1])
		workersAvailable, _ := strconv.Atoi(matches[2])
		event.WorkersUsed = workersUsed
		event.WorkersAvailable = workersAvailable
		return nil
	}

	// Parse evacuation phase timing: "Object Copy (ms): Min: 0.1, Avg: 2.3, Max: 4.5, Diff: 4.4, Sum: 25.3, Workers: 11"
	if matches := evacuationPhaseRegex.FindStringSubmatch(line); len(matches) >= 8 {
		return wtp.parseEvacuationPhase(matches, event)
	}

	// Parse post-evacuation phase timing: "Reference Processing: 2.5ms"
	if matches := postEvacuatePhaseRegex.FindStringSubmatch(line); len(matches) >= 3 {
		return wtp.parsePostEvacuationPhase(matches, event)
	}

	return nil
}

func (wtp *WorkerTimingParser) parseEvacuationPhase(matches []string, event *GCEvent) error {
	phaseName := matches[1]
	avgTime, _ := strconv.ParseFloat(matches[3], 64)
	workers, _ := strconv.Atoi(matches[7])

	event.WorkersUsed = workers
	duration := time.Duration(avgTime * float64(time.Millisecond))

	switch phaseName {
	case "Ext Root Scanning":
		event.ExtRootScanTime = duration
	case "Update RS":
		event.UpdateRSTime = duration
	case "Scan RS":
		event.ScanRSTime = duration
	case "Code Root Scanning":
		event.CodeRootScanTime = duration
	case "Object Copy":
		event.ObjectCopyTime = duration
	case "Termination":
		event.TerminationTime = duration
	case "GC Worker Other":
		event.WorkerOtherTime = duration
	}

	return nil
}

func (wtp *WorkerTimingParser) parsePostEvacuationPhase(matches []string, event *GCEvent) error {
	phaseName := matches[1]
	duration, _ := strconv.ParseFloat(matches[2], 64)

	phaseTime := time.Duration(duration * float64(time.Millisecond))

	switch phaseName {
	case "Reference Processing":
		event.ReferenceProcessingTime = phaseTime
	case "Evacuation Failure":
		event.EvacuationFailureTime = phaseTime
	}

	return nil
}

// CPUTimingParser handles GC CPU timing information
type CPUTimingParser struct{}

func NewCPUTimingParser() *CPUTimingParser {
	return &CPUTimingParser{}
}

func (ctp *CPUTimingParser) CanParse(line string, context *ParseContext) bool {
	return gcCPUPattern.MatchString(line)
}

func (ctp *CPUTimingParser) Parse(line string, context *ParseContext) error {
	matches := gcCPUPattern.FindStringSubmatch(line)
	if len(matches) < 5 {
		return nil
	}

	gcID, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("invalid GC ID: %v", err)
	}

	event, exists := context.ActiveEvents[gcID]
	if !exists {
		return nil // Skip CPU lines for concurrent events
	}

	// Parse timing values
	userTime, err := ctp.parseTimeValue(matches[2])
	if err != nil {
		return fmt.Errorf("invalid user time: %v", err)
	}

	sysTime, err := ctp.parseTimeValue(matches[3])
	if err != nil {
		return fmt.Errorf("invalid sys time: %v", err)
	}

	realTime, err := ctp.parseTimeValue(matches[4])
	if err != nil {
		return fmt.Errorf("invalid real time: %v", err)
	}

	event.UserTime = userTime
	event.SystemTime = sysTime
	event.RealTime = realTime

	// Event is complete, finalize it
	ctp.finalizeEvent(event, context, gcID)

	return nil
}

func (ctp *CPUTimingParser) parseTimeValue(value string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func (ctp *CPUTimingParser) finalizeEvent(event *GCEvent, context *ParseContext, gcID int) {
	// Calculate derived values
	ctp.calculateDerivedValues(event)

	// Remove from active events
	delete(context.ActiveEvents, gcID)
}

func (ctp *CPUTimingParser) calculateDerivedValues(event *GCEvent) {
	// Calculate young generation totals
	event.YoungRegionsBefore = event.EdenRegionsBefore + event.SurvivorRegionsBefore
	event.YoungRegionsAfter = event.EdenRegionsAfter + event.SurvivorRegionsAfter
	event.YoungMemoryBefore = event.EdenMemoryBefore + event.SurvivorMemoryBefore
	event.YoungMemoryAfter = event.EdenMemoryAfter + event.SurvivorMemoryAfter

	// Calculate old memory approximations
	if event.HeapBefore > 0 && event.HeapAfter > 0 {
		if event.OldMemoryBefore == 0 {
			event.OldMemoryBefore = max(event.HeapBefore-event.YoungMemoryBefore-event.HumongousMemoryBefore, 0)
		}
		if event.OldMemoryAfter == 0 {
			event.OldMemoryAfter = max(event.HeapAfter-event.YoungMemoryAfter-event.HumongousMemoryAfter, 0)
		}
	}
}

// GCTypeInfo holds parsed GC type information
type GCTypeInfo struct {
	Type    string
	Subtype string
	Cause   string
}

// GCTypeParser handles parsing GC type strings
type GCTypeParser struct{}

func NewGCTypeParser() *GCTypeParser {
	return &GCTypeParser{}
}

func (gtp *GCTypeParser) Parse(typeString string) GCTypeInfo {
	parts := strings.Fields(typeString)
	if len(parts) == 0 {
		return GCTypeInfo{}
	}

	info := GCTypeInfo{Type: parts[0]}
	parentheticals := gtp.extractParentheses(typeString)

	// Apply parsing rules
	info = gtp.applyTypeOverrides(info, parentheticals)
	info = gtp.extractCauseAndSubtype(info, parentheticals)

	return info
}

func (gtp *GCTypeParser) extractParentheses(text string) []string {
	var results []string
	start := -1

	for i, char := range text {
		if char == '(' {
			start = i + 1
		} else if char == ')' && start != -1 {
			results = append(results, text[start:i])
			start = -1
		}
	}

	return results
}

func (gtp *GCTypeParser) applyTypeOverrides(info GCTypeInfo, parentheticals []string) GCTypeInfo {
	for _, paren := range parentheticals {
		if strings.Contains(strings.ToLower(paren), "mixed") {
			info.Type = GCTypeMixed
			break
		}
	}
	return info
}

func (gtp *GCTypeParser) extractCauseAndSubtype(info GCTypeInfo, parentheticals []string) GCTypeInfo {
	causePatterns := []string{"Allocation", "Pause", "System.gc", "Compaction", "Periodic Collection", "Ergonomics", "GCLocker"}

	for _, paren := range parentheticals {
		if gtp.containsAny(paren, causePatterns) {
			info.Cause = paren
		} else if info.Subtype == "" && !strings.Contains(strings.ToLower(paren), "mixed") {
			info.Subtype = paren
		}
	}

	// Fallback
	if info.Cause == "" && len(parentheticals) > 0 {
		info.Cause = parentheticals[len(parentheticals)-1]
	}

	return info
}

func (gtp *GCTypeParser) containsAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

type Parser struct {
	parsers []LineParser
}

func NewParser() *Parser {
	parsers := []LineParser{
		NewConfigurationParser(),
		NewConcurrentCycleParser(),
		NewGCEventParser(),
		NewRegionDetailsParser(),
		NewWorkerTimingParser(),
		NewCPUTimingParser(),
	}

	return &Parser{
		parsers: parsers,
	}
}

// ParseFile parses a GC log file using the configured parsers
func (p *Parser) ParseFile(filename string) ([]*GCEvent, *GCAnalysis, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	context := NewParseContext()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		context.LineNumber = lineNum

		if err := p.parseLine(line, context); err != nil {
			return nil, nil, ParseError{
				Line:    line,
				LineNum: lineNum,
				Err:     err,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scanner error: %v", err)
	}

	return context.Events, context.Analysis, nil
}

func (p *Parser) parseLine(line string, context *ParseContext) error {
	// Extract timestamp first - every line potentially has one
	extractTimestamp(line, context)

	// Run all other parsers
	for _, parser := range p.parsers {
		if parser.CanParse(line, context) {
			if err := parser.Parse(line, context); err != nil {
				return err
			}
		}
	}
	return nil
}
