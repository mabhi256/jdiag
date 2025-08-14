package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

/*
*	ParseHeapSummary parses a HPROF_HEAP_SUMMARY record
*
*	u4		total live bytes
*	u4		total live instances
*	u8		total bytes allocated
*	u8		total instances allocated
 */
func ParseHeapSummary(reader *BinaryReader) (*model.HeapSummary, error) {
	// Read live bytes
	liveBytes, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read live bytes: %w", err)
	}

	// Read live instances
	liveInstances, err := reader.ReadU4()
	if err != nil {
		return nil, fmt.Errorf("failed to read live instances: %w", err)
	}

	// Read bytes allocated
	bytesAlloc, err := reader.ReadU8()
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes allocated: %w", err)
	}

	// Read instances allocated
	instancesAlloc, err := reader.ReadU8()
	if err != nil {
		return nil, fmt.Errorf("failed to read instances allocated: %w", err)
	}

	heapSummary := &model.HeapSummary{
		LiveBytes:      liveBytes,
		LiveInstances:  liveInstances,
		BytesAlloc:     bytesAlloc,
		InstancesAlloc: instancesAlloc,
	}

	return heapSummary, nil
}
