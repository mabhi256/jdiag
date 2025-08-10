package utils

import (
	"math"
	"time"
)

// MultiValueTimePoint for storing multiple related values
type MultiValueTimePoint struct {
	Timestamp time.Time
	Values    map[string]float64
}

func NewMultiValuePoint(timestamp time.Time) *MultiValueTimePoint {
	return &MultiValueTimePoint{
		Timestamp: timestamp,
		Values:    make(map[string]float64),
	}
}

func (m *MultiValueTimePoint) Set(name string, value float64) {
	m.Values[name] = value
}

func (m *MultiValueTimePoint) Get(name string) (float64, bool) {
	val, exists := m.Values[name]
	return val, exists
}

func (m *MultiValueTimePoint) GetOrDefault(name string, defaultValue float64) float64 {
	if val, exists := m.Values[name]; exists {
		return val
	}
	return defaultValue
}

func (m *MultiValueTimePoint) SetMemoryValues(used, committed, max int64) {
	m.Set("used_mb", float64(used)/(1024*1024))
	m.Set("committed_mb", float64(committed)/(1024*1024))

	if max > 0 {
		m.Set("max_mb", float64(max)/(1024*1024))
		m.Set("usage_percent", float64(used)/float64(max))
	}
}

func (m *MultiValueTimePoint) GetUsedMB() float64 {
	return m.GetOrDefault("used_mb", 0)
}

func (m *MultiValueTimePoint) GetCommittedMB() float64 {
	return m.GetOrDefault("committed_mb", 0)
}

func (m *MultiValueTimePoint) GetMaxMB() float64 {
	return m.GetOrDefault("max_mb", 0)
}

func (m *MultiValueTimePoint) GetUsagePercent() float64 {
	return m.GetOrDefault("usage_percent", 0)
}

func LinearRegression(x, y []float64) (slope, correlation float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0
	}

	n := float64(len(x))
	var sumX, sumY, sumXY, sumXX, sumYY float64

	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumXX += x[i] * x[i]
		sumYY += y[i] * y[i]
	}

	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	numerator := n*sumXY - sumX*sumY
	denominatorCorr := math.Sqrt((n*sumXX - sumX*sumX) * (n*sumYY - sumY*sumY))

	if denominatorCorr == 0 {
		correlation = 0
	} else {
		correlation = numerator / denominatorCorr
	}

	return slope, correlation
}

type Point interface {
	GetTimestamp() time.Time
}

// Generic trend calculation function
func CalculateTrend[T Point](points []T, window time.Duration, valueExtractor func(T) float64) (slope, correlation float64) {
	if len(points) < 2 {
		return 0, 0
	}

	// Filter points within window
	cutoff := time.Now().Add(-window)
	var windowPoints []T
	for _, point := range points {
		if point.GetTimestamp().After(cutoff) {
			windowPoints = append(windowPoints, point)
		}
	}

	if len(windowPoints) < 2 {
		return 0, 0
	}

	// Prepare X,Y arrays for LinearRegression utility
	x := make([]float64, len(windowPoints))
	y := make([]float64, len(windowPoints))
	for i, point := range windowPoints {
		x[i] = float64(i) // Use array index as time
		y[i] = valueExtractor(point)
	}

	slope, correlation = LinearRegression(x, y)

	if len(windowPoints) > 1 {
		timeSpan := windowPoints[len(windowPoints)-1].GetTimestamp().Sub(windowPoints[0].GetTimestamp())
		if timeSpan.Minutes() > 0 {
			slope = slope * (60.0 / timeSpan.Minutes())
		}
	}

	return slope, correlation
}

// Update your existing types to implement the Point interface
func (h TimePoint) GetTimestamp() time.Time {
	return h.Time
}

func (h MultiValueTimePoint) GetTimestamp() time.Time {
	return h.Timestamp
}

// Convenience functions that wrap the generic function
func CalculateHistoryTrend(points []TimePoint, window time.Duration) (slope, correlation float64) {
	return CalculateTrend(points, window, func(p TimePoint) float64 {
		return p.Value
	})
}

func CalculateMultiValueTrend(points []MultiValueTimePoint, fieldName string, window time.Duration) (slope, correlation float64) {
	return CalculateTrend(points, window, func(p MultiValueTimePoint) float64 {
		return p.GetOrDefault(fieldName, 0)
	})
}
