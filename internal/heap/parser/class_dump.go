package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
* parseClassDump parses a HPROF_GC_CLASS_DUMP sub-record:
*
* GC_CLASS_DUMP contains complete class definition including:
* - Class metadata (superclass, class loader, etc.)
* - Constant pool entries
* - Static field definitions with values
* - Instance field definitions (without values)
*
* This is the core record type that defines class structure in the heap.
*
* Format:
* 	id    						Class object ID
* 	u4    						Stack trace where class was loaded
* 	id    						Superclass object ID (0 for java.lang.Object)
* 	id    						Class loader object ID (0 for bootstrap)
* 	id    						Signers object ID (usually 0)
* 	id    						Protection domain object ID (usually 0)
* 	id    						Reserved field (always 0)
* 	id    						Reserved field (always 0)
* 	u4    						Size of instances of this class in bytes
*
* 	u2							Number of constant pool entries
* 	[constant_pool_entry]*      Constant pool entries
*
* 	u2    				        Number of static fields
* 	[static_field]*             Static field definitions with values
*
* 	u2							Number of instance fields
* 	[instance_field]*           Instance field definitions (no values)
*
* Constant pool entry format:
* 	u2                          Constant pool index
* 	u1                          Value type (HProfTagFieldType)
* 	[value]                     Value data (size depends on type)
*
* Static field format:
* 	id                         	Field name string ID
* 	u1                         	Field type (HProfTagFieldType)
* 	[value]                     Field value (size depends on type)
*
* Instance field format:
* 	id                         	Field name string ID
* 	u1                          Field type (HProfTagFieldType)
*                               (No value - values are in INSTANCE_DUMP records)
 */
func parseClassDump(reader *BinaryReader, classDumpReg *registry.ClassDumpRegistry) error {
	classDump := &model.ClassDump{}

	// Parse class header
	var err error
	classDump.ClassObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read class object ID: %w", err)
	}

	stackTraceSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read stack trace serial: %w", err)
	}
	classDump.StackTraceSerialNumber = model.SerialNum(stackTraceSerial)

	classDump.SuperClassObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read superclass object ID: %w", err)
	}

	classDump.ClassLoaderObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read class loader object ID: %w", err)
	}

	classDump.SignerObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read signer object ID: %w", err)
	}

	classDump.ProtectionDomainObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read protection domain object ID: %w", err)
	}

	classDump.Reserved1, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read reserved1: %w", err)
	}

	classDump.Reserved2, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read reserved2: %w", err)
	}

	classDump.InstanceSize, err = reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read instance size: %w", err)
	}

	// Parse constant pool
	classDump.ConstantPoolSize, err = reader.ReadU2()
	if err != nil {
		return fmt.Errorf("failed to read constant pool size: %w", err)
	}

	classDump.ConstantPool = make([]*model.ConstantPoolEntry, classDump.ConstantPoolSize)
	for i := uint16(0); i < classDump.ConstantPoolSize; i++ {
		entry, err := parseConstantPoolEntry(reader)
		if err != nil {
			return fmt.Errorf("failed to parse constant pool entry %d: %w", i, err)
		}
		classDump.ConstantPool[i] = entry
	}

	// Parse static fields
	classDump.StaticFieldsCount, err = reader.ReadU2()
	if err != nil {
		return fmt.Errorf("failed to read static fields count: %w", err)
	}

	classDump.StaticFields = make([]*model.StaticField, classDump.StaticFieldsCount)
	for i := uint16(0); i < classDump.StaticFieldsCount; i++ {
		field, err := parseStaticField(reader)
		if err != nil {
			return fmt.Errorf("failed to parse static field %d: %w", i, err)
		}
		classDump.StaticFields[i] = field
	}

	// Parse instance fields
	classDump.InstanceFieldsCount, err = reader.ReadU2()
	if err != nil {
		return fmt.Errorf("failed to read instance fields count: %w", err)
	}

	classDump.InstanceFields = make([]*model.InstanceField, classDump.InstanceFieldsCount)
	for i := uint16(0); i < classDump.InstanceFieldsCount; i++ {
		field, err := parseInstanceField(reader)
		if err != nil {
			return fmt.Errorf("failed to parse instance field %d: %w", i, err)
		}
		classDump.InstanceFields[i] = field
	}

	// Add to registry
	classDumpReg.AddClassDump(classDump)

	return nil
}

// parseConstantPoolEntry parses a single constant pool entry
func parseConstantPoolEntry(reader *BinaryReader) (*model.ConstantPoolEntry, error) {
	entry := &model.ConstantPoolEntry{}

	var err error
	entry.Index, err = reader.ReadU2()
	if err != nil {
		return nil, fmt.Errorf("failed to read constant pool index: %w", err)
	}

	typeValue, err := reader.ReadU1()
	if err != nil {
		return nil, fmt.Errorf("failed to read constant pool type: %w", err)
	}
	entry.Type = model.HProfTagFieldType(typeValue)

	// Read value based on type
	valueSize := entry.Type.Size(reader.Header().IdentifierSize)
	if valueSize == 0 {
		return nil, fmt.Errorf("unknown constant pool value type: 0x%02x", typeValue)
	}

	entry.Value = make([]byte, valueSize)
	err = reader.ReadBytes(entry.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to read constant pool value: %w", err)
	}

	return entry, nil
}

// parseStaticField parses a single static field definition with value
func parseStaticField(reader *BinaryReader) (*model.StaticField, error) {
	field := &model.StaticField{}

	var err error
	field.NameID, err = reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read static field name ID: %w", err)
	}

	typeValue, err := reader.ReadU1()
	if err != nil {
		return nil, fmt.Errorf("failed to read static field type: %w", err)
	}
	field.Type = model.HProfTagFieldType(typeValue)

	// Read field value based on type
	valueSize := field.Type.Size(reader.Header().IdentifierSize)
	if valueSize == 0 {
		return nil, fmt.Errorf("unknown static field type: 0x%02x", typeValue)
	}

	field.Value = make([]byte, valueSize)
	err = reader.ReadBytes(field.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to read static field value: %w", err)
	}

	return field, nil
}

// parseInstanceField parses a single instance field definition (no value)
func parseInstanceField(reader *BinaryReader) (*model.InstanceField, error) {
	field := &model.InstanceField{}

	var err error
	field.NameID, err = reader.ReadID()
	if err != nil {
		return nil, fmt.Errorf("failed to read instance field name ID: %w", err)
	}

	typeValue, err := reader.ReadU1()
	if err != nil {
		return nil, fmt.Errorf("failed to read instance field type: %w", err)
	}
	field.Type = model.HProfTagFieldType(typeValue)

	// Instance fields don't store values in CLASS_DUMP - values are in INSTANCE_DUMP

	return field, nil
}
