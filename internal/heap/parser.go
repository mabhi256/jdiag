package heap

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*
*	HProf binary format described here
*	https://github.com/openjdk/jdk/blob/master/src/hotspot/share/services/heapDumper.cpp
 */

type HProfTagRecord byte

const (
	// top-level records
	HPROF_UTF8             HProfTagRecord = 0x01
	HPROF_LOAD_CLASS                      = 0x02
	HPROF_UNLOAD_CLASS                    = 0x03
	HPROF_FRAME                           = 0x04
	HPROF_TRACE                           = 0x05
	HPROF_ALLOC_SITES                     = 0x06
	HPROF_HEAP_SUMMARY                    = 0x07
	HPROF_START_THREAD                    = 0x0A
	HPROF_END_THREAD                      = 0x0B
	HPROF_HEAP_DUMP                       = 0x0C
	HPROF_CPU_SAMPLES                     = 0x0D
	HPROF_CONTROL_SETTINGS                = 0x0E

	// 1.0.2 record types
	HPROF_HEAP_DUMP_SEGMENT = 0x1C
	HPROF_HEAP_DUMP_END     = 0x2C
)

func (h HProfTagRecord) String() string {
	switch h {
	case HPROF_UTF8:
		return "UTF8"
	case HPROF_LOAD_CLASS:
		return "LOAD_CLASS"
	case HPROF_UNLOAD_CLASS:
		return "UNLOAD_CLASS"
	case HPROF_FRAME:
		return "STACK_FRAME"
	case HPROF_TRACE:
		return "STACK_TRACE"
	case HPROF_ALLOC_SITES:
		return "ALLOC_SITES"
	case HPROF_HEAP_SUMMARY:
		return "HEAP_SUMMARY"
	case HPROF_START_THREAD:
		return "START_THREAD"
	case HPROF_END_THREAD:
		return "END_THREAD"
	case HPROF_HEAP_DUMP:
		return "HEAP_DUMP"
	case HPROF_CPU_SAMPLES:
		return "CPU_SAMPLES"
	case HPROF_CONTROL_SETTINGS:
		return "CONTROL_SETTINGS"
	case HPROF_HEAP_DUMP_SEGMENT:
		return "HEAP_DUMP_SEGMENT"
	case HPROF_HEAP_DUMP_END:
		return "HEAP_DUMP_END"
	default:
		return fmt.Sprintf("HProfTagRecord(0x%02X)", byte(h))
	}
}

type HProfTagFieldType byte

//staticcheck:ignore SA9004 Type inference is intentional here
const (
	HPROF_ARRAY_OBJECT  HProfTagFieldType = 0x01
	HPROF_NORMAL_OBJECT                   = 0x02
	HPROF_BOOLEAN                         = 0x04
	HPROF_CHAR                            = 0x05
	HPROF_FLOAT                           = 0x06
	HPROF_DOUBLE                          = 0x07
	HPROF_BYTE                            = 0x08
	HPROF_SHORT                           = 0x09
	HPROF_INT                             = 0x0A
	HPROF_LONG                            = 0x0B
)

type HProfTagSubRecord byte

const (
	HPROF_GC_ROOT_UNKNOWN      HProfTagSubRecord = 0xFF
	HPROF_GC_ROOT_JNI_GLOBAL                     = 0x01
	HPROF_GC_ROOT_JNI_LOCAL                      = 0x02
	HPROF_GC_ROOT_JAVA_FRAME                     = 0x03
	HPROF_GC_ROOT_NATIVE_STACK                   = 0x04
	HPROF_GC_ROOT_STICKY_CLASS                   = 0x05
	HPROF_GC_ROOT_THREAD_BLOCK                   = 0x06
	HPROF_GC_ROOT_MONITOR_USED                   = 0x07
	HPROF_GC_ROOT_THREAD_OBJ                     = 0x08
	HPROF_GC_CLASS_DUMP                          = 0x20
	HPROF_GC_INSTANCE_DUMP                       = 0x21
	HPROF_GC_OBJ_ARRAY_DUMP                      = 0x22
	HPROF_GC_PRIM_ARRAY_DUMP                     = 0x23
)

type HprofHeader struct {
	Format         string    // Typically "JAVA PROFILE 1.0.2"
	IdentifierSize uint32    // u4 size of object IDs
	Timestamp      time.Time // u4 + u4, stored as uint64, milliseconds since 0:00 GMT, 1/1/70
}

type HprofRecord struct {
	Type       HProfTagRecord // u1 tag
	TimeOffset uint32         // u4 - microseconds since header timestamp
	Length     uint32         // u4 bytes remaining (excludes tag+length)
	Data       []byte         // variable length body
}

type ID uint64        // Represent memory address, 4 or 8 bytes depending on platform
type SerialNum uint32 // u4, just a counter

type UTF8Body struct {
	StringID ID
	Text     string
}

type LoadClassBody struct {
	ClassSerialNumber      SerialNum // u4
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	ClassNameID            ID // It is a pointer to a UTF8 string
}

type UnloadClassBody struct {
	ClassSerialNumber SerialNum
}

type FrameBody struct {
	StackFrameID      ID
	MethodNameID      ID // References UTF8
	MethodSignatureID ID // References UTF8
	SourceFileNameID  ID // References UTF8
	ClassSerialNumber SerialNum
	LineNumber        int32 // signed int because it can be negative
	// >0: normal line, -1: unknown, -2: compiled method, -3: native method
}

type TraceBody struct {
	StackTraceSerialNumber SerialNum
	ThreadSerialNumber     SerialNum // Thread that produced this trace
	NumFrames              uint32
	StackFrameIDs          []ID
}

type AllocSite struct {
	IsArray uint8 // 0: normal object, 2: object array, 4: boolean array, 5: char array,
	// 6: float array, 7: double array, 8: byte array, 9: short array, 10 int array, 11: long array
	ClassSerialNumber      SerialNum
	StackTraceSerialNumber SerialNum
	BytesAlive             uint32
	InstancesAlive         uint32
	BytesAlloc             uint32
	InstancesAlloc         uint32
}

type AllocSiteGroup struct {
	Flags               uint16
	CutoffRatio         uint32
	TotalLiveBytes      uint32
	TotalLiveInstances  uint32
	TotalBytesAlloc     uint64
	TotalInstancesAlloc uint64
	NumAllocSites       uint32
	Sites               []AllocSite
}

const (
	ALLOC_TYPE = 0x0001 // incremental vs complete
	ALLOC_SORT = 0x0002 // sorted by allocation vs live
	ALLOC_GC   = 0x0004 // force GC
)

// Helper methods for AllocSiteGroup flags
func (asg *AllocSiteGroup) IsIncremental() bool {
	return (asg.Flags & ALLOC_TYPE) != 0
}

func (asg *AllocSiteGroup) IsSortedByAllocation() bool {
	return (asg.Flags & ALLOC_SORT) != 0
}

func (asg *AllocSiteGroup) ForcedGC() bool {
	return (asg.Flags & ALLOC_GC) != 0
}

type StartThreadBody struct {
	ThreadSerialNumber      SerialNum
	ThreadObjectID          ID
	StackTraceSerialNumber  SerialNum
	ThreadNameID            ID // References UTF8
	ThreadGroupNameID       ID
	ParentThreadGroupNameID ID
}

type EndThreadBody struct {
	ThreadSerialNumber SerialNum
}

type HeapSummary struct {
	LiveBytes      uint32
	LiveInstances  uint32
	BytesAlloc     uint64
	InstancesAlloc uint64
}

type GCRootUnkown struct {
	ObjectID ID
}

type GCRootThreadObject struct {
	ThreadObjectID         ID
	ThreadSerialNumber     SerialNum
	StackTraceSerialNumber SerialNum
}

type GCRootJniGlobal struct {
	ObjectID       ID
	JniGlobalRefID ID
}

const EmptyFrame int32 = -1

type GCRootJniLocal struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
	FrameNumber        SerialNum
}

type GCRootJavaFrame struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
	FrameNumber        SerialNum
}

type GCRootNativeStack struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
}

type GCRootStickyClass struct {
	ObjectID ID
}

type GCRootThreadBlock struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
}

type GCRootMonitorUsed struct {
	ObjectID ID
}

type ClassDump struct {
	ClassObjectID            ID
	StackTraceSerialNumber   SerialNum
	SuperClassObjectID       ID
	ClassLoaderObjectID      ID
	SignerObjectID           ID
	ProtectionDomainObjectID ID
	Reserved1                ID
	Reserved2                ID
	InstanceSize             uint32
	ConstantPoolSize         uint16
	ConstantPool             []*ConstantPoolEntry
	StaticFieldsCount        uint16
	StaticFields             []*StaticField
	InstanceFieldsCount      uint16
	InstanceFields           []*InstanceField
}

type ConstantPoolEntry struct {
	Index uint16
	Type  HProfTagFieldType
	Value []byte
}

type StaticField struct {
	NameID ID
	Type   HProfTagFieldType
	Value  []byte
}

type InstanceField struct {
	NameID ID
	Type   HProfTagFieldType
}

type FieldInfo struct {
	NameStringID uint64
	Name         string
	Type         uint8
	Value        interface{}
}

type GCInstanceDump struct {
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	ClassObjectID          ID
	Size                   uint32
	InstanceData           []byte
}

type GCObjectArrayDump struct {
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	Size                   uint32
	ClassID                ID
	Elements               []ID
}

type GCPrimitiveArrayDump struct {
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	Size                   uint32
	Type                   HProfTagFieldType // Other than HPROF_ARRAY_OBJECT and HPROF_NORMAL_OBJECT
	Elements               []byte
}

type CPUSample struct {
	TotalSamples uint32         // Total number of samples
	NumTraces    uint32         // Number of traces that follow
	Traces       []CPUTraceInfo // Array of trace information
}

type CPUTraceInfo struct {
	NumSamples             uint32
	StackTraceSerialNumber uint32 // References a HPROF_TRACE record
}

type ControlSettings struct {
	Flags           uint32 // Bit flags for various settings
	StackTraceDepth uint16
}

const (
	CONTROL_ALLOC_TRACES = 0x00000001 // Allocation traces on/off
	CONTROL_CPU_SAMPLING = 0x00000002 // CPU sampling on/off
)

// Helper methods for ControlSettings
func (cs *ControlSettings) IsAllocTracesEnabled() bool {
	return (cs.Flags & CONTROL_ALLOC_TRACES) != 0
}

func (cs *ControlSettings) IsCPUSamplingEnabled() bool {
	return (cs.Flags & CONTROL_CPU_SAMPLING) != 0
}

// Helper method to validate ID size handling
func (h *HprofHeader) ReadID(data []byte, offset int) (ID, int) {
	if h.IdentifierSize == 4 {
		return ID(binary.BigEndian.Uint32(data[offset:])), offset + 4
	} else {
		return ID(binary.BigEndian.Uint64(data[offset:])), offset + 8
	}
}

// Validation that the field type sizes are correct
func (ft HProfTagFieldType) Size(identifierSize uint32) int {
	switch ft {
	case HPROF_BOOLEAN, HPROF_BYTE:
		return 1
	case HPROF_CHAR, HPROF_SHORT:
		return 2
	case HPROF_INT, HPROF_FLOAT:
		return 4
	case HPROF_LONG, HPROF_DOUBLE:
		return 8
	case HPROF_NORMAL_OBJECT, HPROF_ARRAY_OBJECT:
		return int(identifierSize)
	default:
		return 0
	}
}

type Parser struct {
	file       *os.File
	reader     *bufio.Reader
	bytesRead  int64
	outputFile *os.File // For debugging output

	header *HprofHeader
}

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
		return nil, fmt.Errorf("unable to create file: %w", err)
	}

	parser := &Parser{
		file:       file,
		reader:     bufio.NewReader(file),
		outputFile: outputFile,
	}

	return parser, nil
}

func (p *Parser) Close() error {
	var err error
	if p.file != nil {
		err = p.file.Close()
	}

	if p.outputFile != nil {
		err = p.outputFile.Close()
	}

	return err
}

// Write to debug file
func (p *Parser) debugf(format string, args ...any) {
	fmt.Fprintf(p.outputFile, format, args...)
}

func (p *Parser) ReadNBytes(n int) ([]byte, error) {
	buf := make([]byte, n)

	bytesRead, err := io.ReadFull(p.reader, buf)
	if err != nil {
		return nil, err
	}

	p.bytesRead += int64(bytesRead)

	return buf, nil
}

// Read a null-terminated string
func (p *Parser) readString() (string, error) {
	str, err := p.reader.ReadString('\x00') // null character
	if err != nil {
		return "", err
	}

	p.bytesRead += int64(len(str))

	// Remove the null terminator from the end
	return str[:len(str)-1], nil
}

// Read a single unsigned byte integer
func (p *Parser) readU1() (uint8, error) {
	b, err := p.reader.ReadByte()
	if err != nil {
		return 0, err
	}

	p.bytesRead++

	return b, nil
}

// Read 2-byte unsigned integer (big-endian), because JVM is always big-endian
func (p *Parser) readU2() (uint16, error) {
	buf, err := p.ReadNBytes(2)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(buf), nil
}

// Read 4-byte unsigned integer (big-endian)
func (p *Parser) readU4() (uint32, error) {
	buf, err := p.ReadNBytes(4)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buf), nil
}

// Read 8-byte unsigned integer (big-endian)
func (p *Parser) readU8() (uint64, error) {
	buf, err := p.ReadNBytes(8)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(buf), nil
}

// Read an object ID (size depends on header.IdentifierSize)
func (p *Parser) readID() (ID, error) {
	if p.header == nil {
		return 0, fmt.Errorf("header not parsed yet")
	}

	switch p.header.IdentifierSize {
	case 4:
		val, err := p.readU4()
		return ID(val), err
	case 8:
		val, err := p.readU8()
		return ID(val), err
	default:
		return 0, fmt.Errorf("invalid identifier size: %d", p.header.IdentifierSize)
	}
}

/*
Header has the format

"JAVA PROFILE 1.0.2\0"      Null-terminated string
u4                          Size of IDs (usually pointer size)
u4                          High word of timestamp
u4                          Low word of timestamp (ms since 1/1/70)
*/
func (p *Parser) parseHeader() error {
	p.debugf("--- Parsing Header ---")
	p.debugf("Byte offset: %d\n", p.bytesRead)

	// Read magic string
	hprofFormat, err := p.readString()
	if err != nil {
		return fmt.Errorf("unable to read format: %w", err)
	}
	if hprofFormat != "JAVA PROFILE 1.0.2" {
		return fmt.Errorf("invalid format: %s", hprofFormat)
	}
	p.debugf("Format: %s\n", hprofFormat)

	// Read identifier size
	identifierSize, err := p.readU4()
	if err != nil {
		return fmt.Errorf("failed to read identifier size: %w", err)
	}
	if identifierSize != 4 && identifierSize != 8 {
		return fmt.Errorf("invalid identifierSize: %d", identifierSize)
	}
	p.debugf("Identifier size: %d bytes\n", identifierSize)

	// Read high & low word of timestamp
	tsHigh, err := p.readU4()
	if err != nil {
		return fmt.Errorf("failed to read timestamp high word: %w", err)
	}

	tsLow, err := p.readU4()
	if err != nil {
		return fmt.Errorf("failed to read timestamp low word: %w", err)
	}

	// Combine high & low word of timestamp and convert to Unix epoch ms
	tsMilli := (uint64(tsHigh) << 32) | uint64(tsLow)
	timestampTime := time.UnixMilli(int64(tsMilli))

	p.debugf("Raw timestamp: %d (high: %d, low: %d)\n", tsMilli, tsHigh, tsLow)
	p.debugf("Timestamp: %s\n", timestampTime.Format(time.RFC3339))

	// Create header
	p.header = &HprofHeader{
		Format:         hprofFormat,
		IdentifierSize: identifierSize,
		Timestamp:      timestampTime,
	}

	p.debugf("Header bytes read: %d\n", p.bytesRead)
	p.debugf("Header parsed successfully!\n\n")

	return nil
}

/*
A record has this structure:

u1      	a TAG denoting record type
u4      	Microseconds since header timestamp
u4      	Bytes remaining in record (excluding tag + ts + length)
[body]  	Record-specific data
*/
func (p *Parser) parseRecordHeader() (*HprofRecord, error) {
	// Read record type tag
	recordType, err := p.readU1()
	if err == io.EOF {
		return nil, err // Normal end of file
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read record type: %w", err)
	}

	// Read time offset
	offset, err := p.readU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read header timestamp offset: %w", err)
	}

	// Read remaining byte size (excluding tag + ts + length)
	length, err := p.readU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read header timestamp offset: %w", err)
	}

	record := HprofRecord{
		Type:       HProfTagRecord(recordType),
		TimeOffset: offset,
		Length:     length,
		Data:       nil,
	}

	return &record, nil
}

func (p *Parser) skipRecordData(length uint32) error {
	// For now, we'll just skip the data
	data, err := p.ReadNBytes(int(length))
	if err != nil {
		return fmt.Errorf("failed to skip %d bytes: %w", length, err)
	}
	p.debugf("  Skipped %d bytes of record data\n", len(data))
	return nil
}

func (p *Parser) parseRecords() error {
	p.debugf("--- Parsing Records ---\n")

	recordCount := 0
	recordCountMap := make(map[HProfTagRecord]int)

	for {
		cursor := p.bytesRead

		record, err := p.parseRecordHeader()
		if err == io.EOF {
			p.debugf("Reached EOF. Parsed %d records.\n", recordCount)
			break
		}
		if err != nil {
			return fmt.Errorf("cursor %d - failed to read record header: %w", recordCount, err)
		}

		recordCount++
		recordCountMap[record.Type]++

		p.debugf("Record #%d at offset %d:\n", recordCount, cursor)
		p.debugf("  Type: %s (0x%02x)\n", record.Type, record.Type)
		p.debugf("  Time offset: %d ms\n", record.TimeOffset)
		p.debugf("  Length: %d bytes\n", record.Length)

		newCursorExpected := cursor + 9 + int64(record.Length)
		p.skipRecordData(record.Length)

		if p.bytesRead != newCursorExpected {
			return fmt.Errorf(
				"position mismatch after %s record: expected %d, got %d",
				record.Type, newCursorExpected, p.bytesRead)
		}

		p.debugf("  Processed successfully, now at offset %d\n\n", p.bytesRead)
	}

	// Print summary
	p.debugf("--- Record Summary ---\n")
	p.debugf("Total records: %d\n", recordCount)
	p.debugf("Record type breakdown:\n")
	for recordType, count := range recordCountMap {
		p.debugf("  %s: %d\n", recordType, count)
	}
	p.debugf("Total bytes processed: %d\n", p.bytesRead)

	return nil
}

/*
ParseHprof parses an hprof file. Each hprof file follows this structure:

[Header]
[Record 1]
[Record 2]
...
[Record N]
*/
func (p *Parser) ParseHprof() error {
	defer p.Close()

	p.debugf("🔍 Starting HPROF analysis of: %s\n", p.file.Name())

	if err := p.parseHeader(); err != nil {
		return err
	}

	if err := p.parseRecords(); err != nil {
		return err
	}

	p.debugf("--- PARSING COMPLETE ---\n")

	return nil
}
