package pipeline

import (
	"container/heap"
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/textmatch"
	"sort"
	"sync"
)

// scoredPhoto holds a photo and its calculated score.
type scoredPhoto struct {
	photo meta.PhotoMetadata
	score float32
}

// photoHeap implements heap.Interface for a min-heap of scoredPhoto.
// This is identical to the heap logic in bsp/rank.go and textmatch/score.go.
type photoHeap []scoredPhoto

func (h photoHeap) Len() int { return len(h) }
func (h photoHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
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

// runScorerWorkers spawns numScorers goroutines.
// Each goroutine reads photos from candidatesCh, scores them against queryString,
// and sends the scoredPhoto to resultsCh.
func runScorerWorkers(
	candidatesCh <-chan meta.PhotoMetadata,
	resultsCh chan<- scoredPhoto,
	queryString string,
	numScorers int,
	scorerWg *sync.WaitGroup,
) {
	for i := 0; i < numScorers; i++ {
		scorerWg.Add(1)
		go func() {
			defer scorerWg.Done()
			for photo := range candidatesCh {
				score := textmatch.Score(queryString, photo)
				resultsCh <- scoredPhoto{photo: photo, score: score}
			}
		}()
	}
}

// collectResults reads scoredPhotos from resultsCh, maintains a top-K heap,
// and returns the final sorted list of PhotoMetadata.
func collectResults(resultsCh <-chan scoredPhoto, topK int) []meta.PhotoMetadata {
	if topK <= 0 {
		// Drain resultsCh to prevent deadlocks if topK is invalid, though RankPipeline should guard this.
		for range resultsCh {
		}
		return nil
	}

	h := &photoHeap{}
	heap.Init(h)

	for sp := range resultsCh {
		if h.Len() < topK {
			heap.Push(h, sp)
		} else if h.Len() > 0 {
			root := (*h)[0]
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
				heap.Pop(h)
				heap.Push(h, sp)
			}
		}
	}

	// Extract photos from heap and sort them.
	numResults := h.Len()
	scoredResults := make([]scoredPhoto, numResults)
	for i := numResults - 1; i >= 0; i-- { // Pop smallest first, fill from end
		scoredResults[i] = heap.Pop(h).(scoredPhoto)
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
	for i, res := range scoredResults {
		out[i] = res.photo
	}
	return out
}
