package parser

import (
	"fmt"
	"time"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

/*
*	ParseHeader parses the HPROF file header
*
*	"JAVA PROFILE 1.0.2\0"		Null-terminated string
*	u4                    		Size of IDs (usually pointer size)
*	u4                    		High word of timestamp
*	u4                    		Low word of timestamp (ms since 1/1/70)
 */
func ParseHeader(reader *BinaryReader) (*model.HprofHeader, error) {
	// Read magic string
	hprofFormat, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("unable to read format: %w", err)
	}

	if hprofFormat != "JAVA PROFILE 1.0.2" {
		return nil, fmt.Errorf("invalid format: %s", hprofFormat)
	}

	// Read identifier size
	identifierSize, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read identifier size: %w", err)
	}

	if identifierSize != 4 && identifierSize != 8 {
		return nil, fmt.Errorf("invalid identifierSize: %d", identifierSize)
	}

	// Read high & low word of timestamp
	tsHigh, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read timestamp high word: %w", err)
	}

	tsLow, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read timestamp low word: %w", err)
	}

	// Combine high & low word of timestamp and convert to Unix epoch ms
	tsMilli := (uint64(tsHigh) << 32) | uint64(tsLow)
	timestampTime := time.UnixMilli(int64(tsMilli))

	// Create header
	header := &model.HprofHeader{
		Format:         hprofFormat,
		IdentifierSize: identifierSize,
		Timestamp:      timestampTime,
	}

	// Set the header in the reader so it can be used for ID reading
	reader.SetHeader(header)

	return header, nil
}
