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

	header    *model.HprofHeader
	stringReg *registry.StringRegistry

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
		utf8Body, err := ParseUTF8(p.reader, record.Length, p.stringReg)
		if err != nil {
			return fmt.Errorf("failed to parse UTF8 record: %w", err)
		}
		p.debugf("  String ID: 0x%x\n", uint64(utf8Body.StringID))
		p.debugf("  Text: \"%s\" (%d chars)\n", utf8Body.Text, len(utf8Body.Text))

	default:
		// For all other record types, skip the data for now
		err := p.skipRecordData(record.Length)
		if err != nil {
			return fmt.Errorf("failed to skip record data: %w", err)
		}
	}

	return nil
}

// parseUTF8Record parses a UTF8 record
func (p *Parser) parseUTF8Record(length uint32) error {
	utf8Body, err := ParseUTF8(p.reader, length, p.stringReg)
	if err != nil {
		return fmt.Errorf("failed to parse UTF8 record: %w", err)
	}

	p.debugf("  String ID: 0x%x\n", uint64(utf8Body.StringID))
	p.debugf("  Text: \"%s\" (%d chars)\n", utf8Body.Text, len(utf8Body.Text))

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
	sampleCount := 0
	maxSamples := 10
	for id, text := range p.stringReg.GetAll() {
		if sampleCount >= maxSamples {
			break
		}
		if len(text) > 50 {
			p.debugf("  0x%x: \"%.50s...\" (%d chars)\n", uint64(id), text, len(text))
		} else {
			p.debugf("  0x%x: \"%s\"\n", uint64(id), text)
		}
		sampleCount++
	}

	p.debugf("Total strings in table: %d\n", p.stringReg.Count())
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
