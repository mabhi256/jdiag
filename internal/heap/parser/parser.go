package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
*	HProf binary format described here
*	https://github.com/openjdk/jdk/blob/master/src/hotspot/share/services/heapDumper.cpp
 */

// Parser represents the main HPROF file parser
type Parser struct {
	file       *os.File
	reader     *BinaryReader
	outputFile *os.File // For debugging output

	header       *model.HprofHeader
	stringReg    *registry.StringRegistry
	classReg     *registry.ClassRegistry
	stackReg     *registry.StackRegistry
	threadReg    *registry.ThreadRegistry
	rootReg      *registry.GCRootRegistry
	classDumpReg *registry.ClassDumpRegistry
	objectReg    *registry.ObjectRegistry

	// Statistics
	recordCount          int
	recordCountMap       map[model.HProfTagRecord]int
	heapDumpSegmentCount int
	heapDumpEnded        bool

	// Debug settings
	enableDetailedDebug bool
}

// NewParser creates a new HPROF parser
func NewParser(filename string) (*Parser, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %w", err)
	}

	// Create debug output file (remove extension, add .debug)
	ext := filepath.Ext(filename)
	baseFilename := strings.TrimSuffix(filename, ext)
	debugFilename := baseFilename + ".debug"

	outputFile, err := os.Create(debugFilename)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("unable to create debug file: %w", err)
	}

	parser := &Parser{
		file:                file,
		reader:              NewBinaryReader(file),
		outputFile:          outputFile,
		stringReg:           registry.NewStringRegistry(),
		classReg:            registry.NewClassRegistry(),
		stackReg:            registry.NewStackRegistry(),
		threadReg:           registry.NewThreadRegistry(),
		rootReg:             registry.NewGCRootRegistry(),
		classDumpReg:        registry.NewClassDumpRegistry(),
		objectReg:           registry.NewObjectRegistry(),
		recordCountMap:      make(map[model.HProfTagRecord]int),
		enableDetailedDebug: true, // Enable detailed position tracking
	}

	return parser, nil
}

// Close closes the parser and its files
func (p *Parser) Close() error {
	var err error
	if p.file != nil {
		err = p.file.Close()
	}
	if p.outputFile != nil {
		p.outputFile.Close()
	}
	return err
}

// debugf writes debug information to our output file
func (p *Parser) debugf(format string, args ...interface{}) {
	fmt.Fprintf(p.outputFile, format, args...)
}

// debugPositionf writes debug information with current byte position
func (p *Parser) debugPositionf(format string, args ...interface{}) {
	if p.enableDetailedDebug {
		position := p.reader.BytesRead()
		fmt.Fprintf(p.outputFile, "[0x%08X] %s", position, fmt.Sprintf(format, args...))
	}
}

// parseHeader parses the HPROF file header
func (p *Parser) parseHeader() error {
	p.debugf("--- Parsing Header ---\n")
	p.debugPositionf("Header start\n")

	header, err := ParseHeader(p.reader)
	if err != nil {
		return err
	}

	p.header = header

	p.debugf("Format: %s\n", header.Format)
	p.debugf("Identifier size: %d bytes\n", header.IdentifierSize)
	p.debugf("Raw timestamp: %d\n", header.Timestamp.UnixMilli())
	p.debugf("Timestamp: %s\n", header.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	p.debugPositionf("Header end (total header size: %d bytes)\n\n", p.reader.BytesRead())

	return nil
}

// parseRecord parses a single record based on its type
func (p *Parser) parseRecord(record *model.HprofRecord) error {
	switch record.Type {
	case model.HPROF_UTF8:
		return p.parseUTF8Record(record.Length)

	case model.HPROF_LOAD_CLASS:
		return p.parseLoadClassRecord()

	case model.HPROF_UNLOAD_CLASS:
		return p.parseUnloadClassRecord()

	case model.HPROF_FRAME:
		return p.parseFrameRecord()

	case model.HPROF_TRACE:
		return p.parseTraceRecord()

	case model.HPROF_HEAP_DUMP, model.HPROF_HEAP_DUMP_SEGMENT:
		return p.parseHeapDumpSegmentRecord(record.Length)

	case model.HPROF_HEAP_DUMP_END:
		return p.parseHeapDumpEndRecord(record.Length)

	case model.HPROF_ALLOC_SITES:
		p.debugPositionf("Skipping deprecated ALLOC_SITES record (legacy HPROF agent)\n")
		return p.skipRecordData(record.Length)

	case model.HPROF_HEAP_SUMMARY:
		p.debugPositionf("Skipping legacy HEAP_SUMMARY record (rarely used)\n")
		return p.skipRecordData(record.Length)

	case model.HPROF_START_THREAD, model.HPROF_END_THREAD:
		p.debugPositionf("Skipping rarely-used thread lifecycle record (%s)\n", record.Type)
		return p.skipRecordData(record.Length)

	case model.HPROF_CPU_SAMPLES:
		p.debugPositionf("Skipping deprecated CPU_SAMPLES record (legacy HPROF agent)\n")
		return p.skipRecordData(record.Length)

	case model.HPROF_CONTROL_SETTINGS:
		p.debugPositionf("Skipping deprecated CONTROL_SETTINGS record (legacy HPROF agent)\n")
		return p.skipRecordData(record.Length)

	default:
		p.debugPositionf("Skipping unknown record type 0x%02x\n", record.Type)
		return p.skipRecordData(record.Length)
	}
}

// parseUTF8Record parses a HPROF_UTF8 record
func (p *Parser) parseUTF8Record(length uint32) error {
	startPos := p.reader.BytesRead()

	utf8Body, err := ParseUTF8(p.reader, length, p.stringReg)
	if err != nil {
		return fmt.Errorf("failed to parse UTF8 record: %w", err)
	}

	if p.enableDetailedDebug {
		p.debugf("  [0x%08X] UTF8 Record:\n", startPos)
		p.debugf("    [0x%08X] String ID: 0x%x\n", startPos, uint64(utf8Body.StringID))
		p.debugf("    [0x%08X] Text: \"%s\" (%d chars)\n", startPos+int64(p.header.IdentifierSize), utf8Body.Text, len(utf8Body.Text))
	}

	return nil
}

// parseLoadClassRecord parses a HPROF_LOAD_CLASS record
func (p *Parser) parseLoadClassRecord() error {
	startPos := p.reader.BytesRead()

	loadClassBody, err := ParseLoadClass(p.reader, p.stringReg, p.classReg)
	if err != nil {
		return fmt.Errorf("failed to parse LOAD_CLASS record: %w", err)
	}

	if p.enableDetailedDebug {
		className := p.stringReg.GetOrUnresolved(loadClassBody.ClassNameID)
		p.debugf("  [0x%08X] LOAD_CLASS Record:\n", startPos)
		p.debugf("    [0x%08X] Class Serial: %d\n", startPos, loadClassBody.ClassSerialNumber)
		p.debugf("    [0x%08X] Object ID: 0x%x\n", startPos+4, uint64(loadClassBody.ObjectID))
		p.debugf("    [0x%08X] Stack Trace Serial: %d\n", startPos+4+int64(p.header.IdentifierSize), loadClassBody.StackTraceSerialNumber)
		p.debugf("    [0x%08X] Class Name ID: 0x%x -> \"%s\"\n", startPos+8+int64(p.header.IdentifierSize), uint64(loadClassBody.ClassNameID), className)
	}

	return nil
}

// parseUnloadClassRecord parses an UNLOAD_CLASS record
func (p *Parser) parseUnloadClassRecord() error {
	startPos := p.reader.BytesRead()

	unloadClassBody, err := ParseUnloadClass(p.reader, p.classReg)
	if err != nil {
		return fmt.Errorf("failed to parse UNLOAD_CLASS record: %w", err)
	}

	p.debugf("  [0x%08X] UNLOAD_CLASS - Class Serial: %d\n", startPos, unloadClassBody.ClassSerialNumber)
	return nil
}

func (p *Parser) parseFrameRecord() error {
	startPos := p.reader.BytesRead()

	frameBody, err := ParseStackFrame(p.reader, p.stackReg)
	if err != nil {
		return fmt.Errorf("failed to parse FRAME record: %w", err)
	}

	if p.enableDetailedDebug {
		p.debugf("  [0x%08X] FRAME Record:\n", startPos)
		p.debugf("    [0x%08X] Frame ID: 0x%x\n", startPos, uint64(frameBody.StackFrameID))
		p.debugf("    [0x%08X] Method: %s\n", startPos+int64(p.header.IdentifierSize), p.stringReg.GetOrUnresolved(frameBody.MethodNameID))
		p.debugf("    [0x%08X] Signature: %s\n", startPos+int64(p.header.IdentifierSize)*2, p.stringReg.GetOrUnresolved(frameBody.MethodSignatureID))
		p.debugf("    [0x%08X] Source: %s\n", startPos+int64(p.header.IdentifierSize)*3, p.stringReg.GetOrUnresolved(frameBody.SourceFileNameID))
		p.debugf("    [0x%08X] Class Serial: %d\n", startPos+int64(p.header.IdentifierSize)*4, frameBody.ClassSerialNumber)
		p.debugf("    [0x%08X] Line: %d\n", startPos+int64(p.header.IdentifierSize)*4+4, frameBody.LineNumber)
	}

	return nil
}

func (p *Parser) parseTraceRecord() error {
	startPos := p.reader.BytesRead()

	traceBody, err := ParseStackTrace(p.reader, p.stackReg)
	if err != nil {
		return fmt.Errorf("failed to parse TRACE record: %w", err)
	}

	if p.enableDetailedDebug {
		p.debugf("  [0x%08X] TRACE Record:\n", startPos)
		p.debugf("    [0x%08X] Trace Serial: %d\n", startPos, traceBody.StackTraceSerialNumber)
		p.debugf("    [0x%08X] Thread Serial: %d\n", startPos+4, traceBody.ThreadSerialNumber)
		p.debugf("    [0x%08X] Frame Count: %d\n", startPos+8, traceBody.NumFrames)

		frameIDsStart := startPos + 12
		p.debugf("    [0x%08X] Frame IDs: ", frameIDsStart)
		for i, frameID := range traceBody.StackFrameIDs {
			if i > 0 {
				p.debugf(", ")
			}
			p.debugf("0x%x", uint64(frameID))
		}
		p.debugf("\n")
	}

	return nil
}

func (p *Parser) parseHeapDumpSegmentRecord(length uint32) error {
	p.heapDumpSegmentCount++
	startPos := p.reader.BytesRead()

	subRecordCount, subRecordCountMap, err := ParseHeapDumpSegment(p.reader, length,
		p.rootReg, p.classDumpReg, p.objectReg, p.stringReg)
	if err != nil {
		return fmt.Errorf("failed to parse HEAP_DUMP_SEGMENT record: %w", err)
	}

	p.debugf("  [0x%08X] HEAP_DUMP_SEGMENT #%d: %d bytes, %d sub-records\n",
		startPos, p.heapDumpSegmentCount, length, subRecordCount)

	p.debugf("  Sub-record breakdown:\n")
	for subRecordType, count := range subRecordCountMap {
		p.debugf("    %s: %d\n", subRecordType, count)
	}

	return nil
}

func (p *Parser) parseHeapDumpEndRecord(length uint32) error {
	startPos := p.reader.BytesRead()

	err := ParseHeapDumpEnd(length)
	if err != nil {
		return fmt.Errorf("failed to parse HEAP_DUMP_END record: %w", err)
	}

	p.heapDumpEnded = true
	p.debugf("  [0x%08X] HEAP_DUMP_END - Heap dump sequence completed\n", startPos)

	return nil
}

// skipRecordData skips record data that we don't parse yet
func (p *Parser) skipRecordData(length uint32) error {
	startPos := p.reader.BytesRead()

	err := p.reader.Skip(int(length))
	if err != nil {
		return fmt.Errorf("failed to skip %d bytes: %w", length, err)
	}
	p.debugf("  [0x%08X] Skipped %d bytes of record data\n", startPos, length)
	return nil
}

// parseRecords parses all records in the file
func (p *Parser) parseRecords() error {
	p.debugf("--- Parsing Records ---\n")

	for {
		cursor := p.reader.BytesRead()

		record, err := p.reader.ReadRecordHeader()
		if err == io.EOF {
			p.debugf("Reached EOF at position 0x%08X. Parsed %d records.\n", cursor, p.recordCount)
			break
		}
		if err != nil {
			return fmt.Errorf("cursor %d - failed to read record header: %w", p.recordCount, err)
		}

		p.recordCount++
		p.recordCountMap[record.Type]++

		p.debugf("Record #%d at position 0x%08X:\n", p.recordCount, cursor)
		p.debugf("  [0x%08X] Type: %s (0x%02x)\n", cursor, record.Type, record.Type)
		p.debugf("  [0x%08X] Time offset: %d ms\n", cursor+1, record.TimeOffset)
		p.debugf("  [0x%08X] Length: %d bytes\n", cursor+5, record.Length)

		newCursorExpected := cursor + 9 + int64(record.Length)

		err = p.parseRecord(record)
		if err != nil {
			return fmt.Errorf("failed to parse record %s at position 0x%08X: %w", record.Type, cursor, err)
		}

		if p.reader.BytesRead() != newCursorExpected {
			return fmt.Errorf(
				"position mismatch after %s record: expected 0x%08X, got 0x%08X",
				record.Type, newCursorExpected, p.reader.BytesRead())
		}
	}

	return nil
}

// SetDetailedDebug enables or disables detailed position tracking
func (p *Parser) SetDetailedDebug(enabled bool) {
	p.enableDetailedDebug = enabled
}

// GetCurrentPosition returns the current byte position in the file
func (p *Parser) GetCurrentPosition() int64 {
	return p.reader.BytesRead()
}

// printSummary prints a summary of parsing results
func (p *Parser) printSummary() {
	p.debugf("--- Record Summary ---\n")
	p.debugf("Total records: %d\n", p.recordCount)
	p.debugf("Record type breakdown:\n")
	for recordType, count := range p.recordCountMap {
		p.debugf("  %s: %d\n", recordType, count)
	}
	p.debugf("Total bytes processed: %d (0x%08X)\n", p.reader.BytesRead(), p.reader.BytesRead())

	// Show some example strings
	p.debugf("\nSample strings from table:\n")
	stringSampleCount := 0
	maxStringSamples := 10
	for id, text := range p.stringReg.GetAll() {
		if stringSampleCount >= maxStringSamples {
			break
		}
		if len(text) > 50 {
			p.debugf("  0x%x: \"%.50s...\" (%d chars)\n", uint64(id), text, len(text))
		} else {
			p.debugf("  0x%x: \"%s\"\n", uint64(id), text)
		}
		stringSampleCount++
	}

	p.debugf("Total strings in table: %d\n", p.stringReg.Count())

	p.debugf("\nSample loaded classes:\n")
	loadedClasses := p.classReg.GetLoadedClasses()
	maxClassSamples := 15
	for i, classInfo := range loadedClasses {
		if i >= maxClassSamples {
			p.debugf("  ... and %d more classes\n", len(loadedClasses)-maxClassSamples)
			break
		}
		p.debugf("  %d. %s (id: 0x%x)\n",
			classInfo.LoadClassBody.ClassSerialNumber,
			classInfo.ClassName,
			uint64(classInfo.LoadClassBody.ObjectID))
	}

	p.debugf("\n--- GC Root Summary ---\n")
	p.debugf("Total GC roots: %d\n", p.rootReg.GetTotalRoots())

	rootTypeCounts := p.rootReg.GetRootTypeCounts()
	p.debugf("GC root type breakdown:\n")
	for rootType, count := range rootTypeCounts {
		p.debugf("  %s: %d\n", rootType, count)
	}

	p.debugf("\n--- Object Instance Summary ---\n")
	p.debugf("Total object instances: %d\n", p.objectReg.GetCount())
	p.debugf("Total instance memory: %d bytes (%.2f MB)\n",
		p.objectReg.GetTotalSize(),
		float64(p.objectReg.GetTotalSize())/(1024*1024))

	// Show instance counts by class
	classCounts := p.objectReg.GetInstanceClassCounts()
	if len(classCounts) > 0 {
		p.debugf("\nTop classes by instance count:\n")
		// Simple display of first few classes
		count := 0
		maxClassInstanceSamples := 10
		for classID, instanceCount := range classCounts {
			if count >= maxClassInstanceSamples {
				p.debugf("  ... and %d more classes\n", len(classCounts)-maxClassInstanceSamples)
				break
			}
			p.debugf("  Class 0x%x: %d instances\n", uint64(classID), instanceCount)
			count++
		}
	}

	// Show Thread instances (Phase 9.4)
	threadInstanceCount := p.objectReg.GetThreadCount()
	if threadInstanceCount > 0 {
		p.debugf("\nThread Object Instances: %d\n", threadInstanceCount)
		threadInstances := p.objectReg.GetAllThreadInstances()
		maxThreadInstanceSamples := 5
		count := 0
		for objectID, threadData := range threadInstances {
			if count >= maxThreadInstanceSamples {
				p.debugf("  ... and %d more thread instances\n", len(threadInstances)-maxThreadInstanceSamples)
				break
			}

			name := threadData.Name
			if name == "" {
				name = "<unknown>"
			}
			p.debugf("  Thread 0x%x (TID: %d): Name=\"%s\", Priority=%d, Daemon=%t\n",
				uint64(objectID), threadData.ThreadID, name, threadData.Priority, threadData.Daemon)
			count++
		}
	}
}

// ParseHprof parses an HPROF file completely
func (p *Parser) ParseHprof() error {
	defer p.Close()

	p.debugf("🔍 Starting HPROF analysis of: %s\n", p.file.Name())
	p.debugf("File size: %d bytes\n\n", p.getFileSize())

	if err := p.parseHeader(); err != nil {
		return err
	}

	if err := p.parseRecords(); err != nil {
		return err
	}

	p.printSummary()
	p.debugf("--- PARSING COMPLETE ---\n")

	return nil
}

// getFileSize returns the total file size
func (p *Parser) getFileSize() int64 {
	if stat, err := p.file.Stat(); err == nil {
		return stat.Size()
	}
	return 0
}

// GetHeader returns the parsed header
func (p *Parser) GetHeader() *model.HprofHeader {
	return p.header
}

// GetStringRegistry returns the string registry
func (p *Parser) GetStringRegistry() *registry.StringRegistry {
	return p.stringReg
}
