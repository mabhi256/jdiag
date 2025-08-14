package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
*	ParseStackFame parses a HPROF_FRAME record:
*
*	id      stack frame ID
*	id      Method name ID (UTF8 reference)
*	id      Method signature ID (UTF8 reference)
*	id      Source file name ID (UTF8 reference)
*	u4      Class serial number
*	i4      Line number. 	>0: normal line
*							-1: unknown
*							-2: compiled method
*							-3: native method
 */
func ParseStackFrame(reader *BinaryReader, length uint32,
	stackReg *registry.StackRegistry,
) (*model.FrameBody, error) {
	stackFrameID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read stack frame ID: %w", err)
	}

	methodNameID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read method name ID: %w", err)
	}

	methodSignatureID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read method signature ID: %w", err)
	}

	sourceFileNameID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read source file name ID: %w", err)
	}

	classSerialNumber, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read class serial number: %w", err)
	}

	lineNumber, err := reader.ReadI4()
	if err != nil {
		return nil, fmt.Errorf("failed to read line number: %w", err)
	}

	frameBody := &model.FrameBody{
		StackFrameID:      stackFrameID,
		MethodNameID:      methodNameID,
		MethodSignatureID: methodSignatureID,
		SourceFileNameID:  sourceFileNameID,
		ClassSerialNumber: model.SerialNum(classSerialNumber),
		LineNumber:        lineNumber,
	}

	// Add frame to registry
	stackReg.AddFrame(frameBody)

	return frameBody, nil
}

/*
ParseStackTrace parses a HPROF_TRACE record:

u4          Stack trace serial number
u4          Thread serial number that produced this trace
u4          Number of frames
[id]*       Array of stack frame IDs (references HPROF_FRAME records)
*/
func ParseStackTrace(reader *BinaryReader, length uint32,
	stackReg *registry.StackRegistry,
) (*model.TraceBody, error) {
	stackTraceSerialNumber, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read stack trace serial number: %w", err)
	}

	threadSerialNumber, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read thread serial number: %w", err)
	}

	numFrames, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read number of frames: %w", err)
	}

	stackFrameIDs := make([]model.ID, numFrames)
	for i := uint32(0); i < numFrames; i++ {
		frameID, err := reader.ReadID()
		if err != nil {
			return nil, fmt.Errorf("failed to read frame ID %d: %w", i, err)
		}
		stackFrameIDs[i] = frameID
	}

	traceBody := &model.TraceBody{
		StackTraceSerialNumber: model.SerialNum(stackTraceSerialNumber),
		ThreadSerialNumber:     model.SerialNum(threadSerialNumber),
		NumFrames:              numFrames,
		StackFrameIDs:          stackFrameIDs,
	}

	// Add trace to registry
	stackReg.AddTrace(traceBody)

	return traceBody, nil
}
