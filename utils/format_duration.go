package utils

import (
	"fmt"
	"math"
	"time"
)

func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1000000)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), math.Mod(d.Seconds(), 60))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - 60*hours
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
