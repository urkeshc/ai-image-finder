package pipeline

import (
	"proj3-redesigned/internal/meta"
	"runtime"
	"sync"
)

// Channel type definitions (can be kept or removed, not strictly necessary)
// type photosChType chan meta.PhotoMetadata // This specific one is removed
type candidatesChType chan meta.PhotoMetadata // Conceptually, this is what scorers read
type resultsChType chan scoredPhoto           // scoredPhoto is defined in scorer.go

// RankScoringPipeline processes a slice of candidate photos through a parallel scoring pipeline.
// queryString: The raw user query string for scoring.
// candidatePhotos: The slice of photos that have already passed metadata filtering.
// topK: The number of top results to return.
// numScorers: The number of goroutines for the scoring stage.
func RankScoringPipeline(queryString string, candidatePhotos []meta.PhotoMetadata, topK, numScorers int) []meta.PhotoMetadata {
	if topK <= 0 || len(candidatePhotos) == 0 {
		return nil
	}
	if numScorers <= 0 {
		numScorers = runtime.NumCPU() // Default to NumCPU if invalid
	}
	if numScorers > len(candidatePhotos) { // Optimization
		numScorers = len(candidatePhotos)
	}

	// 1. Create channels
	// photosToScoreCh will carry the already-filtered candidate photos to the scorers.
	photosToScoreCh := make(chan meta.PhotoMetadata, len(candidatePhotos))
	resultsCh := make(chan scoredPhoto, numScorers*2) // Buffer for scored results

	// 2. Setup WaitGroup for scorer workers
	var scorerWg sync.WaitGroup

	// 3. Start the collector goroutine
	finalRankedPhotosCh := make(chan []meta.PhotoMetadata)
	go func() {
		finalRankedPhotosCh <- collectResults(resultsCh, topK)
		close(finalRankedPhotosCh)
	}()

	// 4. Start scorer workers
	// These read from photosToScoreCh and write to resultsCh.
	// resultsCh should be closed after all scorers are done.
	runScorerWorkers(photosToScoreCh, resultsCh, queryString, numScorers, &scorerWg)
	go func() {
		scorerWg.Wait()
		close(resultsCh)
	}()

	// 5. Feed candidate photos into the scoring pipeline
	for i := range candidatePhotos {
		photosToScoreCh <- candidatePhotos[i]
	}
	close(photosToScoreCh) // Signal scorer workers that no more photos are coming

	// 6. Wait for and return the final result from the collector
	return <-finalRankedPhotosCh
}
