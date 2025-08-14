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

	header          *model.HprofHeader
	stringReg       *registry.StringRegistry
	classReg        *registry.ClassRegistry
	stackReg        *registry.StackRegistry
	threadReg       *registry.ThreadRegistry
	controlSettings *model.ControlSettings

	// Statistics
	recordCount    int
	recordCountMap map[model.HProfTagRecord]int
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
		file:           file,
		reader:         NewBinaryReader(file),
		outputFile:     outputFile,
		stringReg:      registry.NewStringRegistry(),
		classReg:       registry.NewClassRegistry(),
		stackReg:       registry.NewStackRegistry(),
		threadReg:      registry.NewThreadRegistry(),
		recordCountMap: make(map[model.HProfTagRecord]int),
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

// parseHeader parses the HPROF file header
func (p *Parser) parseHeader() error {
	p.debugf("--- Parsing Header ---\n")
	p.debugf("Byte offset: %d\n", p.reader.BytesRead())

	header, err := ParseHeader(p.reader)
	if err != nil {
		return err
	}

	p.header = header

	p.debugf("Format: %s\n", header.Format)
	p.debugf("Identifier size: %d bytes\n", header.IdentifierSize)
	p.debugf("Raw timestamp: %d\n", header.Timestamp.UnixMilli())
	p.debugf("Timestamp: %s\n", header.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	p.debugf("Header bytes read: %d\n", p.reader.BytesRead())
	p.debugf("Header parsed successfully!\n\n")

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

	case model.HPROF_START_THREAD:
		return p.parseStartThreadRecord()

	case model.HPROF_END_THREAD:
		return p.parseEndThreadRecord()

	case model.HPROF_CONTROL_SETTINGS:
		return p.parseControlSettingsRecord()

	default:
		// For all other record types, skip the data for now
		return p.skipRecordData(record.Length)
	}
}

// parseUTF8Record parses a HPROF_UTF8 record
func (p *Parser) parseUTF8Record(length uint32) error {
	utf8Body, err := ParseUTF8(p.reader, length, p.stringReg)
	if err != nil {
		return fmt.Errorf("failed to parse UTF8 record: %w", err)
	}

	p.debugf("  String ID: 0x%x\n", uint64(utf8Body.StringID))
	p.debugf("  Text: \"%s\" (%d chars)\n", utf8Body.Text, len(utf8Body.Text))

	return nil
}

// parseLoadClassRecord parses a HPROF_LOAD_CLASS record
func (p *Parser) parseLoadClassRecord() error {
	loadClassBody, err := ParseLoadClass(p.reader, p.stringReg, p.classReg)
	if err != nil {
		return fmt.Errorf("failed to parse LOAD_CLASS record: %w", err)
	}

	className := p.stringReg.GetOrUnresolved(loadClassBody.ClassNameID)
	p.debugf("  Class Serial: %d\n", loadClassBody.ClassSerialNumber)
	p.debugf("  Object ID: 0x%x\n", uint64(loadClassBody.ObjectID))
	p.debugf("  Stack Trace Serial: %d\n", loadClassBody.StackTraceSerialNumber)
	p.debugf("  Class Name: \"%s\"\n", className)

	return nil
}

// parseUnloadClassRecord parses an UNLOAD_CLASS record
func (p *Parser) parseUnloadClassRecord() error {
	unloadClassBody, err := ParseUnloadClass(p.reader, p.classReg)
	if err != nil {
		return fmt.Errorf("failed to parse UNLOAD_CLASS record: %w", err)
	}

	p.debugf("  Class Serial: %d (unloaded)\n", unloadClassBody.ClassSerialNumber)

	return nil
}

func (p *Parser) parseFrameRecord() error {
	frameBody, err := ParseStackFrame(p.reader, p.stackReg)
	if err != nil {
		return fmt.Errorf("failed to parse FRAME record: %w", err)
	}

	p.debugf("  Frame ID: 0x%x\n", uint64(frameBody.StackFrameID))
	p.debugf("  Method: %s\n", p.stringReg.GetOrUnresolved(frameBody.MethodNameID))
	p.debugf("  Signature: %s\n", p.stringReg.GetOrUnresolved(frameBody.MethodSignatureID))
	p.debugf("  Source: %s\n", p.stringReg.GetOrUnresolved(frameBody.SourceFileNameID))
	p.debugf("  Class Serial: %d\n", frameBody.ClassSerialNumber)
	p.debugf("  Line: %d\n", frameBody.LineNumber)

	return nil
}

func (p *Parser) parseTraceRecord() error {
	traceBody, err := ParseStackTrace(p.reader, p.stackReg)
	if err != nil {
		return fmt.Errorf("failed to parse TRACE record: %w", err)
	}

	p.debugf("  Trace Serial: %d\n", traceBody.StackTraceSerialNumber)
	p.debugf("  Thread Serial: %d\n", traceBody.ThreadSerialNumber)
	p.debugf("  Frame Count: %d\n", traceBody.NumFrames)
	p.debugf("  Frame IDs: ")
	for i, frameID := range traceBody.StackFrameIDs {
		if i > 0 {
			p.debugf(", ")
		}
		p.debugf("0x%x", uint64(frameID))
	}
	p.debugf("\n")

	return nil
}

func (p *Parser) parseStartThreadRecord() error {
	startThread, err := ParseStartThread(p.reader, p.threadReg, p.stringReg)
	if err != nil {
		return fmt.Errorf("failed to parse START_THREAD record: %w", err)
	}

	p.debugf("  Thread serial: %d\n", startThread.ThreadSerialNumber)
	p.debugf("  Thread object ID: 0x%x\n", uint64(startThread.ThreadObjectID))
	p.debugf("  Stack trace serial: %d\n", startThread.StackTraceSerialNumber)
	p.debugf("  Thread name: %s\n", p.stringReg.GetOrUnresolved(startThread.ThreadNameID))
	p.debugf("  Thread group: %s\n", p.stringReg.GetOrUnresolved(startThread.ThreadGroupNameID))
	p.debugf("  Parent group: %s\n", p.stringReg.GetOrUnresolved(startThread.ParentThreadGroupNameID))

	return nil
}

func (p *Parser) parseEndThreadRecord() error {
	endThread, err := ParseEndThread(p.reader, p.threadReg)
	if err != nil {
		return fmt.Errorf("failed to parse END_THREAD record: %w", err)
	}

	p.debugf("  Thread serial: %d\n", endThread.ThreadSerialNumber)

	return nil
}

func (p *Parser) parseControlSettingsRecord() error {
	controlSettings, err := ParseControlSettings(p.reader)
	if err != nil {
		return fmt.Errorf("failed to parse CONTROL_SETTINGS record: %w", err)
	}

	p.controlSettings = controlSettings

	p.debugf("  Flags: 0x%08x\n", controlSettings.Flags)
	p.debugf("  Allocation traces: %t\n", controlSettings.IsAllocTracesEnabled())
	p.debugf("  CPU sampling: %t\n", controlSettings.IsCPUSamplingEnabled())
	p.debugf("  Stack trace depth: %d\n", controlSettings.StackTraceDepth)

	return nil
}

// skipRecordData skips record data that we don't parse yet
func (p *Parser) skipRecordData(length uint32) error {
	err := p.reader.Skip(int(length))
	if err != nil {
		return fmt.Errorf("failed to skip %d bytes: %w", length, err)
	}
	p.debugf("  Skipped %d bytes of record data\n", length)
	return nil
}

// parseRecords parses all records in the file
func (p *Parser) parseRecords() error {
	p.debugf("--- Parsing Records ---\n")

	for {
		cursor := p.reader.BytesRead()

		record, err := p.reader.ReadRecordHeader()
		if err == io.EOF {
			p.debugf("Reached EOF. Parsed %d records.\n", p.recordCount)
			break
		}
		if err != nil {
			return fmt.Errorf("cursor %d - failed to read record header: %w", p.recordCount, err)
		}

		p.recordCount++
		p.recordCountMap[record.Type]++

		p.debugf("Record #%d at offset %d:\n", p.recordCount, cursor)
		p.debugf("  Type: %s (0x%02x)\n", record.Type, record.Type)
		p.debugf("  Time offset: %d ms\n", record.TimeOffset)
		p.debugf("  Length: %d bytes\n", record.Length)

		newCursorExpected := cursor + 9 + int64(record.Length)

		p.parseRecord(record)

		if p.reader.BytesRead() != newCursorExpected {
			return fmt.Errorf(
				"position mismatch after %s record: expected %d, got %d",
				record.Type, newCursorExpected, p.reader.BytesRead())
		}

		p.debugf("  Processed successfully, now at offset %d\n\n", p.reader.BytesRead())
	}

	return nil
}

// printSummary prints a summary of parsing results
func (p *Parser) printSummary() {
	p.debugf("--- Record Summary ---\n")
	p.debugf("Total records: %d\n", p.recordCount)
	p.debugf("Record type breakdown:\n")
	for recordType, count := range p.recordCountMap {
		p.debugf("  %s: %d\n", recordType, count)
	}
	p.debugf("Total bytes processed: %d\n", p.reader.BytesRead())

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

	p.debugf("\nSample frames from registry:\n")
	frameSampleCount := 0
	maxFrameSamples := 5
	for frameID, frame := range p.stackReg.GetAllFrames() {
		if frameSampleCount >= maxFrameSamples {
			break
		}

		methodName := p.stringReg.GetOrUnresolved(frame.MethodNameID)
		sourceFile := p.stringReg.GetOrUnresolved(frame.SourceFileNameID)

		p.debugf("  Frame 0x%x: %s (%s:%d)\n",
			uint64(frameID), methodName, sourceFile, frame.LineNumber)
		frameSampleCount++
	}

	p.debugf("\nSample traces from registry:\n")
	traceSampleCount := 0
	maxTraceSamples := 5
	for frameID, frame := range p.stackReg.GetAllFrames() {
		if traceSampleCount >= maxTraceSamples {
			break
		}

		methodName := p.stringReg.GetOrUnresolved(frame.MethodNameID)
		sourceFile := p.stringReg.GetOrUnresolved(frame.SourceFileNameID)

		p.debugf("  Frame 0x%x: %s (%s:%d)\n",
			uint64(frameID), methodName, sourceFile, frame.LineNumber)
		traceSampleCount++
	}

	p.debugf("\nSample threads:\n")
	sampleCount := 0
	maxSamples := 5
	for _, threadInfo := range p.threadReg.GetAllThreads() {
		if sampleCount >= maxSamples {
			break
		}
		status := "active"
		if !threadInfo.IsActive {
			status = "ended"
		}
		p.debugf("  Thread %d: \"%s\" (%s)\n", threadInfo.StartRecord.ThreadSerialNumber, threadInfo.ThreadName, status)
		if threadInfo.ThreadGroupName != "" && threadInfo.ThreadGroupName != threadInfo.ThreadName {
			p.debugf("    Group: \"%s\"\n", threadInfo.ThreadGroupName)
		}
		sampleCount++
	}

	if p.controlSettings != nil {
		p.debugf("\n--- Control Settings ---\n")
		p.debugf("Flags: 0x%08x\n", p.controlSettings.Flags)
		p.debugf("Allocation traces enabled: %t\n", p.controlSettings.IsAllocTracesEnabled())
		p.debugf("CPU sampling enabled: %t\n", p.controlSettings.IsCPUSamplingEnabled())
		p.debugf("Stack trace depth: %d\n", p.controlSettings.StackTraceDepth)
	}
}

// ParseHprof parses an HPROF file completely
// Each HPROF file follows this structure:
// [Header]
// [Record 1]
// [Record 2]
// ...
// [Record N]
func (p *Parser) ParseHprof() error {
	defer p.Close()

	p.debugf("🔍 Starting HPROF analysis of: %s\n", p.file.Name())

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

// GetHeader returns the parsed header
func (p *Parser) GetHeader() *model.HprofHeader {
	return p.header
}

// GetStringRegistry returns the string registry
func (p *Parser) GetStringRegistry() *registry.StringRegistry {
	return p.stringReg
}
