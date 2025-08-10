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
