package engine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTrackerConfiguration(t *testing.T) {
	// Test that tracker doesn't start when disabled
	config := GetConfigReference()
	originalEnabled := config.TrackerConfig.Enabled
	config.TrackerConfig.Enabled = false

	// Reset tracker state
	trackerChannel = nil
	atomic.StoreInt32(&trackerEnabled, 0)

	StartTracker(4)

	// Verify tracker didn't start
	if trackerChannel != nil {
		t.Error("Tracker channel should be nil when disabled")
	}

	if IsTrackerEnabled() {
		t.Error("Tracker should be disabled")
	}

	// Restore original config
	config.TrackerConfig.Enabled = originalEnabled
}

func TestTrackerSafeChannelSend(t *testing.T) {
	// Test that sending to nil channel doesn't panic
	trackerChannel = nil
	atomic.StoreInt32(&trackerEnabled, 0)

	// This should not panic
	entry := TrackerEntry{
		LogId:   "test",
		BoxId:   "test",
		BoxName: "test",
	}

	// Simulate the check from engine.go
	if trackerChannel != nil && IsTrackerEnabled() {
		select {
		case trackerChannel <- entry:
		default:
		}
	}

	// If we get here without panic, test passed
}

func TestTrackerPerformance(t *testing.T) {
	// Skip if tracker is disabled in config
	config := GetConfigReference()
	if !config.TrackerConfig.Enabled {
		t.Skip("Tracker is disabled in configuration")
	}

	// Measure performance impact
	start := time.Now()

	// Send many entries
	for i := 0; i < 10000; i++ {
		entry := TrackerEntry{
			LogId:       "test",
			BoxId:       "test",
			BoxName:     "test",
			BoxType:     "test",
			Diff:        time.Millisecond * 100,
			JSONPayload: []byte(`{"test": "data"}`),
		}

		if trackerChannel != nil && IsTrackerEnabled() {
			select {
			case trackerChannel <- entry:
			default:
			}
		}
	}

	elapsed := time.Since(start)

	// Should complete in less than 100ms even with 10k entries
	if elapsed > 100*time.Millisecond {
		t.Errorf("Tracker performance issue: took %v for 10k entries", elapsed)
	}
}
