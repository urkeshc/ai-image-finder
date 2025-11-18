package bsp

import (
	"container/heap"
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/textmatch" // For textmatch.Score
	"sort"
	"sync"
	"time"
)

// scoredPhoto holds a photo and its calculated score for ranking.
// Copied from internal/textmatch/score.go for local use.
type scoredPhoto struct {
	photo meta.PhotoMetadata
	score float32
}

// photoHeap implements heap.Interface for a min-heap of scoredPhoto.
// Copied from internal/textmatch/score.go for local use.
type photoHeap []scoredPhoto

func (h photoHeap) Len() int { return len(h) }
func (h photoHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score // score ascending for min-heap
	}
	if h[i].photo.StatsDownloads != h[j].photo.StatsDownloads {
		return h[i].photo.StatsDownloads < h[j].photo.StatsDownloads
	}
	return h[i].photo.PhotoID > h[j].photo.PhotoID
}
func (h photoHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *photoHeap) Push(x interface{}) { *h = append(*h, x.(scoredPhoto)) }
func (h *photoHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// RankBSP computes scores for each photo against the query in parallel using numWorkers
// and returns the top-k photos and the duration of the ranking process.
// Ties are broken by stats_downloads (descending) then photo_id (ascending).
func RankBSP(query string, photos []meta.PhotoMetadata, k int, numWorkers int) ([]meta.PhotoMetadata, time.Duration) {
	startTime := time.Now()

	if k <= 0 || len(photos) == 0 {
		return nil, time.Since(startTime)
	}
	if numWorkers <= 0 {
		numWorkers = 1 // Default to sequential if invalid numWorkers
	}
	if numWorkers > len(photos) { // Optimization: don't use more workers than photos
		numWorkers = len(photos)
	}

	barrier := New(numWorkers)
	localHeaps := make([]*photoHeap, numWorkers)
	var wg sync.WaitGroup

	chunkSize := (len(photos) + numWorkers - 1) / numWorkers // Ceiling division

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := (i + 1) * chunkSize
		if end > len(photos) {
			end = len(photos)
		}
		photoChunk := photos[start:end]

		localHeaps[i] = &photoHeap{}
		heap.Init(localHeaps[i])

		go func(workerID int, chunk []meta.PhotoMetadata, localH *photoHeap) {
			defer wg.Done()

			for _, p := range chunk {
				s := textmatch.Score(query, p) // Use Score from textmatch package
				currentScoredPhoto := scoredPhoto{photo: p, score: s}

				if localH.Len() < k {
					heap.Push(localH, currentScoredPhoto)
				} else if localH.Len() > 0 {
					root := (*localH)[0]
					shouldReplace := false
					if currentScoredPhoto.score > root.score {
						shouldReplace = true
					} else if currentScoredPhoto.score == root.score {
						if currentScoredPhoto.photo.StatsDownloads > root.photo.StatsDownloads {
							shouldReplace = true
						} else if currentScoredPhoto.photo.StatsDownloads == root.photo.StatsDownloads {
							if currentScoredPhoto.photo.PhotoID < root.photo.PhotoID {
								shouldReplace = true
							}
						}
					}
					if shouldReplace {
						heap.Pop(localH)
						heap.Push(localH, currentScoredPhoto)
					}
				}
			}
			barrier.Wait() // Worker finished its local heap, wait for others
		}(i, photoChunk, localHeaps[i])
	}

	wg.Wait() // Wait for all goroutines to finish (including passing the barrier)

	// Reduce phase (done by the main goroutine after all workers passed the barrier)
	// Worker 0 could do this, but it's simpler for the main goroutine that orchestrates.
	finalHeap := &photoHeap{}
	heap.Init(finalHeap)

	for _, localH := range localHeaps {
		for localH.Len() > 0 {
			sp := heap.Pop(localH).(scoredPhoto)
			// Push sp onto finalHeap, maintaining top-k
			if finalHeap.Len() < k {
				heap.Push(finalHeap, sp)
			} else {
				root := (*finalHeap)[0]
				shouldReplace := false
				if sp.score > root.score {
					shouldReplace = true
				} else if sp.score == root.score {
					if sp.photo.StatsDownloads > root.photo.StatsDownloads {
						shouldReplace = true
					} else if sp.photo.StatsDownloads == root.photo.StatsDownloads {
						if sp.photo.PhotoID < root.photo.PhotoID {
							shouldReplace = true
						}
					}
				}
				if shouldReplace {
					heap.Pop(finalHeap)
					heap.Push(finalHeap, sp)
				}
			}
		}
	}

	// Extract photos from finalHeap and sort them.
	numResults := finalHeap.Len()
	scoredResults := make([]scoredPhoto, numResults)
	for i := numResults - 1; i >= 0; i-- { // Pop smallest first, fill from end for descending sort later
		scoredResults[i] = heap.Pop(finalHeap).(scoredPhoto)
	}

	// Sort the final k items: score desc, downloads desc, photoID asc
	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		if scoredResults[i].photo.StatsDownloads != scoredResults[j].photo.StatsDownloads {
			return scoredResults[i].photo.StatsDownloads > scoredResults[j].photo.StatsDownloads
		}
		return scoredResults[i].photo.PhotoID < scoredResults[j].photo.PhotoID
	})

	out := make([]meta.PhotoMetadata, numResults)
	for i, sp := range scoredResults {
		out[i] = sp.photo
	}

	duration := time.Since(startTime)
	return out, duration
}
