package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// MemorySize represents a memory size in bytes
type MemorySize int64

const (
	Byte MemorySize = 1
	KB   MemorySize = 1024 * Byte
	MB   MemorySize = 1024 * KB
	GB   MemorySize = 1024 * MB
	TB   MemorySize = 1024 * GB
	PB   MemorySize = 1024 * TB
)

// String returns a human-readable representation of the memory size
func (m MemorySize) String() string {
	if m <= 0 {
		return "0B"
	}

	formatValue := func(val float64, unit string) string {
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f%s", val, unit)
		}
		return fmt.Sprintf("%.2f%s", val, unit)
	}

	switch {
	case m >= PB:
		return formatValue(float64(m)/float64(TB), "P")
	case m >= TB:
		return formatValue(float64(m)/float64(TB), "T")
	case m >= GB:
		return formatValue(float64(m)/float64(GB), "G")
	case m >= MB:
		return formatValue(float64(m)/float64(MB), "M")
	case m >= KB:
		return formatValue(float64(m)/float64(KB), "K")
	default:
		return fmt.Sprintf("%dB", m)
	}
}

// Bytes returns the memory size as bytes
func (m MemorySize) Bytes() int64 {
	return int64(m)
}

// KB returns the memory size as kilobytes
func (m MemorySize) KB() float64 {
	return float64(m) / float64(KB)
}

// MB returns the memory size as megabytes
func (m MemorySize) MB() float64 {
	return float64(m) / float64(MB)
}

// GB returns the memory size as gigabytes
func (m MemorySize) GB() float64 {
	return float64(m) / float64(GB)
}

// TB returns the memory size as terabytes
func (m MemorySize) TB() float64 {
	return float64(m) / float64(TB)
}

// ParseMemorySize parses a memory size string like "9M", "2G", "1024K"
func ParseMemorySize(s string) (MemorySize, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty memory size string")
	}

	// Check if it ends with a unit
	lastChar := s[len(s)-1:]
	var multiplier MemorySize = Byte
	var valueStr string

	switch strings.ToUpper(lastChar) {
	case "T":
		multiplier = TB
		valueStr = s[:len(s)-1]
	case "G":
		multiplier = GB
		valueStr = s[:len(s)-1]
	case "M":
		multiplier = MB
		valueStr = s[:len(s)-1]
	case "K":
		multiplier = KB
		valueStr = s[:len(s)-1]
	case "B":
		multiplier = Byte
		valueStr = s[:len(s)-1]
	default:
		// No unit, assume bytes
		multiplier = Byte
		valueStr = s
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory size: %s", s)
	}

	return MemorySize(value * float64(multiplier)), nil
}

// MustParseMemorySize is like ParseMemorySize but panics on error
func MustParseMemorySize(s string) MemorySize {
	size, err := ParseMemorySize(s)
	if err != nil {
		panic(err)
	}
	return size
}

// Add implements addition for MemorySize
func (m MemorySize) Add(other MemorySize) MemorySize {
	return m + other
}

// Sub implements subtraction for MemorySize
func (m MemorySize) Sub(other MemorySize) MemorySize {
	return m - other
}

// Mul implements multiplication for MemorySize
func (m MemorySize) Mul(factor float64) MemorySize {
	return MemorySize(float64(m) * factor)
}

// Div implements division for MemorySize
func (m MemorySize) Div(divisor float64) MemorySize {
	return MemorySize(float64(m) / divisor)
}

// Ratio returns the ratio of this memory size to another
func (m MemorySize) Ratio(other MemorySize) float64 {
	if other == 0 {
		return 0
	}
	return float64(m) / float64(other)
}

// MarshalJSON implements json.Marshaler
func (m MemorySize) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, m.String())), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (m *MemorySize) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	size, err := ParseMemorySize(str)
	if err != nil {
		return err
	}
	*m = size
	return nil
}
