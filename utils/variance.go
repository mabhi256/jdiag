package utils

import (
	"time"
)

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Calculate mean of numeric values
func CalculateMean[T Numeric](values []T) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += float64(v)
	}
	return sum / float64(len(values))
}

// Calculates both population and sample variance
func calculateVarianceCore[T Numeric](values []T, mean float64, isSample bool) float64 {
	if len(values) < 2 {
		return 0
	}

	variance := 0.0
	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}

	denominator := len(values)
	if isSample {
		denominator--
	}

	return variance / float64(denominator)
}

// Population variance (divides by n)
func CalculateVariance[T Numeric](values []T, mean T) float64 {
	return calculateVarianceCore(values, float64(mean), false)
}

// Sample variance (divides by n-1)
func CalculateSampleVariance[T Numeric](values []T, mean T) float64 {
	return calculateVarianceCore(values, float64(mean), true)
}

// Calculate both variance and mean in one pass
func CalculateVarianceWithMean[T Numeric](values []T) (variance float64, mean float64) {
	if len(values) < 2 {
		return 0, 0
	}

	mean = CalculateMean(values)
	variance = calculateVarianceCore(values, mean, false)
	return variance, mean
}

// Coefficient of variation squared (normalized variance)
func CalculateNormalizedVariance[T Numeric](values []T, mean T) float64 {
	variance := CalculateVariance(values, mean)
	meanFloat := float64(mean)
	if meanFloat > 0 {
		return variance / (meanFloat * meanFloat)
	}
	return 0
}

// Duration variance calculation using normalized variance
func CalculateDurationVariance(durations []time.Duration, avgPause time.Duration) float64 {
	if len(durations) < 2 {
		return 0
	}

	// Convert to nanoseconds and calculate normalized variance
	nanos := make([]int64, len(durations))
	for i, d := range durations {
		nanos[i] = d.Nanoseconds()
	}

	return CalculateNormalizedVariance(nanos, avgPause.Nanoseconds())
}
