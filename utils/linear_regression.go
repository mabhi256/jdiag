package utils

import (
	"math"
	"time"
)

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

type HistoryPoint struct {
	Timestamp time.Time
	Value     float64
}

func CalculateTrend(points []HistoryPoint, window time.Duration) (slope, correlation float64) {
	if len(points) < 2 {
		return 0, 0
	}

	// Filter points within window
	cutoff := time.Now().Add(-window)
	var windowPoints []HistoryPoint
	for _, point := range points {
		if point.Timestamp.After(cutoff) {
			windowPoints = append(windowPoints, point)
		}
	}

	if len(windowPoints) < 2 {
		return 0, 0
	}

	// Prepare X,Y arrays for your LinearRegression utility
	x := make([]float64, len(windowPoints))
	y := make([]float64, len(windowPoints))

	for i, point := range windowPoints {
		x[i] = float64(i) // Use array index as time
		y[i] = point.Value
	}

	slope, correlation = LinearRegression(x, y)

	// Convert to per-minute rate if we have time span
	if len(windowPoints) > 1 {
		timeSpan := windowPoints[len(windowPoints)-1].Timestamp.Sub(windowPoints[0].Timestamp)
		if timeSpan.Minutes() > 0 {
			slope = slope * (60.0 / timeSpan.Minutes())
		}
	}

	return slope, correlation
}
