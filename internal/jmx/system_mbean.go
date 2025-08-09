package jmx

import (
	"fmt"
	"time"
)

// ===== OPERATING SYSTEM METRICS =====
func (jc *JMXPoller) collectOperatingSystemMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	osInfo, err := client.QueryMBean("java.lang:type=OperatingSystem")
	if err != nil {
		return fmt.Errorf("failed to query OS metrics: %w", err)
	}

	// System identification
	if name, ok := osInfo["Name"].(string); ok {
		metrics.OS.Name = name
	}
	if version, ok := osInfo["Version"].(string); ok {
		metrics.OS.Version = version
	}
	if arch, ok := osInfo["Arch"].(string); ok {
		metrics.OS.Arch = arch
	}

	// CPU metrics
	if processors, ok := osInfo["AvailableProcessors"].(float64); ok {
		metrics.OS.AvailableProcessors = int64(processors)
	}
	if systemCpu, ok := osInfo["SystemCpuLoad"].(float64); ok && systemCpu >= 0 {
		metrics.OS.SystemCpuLoad = systemCpu
	}
	if processCpu, ok := osInfo["ProcessCpuLoad"].(float64); ok && processCpu >= 0 {
		metrics.OS.ProcessCpuLoad = processCpu
	}
	if processCpuTime, ok := osInfo["ProcessCpuTime"].(float64); ok {
		metrics.OS.ProcessCpuTime = int64(processCpuTime)
	}
	if loadAvg, ok := osInfo["SystemLoadAverage"].(float64); ok && loadAvg >= 0 {
		metrics.OS.SystemLoadAverage = loadAvg
	}

	// Memory metrics
	if totalMem, ok := osInfo["TotalPhysicalMemorySize"].(float64); ok {
		metrics.OS.TotalPhysicalMemory = int64(totalMem)
	}
	if freeMem, ok := osInfo["FreePhysicalMemorySize"].(float64); ok {
		metrics.OS.FreePhysicalMemory = int64(freeMem)
	}
	if totalSwap, ok := osInfo["TotalSwapSpaceSize"].(float64); ok {
		metrics.OS.TotalSwapSpace = int64(totalSwap)
	}
	if freeSwap, ok := osInfo["FreeSwapSpaceSize"].(float64); ok {
		metrics.OS.FreeSwapSpace = int64(freeSwap)
	}
	if virtualMem, ok := osInfo["CommittedVirtualMemorySize"].(float64); ok {
		metrics.OS.CommittedVirtualMemorySize = int64(virtualMem)
	}

	// File descriptor metrics (Unix/Linux only)
	if openFDs, ok := osInfo["OpenFileDescriptorCount"].(float64); ok {
		metrics.OS.OpenFileDescriptorCount = int64(openFDs)
	}
	if maxFDs, ok := osInfo["MaxFileDescriptorCount"].(float64); ok {
		metrics.OS.MaxFileDescriptorCount = int64(maxFDs)
	}

	return nil
}

// ===== RUNTIME METRICS =====
func (jc *JMXPoller) collectRuntimeMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	runtime, err := client.QueryMBean("java.lang:type=Runtime")
	if err != nil {
		return fmt.Errorf("failed to query runtime metrics: %w", err)
	}

	// Process identification
	if pid, ok := runtime["Pid"].(float64); ok {
		metrics.Runtime.ProcessID = int64(pid)
	}
	if processName, ok := runtime["Name"].(string); ok {
		metrics.Runtime.ProcessName = processName
	}

	// JVM information
	if name, ok := runtime["VmName"].(string); ok {
		metrics.Runtime.VmName = name
	}
	if vendor, ok := runtime["VmVendor"].(string); ok {
		metrics.Runtime.VmVendor = vendor
	}
	if version, ok := runtime["VmVersion"].(string); ok {
		metrics.Runtime.VmVersion = version
	}

	// JVM specification
	if specName, ok := runtime["SpecName"].(string); ok {
		metrics.Runtime.SpecName = specName
	}
	if specVendor, ok := runtime["SpecVendor"].(string); ok {
		metrics.Runtime.SpecVendor = specVendor
	}
	if specVersion, ok := runtime["SpecVersion"].(string); ok {
		metrics.Runtime.SpecVersion = specVersion
	}

	// Timing
	if startTime, ok := runtime["StartTime"].(float64); ok {
		metrics.Runtime.StartTime = time.Unix(int64(startTime/1000), 0)
	}
	if uptime, ok := runtime["Uptime"].(float64); ok {
		metrics.Runtime.Uptime = time.Duration(uptime) * time.Millisecond
	}

	// Configuration
	if args, ok := runtime["InputArguments"].([]interface{}); ok {
		for _, arg := range args {
			if argStr, ok := arg.(string); ok {
				metrics.Runtime.InputArguments = append(metrics.Runtime.InputArguments, argStr)
			}
		}
	}

	if classPath, ok := runtime["ClassPath"].(string); ok {
		metrics.Runtime.ClassPath = classPath
	}
	if libPath, ok := runtime["LibraryPath"].(string); ok {
		metrics.Runtime.LibraryPath = libPath
	}
	if bootClassPath, ok := runtime["BootClassPath"].(string); ok {
		metrics.Runtime.BootClassPath = bootClassPath
	}
	if bootClassPathSupported, ok := runtime["BootClassPathSupported"].(bool); ok {
		metrics.Runtime.BootClassPathSupported = bootClassPathSupported
	}
	if mgmtSpecVersion, ok := runtime["ManagementSpecVersion"].(string); ok {
		metrics.Runtime.ManagementSpecVersion = mgmtSpecVersion
	}

	// System Properties
	if props, ok := runtime["SystemProperties"].(map[string]any); ok {
		metrics.Runtime.SystemProperties = make(map[string]string)
		for key, value := range props {
			if propMap, ok := value.(map[string]any); ok {
				if valueStr, ok := propMap["value"].(string); ok {
					metrics.Runtime.SystemProperties[key] = valueStr

					// Extract commonly used properties
					switch key {
					case "java.version":
						metrics.Runtime.JavaVersion = valueStr
					case "java.vendor":
						metrics.Runtime.JavaVendor = valueStr
					case "java.home":
						metrics.Runtime.JavaHome = valueStr
					case "user.name":
						metrics.Runtime.UserName = valueStr
					case "user.home":
						metrics.Runtime.UserHome = valueStr
					case "user.dir":
						metrics.Runtime.UserDir = valueStr
					}
				}
			}
		}
	}

	return nil
}
