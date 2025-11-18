package bsp

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test that Barrier releases all goroutines only after N arrivals,
// and that it resets correctly for a second round.
func TestBarrierBasicAndReset(t *testing.T) {
	const workers = 5

	bar := New(workers)
	var (
		mu      sync.Mutex
		hits    []int
		wg      sync.WaitGroup
		arrived = make(chan struct{}, workers)
	)

	// spawn workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) { // id is the parameter for this goroutine
			defer wg.Done()

			// signal arrival
			mu.Lock()
			hits = append(hits, id*10) // pre-barrier marker
			mu.Unlock()

			arrived <- struct{}{}
			bar.Wait()

			mu.Lock()
			hits = append(hits, id*10+1) // post-barrier marker
			mu.Unlock()
		}(i) // Pass the loop variable i to the id parameter
	}

	// ensure all have signaled arrival before timeout
	timeout := time.After(500 * time.Millisecond)
	for i := 0; i < workers; i++ {
		select {
		case <-arrived:
		case <-timeout:
			t.Fatalf("timed out waiting for worker %d arrival", i)
		}
	}
	// now wait for everyone to pass barrier
	wg.Wait()

	// check that for each id there was pre- then post-
	seen := make(map[int]bool)
	for _, v := range hits {
		seen[v] = true
	}
	for i := 0; i < workers; i++ {
		if !seen[i*10] || !seen[i*10+1] {
			t.Errorf("worker %d did not record both pre and post barrier markers", i)
		}
	}

	// Second round should work too (count resets)
	hits = hits[:0] // Clear hits for the second round
	// wg = sync.WaitGroup{} // Re-initialize WaitGroup for the second round
	// The original wg.Wait() has completed, so we need a new one or Add to the existing one before new goroutines.
	// For clarity, let's re-initialize.
	var wg2 sync.WaitGroup // Use a new WaitGroup or re-initialize the existing one.

	for i := 0; i < workers; i++ {
		wg2.Add(1)
		go func(id int) { // id is the parameter for this goroutine
			defer wg2.Done()
			bar.Wait()
			mu.Lock()
			hits = append(hits, id*100+1) // Use the passed id
			mu.Unlock()
		}(i) // Pass the loop variable i to the id parameter
	}
	wg2.Wait() // Wait for the second batch of goroutines

	// Rebuild the 'seen' map for the second round's markers or adjust the check
	seenSecondRound := make(map[int]bool)
	for _, v := range hits {
		seenSecondRound[v] = true
	}

	// confirm all post-barrier markers recorded for the second round
	for i := 0; i < workers; i++ {
		if !seenSecondRound[i*100+1] { // Check against the new 'seen' map and correct marker value
			t.Fatalf("Round 2: worker %d did not record post-barrier marker %d. Hits: %v", i, i*100+1, hits)
		}
	}
	if len(hits) != workers {
		t.Fatalf("Round 2: expected %d post-barrier markers but got %d. Hits: %v", workers, len(hits), hits)
	}
}

func TestBarrierReuse(t *testing.T) {
	numGoroutines := 3
	numCycles := 3
	barrier := New(numGoroutines)
	var wg sync.WaitGroup
	counters := make([]atomic.Int32, numCycles) // counters[cycle] will count goroutines passing for that cycle

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		// Pass the loop variable 'i' as a parameter 'goroutineID' to the goroutine
		// to avoid capturing the loop variable in a closure, which can lead to unexpected behavior.
		go func(goroutineID int) {
			defer wg.Done()
			for cycle := 0; cycle < numCycles; cycle++ {
				// Simulate work specific to this goroutine and cycle
				// Using 'goroutineID' which is the correctly passed parameter.
				time.Sleep(time.Duration(goroutineID*5+cycle*2) * time.Millisecond)
				// t.Logf("Goroutine %d, Cycle %d reaching barrier", goroutineID, cycle)
				barrier.Wait()
				// t.Logf("Goroutine %d, Cycle %d passed barrier", goroutineID, cycle)
				counters[cycle].Add(1)
			}
		}(i) // Pass 'i' to the goroutineID parameter
	}

	wg.Wait() // Wait for all goroutines to complete all their cycles

	for cycle := 0; cycle < numCycles; cycle++ {
		if count := counters[cycle].Load(); count != int32(numGoroutines) {
			t.Errorf("Cycle %d: Expected %d goroutines to pass, got %d", cycle, numGoroutines, count)
		}
	}
	// t.Logf("All %d goroutines completed %d cycles.", numGoroutines, numCycles)
}

// ...existing code... TestBarrierSingleGoroutine, TestBarrierPanicZero, TestBarrierPanicNegative ...
