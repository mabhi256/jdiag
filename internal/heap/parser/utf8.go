package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
ParseUTF8 parses a HPROF_UTF8 record

id   		ID for this string
[u1]*		UTF-8 characters (no null terminator)
*/
func ParseUTF8(reader *BinaryReader, length uint32,
	stringReg *registry.StringRegistry,
) (*model.UTF8Body, error) {
	stringID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read string ID: %w", err)
	}

	// Calculate remaining bytes for the actual string
	stringLength := int(length) - int(reader.Header().IdentifierSize)
	if stringLength < 0 {
		return nil, fmt.Errorf("invalid string length: %d", stringLength)
	}

	// Read the string bytes (UTF-8 encoded, no null terminator)
	text, err := reader.ReadUtf8String(stringLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read string data: %w", err)
	}

	utf8Body := &model.UTF8Body{
		StringID: stringID,
		Text:     text,
	}

	// Add string to registry
	stringReg.Add(stringID, text)

	return utf8Body, nil
}
