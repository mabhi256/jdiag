package jmx

import (
	"fmt"
	"strings"
	"time"
)

// ===== SYSTEM METRICS =====
// Fetch ALL available system-related attributes
func (jc *JMXPoller) collectSystemMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	// 1. Operating System Metrics - Get ALL available attributes
	osInfo, err := client.QueryMBean("java.lang:type=OperatingSystem")
	if err != nil {
		return fmt.Errorf("failed to query OS metrics: %w", err)
	}

	// CPU metrics
	if processCpu, ok := osInfo["ProcessCpuLoad"].(float64); ok && processCpu >= 0 {
		metrics.ProcessCpuLoad = processCpu
	}
	if systemCpu, ok := osInfo["SystemCpuLoad"].(float64); ok && systemCpu >= 0 {
		metrics.SystemCpuLoad = systemCpu
	}
	if cpuLoad, ok := osInfo["CpuLoad"].(float64); ok && cpuLoad >= 0 {
		metrics.CpuLoad = cpuLoad
	}
	if processCpuTime, ok := osInfo["ProcessCpuTime"].(float64); ok {
		metrics.ProcessCpuTime = int64(processCpuTime)
	}
	if processors, ok := osInfo["AvailableProcessors"].(float64); ok {
		metrics.AvailableProcessors = int64(processors)
	}
	if loadAvg, ok := osInfo["SystemLoadAverage"].(float64); ok && loadAvg >= 0 {
		metrics.SystemLoadAverage = loadAvg
	}

	// Memory metrics
	if totalMem, ok := osInfo["TotalPhysicalMemorySize"].(float64); ok {
		metrics.TotalSystemMemory = int64(totalMem)
	}
	if freeMem, ok := osInfo["FreePhysicalMemorySize"].(float64); ok {
		metrics.FreeSystemMemory = int64(freeMem)
	}
	if totalMemSize, ok := osInfo["TotalMemorySize"].(float64); ok {
		metrics.TotalMemorySize = int64(totalMemSize)
	}
	if freeMemSize, ok := osInfo["FreeMemorySize"].(float64); ok {
		metrics.FreeMemorySize = int64(freeMemSize)
	}
	if virtualMem, ok := osInfo["CommittedVirtualMemorySize"].(float64); ok {
		metrics.ProcessVirtualMemorySize = int64(virtualMem)
	}

	// Swap metrics
	if totalSwap, ok := osInfo["TotalSwapSpaceSize"].(float64); ok {
		metrics.TotalSwapSize = int64(totalSwap)
	}
	if freeSwap, ok := osInfo["FreeSwapSpaceSize"].(float64); ok {
		metrics.FreeSwapSize = int64(freeSwap)
	}

	// File descriptor metrics (Unix/Linux only)
	if openFDs, ok := osInfo["OpenFileDescriptorCount"].(float64); ok {
		metrics.OpenFileDescriptorCount = int64(openFDs)
	}
	if maxFDs, ok := osInfo["MaxFileDescriptorCount"].(float64); ok {
		metrics.MaxFileDescriptorCount = int64(maxFDs)
	}

	// OS details
	if osName, ok := osInfo["Name"].(string); ok {
		metrics.OSName = osName
	}
	if osVersion, ok := osInfo["Version"].(string); ok {
		metrics.OSVersion = osVersion
	}
	if osArch, ok := osInfo["Arch"].(string); ok {
		metrics.OSArch = osArch
	}

	// 2. Try Sun/Oracle-specific OS MBean for additional process memory
	sunOSInfo, err := client.QueryMBean("com.sun.management:type=OperatingSystem")
	if err == nil {
		if privateBytes, ok := sunOSInfo["ProcessPrivateBytes"].(float64); ok {
			metrics.ProcessPrivateMemory = int64(privateBytes)
		}
		if sharedBytes, ok := sunOSInfo["ProcessSharedBytes"].(float64); ok {
			metrics.ProcessSharedMemory = int64(sharedBytes)
		}
		if metrics.ProcessPrivateMemory > 0 || metrics.ProcessSharedMemory > 0 {
			metrics.ProcessResidentSetSize = metrics.ProcessPrivateMemory + metrics.ProcessSharedMemory
		}
	}

	// 3. Runtime and Configuration Metrics - Get ALL available attributes
	runtime, err := client.QueryMBean("java.lang:type=Runtime")
	if err != nil {
		return fmt.Errorf("failed to query runtime metrics: %w", err)
	}

	// Basic JVM info
	if name, ok := runtime["VmName"].(string); ok {
		metrics.JVMName = name
	}
	if vendor, ok := runtime["VmVendor"].(string); ok {
		metrics.JVMVendor = vendor
	}
	if version, ok := runtime["VmVersion"].(string); ok {
		metrics.JVMVersion = version
	}
	if startTime, ok := runtime["StartTime"].(float64); ok {
		metrics.JVMStartTime = time.Unix(int64(startTime/1000), 0)
	}
	if uptime, ok := runtime["Uptime"].(float64); ok {
		metrics.JVMUptime = time.Duration(uptime) * time.Millisecond
	}

	// Process info
	if pid, ok := runtime["Pid"].(float64); ok {
		metrics.ProcessID = int64(pid)
	}
	if processName, ok := runtime["Name"].(string); ok {
		metrics.ProcessName = processName
	}

	// JVM specification info
	if specName, ok := runtime["SpecName"].(string); ok {
		metrics.JVMSpecName = specName
	}
	if specVendor, ok := runtime["SpecVendor"].(string); ok {
		metrics.JVMSpecVendor = specVendor
	}
	if specVersion, ok := runtime["SpecVersion"].(string); ok {
		metrics.JVMSpecVersion = specVersion
	}
	if mgmtSpecVersion, ok := runtime["ManagementSpecVersion"].(string); ok {
		metrics.ManagementSpecVersion = mgmtSpecVersion
	}

	// Paths and configuration
	if classPath, ok := runtime["ClassPath"].(string); ok {
		metrics.JavaClassPath = classPath
	}
	if libPath, ok := runtime["LibraryPath"].(string); ok {
		metrics.JavaLibraryPath = libPath
	}
	if bootClassPath, ok := runtime["BootClassPath"].(string); ok {
		metrics.BootClassPath = bootClassPath
	}
	if bootClassPathSupported, ok := runtime["BootClassPathSupported"].(bool); ok {
		metrics.BootClassPathSupported = bootClassPathSupported
	}

	// JVM Arguments
	if args, ok := runtime["InputArguments"].([]interface{}); ok {
		for _, arg := range args {
			if argStr, ok := arg.(string); ok {
				metrics.JVMArguments = append(metrics.JVMArguments, argStr)
			}
		}
	}

	// System Properties
	if props, ok := runtime["SystemProperties"].(map[string]any); ok {
		metrics.SystemProperties = make(map[string]string)
		metrics.GCSettings = make(map[string]string)

		for key, value := range props {
			if valueStr, ok := value.(string); ok {
				metrics.SystemProperties[key] = valueStr

				// Extract commonly used properties
				switch key {
				case "java.version":
					metrics.JavaVersion = valueStr
				case "java.vendor":
					metrics.JavaVendor = valueStr
				case "java.home":
					metrics.JavaHome = valueStr
				case "user.name":
					metrics.UserName = valueStr
				case "user.home":
					metrics.UserHome = valueStr
				case "os.name":
					if metrics.OSName == "" { // Prefer OS MBean value
						metrics.OSName = valueStr
					}
				case "os.arch":
					if metrics.OSArch == "" { // Prefer OS MBean value
						metrics.OSArch = valueStr
					}
				case "os.version":
					if metrics.OSVersion == "" { // Prefer OS MBean value
						metrics.OSVersion = valueStr
					}
				}

				// Extract GC-related properties
				if strings.HasPrefix(key, "gc.") ||
					strings.Contains(key, "GC") ||
					strings.Contains(key, "UseG1GC") ||
					strings.Contains(key, "UseParallelGC") ||
					strings.Contains(key, "UseConcMarkSweepGC") ||
					strings.Contains(key, "G1HeapRegionSize") {
					metrics.GCSettings[key] = valueStr
				}
			}
		}
	}

	// 4. Compilation Metrics - Get ALL available attributes
	compilation, err := client.QueryMBean("java.lang:type=Compilation")
	if err == nil {
		if totalTime, ok := compilation["TotalCompilationTime"].(float64); ok {
			metrics.TotalCompilationTime = int64(totalTime)
		}
		if name, ok := compilation["Name"].(string); ok {
			metrics.CompilerName = name
		}
		if supported, ok := compilation["CompilationTimeMonitoringSupported"].(bool); ok {
			metrics.CompilationTimeMonitoringSupported = supported
		}
	}

	// 5. Safepoint Metrics (HotSpot-specific) - Get ALL available attributes
	safepointInfo, err := client.QueryMBean("sun.management:type=HotspotRuntime")
	if err == nil {
		if count, ok := safepointInfo["SafepointCount"].(float64); ok {
			metrics.SafepointCount = int64(count)
		}
		if totalTime, ok := safepointInfo["TotalSafepointTime"].(float64); ok {
			metrics.SafepointTime = int64(totalTime)
		}
		if syncTime, ok := safepointInfo["SafepointSyncTime"].(float64); ok {
			metrics.SafepointSyncTime = int64(syncTime)
		}
		if appTime, ok := safepointInfo["ApplicationTime"].(float64); ok {
			metrics.ApplicationTime = int64(appTime)
		}

		if metrics.SafepointTime > 0 && metrics.ApplicationTime > 0 {
			metrics.ApplicationStoppedTime = metrics.SafepointTime
		}
	}

	return nil
}
