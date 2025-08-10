package utils

import (
	"math"
	"time"
)

// TimeMap for storing multiple related values
type TimeMap struct {
	Timestamp time.Time
	Values    map[string]float64
}

func NewTimeMap(timestamp time.Time) *TimeMap {
	return &TimeMap{
		Timestamp: timestamp,
		Values:    make(map[string]float64),
	}
}

func (m *TimeMap) Set(name string, value float64) {
	m.Values[name] = value
}

func (m *TimeMap) Get(name string) (float64, bool) {
	val, exists := m.Values[name]
	return val, exists
}

func (m *TimeMap) GetOrDefault(name string, defaultValue float64) float64 {
	if val, exists := m.Values[name]; exists {
		return val
	}
	return defaultValue
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
