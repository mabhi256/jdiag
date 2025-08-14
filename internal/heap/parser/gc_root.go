package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
* parseGCRootUnknown parses a GC_ROOT_UNKNOWN sub-record:
*
* GC_ROOT_UNKNOWN represents an object of unknown root type.
* This is typically used as a fallback or for debugging.
*
* Format:
* 	id    Object ID that is a GC root
 */
func parseGCRootUnknown(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	root := model.GCRootUnknown{
		ObjectID: objectID,
	}

	rootReg.AddUnknownRoot(root)
	return nil
}

/*
* parseGCRootJniGlobal parses a GC_ROOT_JNI_GLOBAL sub-record:
*
* GC_ROOT_JNI_GLOBAL represents a global JNI reference.
* These are objects referenced from native code via JNI global references.
*
* Format:
* 	id    Object ID that is referenced
* 	id    JNI global reference ID (used internally by JVM)
 */
func parseGCRootJniGlobal(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	jniGlobalRefID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read JNI global ref ID: %w", err)
	}

	root := model.GCRootJniGlobal{
		ObjectID:       objectID,
		JniGlobalRefID: jniGlobalRefID,
	}

	rootReg.AddJniGlobalRoot(root)
	return nil
}

/*
* parseGCRootJniLocal parses a GC_ROOT_JNI_LOCAL sub-record:
*
* GC_ROOT_JNI_LOCAL represents a local JNI reference from a specific stack frame.
* These are objects referenced from native code via JNI local references.
*
* Format:
* 	id    Object ID that is referenced
* 	u4    Thread serial number that owns this reference
* 	u4    Frame number in stack trace (-1 for empty/unknown frame)
 */
func parseGCRootJniLocal(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	threadSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read thread serial: %w", err)
	}

	frameNumber, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read frame number: %w", err)
	}

	root := model.GCRootJniLocal{
		ObjectID:           objectID,
		ThreadSerialNumber: model.SerialNum(threadSerial),
		FrameNumber:        model.SerialNum(frameNumber),
	}

	rootReg.AddJniLocalRoot(root)
	return nil
}

/*
* parseGCRootJavaFrame parses a GC_ROOT_JAVA_FRAME sub-record:
*
* GC_ROOT_JAVA_FRAME represents a local variable in a Java stack frame.
* These are objects held in local variables or parameters of method calls.
*
* Format:
* 	id    Object ID that is referenced
* 	u4    Thread serial number that owns this stack frame
* 	u4    Frame number in stack trace (-1 for empty/unknown frame)
 */
func parseGCRootJavaFrame(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	threadSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read thread serial: %w", err)
	}

	frameNumber, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read frame number: %w", err)
	}

	root := model.GCRootJavaFrame{
		ObjectID:           objectID,
		ThreadSerialNumber: model.SerialNum(threadSerial),
		FrameNumber:        model.SerialNum(frameNumber),
	}

	rootReg.AddJavaFrameRoot(root)
	return nil
}

/*
* parseGCRootNativeStack parses a GC_ROOT_NATIVE_STACK sub-record:
*
* GC_ROOT_NATIVE_STACK represents an object referenced from native stack.
* These are objects held by native code that called into Java.
*
* Format:
* 	id    Object ID that is referenced
* 	u4    Thread serial number that owns this native stack
 */
func parseGCRootNativeStack(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	threadSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read thread serial: %w", err)
	}

	root := model.GCRootNativeStack{
		ObjectID:           objectID,
		ThreadSerialNumber: model.SerialNum(threadSerial),
	}

	rootReg.AddNativeStackRoot(root)
	return nil
}

/*
* parseGCRootStickyClass parses a GC_ROOT_STICKY_CLASS sub-record:
*
* GC_ROOT_STICKY_CLASS represents a class that is "sticky" and cannot be unloaded.
* These are typically system classes or classes held by system class loaders.
*
* Format:
* 	id    Class object ID that cannot be unloaded
 */
func parseGCRootStickyClass(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	root := model.GCRootStickyClass{
		ObjectID: objectID,
	}

	rootReg.AddStickyClassRoot(root)
	return nil
}

/*
* parseGCRootThreadBlock parses a GC_ROOT_THREAD_BLOCK sub-record:
*
* GC_ROOT_THREAD_BLOCK represents an object that is being waited on by a thread.
* These are objects involved in thread synchronization (monitors).
*
* Format:
* 	id    Object ID that is being waited on
* 	u4    Thread serial number that is waiting
 */
func parseGCRootThreadBlock(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	threadSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read thread serial: %w", err)
	}

	root := model.GCRootThreadBlock{
		ObjectID:           objectID,
		ThreadSerialNumber: model.SerialNum(threadSerial),
	}

	rootReg.AddThreadBlockRoot(root)
	return nil
}

/*
* parseGCRootMonitorUsed parses a GC_ROOT_MONITOR_USED sub-record:
*
* GC_ROOT_MONITOR_USED represents an object that has a monitor associated with it.
* These are objects that have been used for synchronization.
*
* Format:
* 	id    Object ID that has an associated monitor
 */
func parseGCRootMonitorUsed(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	objectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	root := model.GCRootMonitorUsed{
		ObjectID: objectID,
	}

	rootReg.AddMonitorUsedRoot(root)
	return nil
}

/*
* parseGCRootThreadObj parses a GC_ROOT_THREAD_OBJ sub-record:
*
* GC_ROOT_THREAD_OBJ represents a thread object itself.
* This identifies the java.lang.Thread instances and connects them to stack traces.
*
* Format:
* 	id    Thread object ID (may be 0 for threads attached via JNI)
* 	u4    Thread sequence number (unique identifier for the thread)
* 	u4    Stack trace sequence number (links to TRACE records)
 */
func parseGCRootThreadObj(reader *BinaryReader, rootReg *registry.GCRootRegistry) error {
	threadObjectID, err := reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read thread object ID: %w", err)
	}

	threadSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read thread serial: %w", err)
	}

	stackTraceSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read stack trace serial: %w", err)
	}

	root := model.GCRootThreadObject{
		ThreadObjectID:         threadObjectID,
		ThreadSerialNumber:     model.SerialNum(threadSerial),
		StackTraceSerialNumber: model.SerialNum(stackTraceSerial),
	}

	rootReg.AddThreadObjectRoot(root)
	return nil
}
