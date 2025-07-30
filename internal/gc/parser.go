package gc

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Parser struct {
	timestampRegex  *regexp.Regexp
	gcSummaryRegex  *regexp.Regexp
	gcCpuRegex      *regexp.Regexp
	versionRegex    *regexp.Regexp
	heapRegionRegex *regexp.Regexp
	heapMaxRegex    *regexp.Regexp

	// Core G1GC patterns based on Microsoft GC Toolkit
	g1CollectionRegex        *regexp.Regexp
	toSpaceExhaustedRegex    *regexp.Regexp
	evacuationPhaseRegex     *regexp.Regexp
	postEvacuatePhaseRegex   *regexp.Regexp
	referenceProcessingRegex *regexp.Regexp
	concurrentPhaseRegex     *regexp.Regexp
	concurrentPhaseEndRegex  *regexp.Regexp
	regionSummaryRegex       *regexp.Regexp
	regionDisbursementRegex  *regexp.Regexp
	heapSummaryRegex         *regexp.Regexp
	metaClassSpaceRegex      *regexp.Regexp

	// Detailed timing patterns
	preEvacuateRegex    *regexp.Regexp
	parallelCountRegex  *regexp.Regexp
	workerSummaryRegex  *regexp.Regexp
	heapRootsRegex      *regexp.Regexp
	eagerReclaimRegex   *regexp.Regexp
	rememberedSetsRegex *regexp.Regexp
	scanHeapRootsRegex  *regexp.Regexp
	codeRootScanRegex   *regexp.Regexp

	// Advanced G1GC patterns
	concurrentCycleStartRegex *regexp.Regexp
	concurrentCycleEndRegex   *regexp.Regexp
	concurrentMarkStartRegex  *regexp.Regexp
	concurrentMarkEndRegex    *regexp.Regexp
	pauseRemarkRegex          *regexp.Regexp
	pauseCleanupRegex         *regexp.Regexp

	// Full GC patterns (should be rare in G1)
	fullPhaseRegex *regexp.Regexp

	// Region and memory details
	heapRegionSizeRegex *regexp.Regexp
	heapSizeRegex       *regexp.Regexp
}

func NewParser() *Parser {
	// Common pattern components (from Microsoft GC Toolkit)
	counter := `(\d+)`
	pauseTime := `([\d.]+)ms`
	concurrentTime := `([\d.]+)ms`
	workerSummaryReal := `Min:\s*([\d.]+),\s*Avg:\s*([\d.]+),\s*Max:\s*([\d.]+),\s*Diff:\s*([\d.]+),\s*Sum:\s*([\d.]+),\s*Workers:\s*(\d+)`
	workerSummaryInt := `Min:\s*(\d+),\s*Avg:\s*([\d.]+),\s*Max:\s*(\d+),\s*Diff:\s*(\d+),\s*Sum:\s*(\d+),\s*Workers:\s*(\d+)`
	beforeAfter := `(\d+[KMGT]?)->(\d+[KMGT]?)`
	gcCause := `\(([^)]+)\)`

	return &Parser{
		// Version: 21.0.8+9-Ubuntu-0ubuntu124.04.1 (release)
		versionRegex: regexp.MustCompile(`\[gc,init\]\s+Version:\s+([^\s(]+)`),

		// Heap region size: 1M
		heapRegionRegex: regexp.MustCompile(`\[gc,init\]\s+Heap Region Size:\s+(\d+[KMGT])`),

		// Maximum heap size: 256M
		heapMaxRegex: regexp.MustCompile(`\[gc,init\]\s+Heap Max Capacity:\s+(\d+[KMGT])`),

		// [2025-07-27T06:54:55.176-0400]
		timestampRegex: regexp.MustCompile(`\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{4})\]`),

		// GC(0) Pause Young (Normal) (G1 Evacuation Pause) 9M->2M(16M) 5.326ms
		gcSummaryRegex: regexp.MustCompile(`GC\((\d+)\)\s+Pause\s+(.+?)\s+(\d+[KMGT])->(\d+[KMGT])\((\d+[KMGT])\)\s+([\d.]+)ms`),

		// GC(0) User=0.00s Sys=0.00s Real=0.01s
		gcCpuRegex: regexp.MustCompile(`GC\((\d+)\)\s+User=([\d.]+)s\s+Sys=([\d.]+)s\s+Real=([\d.]+)s`),

		// Enhanced Microsoft GC Toolkit patterns for G1GC
		// Pause Young (Normal) (G1 Evacuation Pause)
		// Pause Mixed (Prepare Mixed) (G1 Evacuation Pause)
		// Pause Full (System.gc())
		g1CollectionRegex: regexp.MustCompile(`Pause (Young|Mixed|Initial Mark|Full)( \((Normal|Prepare Mixed|Mixed|Concurrent Start|Concurrent End)\))? ` + gcCause + `$`),

		// To-space exhausted
		toSpaceExhaustedRegex: regexp.MustCompile(`To-space exhausted`),

		// Ext Root Scanning (ms):            Min:  0.1, Avg:  0.2, Max:  0.4, Diff:  0.3, Sum:  2.1, Workers: 8
		// Update RS (ms):                    Min:  0.0, Avg:  0.1, Max:  0.2, Diff:  0.2, Sum:  0.8, Workers: 8
		// Object Copy (ms):                  Min:  0.5, Avg:  1.2, Max:  2.1, Diff:  1.6, Sum:  9.6, Workers: 8
		evacuationPhaseRegex: regexp.MustCompile(`(Ext Root Scanning|Update RS|Scan RS|Code Root Scanning|Object Copy|Termination|GC Worker Other|GC Worker Total) \(ms\):\s+` + workerSummaryReal),

		// Code Roots Fixup: 0.1ms
		// Reference Processing: 2.5ms
		// Clear Card Table: 0.3ms
		// Free Collection Set: 0.8ms
		postEvacuatePhaseRegex: regexp.MustCompile(`(Code Roots Fixup|Preserve CM Refs|Reference Processing|Clear Card Table|Evacuation Failure|Reference Enqueuing|Merge Per-Thread State|Code Roots Purge|Redirty Cards|Clear Claimed Marks|Free Collection Set|Humongous Reclaim|Expand Heap After Collection): ` + pauseTime),

		// Reference Processing: 2.5ms
		referenceProcessingRegex: regexp.MustCompile(`Reference Processing: ` + pauseTime),

		// Eden regions: 50->0(50)
		// Survivor regions: 2->3(8)
		// Old regions: 100->105(200) or just 100->105 without the (200)
		// Humongous regions: 5->3(50)
		regionSummaryRegex: regexp.MustCompile(`(Eden|Survivor|Old|Humongous|Archive) regions: ` + beforeAfter + `(?:\((\d+[KMGT]?)\))?`),

		// region size 1024K, 571 young (584704K), 1 survivors (1024K)
		regionDisbursementRegex: regexp.MustCompile(`region size ` + counter + `K, ` + counter + ` young \(` + counter + `K\), ` + counter + ` survivors \(` + counter + `K\)`),

		// garbage-first heap   total 975872K, used 587987K
		heapSummaryRegex: regexp.MustCompile(`garbage-first heap   total ` + counter + `K, used ` + counter + `K`),

		// Metaspace       used 16279K, capacity 17210K, committed 17408K, reserved 1064960K
		// class space    used 1773K, capacity 1988K, committed 2048K, reserved 1048576K
		metaClassSpaceRegex: regexp.MustCompile(`(Metaspace|class space)\s+used ` + counter + `K, capacity ` + counter + `K, committed ` + counter + `K, reserved ` + counter + `K`),

		// Pre Evacuate Collection Set: 0.5ms
		// Post Evacuate Collection Set: 1.2ms
		// Evacuate Collection Set: 8.5ms
		preEvacuateRegex: regexp.MustCompile(`(Pre|Post)? Evacuate Collection Set: ` + pauseTime),

		// Processed Buffers:               Min: 5, Avg: 12.5, Max: 25, Diff: 20, Sum: 100, Workers: 8
		// Termination Attempts:            Min: 1, Avg: 3.2, Max: 8, Diff: 7, Sum: 26, Workers: 8
		parallelCountRegex: regexp.MustCompile(`(Processed Buffers|Termination Attempts):\s+` + workerSummaryInt),

		// Using 8 workers of 8 for evacuation
		// Using 4 workers of 8 to rebuild remembered set
		workerSummaryRegex: regexp.MustCompile(`Using ` + counter + ` workers of ` + counter + ` (for evacuation|to rebuild remembered set)`),

		// Prepare Heap Roots: 0.2ms
		// Merge Heap Roots: 0.5ms
		// Prepare Merge Heap Roots: 0.1ms
		heapRootsRegex: regexp.MustCompile(`(Prepare|Merge|Prepare Merge) Heap Roots: ` + pauseTime),

		// Eager Reclaim (ms):              Min:  0.0, Avg:  0.1, Max:  0.2, Diff:  0.2, Sum:  0.8, Workers: 8
		eagerReclaimRegex: regexp.MustCompile(`Eager Reclaim \(ms\):\s+` + workerSummaryReal),

		// Remembered Sets (ms):            Min:  0.1, Avg:  0.3, Max:  0.8, Diff:  0.7, Sum:  2.4, Workers: 8
		rememberedSetsRegex: regexp.MustCompile(`Remembered Sets \(ms\):\s+` + workerSummaryReal),

		// Scan Heap Roots (ms):           Min:  0.2, Avg:  0.5, Max:  1.1, Diff:  0.9, Sum:  4.0, Workers: 8
		scanHeapRootsRegex: regexp.MustCompile(`Scan Heap Roots \(ms\):\s+` + workerSummaryReal),

		// Code Root Scan (ms):            Min:  0.0, Avg:  0.1, Max:  0.2, Diff:  0.2, Sum:  0.8, Workers: 8
		codeRootScanRegex: regexp.MustCompile(`Code Root Scan \(ms\):\s+` + workerSummaryReal),

		// Concurrent Cycle
		// Concurrent Mark Cycle
		concurrentCycleStartRegex: regexp.MustCompile(`Concurrent (?:Mark )?Cycle$`),

		// Concurrent Cycle 89.437ms
		// Concurrent Mark Cycle 125.683ms
		concurrentCycleEndRegex: regexp.MustCompile(`Concurrent (?:Mark )?Cycle ` + concurrentTime),

		// Concurrent Mark
		// Concurrent Clear Claimed Marks
		// Concurrent Scan Root Regions
		// Concurrent Rebuild Remembered Sets
		concurrentPhaseRegex: regexp.MustCompile(`Concurrent (Mark|Clear Claimed Marks|Scan Root Regions|Rebuild Remembered Sets|Create Live Data|Complete Cleanup|Cleanup for Next Mark)$`),

		// Concurrent Mark 53.902ms
		// Concurrent Clear Claimed Marks 0.018ms
		// Concurrent Scan Root Regions 2.325ms
		concurrentPhaseEndRegex: regexp.MustCompile(`Concurrent (Mark|Clear Claimed Marks|Scan Root Regions|Rebuild Remembered Sets|Create Live Data|Complete Cleanup|Cleanup for Next Mark) ` + concurrentTime),

		// Concurrent Mark (73.084s)
		concurrentMarkStartRegex: regexp.MustCompile(`Concurrent (Mark) \(.+\)$`),

		// Concurrent Mark (73.084s, 73.138s) 53.954ms
		concurrentMarkEndRegex: regexp.MustCompile(`Concurrent (Mark) \(.+\) ` + concurrentTime),

		// Pause Remark 211M->211M(256M) 21.685ms
		pauseRemarkRegex: regexp.MustCompile(`Pause Remark ` + beforeAfter + ` ` + pauseTime),

		// Pause Cleanup 223M->213M(256M) 0.271ms
		pauseCleanupRegex: regexp.MustCompile(`Pause Cleanup ` + beforeAfter + ` ` + pauseTime),

		// Phase 1: Mark live objects
		// Phase 2: Compute new object addresses 15.2ms
		// Phase 3: Adjust pointers 8.7ms
		// Phase 4: Move objects 12.3ms
		fullPhaseRegex: regexp.MustCompile(`Phase ` + counter + `: (Mark live objects|Compute new object addresses|Adjust pointers|Move objects|Prepare for compaction|Compact heap)( ` + pauseTime + `)?`),

		// Heap region size: 1M
		heapRegionSizeRegex: regexp.MustCompile(`Heap region size: (\d+[KMGT]?)`),

		// Minimum heap 256  Initial heap 256  Maximum heap 4096
		heapSizeRegex: regexp.MustCompile(`Minimum heap ` + counter + `  Initial heap ` + counter + `  Maximum heap ` + counter),
	}
}

func (p *Parser) ParseFile(filename string) (*GCLog, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	log := &GCLog{
		Events: make([]GCEvent, 0),
	}

	// GC events are split across multiple lines:
	// Line 1: GC(0) Pause Young ... 9M->2M(16M) 5.326ms  (summary info)
	// ...
	// Line 10: GC(0) User=0.00s Sys=0.00s Real=0.01s      (CPU timing)
	// We collect partial events here until we have both parts
	processingEvents := make(map[int]*GCEvent)
	concurrentCycles := make(map[int]time.Time)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		timestamp := p.extractTimestamp(line)
		p.parseConfiguration(line, log)

		gcId := extractGCId(line)
		// Skip lines without GC ID for event-specific parsing
		if gcId == -1 {
			continue
		}

		// Get or create event
		var event *GCEvent
		if existingEvent, exists := processingEvents[gcId]; exists {
			event = existingEvent
		} else {
			newEvent := &GCEvent{ID: gcId}
			processingEvents[gcId] = newEvent
			event = newEvent
		}

		p.parseG1Timing(line, event)
		p.parseG1RegionDetails(line, event)
		p.parseG1ConcurrentPhases(line, timestamp, concurrentCycles, event)
		p.parseGCSummary(line, timestamp, event)

		// Finalize events with CPU info
		if id, userTime, systemTime, realTime := p.parseGCCpu(line); id != -1 {
			if event, exists := processingEvents[id]; exists {
				event.UserTime = userTime
				event.SystemTime = systemTime
				event.RealTime = realTime

				log.Events = append(log.Events, *event)
				delete(processingEvents, id)
			}
		}

	}

	p.setLogBounds(log)
	return log, scanner.Err()
}

func (p *Parser) parseConfiguration(line string, log *GCLog) {
	// Parse JVM version
	matches := p.versionRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		log.JVMVersion = matches[1]
		return
	}

	// Parse heap region size
	matches = p.heapRegionRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		log.HeapRegionSize, _ = ParseMemorySize(matches[1])
		return
	}

	// Parse maximum heap size
	matches = p.heapMaxRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		log.HeapMax, _ = ParseMemorySize(matches[1])
		return
	}
}

func (p *Parser) extractTimestamp(line string) time.Time {
	//   matches[0] = "[2025-07-27T06:54:55.176-0400]"      // Full match
	//   matches[1] = "2025-07-27T06:54:55.176-0400"        // Capture group
	matches := p.timestampRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return time.Time{}
	}

	timestamp, err := time.Parse("2006-01-02T15:04:05.000-0700", matches[1])
	if err != nil {
		// handle error
		return time.Time{}
	}

	return timestamp
}

func (p *Parser) parseGCSummary(line string, ts time.Time, event *GCEvent) {
	/*
		matches[0] = "GC(0) Pause Young (Normal) (G1 Evacuation Pause) 9M->2M(16M) 5.326ms"  // Full match
		matches[1] = "0"                    // GC ID: (\d+)
		matches[2] = "Young "               // GC Type: ([^(]+)
		matches[3] = "Normal"               // Subtype: (?:\(([^)]+)\))?
		matches[4] = "G1 Evacuation Pause"  // Cause: (?:\(([^)]+)\))?
		matches[5] = "9M"                   // Heap Before: (\d+[MGK])
		matches[6] = "2M"                   // Heap After: (\d+[MGK])
		matches[7] = "16M"                  // Heap Total: (\d+[MGK])
		matches[8] = "5.326"                // Duration: ([\d.]+)
	*/
	matches := p.gcSummaryRegex.FindStringSubmatch(line)
	if len(matches) < 7 {
		return
	}

	fullTypeString := matches[2] // Young (Mixed) (G1 Humongous Allocation) (Evacuation Failure)
	gcType, subType, cause := parseGCTypeString(fullTypeString)

	heapBefore, _ := ParseMemorySize(matches[3])
	heapAfter, _ := ParseMemorySize(matches[4])
	heapTotal, _ := ParseMemorySize(matches[5])

	duration, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return
	}

	event.Timestamp = ts
	event.Type = gcType
	event.Subtype = subType
	event.Cause = cause
	event.Duration = time.Duration(duration * float64(time.Millisecond))
	event.HeapBefore = heapBefore
	event.HeapAfter = heapAfter
	event.HeapTotal = heapTotal

	// Check for G1GC-specific indicators
	event.ToSpaceExhausted = p.toSpaceExhaustedRegex.MatchString(line)

}

func (p *Parser) parseGCCpu(line string) (int, time.Duration, time.Duration, time.Duration) {
	matches := p.gcCpuRegex.FindStringSubmatch(line)
	if len(matches) < 5 {
		return -1, 0, 0, 0
	}

	id, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1, 0, 0, 0
	}

	user, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return -1, 0, 0, 0
	}

	sys, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return -1, 0, 0, 0
	}

	real, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return -1, 0, 0, 0
	}

	return id,
		time.Duration(user * float64(time.Second)),
		time.Duration(sys * float64(time.Second)),
		time.Duration(real * float64(time.Second))
}

func (p *Parser) parseG1Timing(line string, event *GCEvent) {

	/*
		Object Copy (ms):          Min:  0.1, Avg:  2.3, Max:  4.5, Diff:  4.4, Sum: 25.3, Workers: 11
		matches[1] = Phase name: "Object Copy"
		matches[2] = Min time: "0.1"
		matches[3] = Avg time: "2.3"
		matches[4] = Max time: "4.5"
		matches[5] = Diff time: "4.4"
		matches[6] = Sum time: "25.3"
		matches[7] = Workers count: "11"
	*/
	if matches := p.evacuationPhaseRegex.FindStringSubmatch(line); len(matches) >= 7 {
		phaseName := matches[1]
		avgTime, _ := strconv.ParseFloat(matches[3], 64)
		workers, _ := strconv.Atoi(matches[7])

		event.WorkersUsed = workers

		// Store specific phase timings
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
		return
	}

	// Parse post-evacuation phases
	if matches := p.postEvacuatePhaseRegex.FindStringSubmatch(line); len(matches) >= 3 {
		phaseName := matches[1]
		duration, _ := strconv.ParseFloat(matches[2], 64)

		phaseTime := time.Duration(duration * float64(time.Millisecond))

		switch phaseName {
		case "Reference Processing":
			event.ReferenceProcessingTime = phaseTime
		case "Evacuation Failure":
			event.EvacuationFailureTime = phaseTime
		}
		return
	}

	// Parse pre/post evacuation timing
	if matches := p.preEvacuateRegex.FindStringSubmatch(line); len(matches) >= 3 {
		phase := matches[1]
		duration, _ := strconv.ParseFloat(matches[2], 64)

		switch phase {
		case "Pre":
			event.PreEvacuateTime = time.Duration(duration * float64(time.Millisecond))
		case "Post":
			event.PostEvacuateTime = time.Duration(duration * float64(time.Millisecond))
		}
		return
	}

	// Parse worker information
	if matches := p.workerSummaryRegex.FindStringSubmatch(line); len(matches) >= 4 {
		workersUsed, _ := strconv.Atoi(matches[1])
		workersAvailable, _ := strconv.Atoi(matches[2])

		event.WorkersUsed = workersUsed
		event.WorkersAvailable = workersAvailable
	}
}

func (p *Parser) parseG1RegionDetails(line string, event *GCEvent) {
	// Parse region disbursement (Microsoft pattern)
	if matches := p.regionDisbursementRegex.FindStringSubmatch(line); len(matches) >= 6 {
		regionSize, _ := ParseMemorySize(matches[1] + "K")
		youngRegions, _ := strconv.Atoi(matches[2])
		youngMemory, _ := ParseMemorySize(matches[3] + "K")
		survivorRegions, _ := strconv.Atoi(matches[4])
		survivorMemory, _ := ParseMemorySize(matches[5] + "K")

		event.RegionSize = regionSize
		event.YoungRegions = youngRegions
		event.YoungRegionsMemory = youngMemory
		event.SurvivorRegions = survivorRegions
		event.SurvivorRegionsMemory = survivorMemory
		return
	}

	if matches := p.regionSummaryRegex.FindStringSubmatch(line); len(matches) >= 4 {
		regionType := matches[1]
		// regionsBefore, _ := strconv.Atoi(matches[2])
		regionsAfter, _ := strconv.Atoi(matches[3])
		// regionsConfigured, _ := strconv.Atoi(matches[4])

		switch regionType {
		case "Eden":
			event.EdenRegions = regionsAfter
		case "Survivor":
			event.SurvivorRegionsAfter = regionsAfter
		case "Old":
			event.OldRegions = regionsAfter
		case "Humongous":
			event.HumongousRegions = regionsAfter
		}
		return
	}

	// Parse heap summary
	if matches := p.heapSummaryRegex.FindStringSubmatch(line); len(matches) >= 3 {
		totalMemory, _ := ParseMemorySize(matches[1] + "K")
		usedMemory, _ := ParseMemorySize(matches[2] + "K")

		if event.RegionSize > 0 {
			event.HeapTotalRegions = int(totalMemory.Bytes() / event.RegionSize.Bytes())
			event.HeapUsedRegions = int(usedMemory.Bytes() / event.RegionSize.Bytes())
		}
		return
	}

	// Parse metaspace information
	if matches := p.metaClassSpaceRegex.FindStringSubmatch(line); len(matches) >= 6 {
		spaceType := matches[1]
		used, _ := ParseMemorySize(matches[2] + "K")
		capacity, _ := ParseMemorySize(matches[3] + "K")
		committed, _ := ParseMemorySize(matches[4] + "K")
		reserved, _ := ParseMemorySize(matches[5] + "K")

		switch spaceType {
		case "Metaspace":
			event.MetaspaceUsed = used
			event.MetaspaceCapacity = capacity
			event.MetaspaceCommitted = committed
			event.MetaspaceReserved = reserved
		case "class space":
			event.ClassSpaceUsed = used
			event.ClassSpaceCapacity = capacity
		}
	}
}

func (p *Parser) parseG1ConcurrentPhases(line string, timestamp time.Time, cycles map[int]time.Time, event *GCEvent) {
	// Track concurrent cycle starts
	if p.concurrentCycleStartRegex.MatchString(line) {
		cycles[event.ID] = timestamp
	}

	// Track concurrent cycle ends
	if matches := p.concurrentCycleEndRegex.FindStringSubmatch(line); len(matches) >= 2 {
		delete(cycles, event.ID)
	}

	// Track concurrent phases
	if matches := p.concurrentPhaseRegex.FindStringSubmatch(line); len(matches) >= 2 {
		phaseName := matches[1]
		event.ConcurrentPhase = phaseName
	}

	if matches := p.concurrentPhaseEndRegex.FindStringSubmatch(line); len(matches) >= 3 {
		phaseName := matches[1]
		duration, _ := strconv.ParseFloat(matches[2], 64)

		if event.ConcurrentPhase == phaseName {
			event.ConcurrentDuration = time.Duration(duration * float64(time.Millisecond))
		}
	}
}

func extractGCId(line string) int {
	gcIdRegex := regexp.MustCompile(`GC\((\d+)\)`)
	if matches := gcIdRegex.FindStringSubmatch(line); len(matches) >= 2 {
		if id, err := strconv.Atoi(matches[1]); err == nil {
			return id
		}
	}
	return -1
}

func parseGCTypeString(typeString string) (gcType, subType, cause string) {
	// Examples to handle:
	// "Young (Concurrent Start) (G1 Humongous Allocation)"
	// "Young (Mixed) (G1 Humongous Allocation) (Evacuation Failure)"
	// "Full (G1 Compaction Pause)"
	// "Remark"

	parts := strings.Fields(typeString)
	if len(parts) == 0 {
		return "", "", ""
	}

	gcType = parts[0] // "Young", "Full", "Remark", etc.

	// Extract all parenthetical expressions
	parentheticals := extractParentheses(typeString)

	if len(parentheticals) > 0 {
		// First parenthetical is usually subtype
		subType = parentheticals[0]

		// Look for common cause patterns
		for _, paren := range parentheticals {
			if strings.Contains(paren, "Allocation") ||
				strings.Contains(paren, "Pause") ||
				strings.Contains(paren, "System.gc") {
				cause = paren
				break
			}
		}

		// If no specific cause found, use the last parenthetical
		if cause == "" && len(parentheticals) > 1 {
			cause = parentheticals[len(parentheticals)-1]
		}
	}

	return gcType, subType, cause
}

func extractParentheses(text string) []string {
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

func (p *Parser) setLogBounds(log *GCLog) {
	if len(log.Events) == 0 {
		return
	}

	// Sort events by timestamp to ensure correct ordering
	sort.Slice(log.Events, func(i, j int) bool {
		return log.Events[i].Timestamp.Before(log.Events[j].Timestamp)
	})

	log.StartTime = log.Events[0].Timestamp
	log.EndTime = log.Events[len(log.Events)-1].Timestamp
}
