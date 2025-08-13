package registry

import (
	"sort"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ClassInfo struct {
	LoadClassBody *model.LoadClassBody
	ClassName     string
	IsLoaded      bool
	LoadOrder     int // Order in which class was loaded
}

type ClassRegistry struct {
	// Maps serial number to class info
	classesBySerial map[model.SerialNum]*ClassInfo

	// Maps object ID to class info
	classesByObjectID map[model.ID]*ClassInfo

	// Maps class name to class info
	classesByName map[string]*ClassInfo

	// Statistics
	loadOrder     int
	loadedCount   int
	unloadedCount int
}

func NewClassRegistry() *ClassRegistry {
	return &ClassRegistry{
		classesBySerial:   make(map[model.SerialNum]*ClassInfo),
		classesByObjectID: make(map[model.ID]*ClassInfo),
		classesByName:     make(map[string]*ClassInfo),
	}
}

func (cr *ClassRegistry) AddLoadedClass(loadClassBody *model.LoadClassBody, className string) {
	cr.loadOrder++

	classInfo := &ClassInfo{
		LoadClassBody: loadClassBody,
		ClassName:     className,
		IsLoaded:      true,
		LoadOrder:     cr.loadOrder,
	}

	// Store in all lookup maps
	cr.classesBySerial[loadClassBody.ClassSerialNumber] = classInfo
	cr.classesByObjectID[loadClassBody.ObjectID] = classInfo
	cr.classesByName[className] = classInfo

	cr.loadedCount++
}

func (cr *ClassRegistry) UnloadClass(serialNumber model.SerialNum) {
	classInfo, exists := cr.classesBySerial[serialNumber]
	if exists && classInfo.IsLoaded {
		classInfo.IsLoaded = false
		cr.loadedCount--
		cr.unloadedCount++
	}
}

func (cr *ClassRegistry) GetBySerial(serialNumber model.SerialNum) (*ClassInfo, bool) {
	classInfo, exists := cr.classesBySerial[serialNumber]
	return classInfo, exists
}

func (cr *ClassRegistry) GetByObjectID(objectID model.ID) (*ClassInfo, bool) {
	classInfo, exists := cr.classesByObjectID[objectID]
	return classInfo, exists
}

func (cr *ClassRegistry) GetByName(className string) (*ClassInfo, bool) {
	classInfo, exists := cr.classesByName[className]
	return classInfo, exists
}

func sortByLoadOrder(classes []*ClassInfo) []*ClassInfo {
	sort.Slice(classes, func(i, j int) bool {
		return classes[i].LoadOrder < classes[j].LoadOrder
	})

	return classes
}

func (cr *ClassRegistry) GetLoadedClasses() []*ClassInfo {
	var loaded []*ClassInfo
	for _, classInfo := range cr.classesBySerial {
		if classInfo.IsLoaded {
			loaded = append(loaded, classInfo)
		}
	}

	return sortByLoadOrder(loaded)
}

func (cr *ClassRegistry) GetUnloadedClasses() []*ClassInfo {
	var unloaded []*ClassInfo
	for _, classInfo := range cr.classesBySerial {
		if !classInfo.IsLoaded {
			unloaded = append(unloaded, classInfo)
		}
	}

	return sortByLoadOrder(unloaded)
}

func (cr *ClassRegistry) GetAllClasses() []*ClassInfo {
	classes := make([]*ClassInfo, 0, len(cr.classesBySerial))
	for _, classInfo := range cr.classesBySerial {
		classes = append(classes, classInfo)
	}

	return sortByLoadOrder(classes)
}

func (cr *ClassRegistry) Clear() {
	cr.classesBySerial = make(map[model.SerialNum]*ClassInfo)
	cr.classesByObjectID = make(map[model.ID]*ClassInfo)
	cr.classesByName = make(map[string]*ClassInfo)
	cr.loadOrder = 0
	cr.loadedCount = 0
	cr.unloadedCount = 0
}
