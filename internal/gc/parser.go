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
}

func NewParser() *Parser {
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
		gcSummaryRegex: regexp.MustCompile(`GC\((\d+)\)\s+Pause\s+([^(]+)\s*(?:\(([^)]+)\))?\s*(?:\(([^)]+)\))?\s+(\d+[KMGT])->(\d+[KMGT])\((\d+[KMGT])\)\s+([\d.]+)ms`),

		// GC(0) User=0.00s Sys=0.00s Real=0.01s
		gcCpuRegex: regexp.MustCompile(`GC\((\d+)\)\s+User=([\d.]+)s\s+Sys=([\d.]+)s\s+Real=([\d.]+)s`),
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

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse init info
		p.parseInitInfo(line, log)

		// Extract timestamp
		timestamp := p.extractTimestamp(line)

		// Try to parse GC summary line
		if event := p.parseGCSummary(line, timestamp); event != nil {
			processingEvents[event.ID] = event
		}

		// Try to parse CPU line
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

func (p *Parser) parseInitInfo(line string, log *GCLog) {
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

func (p *Parser) parseGCSummary(line string, ts time.Time) *GCEvent {
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
	if len(matches) < 9 {
		return nil
	}

	id, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil
	}

	gcType := strings.TrimSpace(matches[2])
	subType := matches[3]
	cause := matches[4]

	heapBefore, _ := ParseMemorySize(matches[5])
	heapAfter, _ := ParseMemorySize(matches[6])
	heapTotal, _ := ParseMemorySize(matches[7])

	duration, err := strconv.ParseFloat(matches[8], 64)
	if err != nil {
		return nil
	}

	return &GCEvent{
		ID:         id,
		Timestamp:  ts,
		Type:       gcType,
		Subtype:    subType,
		Cause:      cause,
		Duration:   time.Duration(duration * float64(time.Millisecond)),
		HeapBefore: heapBefore,
		HeapAfter:  heapAfter,
		HeapTotal:  heapTotal,
	}
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
