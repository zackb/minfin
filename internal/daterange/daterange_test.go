package daterange

import (
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday

	start, end := Resolve("this-week", now)
	if start.Format("2006-01-02") != "2026-06-15" { // Monday
		t.Errorf("this-week start = %s, want 2026-06-15", start.Format("2006-01-02"))
	}
	if end.Format("2006-01-02") != "2026-06-18" {
		t.Errorf("this-week end = %s, want 2026-06-18", end.Format("2006-01-02"))
	}

	start, _ = Resolve("last-30-days", now)
	if start.Format("2006-01-02") != "2026-05-19" {
		t.Errorf("last-30-days start = %s, want 2026-05-19", start.Format("2006-01-02"))
	}

	// Unknown falls back to last-30-days.
	s2, _ := Resolve("bogus", now)
	if !s2.Equal(start) {
		t.Errorf("unknown preset should default to last-30-days")
	}
}
