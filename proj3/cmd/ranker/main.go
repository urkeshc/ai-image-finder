package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/textmatch"
	"proj3-redesigned/internal/util"
)

func main() {
	util.Parse() // Parse flags using util package

	fmt.Println("Loading metadata...")
	loadTimer := util.NewTimer()
	allPhotosSlice, err := meta.LoadMetadata(*util.DBPath) // Use util.DBPath
	util.Check(err)                                        // Use util.Check
	fmt.Printf("Loaded %d photos in %.1f ms\n", len(allPhotosSlice), loadTimer.Ms())

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nEnter your search query (or press Enter to quit): ")
		userQuery, err := reader.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "unexpected newline" || err.Error() == "multiple Read calls return no data or error" { // Handle Ctrl+D or pipe closing
				fmt.Println("\nExiting.")
				break
			}
			util.Check(err) // If it's not EOF, treat as fatal as per original logic
		}

		userQuery = strings.TrimSpace(userQuery)
		if userQuery == "" {
			fmt.Println("Exiting.")
			break
		}

		fmt.Printf("Ranking photos by relevance to query: %q (using Go-native scorer)\n", userQuery)
		rankTimer := util.NewTimer()
		results := textmatch.Rank(userQuery, allPhotosSlice, *util.TopK) // Use util.TopK
		fmt.Printf("Ranking time: %.1f ms\n", rankTimer.Ms())

		if len(results) == 0 {
			fmt.Println("No matching photos found.")
			continue
		}

		fmt.Printf("\nTop %d results:\n", len(results))
		for i, p := range results {
			fmt.Printf("%d) PhotoID=%s downloads=%d\n", i+1, p.PhotoID, p.StatsDownloads)

			desc := p.PhotoDescription
			if desc == "" {
				desc = p.AiDescription
			}
			if desc != "" {
				fmt.Printf("   Description: %s\n", desc)
			}
			scoreTimer := util.NewTimer()
			score := textmatch.Score(userQuery, p)
			fmt.Printf("   Score: %.2f (scoring time: %.3f ms)\n\n", score, scoreTimer.Ms())
		}
	}
}
