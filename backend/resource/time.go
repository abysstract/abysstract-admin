package resource

import "time"

// NormalizeTimestamp defines the common database time representation: convert to UTC, then truncate toward zero to
// microsecond precision (never round up).
func NormalizeTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(time.Microsecond)
}
