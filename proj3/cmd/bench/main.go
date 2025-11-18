package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/parallel/bsp"
	"proj3-redesigned/internal/parallel/pipeline"
	"proj3-redesigned/internal/parallel/ws"
	"proj3-redesigned/internal/query"
	"proj3-redesigned/internal/textmatch"
	"runtime"
	"time"
)

func main() {
	modeFlag := flag.String("mode", "seq", "Execution mode: 'seq', 'bsp', 'pipeline', or 'ws'.")
	threadsFlag := flag.Int("threads", runtime.NumCPU(), "Number of threads/workers/scorers for parallel modes.")
	sizeFlag := flag.Int("size", 1000000, "Number of photos to load from the duplicated dataset.")
	queryStrFlag := flag.String("query", "landscape winter snow mountains", "Textual query for AI ranking.")
	topKFlag := flag.Int("topk", 10, "Number of top results to return.")
	datasetPathFlag := flag.String("dataset", "data/metadata_big.jsonl", "Path to the (duplicated) dataset JSONL file.")

	flag.Parse()

	if *queryStrFlag == "" {
		log.Fatal("Error: --query cannot be empty.")
	}

	// Load metadata
	// fmt.Fprintf(os.Stderr, "Loading metadata from %s...\n", *datasetPathFlag)
	allPhotos, err := meta.LoadMetadataFromJSONL(*datasetPathFlag)
	if err != nil {
		log.Fatalf("Error loading metadata from %s: %v", *datasetPathFlag, err)
	}

	if len(allPhotos) == 0 {
		log.Fatalf("No photo metadata loaded from %s.", *datasetPathFlag)
	}

	actualSize := *sizeFlag
	if *sizeFlag > 0 && *sizeFlag < len(allPhotos) {
		allPhotos = allPhotos[:*sizeFlag]
	} else if *sizeFlag <= 0 || *sizeFlag >= len(allPhotos) {
		actualSize = len(allPhotos) // Use all loaded photos if size is 0, negative, or too large
	}

	if len(allPhotos) == 0 {
		log.Fatalf("Photo set is empty after applying size %d.", actualSize)
	}
	// fmt.Fprintf(os.Stderr, "Loaded %d photos for benchmark.\n", len(allPhotos))

	// Simulate initial query parsing (not timed as part of ranking)
	// fmt.Fprintf(os.Stderr, "Parsing query: %s\n", *queryStrFlag)
	parsedQuery, _, err := query.Parse(*queryStrFlag)
	if err != nil {
		log.Fatalf("Error parsing query '%s': %v", *queryStrFlag, err)
	}

	// Simulate metadata filtering (not timed as part of ranking)
	// fmt.Fprintf(os.Stderr, "Filtering candidates...\n")
	candidatePhotos := meta.FilterPhotos(parsedQuery, allPhotos)
	if len(candidatePhotos) == 0 {
		// If no candidates after filtering, the ranking phase might be very fast or do nothing.
		// This is acceptable, the benchmark will report the time for ranking 0 candidates.
		// fmt.Fprintf(os.Stderr, "Warning: No candidates after metadata filtering for query '%s'. Ranking will be on an empty set.\n", *queryStrFlag)
	}
	// fmt.Fprintf(os.Stderr, "Filtered to %d candidates.\n", len(candidatePhotos))

	var rankedPhotos []meta.PhotoMetadata // To store results, mostly to ensure code runs
	var rankDuration time.Duration

	actualThreads := *threadsFlag
	if *modeFlag == "seq" {
		actualThreads = 1 // Sequential mode is always 1 thread for reporting
	}

	// --- Ranking Phase ---
	// fmt.Fprintf(os.Stderr, "Starting ranking phase: mode=%s, threads=%d\n", *modeFlag, actualThreads)
	startTime := time.Now()

	switch *modeFlag {
	case "seq":
		rankedPhotos = textmatch.Rank(*queryStrFlag, candidatePhotos, *topKFlag)
		rankDuration = time.Since(startTime)
	case "bsp":
		rankedPhotos, rankDuration = bsp.RankBSP(*queryStrFlag, candidatePhotos, *topKFlag, *threadsFlag)
	case "pipeline":
		rankedPhotos = pipeline.RankScoringPipeline(*queryStrFlag, candidatePhotos, *topKFlag, *threadsFlag)
		rankDuration = time.Since(startTime) // RankScoringPipeline doesn't return duration directly
	case "ws":
		rankedPhotos, rankDuration = ws.RankWS(*queryStrFlag, candidatePhotos, *topKFlag, *threadsFlag)
	default:
		log.Fatalf("Invalid mode: %s. Must be 'seq', 'bsp', 'pipeline', or 'ws'.", *modeFlag)
	}
	// fmt.Fprintf(os.Stderr, "Ranking phase completed. Duration: %v. Ranked photos: %d\n", rankDuration, len(rankedPhotos))

	// Basic validation of results (optional, but good for sanity check during development)
	if *topKFlag > 0 && len(rankedPhotos) > *topKFlag {
		fmt.Fprintf(os.Stderr, "Warning: Expected at most %d ranked photos, got %d for mode %s, query %q\n", *topKFlag, len(rankedPhotos), *modeFlag, *queryStrFlag)
	}
	// If candidatePhotos is empty, rankedPhotos will also be empty. rankDuration will be very small. This is fine.

	// Print CSV: mode,threads,size,time_ms
	fmt.Printf("%s,%d,%d,%.3f\n", *modeFlag, actualThreads, actualSize, float64(rankDuration.Nanoseconds())/1e6)
}
