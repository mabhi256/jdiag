package registry

import (
	"sort"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ClassInfo struct {
	LoadClassBody *model.LoadClassBody
	ClassName     string
	IsLoaded      bool
}

type ClassRegistry struct {
	*BaseRegistry[model.SerialNum, *ClassInfo]
	classesByObjectID map[model.ID]*ClassInfo
	classesByName     map[string]*ClassInfo
	loadedCount       int
	unloadedCount     int
}

func NewClassRegistry() *ClassRegistry {
	return &ClassRegistry{
		BaseRegistry:      NewBaseRegistry[model.SerialNum, *ClassInfo](),
		classesByObjectID: make(map[model.ID]*ClassInfo),
		classesByName:     make(map[string]*ClassInfo),
	}
}

func (r *ClassRegistry) AddLoadedClass(loadClassBody *model.LoadClassBody, className string) {
	classInfo := &ClassInfo{
		LoadClassBody: loadClassBody,
		ClassName:     className,
		IsLoaded:      true,
	}

	// Store in all lookup maps
	r.Add(loadClassBody.ClassSerialNumber, classInfo)
	r.classesByObjectID[loadClassBody.ObjectID] = classInfo
	r.classesByName[className] = classInfo
	r.loadedCount++
}

func (r *ClassRegistry) UnloadClass(serialNumber model.SerialNum) {
	if classInfo, exists := r.Get(serialNumber); exists && classInfo.IsLoaded {
		classInfo.IsLoaded = false
		r.loadedCount--
		r.unloadedCount++
	}
}

func (r *ClassRegistry) GetByObjectID(objectID model.ID) (*ClassInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	classInfo, exists := r.classesByObjectID[objectID]
	return classInfo, exists
}

func (r *ClassRegistry) GetByName(className string) (*ClassInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	classInfo, exists := r.classesByName[className]
	return classInfo, exists
}

func (r *ClassRegistry) GetLoadedClasses() []*ClassInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var loaded []*ClassInfo
	for _, classInfo := range r.data {
		if classInfo.IsLoaded {
			loaded = append(loaded, classInfo)
		}
	}

	// Sort by load order
	sort.Slice(loaded, func(i, j int) bool {
		return loaded[i].LoadClassBody.ClassSerialNumber < loaded[j].LoadClassBody.ClassSerialNumber
	})
	return loaded
}

func (r *ClassRegistry) Clear() {
	r.BaseRegistry.Clear()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.classesByObjectID = make(map[model.ID]*ClassInfo)
	r.classesByName = make(map[string]*ClassInfo)
	r.loadedCount = 0
	r.unloadedCount = 0
}

func sortByLoadOrder(classes []*ClassInfo) []*ClassInfo {
	sort.Slice(classes, func(i, j int) bool {
		return classes[i].LoadClassBody.ClassSerialNumber < classes[j].LoadClassBody.ClassSerialNumber
	})

	return classes
}
