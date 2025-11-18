package util

import (
	"time"
)

// Timer helps measure elapsed time.
type Timer struct {
	start time.Time
}

// NewTimer creates and starts a new Timer.
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// Ms returns the elapsed time in milliseconds since the Timer was started.
func (t *Timer) Ms() float64 {
	return float64(time.Since(t.start).Nanoseconds()) / 1e6
}
