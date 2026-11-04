package resource

import (
	"testing"
	"time"
)

func TestNormalizeTimestampTruncatesToMicroseconds(t *testing.T) {
	input := time.Date(2026, 8, 3, 12, 34, 56, 123456789, time.FixedZone("UTC+8", 8*60*60))
	got := NormalizeTimestamp(input)
	want := time.Date(2026, 8, 3, 4, 34, 56, 123456000, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NormalizeTimestamp(%s) = %s, want %s", input, got, want)
	}
	if got.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", got.Location())
	}
}
