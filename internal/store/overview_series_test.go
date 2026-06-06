package store

import (
	"testing"
	"time"
)

func TestPipelineDelta(t *testing.T) {
	if got := pipelineDelta([]int{100, 118}); got != "▲ 18.0%" {
		t.Fatalf("got %q", got)
	}
	if got := pipelineDelta([]int{0, 10}); got != "live" {
		t.Fatalf("got %q", got)
	}
}

func TestOverviewRangeLabel(t *testing.T) {
	if got := overviewRangeLabel("quarter"); got != "Quarter" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateUTC(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-06-06T14:32:00Z")
	trunc := truncateUTC(ts, "hour")
	if trunc.Hour() != 14 || trunc.Minute() != 0 {
		t.Fatalf("unexpected truncate: %v", trunc)
	}
}
