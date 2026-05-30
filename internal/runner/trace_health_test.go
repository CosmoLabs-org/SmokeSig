package runner

import (
	"testing"
)

func TestTraceHealthTracker_RecordsResults(t *testing.T) {
	tr := NewTraceHealthTracker(5)
	tr.Record(true)
	tr.Record(true)
	tr.Record(false)
	if tr.Total() != 3 {
		t.Errorf("Total() = %d, want 3", tr.Total())
	}
	if tr.Confirmed() != 2 {
		t.Errorf("Confirmed() = %d, want 2", tr.Confirmed())
	}
}

func TestTraceHealthTracker_HealthPercentage(t *testing.T) {
	tr := NewTraceHealthTracker(10)
	if tr.HealthPct() != 100.0 {
		t.Errorf("empty health = %.1f, want 100.0", tr.HealthPct())
	}
	tr.Record(true)
	tr.Record(true)
	tr.Record(false)
	if got := tr.HealthPct(); got != 66.7 {
		t.Errorf("HealthPct() = %.1f, want 66.7", got)
	}
}

func TestTraceHealthTracker_SlidingWindow(t *testing.T) {
	tr := NewTraceHealthTracker(3)
	tr.Record(true)
	tr.Record(true)
	tr.Record(true)
	// Window is full, all passing → 100%
	if tr.HealthPct() != 100.0 {
		t.Errorf("HealthPct() = %.1f, want 100.0", tr.HealthPct())
	}
	// Push one failure — slides out oldest success
	tr.Record(false)
	if tr.Total() != 3 {
		t.Errorf("Total() = %d, want 3 (window size)", tr.Total())
	}
	if got := tr.HealthPct(); got != 66.7 {
		t.Errorf("HealthPct() = %.1f, want 66.7 after sliding", got)
	}
}

func TestTraceHealthTracker_Degraded(t *testing.T) {
	tr := NewTraceHealthTracker(5)
	if tr.Degraded(50.0) {
		t.Error("empty tracker should not be degraded")
	}
	tr.Record(true)
	if tr.Degraded(50.0) {
		t.Error("100% health should not be degraded")
	}
	tr.Record(false)
	tr.Record(false)
	// 1/3 = 33.3%, below 50% threshold
	if !tr.Degraded(50.0) {
		t.Error("expected degraded at 33.3% < 50% threshold")
	}
}

func TestTraceHealthTracker_ZeroWindowSize(t *testing.T) {
	tr := NewTraceHealthTracker(0)
	if tr == nil {
		t.Fatal("expected non-nil tracker")
	}
	// windowSize < 1 should default to 10
	tr.Record(true)
	tr.Record(true)
	tr.Record(false)
	if tr.Total() != 3 {
		t.Errorf("Total() = %d, want 3", tr.Total())
	}
	if tr.Confirmed() != 2 {
		t.Errorf("Confirmed() = %d, want 2", tr.Confirmed())
	}
}

func TestTraceHealthTracker_NegativeWindowSize(t *testing.T) {
	tr := NewTraceHealthTracker(-5)
	tr.Record(true)
	if tr.Total() != 1 {
		t.Errorf("Total() = %d, want 1", tr.Total())
	}
	// 1 confirmed out of 1 total = 100% health, should NOT be degraded at 50% threshold
	if tr.Degraded(50.0) {
		t.Error("100% health should not be degraded")
	}
	// Now record failures to push below threshold
	tr.Record(false)
	tr.Record(false)
	// 1/3 = 33.3% < 50%, should be degraded
	if !tr.Degraded(50.0) {
		t.Error("33.3% health should be degraded at 50% threshold")
	}
}

func TestTraceHealthTracker_Reset(t *testing.T) {
	tr := NewTraceHealthTracker(3)
	tr.Record(false)
	tr.Record(false)
	tr.Reset()
	if tr.Total() != 0 {
		t.Errorf("Total() after reset = %d, want 0", tr.Total())
	}
	if tr.Confirmed() != 0 {
		t.Errorf("Confirmed() after reset = %d, want 0", tr.Confirmed())
	}
}
