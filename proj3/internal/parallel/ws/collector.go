package ws

import (
	"container/heap"
	"proj3-redesigned/internal/meta"
	"sort"
	"sync"
)

// scoredPhoto holds a photo and its calculated score.
type scoredPhoto struct {
	photo meta.PhotoMetadata
	score float32
}

// photoHeap implements heap.Interface for a min-heap of scoredPhoto.
type photoHeap []scoredPhoto

func (h photoHeap) Len() int { return len(h) }
func (h photoHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
	}
	if h[i].photo.StatsDownloads != h[j].photo.StatsDownloads {
		return h[i].photo.StatsDownloads < h[j].photo.StatsDownloads
	}
	return h[i].photo.PhotoID > h[j].photo.PhotoID // Min-heap tie-breaking for PhotoID
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

// Collector manages the top-K scored photos in a thread-safe manner.
type Collector struct {
	mu   sync.Mutex
	h    *photoHeap
	topK int
}

// NewCollector creates a new collector for top-K results.
func NewCollector(topK int) *Collector {
	h := &photoHeap{}
	heap.Init(h)
	return &Collector{
		h:    h,
		topK: topK,
	}
}

// Push adds a scoredPhoto to the collector, maintaining the top-K invariant.
func (c *Collector) Push(sp scoredPhoto) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.topK <= 0 {
		return
	}

	if c.h.Len() < c.topK {
		heap.Push(c.h, sp)
	} else if c.h.Len() > 0 {
		root := (*c.h)[0] // Smallest element in the min-heap
		// Determine if sp is "greater" than root for replacement
		shouldReplace := false
		if sp.score > root.score {
			shouldReplace = true
		} else if sp.score == root.score {
			if sp.photo.StatsDownloads > root.photo.StatsDownloads {
				shouldReplace = true
			} else if sp.photo.StatsDownloads == root.photo.StatsDownloads {
				if sp.photo.PhotoID < root.photo.PhotoID { // Prefer smaller ID for "greater"
					shouldReplace = true
				}
			}
		}
		if shouldReplace {
			heap.Pop(c.h)
			heap.Push(c.h, sp)
		}
	}
}

// GetRankedPhotos extracts the top-K photos, sorted appropriately.
func (c *Collector) GetRankedPhotos() []meta.PhotoMetadata {
	c.mu.Lock()
	defer c.mu.Unlock()

	numResults := c.h.Len()
	if numResults == 0 {
		return nil
	}

	scoredResults := make([]scoredPhoto, numResults)
	for i := numResults - 1; i >= 0; i-- { // Pop smallest first, fill from end
		scoredResults[i] = heap.Pop(c.h).(scoredPhoto)
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
