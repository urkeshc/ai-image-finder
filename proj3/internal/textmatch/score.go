package textmatch

import (
	"container/heap"
	"math" // Added for Log
	"proj3-redesigned/internal/meta"
	"sort"
	"strings" // Added for strings.Builder
)

/* ---------- scoring ---------- */

const (
	originalWordMatchBonus = 10.0
	synonymMatchBonus      = 1.5 // Original photo word matches a synonym of an original query word
)

// Score calculates a weighted score based on overlapping tokens.
func Score(query string, photo meta.PhotoMetadata) float32 {
	originalQueryTokenSet := getStemmedTokensSet(query, false) // Stemmed original query words (no synonyms)
	allQueryTokensSet := getStemmedTokensSet(query, true)      // Stemmed original query words + their stemmed synonyms

	// Construct photoText from relevant fields
	var sb strings.Builder
	if photo.PhotoDescription != "" {
		sb.WriteString(photo.PhotoDescription)
		sb.WriteString(" ")
	}
	if photo.AiDescription != "" {
		sb.WriteString(photo.AiDescription)
		sb.WriteString(" ")
	}
	if photo.PhotoLocationCountry != "" {
		sb.WriteString(photo.PhotoLocationCountry)
		sb.WriteString(" ")
	}
	if photo.PhotoLocationCity != "" {
		sb.WriteString(photo.PhotoLocationCity)
		sb.WriteString(" ")
	}
	// Consider adding PhotoLocationName if it's often useful and not too generic
	// if photo.PhotoLocationName != "" {
	//  sb.WriteString(photo.PhotoLocationName)
	//  sb.WriteString(" ")
	// }
	photoText := sb.String()

	// Get original (non-expanded by synonyms) stemmed tokens from the photoText
	photoOriginalsSet := getStemmedTokensSet(photoText, false)

	if len(photoOriginalsSet) == 0 {
		return 0.0 // No scorable tokens in the photo
	}

	var rawScore float32 = 0.0

	for pOrigToken := range photoOriginalsSet {
		if _, isOriginalQueryWord := originalQueryTokenSet[pOrigToken]; isOriginalQueryWord {
			// Original photo word matches an original query word
			rawScore += originalWordMatchBonus
		} else if _, isInAllQueryTokens := allQueryTokensSet[pOrigToken]; isInAllQueryTokens {
			// Original photo word is not an original query word,
			// but it IS in the set of (original query words + their synonyms).
			// This means pOrigToken is a synonym of an original query word.
			rawScore += synonymMatchBonus
		}
	}

	if rawScore == 0.0 {
		return 0.0
	}

	numPhotoOriginalTokens := len(photoOriginalsSet)
	// Normalization factor should not be zero if rawScore > 0,
	// because numPhotoOriginalTokens would be > 0.
	// log(1+N) where N = numPhotoOriginalTokens.
	normalizationFactor := math.Log(1.0 + float64(numPhotoOriginalTokens))

	if normalizationFactor <= 1e-6 { // Should not happen if numPhotoOriginalTokens > 0
		return rawScore // Fallback
	}

	return rawScore / float32(normalizationFactor)
}

/* ---------- ranking (top-k heap) ---------- */

// scoredPhoto holds a photo and its calculated score for ranking.
type scoredPhoto struct {
	photo meta.PhotoMetadata
	score float32 // Changed to float32
}

// photoHeap implements heap.Interface for a min-heap of scoredPhoto.
// The heap is ordered by score (ascending), then stats_downloads (ascending),
// then photo_id (descending).
type photoHeap []scoredPhoto

func (h photoHeap) Len() int { return len(h) }
func (h photoHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score // score ascending for min-heap
	}
	if h[i].photo.StatsDownloads != h[j].photo.StatsDownloads {
		// Fewer downloads means "less preferred" if scores are equal.
		return h[i].photo.StatsDownloads < h[j].photo.StatsDownloads
	}
	// Larger PhotoID means "less preferred" if scores and downloads are equal.
	return h[i].photo.PhotoID > h[j].photo.PhotoID
}
func (h photoHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *photoHeap) Push(x interface{}) {
	*h = append(*h, x.(scoredPhoto))
}

func (h *photoHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Rank computes scores for each photo against the query and returns the top-k photos.
// Ties are broken by stats_downloads (descending) then photo_id (ascending).
func Rank(query string, photos []meta.PhotoMetadata, k int) []meta.PhotoMetadata {
	if k <= 0 || len(photos) == 0 {
		return nil
	}

	h := &photoHeap{}
	heap.Init(h)

	for _, p := range photos {
		s := Score(query, p) // Score now returns float32
		currentScoredPhoto := scoredPhoto{photo: p, score: s}

		if h.Len() < k {
			heap.Push(h, currentScoredPhoto)
		} else if h.Len() > 0 {
			root := (*h)[0] // Smallest element in the min-heap
			// We want to replace the root if currentScoredPhoto is "greater"
			// "Greater" means: higher score OR same score & more downloads OR same score/downloads & smaller PhotoID
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
				heap.Pop(h)
				heap.Push(h, currentScoredPhoto)
			}
		}
	}

	// Extract photos from heap and sort them for the final ranked list.
	scored := make([]scoredPhoto, h.Len())
	for i := 0; h.Len() > 0; i++ { // Pop until heap is empty
		scored[i] = heap.Pop(h).(scoredPhoto)
	}

	// The heap pops smallest first. We need to sort these k items
	// in descending order of score, then descending downloads, then ascending PhotoID.
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score // Score descending
		}
		if scored[i].photo.StatsDownloads != scored[j].photo.StatsDownloads {
			return scored[i].photo.StatsDownloads > scored[j].photo.StatsDownloads // Downloads descending
		}
		return scored[i].photo.PhotoID < scored[j].photo.PhotoID // PhotoID ascending
	})

	out := make([]meta.PhotoMetadata, len(scored))
	for i, sp := range scored {
		out[i] = sp.photo
	}
	return out
}
