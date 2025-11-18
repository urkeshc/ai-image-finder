package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/parallel/bsp"
	"proj3-redesigned/internal/parallel/pipeline"
	"proj3-redesigned/internal/parallel/ws" // Added for Work-Stealing
	"proj3-redesigned/internal/query"
	"proj3-redesigned/internal/textmatch"
	"proj3-redesigned/internal/util"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Regex to extract cost from the message
var costRegex = regexp.MustCompile(`cost: (\$\d+\.\d+)`)

// handlePostPreviewInteraction prompts the user after results are shown.
// Returns true if the program should terminate, false if it should continue (user wants to add info).
func handlePostPreviewInteraction(reader *bufio.Reader) bool {
	for {
		fmt.Print("Did you find the image you were searching for? (yes, quit / add details / use AI model) ▶ ")
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))

		switch ans {
		case "yes":
			fmt.Println("Great! Exiting.")
			return true // Terminate
		case "quit":
			fmt.Println("Exiting.")
			return true // Terminate
		case "add details":
			return false // Continue main loop to add more info
		case "use ai model":
			return false // Continue main loop to use AI model
		default:
			fmt.Println("Invalid input. Please type 'yes', 'quit', 'add details', or 'use ai model'.")
		}
	}
}

func main() {
	// Flags are now managed by internal/util/flags.go's init()
	// We still need to parse them here.
	useDuplicatedSize := flag.Int("duplicated", -1, "Load duplicated data. Provide 0 for all records, or N > 0 for a subset of N records. Omit flag for standard data.")
	// The util package's init function will register "mode", "workers", "scorers", "topk", "limit".
	flag.Parse()

	// Validate mode
	if *util.Mode != "seq" && *util.Mode != "bsp" && *util.Mode != "pipeline" && *util.Mode != "ws" {
		log.Fatalf("Invalid mode: %s. Must be 'seq', 'bsp', 'pipeline', or 'ws'.", *util.Mode)
	}
	if (*util.Mode == "bsp" || *util.Mode == "ws") && *util.Workers <= 0 {
		log.Printf("Warning: Invalid number of workers (%d) for %s mode. Defaulting to %d (NumCPU).\n", *util.Workers, *util.Mode, runtime.NumCPU())
		*util.Workers = runtime.NumCPU()
	}
	if *util.Mode == "pipeline" {
		if *util.Scorers <= 0 {
			log.Printf("Warning: Invalid number of scorers (%d) for pipeline mode. Defaulting to %d (NumCPU).\n", *util.Scorers, runtime.NumCPU())
			*util.Scorers = runtime.NumCPU()
		}
	}

	var allPhotos []meta.PhotoMetadata
	var err error
	var metadataPath string

	if *useDuplicatedSize == -1 { // Flag not provided, use standard data
		metadataPath = "data/metadata"
		fmt.Printf("Loading standard metadata from directory %s...\n", metadataPath)
		allPhotos, err = meta.LoadMetadata(metadataPath)
	} else { // --duplicated flag was provided with a value (0 or N)
		metadataPath = "data/metadata_big.jsonl"
		fmt.Printf("Loading duplicated metadata from %s...\n", metadataPath)
		allPhotos, err = meta.LoadMetadataFromJSONL(metadataPath)
		if err == nil {
			totalLoaded := len(allPhotos)
			if *useDuplicatedSize > 0 && *useDuplicatedSize < totalLoaded {
				allPhotos = allPhotos[:*useDuplicatedSize]
				// Message updated to reflect subset logic for duplicated data
				fmt.Printf("Successfully loaded and using a subset of %d duplicated photo metadata entries (out of %d total) from %s.\n\n", len(allPhotos), totalLoaded, metadataPath)
			} else if *useDuplicatedSize == 0 {
				// Message for loading all duplicated data
				fmt.Printf("Successfully loaded all %d duplicated photo metadata entries from %s.\n\n", len(allPhotos), metadataPath)
			} else if *useDuplicatedSize > 0 && *useDuplicatedSize >= totalLoaded {
				// Requested subset size is >= total available, so all loaded are used
				fmt.Printf("Successfully loaded all %d duplicated photo metadata entries (requested subset %d) from %s.\n\n", len(allPhotos), *useDuplicatedSize, metadataPath)
			}
		}
	}

	if err != nil {
		log.Fatalf("Error loading metadata from %s: %v", metadataPath, err)
	}

	if len(allPhotos) == 0 { // This check is now more general
		log.Fatalf("No photo metadata loaded. Please ensure the path %s is correct and data exists.", metadataPath)
	}
	// The specific count print is now handled within the if/else block above for clarity.

	reader := bufio.NewReader(os.Stdin)

	var q query.Query
	var freeTextQuery string
	firstQuery := true

	knownKeys := map[string]bool{
		"photo_submitted_at":       true,
		"photo_featured":           true,
		"photo_width":              true,
		"photo_height":             true,
		"photo_aspect_ratio":       true,
		"photo_description":        true,
		"photographer_username":    true,
		"photographer_first_name":  true,
		"photographer_last_name":   true,
		"exif_camera_make":         true,
		"exif_camera_model":        true,
		"year":                     true,
		"month":                    true,
		"day":                      true,
		"exif_iso":                 true,
		"exif_aperture_value":      true,
		"exif_focal_length":        true,
		"exif_exposure_time":       true,
		"photo_location_name":      true,
		"photo_location_latitude":  true,
		"photo_location_longitude": true,
		"photo_location_country":   true,
		"photo_location_city":      true,
	}

	for {
		fmt.Print("What image are you looking for? (Type your query, or leave empty to quit if not first query) ▶ ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)
		// fmt.Print("Sending query to OpenAI…\n") // This print was added by user, keeping it.

		if userInput == "" && !firstQuery {
			fmt.Println("Exiting.")
			break
		}

		if firstQuery {
			freeTextQuery = userInput
		} else {
			if userInput != "" {
				freeTextQuery += " " + userInput
			}
		}

		var nextQ query.Query
		var openAIDurationTime time.Duration // Renamed to avoid conflict with util.Timer

		// Timing for OpenAI query is handled within query.Parse/ParseWithHistory
		if firstQuery {
			nextQ, openAIDurationTime, err = query.Parse(userInput)
		} else {
			nextQ, openAIDurationTime, err = query.ParseWithHistory(q, userInput)
		}
		if err != nil { // Don't use util.Check here as we want to continue the loop
			fmt.Printf("Error parsing query for metadata: %v\n", err)
			continue
		}
		q = nextQ
		firstQuery = false

		var parts []string
		for k, v := range q.Metadata {
			if v != nil && knownKeys[k] {
				parts = append(parts, fmt.Sprintf("%q: %v", k, v))
			}
		}
		if len(parts) > 0 {
			fmt.Printf("\nMetadata filters from Open AI: {%s}\n", strings.Join(parts, ", "))
		}
		if freeTextQuery != "" {
			fmt.Printf("Current textual query for AI ranking: %q\n", freeTextQuery)
		}

		costMatches := costRegex.FindStringSubmatch(q.Message)
		if len(costMatches) > 1 {
			fmt.Printf("OpenAI metadata query cost: %s\n", costMatches[1])
		}
		// Format OpenAI duration to ms
		fmt.Printf("OpenAI metadata query time: %.1f ms\n", float64(openAIDurationTime.Nanoseconds())/1e6)

		// Time local filtering
		filterTimer := util.NewTimer() // Use util.Timer
		results := meta.FilterPhotos(q, allPhotos)
		fmt.Printf("Local metadata filtering time: %.1f ms\n", filterTimer.Ms()) // Use util.Timer.Ms()
		fmt.Printf("\nFiltered candidates by metadata: %d / %d\n", len(results), len(allPhotos))

		// --- Decision Tree ---
		if len(results) == 0 {
			fmt.Print("No images match metadata. Add more info for metadata, or use AI to rank ALL images based on text? (add/ai) ▶ ")
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			if ans == "add" {
				continue
			} else if ans == "ai" {
				if freeTextQuery == "" {
					fmt.Print("Please type a textual description for AI ranking: ")
					newText, _ := reader.ReadString('\n')
					freeTextQuery = strings.TrimSpace(newText)
					if freeTextQuery == "" {
						fmt.Println("No text provided for AI ranking. Please try again.")
						continue
					}
				}

				var ranked []meta.PhotoMetadata
				var aiRankDuration time.Duration
				var rankerMsg string

				photosToRank := allPhotos // In this branch, we rank all photos

				if *util.Mode == "bsp" {
					rankerMsg = fmt.Sprintf("Running BSP token-overlap scorer with %d workers on all %d photos…", *util.Workers, len(photosToRank))
					fmt.Println(rankerMsg)
					ranked, aiRankDuration = bsp.RankBSP(freeTextQuery, photosToRank, *util.TopK, *util.Workers)
				} else if *util.Mode == "pipeline" {
					rankerMsg = fmt.Sprintf("Running Pipeline token-overlap scorer with %d scorers on all %d photos…", *util.Scorers, len(photosToRank))
					fmt.Println(rankerMsg)
					pipelineTimer := util.NewTimer()
					ranked = pipeline.RankScoringPipeline(freeTextQuery, photosToRank, *util.TopK, *util.Scorers)
					aiRankDuration = time.Duration(pipelineTimer.Ms() * float64(time.Millisecond))
				} else if *util.Mode == "ws" {
					rankerMsg = fmt.Sprintf("Running Work-Stealing token-overlap scorer with %d workers on all %d photos…", *util.Workers, len(photosToRank))
					fmt.Println(rankerMsg)
					// RankWS returns duration directly
					ranked, aiRankDuration = ws.RankWS(freeTextQuery, photosToRank, *util.TopK, *util.Workers)
				} else { // seq mode
					rankerMsg = fmt.Sprintf("Running sequential token-overlap scorer on all %d photos…", len(photosToRank))
					fmt.Println(rankerMsg)
					aiRankTimer := util.NewTimer()
					ranked = textmatch.Rank(freeTextQuery, photosToRank, *util.TopK)
					aiRankDuration = time.Duration(aiRankTimer.Ms() * float64(time.Millisecond))
				}
				fmt.Printf("AI ranking time: %.1f ms\n", float64(aiRankDuration.Nanoseconds())/1e6)

				if len(ranked) == 0 {
					fmt.Println("AI scorer found no good matches from all photos 🤷 Try adding more detail to your text?")
					continue
				}
				topK := len(ranked)
				fmt.Printf("\nTop %d visual matches from all photos:\n", topK)
				for i, p := range ranked {
					desc := p.PhotoDescription
					if desc == "" {
						desc = p.AiDescription
					}
					// Score needs to be recalculated for display if RankBSP doesn't return it,
					// or RankBSP could be modified to return scoredPhoto.
					// For now, recalculating.
					score := textmatch.Score(freeTextQuery, p)
					fmt.Printf("%2d) %s  —  %s (score %.2f)\n",
						i+1, p.PhotoID, desc, score)
				}
				if handlePostPreviewInteraction(reader) {
					break
				}
				continue
			} else {
				fmt.Println("Invalid input. Please try again.")
				continue
			}
		} else {
			promptMessage := fmt.Sprintf("Applied filtering to metadata, we currently have %d potential candidates. ", len(results))
			if len(results) == len(allPhotos) {
				promptMessage = "Metadata filter was not very specific. " + promptMessage
			}
			promptMessage += "Would you like to see the candidates, add more info for metadata (location/date/time/camera used/photographer name) or would you like to use our AI model to analyze and return images that match your description? (Type \"see\" or \"add\" or \"ai\") ▶ "
			fmt.Print(promptMessage)

			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))

			if ans == "see" {
				previewLimit := *util.Limit // Use util.Limit
				if len(results) < previewLimit {
					previewLimit = len(results)
				}
				fmt.Printf("\nPreviewing up to %d of %d metadata-filtered results:\n", previewLimit, len(results))
				for i, p := range results {
					if i >= previewLimit {
						break
					}
					description := p.PhotoDescription
					if description == "" {
						description = p.AiDescription
					}
					fmt.Printf("- %s: %s\n", p.PhotoID, description)
				}
				if handlePostPreviewInteraction(reader) {
					break
				}
				continue
			} else if ans == "add" {
				continue
			} else if ans == "ai" {
				if freeTextQuery == "" {
					fmt.Print("Please type a textual description for AI ranking: ")
					newText, _ := reader.ReadString('\n')
					freeTextQuery = strings.TrimSpace(newText)
					if freeTextQuery == "" {
						fmt.Println("No text provided for AI ranking. Please try again.")
						continue
					}
				}

				var ranked []meta.PhotoMetadata
				var aiRankDuration time.Duration
				var rankerMsg string

				photosToRank := results // In this branch, we rank metadata-filtered candidates

				if *util.Mode == "bsp" {
					rankerMsg = fmt.Sprintf("Running BSP token-overlap scorer with %d workers on %d metadata-filtered candidates…", *util.Workers, len(photosToRank))
					fmt.Println(rankerMsg)
					ranked, aiRankDuration = bsp.RankBSP(freeTextQuery, photosToRank, *util.TopK, *util.Workers)
				} else if *util.Mode == "pipeline" {
					rankerMsg = fmt.Sprintf("Running Pipeline token-overlap scorer with %d scorers on %d metadata-filtered candidates…", *util.Scorers, len(photosToRank))
					fmt.Println(rankerMsg)
					pipelineTimer := util.NewTimer()
					ranked = pipeline.RankScoringPipeline(freeTextQuery, photosToRank, *util.TopK, *util.Scorers)
					aiRankDuration = time.Duration(pipelineTimer.Ms() * float64(time.Millisecond))
				} else if *util.Mode == "ws" {
					rankerMsg = fmt.Sprintf("Running Work-Stealing token-overlap scorer with %d workers on %d metadata-filtered candidates…", *util.Workers, len(photosToRank))
					fmt.Println(rankerMsg)
					ranked, aiRankDuration = ws.RankWS(freeTextQuery, photosToRank, *util.TopK, *util.Workers)
				} else { // seq mode
					rankerMsg = fmt.Sprintf("Running sequential token-overlap scorer on %d metadata-filtered candidates…", len(photosToRank))
					fmt.Println(rankerMsg)
					aiRankTimer := util.NewTimer()
					ranked = textmatch.Rank(freeTextQuery, photosToRank, *util.TopK)
					aiRankDuration = time.Duration(aiRankTimer.Ms() * float64(time.Millisecond))
				}
				fmt.Printf("AI ranking time: %.1f ms\n", float64(aiRankDuration.Nanoseconds())/1e6)

				if len(ranked) == 0 {
					fmt.Println("AI scorer found no good matches from the filtered set 🤷 Try adding more detail to your text?")
					continue
				}
				topK := len(ranked)
				filtered := photosToRank
				fmt.Printf("\nTop %d visual matches from the %d candidates:\n", topK, len(filtered))
				for i, p := range ranked {
					desc := p.PhotoDescription
					if desc == "" {
						desc = p.AiDescription
					}
					// Recalculate score for display
					score := textmatch.Score(freeTextQuery, p)
					fmt.Printf("%2d) %s  —  %s (score %.2f)\n",
						i+1, p.PhotoID, desc, score)
				}
				if handlePostPreviewInteraction(reader) {
					break
				}
				continue
			} else {
				fmt.Println("Invalid input. Please try again.")
				continue
			}
		}
	}
}
