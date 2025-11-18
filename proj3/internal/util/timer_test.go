package util

import (
	"math"
	"testing"
	"time"
)

func TestTimer_Ms(t *testing.T) {
	timer := NewTimer()
	sleepDuration := 100 * time.Millisecond
	time.Sleep(sleepDuration)
	elapsedMs := timer.Ms()

	expectedMs := float64(sleepDuration.Milliseconds())
	tolerance := 20.0 // Allow 20ms tolerance for scheduling variations

	if math.Abs(elapsedMs-expectedMs) > tolerance {
		t.Errorf("Expected elapsed time to be around %.1f ms, but got %.1f ms", expectedMs, elapsedMs)
	}
}
