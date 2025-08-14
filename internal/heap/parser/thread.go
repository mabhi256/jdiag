package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
* ParseStartThread parses a HPROF_START_THREAD record
*
* 	u4		thread serial number (> 0)
* 	id		thread object ID
* 	u4		stack trace serial number
* 	id		thread name ID (references UTF8)
* 	id		thread group name ID (references UTF8)
* 	id		thread group parent name ID (references UTF8)
 */
func ParseStartThread(reader *BinaryReader,
	threadReg *registry.ThreadRegistry, stringReg *registry.StringRegistry,
) (*model.StartThreadBody, error) {
	// Read thread serial number
	threadSerialNumber, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read thread serial number: %w", err)
	}

	// Read thread object ID
	threadObjectID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read thread object ID: %w", err)
	}

	// Read stack trace serial number
	stackTraceSerialNumber, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read stack trace serial number: %w", err)
	}

	// Read thread name ID
	threadNameID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read thread name ID: %w", err)
	}

	// Read thread group name ID
	threadGroupNameID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read thread group name ID: %w", err)
	}

	// Read parent thread group name ID
	parentThreadGroupNameID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read parent thread group name ID: %w", err)
	}

	startThreadBody := &model.StartThreadBody{
		ThreadSerialNumber:      model.SerialNum(threadSerialNumber),
		ThreadObjectID:          threadObjectID,
		StackTraceSerialNumber:  model.SerialNum(stackTraceSerialNumber),
		ThreadNameID:            threadNameID,
		ThreadGroupNameID:       threadGroupNameID,
		ParentThreadGroupNameID: parentThreadGroupNameID,
	}

	// Add to thread registry
	threadReg.StartThread(startThreadBody, stringReg)

	return startThreadBody, nil
}

/*
* ParseEndThread parses a HPROF_END_THREAD record
*
* u4    - thread serial number
 */
func ParseEndThread(reader *BinaryReader,
	threadReg *registry.ThreadRegistry,
) (*model.EndThreadBody, error) {
	// Read thread serial number
	threadSerialNumber, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read thread serial number: %w", err)
	}

	endThreadBody := &model.EndThreadBody{
		ThreadSerialNumber: model.SerialNum(threadSerialNumber),
	}

	// Add to thread registry
	threadReg.EndThread(endThreadBody)

	return endThreadBody, nil
}
