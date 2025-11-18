package ws

import (
	"math/rand"
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/textmatch"
	"runtime"
	"sync"
	"time"
)

// RankWS computes scores using a work-stealing approach.
func RankWS(queryString string, photos []meta.PhotoMetadata, topK int, numWorkers int) ([]meta.PhotoMetadata, time.Duration) {
	startTime := time.Now()

	if topK <= 0 || len(photos) == 0 {
		return nil, time.Since(startTime)
	}
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	if numWorkers > len(photos) { // Optimization
		numWorkers = len(photos)
	}

	deques := make([]*Deque, numWorkers)
	for i := 0; i < numWorkers; i++ {
		deques[i] = NewDeque()
	}

	// Distribute initial tasks
	var tasksWg sync.WaitGroup
	for i, photo := range photos {
		task := Task{Photo: photo, Query: queryString}
		deques[i%numWorkers].PushBottom(task) // Simple round-robin distribution
		tasksWg.Add(1)
	}

	collector := NewCollector(topK)
	quitSignal := make(chan struct{})
	var workerPoolWg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		workerPoolWg.Add(1)
		go func(workerID int, localDeque *Deque) {
			defer workerPoolWg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID))) // Per-worker RNG

			for {
				var task Task
				var found bool

				// Try local deque
				task, found = localDeque.PopBottom()

				if !found && numWorkers > 1 { // If not found locally, try to steal
					// Attempt to steal from a few random peers
					// More sophisticated stealing strategies exist (e.g., exponential backoff, work-conserving)
					// For simplicity, try a fixed number of random peers or iterate.
					numPeersToTry := numWorkers - 1
					if numPeersToTry > 3 { // Limit steal attempts per cycle
						numPeersToTry = 3
					}

					// Create a random permutation of other worker IDs to try stealing from
					peerOrder := rng.Perm(numWorkers)

					for _, peerIdx := range peerOrder {
						if peerIdx == workerID {
							continue // Don't steal from self
						}
						if numPeersToTry <= 0 {
							break
						}
						task, found = deques[peerIdx].PopTop() // PopTop is steal
						if found {
							break
						}
						numPeersToTry--
					}
				}

				if found {
					// Process task
					sPhoto := scoredPhoto{photo: task.Photo, score: textmatch.Score(task.Query, task.Photo)}
					collector.Push(sPhoto)
					tasksWg.Done()
				} else {
					// No work found locally or by stealing. Check if we should quit.
					select {
					case <-quitSignal:
						return
					default:
						// Brief pause or yield to prevent busy spinning when idle
						// and allow other goroutines (like tasksWg.Wait()) to proceed.
						runtime.Gosched()
					}
				}
			}
		}(i, deques[i])
	}

	tasksWg.Wait()      // Wait for all tasks to be processed
	close(quitSignal)   // Signal workers to exit
	workerPoolWg.Wait() // Wait for all worker goroutines to finish

	rankedPhotos := collector.GetRankedPhotos()
	duration := time.Since(startTime)
	return rankedPhotos, duration
}
