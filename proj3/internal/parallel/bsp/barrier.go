package bsp

import "sync"

// Barrier lets N goroutines wait until everybody reaches it once.
// It can be reused.
type Barrier struct {
	n     int // how many goroutines must call Wait
	count int
	mutex sync.Mutex
	cond  *sync.Cond
}

// New creates a new Barrier that will wait for n goroutines.
func New(n int) *Barrier {
	if n <= 0 {
		panic("barrier size must be positive")
	}
	b := &Barrier{n: n}
	b.cond = sync.NewCond(&b.mutex)
	return b
}

// Wait blocks until n goroutines have called Wait.
// After all n goroutines have arrived, they are all unblocked
// and the barrier resets for the next use.
func (b *Barrier) Wait() {
	b.mutex.Lock()
	b.count++
	if b.count < b.n {
		b.cond.Wait()
	} else {
		// Last goroutine arrived
		b.cond.Broadcast() // Wake up all waiting goroutines
		b.count = 0        // Reset for next use
	}
	b.mutex.Unlock()
}
