package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

/*
*	ParseControlSettings parses a HPROF_CONTROL_SETTINGS record
*
*	u4      flags   // 0x00000001: allocation tracing on/off
*	                // 0x00000002: CPU sampling on/off
*	u2      Maximum stack trace depth
 */
func ParseControlSettings(reader *BinaryReader, length uint32) (*model.ControlSettings, error) {
	if length != 6 {
		return nil, fmt.Errorf("invalid CONTROL_SETTINGS record length: expected 6, got %d", length)
	}

	// Read flags
	flags, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read control flags: %w", err)
	}

	// Read stack trace depth
	stackTraceDepth, err := reader.ReadU2()
	if err != nil {
		return nil, fmt.Errorf("failed to read stack trace depth: %w", err)
	}

	controlSettings := &model.ControlSettings{
		Flags:           flags,
		StackTraceDepth: stackTraceDepth,
	}

	return controlSettings, nil
}
