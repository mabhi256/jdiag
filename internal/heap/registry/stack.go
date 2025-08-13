package registry

import (
	"maps"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type StackRegistry struct {
	frames map[model.ID]*model.FrameBody
	traces map[model.SerialNum]*model.TraceBody
}

func NewStackRegistry() *StackRegistry {
	return &StackRegistry{
		frames: make(map[model.ID]*model.FrameBody),
		traces: make(map[model.SerialNum]*model.TraceBody),
	}
}

func (sr *StackRegistry) AddFrame(frame *model.FrameBody) {
	sr.frames[frame.StackFrameID] = frame
}

func (sr *StackRegistry) AddTrace(trace *model.TraceBody) {
	sr.traces[trace.StackTraceSerialNumber] = trace
}

func (sr *StackRegistry) GetFrame(frameID model.ID) (*model.FrameBody, bool) {
	frame, exists := sr.frames[frameID]
	return frame, exists
}

func (sr *StackRegistry) GetTrace(serialNum model.SerialNum) (*model.TraceBody, bool) {
	trace, exists := sr.traces[serialNum]
	return trace, exists
}

func (sr *StackRegistry) CountFrames() int {
	return len(sr.frames)
}

func (sr *StackRegistry) CountTraces() int {
	return len(sr.traces)
}

func (sr *StackRegistry) GetAllFrames() map[model.ID]*model.FrameBody {
	result := make(map[model.ID]*model.FrameBody, len(sr.frames))
	maps.Copy(result, sr.frames)
	return result
}

func (sr *StackRegistry) GetAllTraces() map[model.SerialNum]*model.TraceBody {
	result := make(map[model.SerialNum]*model.TraceBody, len(sr.traces))
	maps.Copy(result, sr.traces)
	return result
}

func (sr *StackRegistry) Clear() {
	sr.frames = make(map[model.ID]*model.FrameBody)
	sr.traces = make(map[model.SerialNum]*model.TraceBody)
}
