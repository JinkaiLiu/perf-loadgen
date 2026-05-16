package util

import (
	"fmt"
	"time"
)

// FormatDuration returns a human-readable duration string.
// Unlike time.Duration.String() which may produce "1m0s",
// this produces more compact forms like "1.5s", "150ms", "10μs".
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000)
	case d >= time.Microsecond:
		return fmt.Sprintf("%.2fμs", float64(d.Nanoseconds())/1000)
	default:
		return d.String()
	}
}
