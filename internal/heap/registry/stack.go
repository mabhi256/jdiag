package registry

import (
	"github.com/mabhi256/jdiag/internal/heap/model"
)

type StackRegistry struct {
	frames *BaseRegistry[model.ID, *model.FrameBody]
	traces *BaseRegistry[model.SerialNum, *model.TraceBody]
}

func NewStackRegistry() *StackRegistry {
	return &StackRegistry{
		frames: NewBaseRegistry[model.ID, *model.FrameBody](),
		traces: NewBaseRegistry[model.SerialNum, *model.TraceBody](),
	}
}

func (r *StackRegistry) AddFrame(frame *model.FrameBody) {
	r.frames.Add(frame.StackFrameID, frame)
}

func (r *StackRegistry) AddTrace(trace *model.TraceBody) {
	r.traces.Add(trace.StackTraceSerialNumber, trace)
}

func (r *StackRegistry) GetFrame(frameID model.ID) (*model.FrameBody, bool) {
	return r.frames.Get(frameID)
}

func (r *StackRegistry) GetTrace(serialNum model.SerialNum) (*model.TraceBody, bool) {
	return r.traces.Get(serialNum)
}

func (r *StackRegistry) CountFrames() int {
	return r.frames.Count()
}

func (r *StackRegistry) CountTraces() int {
	return r.traces.Count()
}

func (r *StackRegistry) GetAllFrames() map[model.ID]*model.FrameBody {
	return r.frames.GetAll()
}

func (r *StackRegistry) GetAllTraces() map[model.SerialNum]*model.TraceBody {
	return r.traces.GetAll()
}

func (r *StackRegistry) Clear() {
	r.frames.Clear()
	r.traces.Clear()
}
