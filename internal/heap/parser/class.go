package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
ParseLoadClass parses a HPROF_LOAD_CLASS record:

u4      Unique class serial number
id      Object ID of the Class object
u4      Stack trace serial number when loaded
id      class name ID - reference to UTF8 string
*/
func ParseLoadClass(reader *BinaryReader, length uint32,
	stringReg *registry.StringRegistry, classReg *registry.ClassRegistry,
) (*model.LoadClassBody, error) {
	serialNum, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read class serial number: %w", err)
	}

	objectID, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read class object ID: %w", err)
	}

	stackTraceSerialNum, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read stack trace serial number: %w", err)
	}

	nameId, err := reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read class name ID: %w", err)
	}

	loadClassBody := &model.LoadClassBody{
		ClassSerialNumber:      model.SerialNum(serialNum),
		ObjectID:               objectID,
		StackTraceSerialNumber: model.SerialNum(stackTraceSerialNum),
		ClassNameID:            nameId,
	}

	className := stringReg.GetOrUnresolved(nameId)
	classReg.AddLoadedClass(loadClassBody, className)

	return loadClassBody, nil
}

/*
ParseUnloadClass parses a HPROF_UNLOAD_CLASS record:

u4      Serial number of unloaded class
*/
func ParseUnloadClass(reader *BinaryReader, length uint32,
	classReg *registry.ClassRegistry,
) (*model.UnloadClassBody, error) {
	serialNum, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read class serial number: %w", err)
	}

	unloadClassBody := &model.UnloadClassBody{
		ClassSerialNumber: model.SerialNum(serialNum),
	}

	// Mark the class as unloaded
	classReg.UnloadClass(model.SerialNum(serialNum))

	return unloadClassBody, nil
}
