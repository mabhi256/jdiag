package parser

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// Provides utilities for reading binary data in big-endian format
type BinaryReader struct {
	reader    *bufio.Reader
	bytesRead int64
	header    *model.HprofHeader
}

func NewBinaryReader(reader io.Reader) *BinaryReader {
	return &BinaryReader{
		reader: bufio.NewReader(reader),
	}
}

func (br *BinaryReader) BytesRead() int64 {
	return br.bytesRead
}

// may be nil if not yet parsed
func (br *BinaryReader) Header() *model.HprofHeader {
	return br.header
}

func (br *BinaryReader) SetHeader(header *model.HprofHeader) {
	br.header = header
}

// ReadNBytes reads exactly n bytes and tracks position
func (br *BinaryReader) ReadNBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	bytesRead, err := io.ReadFull(br.reader, buf)
	if err != nil {
		return nil, err
	}
	br.bytesRead += int64(bytesRead)
	return buf, nil
}

// ReadString reads a null-terminated string
func (br *BinaryReader) ReadString() (string, error) {
	str, err := br.reader.ReadString('\x00') // null character
	if err != nil {
		return "", err
	}
	br.bytesRead += int64(len(str))
	// Remove the null terminator from the end
	return str[:len(str)-1], nil
}

// ReadU1 reads a single unsigned byte
func (br *BinaryReader) ReadU1() (uint8, error) {
	b, err := br.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	br.bytesRead++
	return b, nil
}

// ReadU2 reads a 2-byte unsigned integer (big-endian)
func (br *BinaryReader) ReadU2() (uint16, error) {
	buf, err := br.ReadNBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf), nil
}

// ReadU4 reads a 4-byte unsigned integer (big-endian)
func (br *BinaryReader) ReadU4() (uint32, error) {
	buf, err := br.ReadNBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf), nil
}

// ReadU8 reads an 8-byte unsigned integer (big-endian)
func (br *BinaryReader) ReadU8() (uint64, error) {
	buf, err := br.ReadNBytes(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(buf), nil
}

// ReadI4 reads a 4-byte signed integer (big-endian)
func (br *BinaryReader) ReadI4() (int32, error) {
	buf, err := br.ReadNBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf)), nil
}

// ReadID reads an object ID (size depends on header.IdentifierSize)
func (br *BinaryReader) ReadID() (model.ID, error) {
	if br.header == nil {
		return 0, fmt.Errorf("header not parsed yet")
	}

	switch br.header.IdentifierSize {
	case 4:
		val, err := br.ReadU4()
		return model.ID(val), err
	case 8:
		val, err := br.ReadU8()
		return model.ID(val), err
	default:
		return 0, fmt.Errorf("invalid identifier size: %d", br.header.IdentifierSize)
	}
}

// Skip skips n bytes in the stream
func (br *BinaryReader) Skip(n int) error {
	_, err := br.ReadNBytes(n) // Just discard the data
	if err != nil {
		return fmt.Errorf("failed to skip %d bytes: %w", n, err)
	}
	return nil
}

// ReadRecordHeader reads the header of an HPROF record
func (br *BinaryReader) ReadRecordHeader() (*model.HprofRecord, error) {
	// Read record type tag
	recordType, err := br.ReadU1()
	if err == io.EOF {
		return nil, err // Normal end of file
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read record type: %w", err)
	}

	// Read time offset
	offset, err := br.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read time offset: %w", err)
	}

	// Read remaining byte size (excluding tag + ts + length)
	length, err := br.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read record length: %w", err)
	}

	return &model.HprofRecord{
		Type:       model.HProfTagRecord(recordType),
		TimeOffset: offset,
		Length:     length,
		Data:       nil, // Will be read by specific parsers
	}, nil
}

// ReadUtf8String reads a UTF-8 string of specified length (no null terminator)
func (br *BinaryReader) ReadUtf8String(length int) (string, error) {
	if length < 0 {
		return "", fmt.Errorf("invalid string length: %d", length)
	}

	if length == 0 {
		return "", nil
	}

	stringBytes, err := br.ReadNBytes(length)
	if err != nil {
		return "", fmt.Errorf("failed to read string data: %w", err)
	}

	return string(stringBytes), nil
}

// ReadFieldValue reads a field value based on its type
func (br *BinaryReader) ReadFieldValue(fieldType model.HProfTagFieldType) ([]byte, error) {
	size := fieldType.Size(br.header.IdentifierSize)
	if size == 0 {
		return nil, fmt.Errorf("unknown field type: %d", fieldType)
	}

	return br.ReadNBytes(size)
}
