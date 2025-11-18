// dup.go  –  go run dup.go -n 25 >proj3/data/metadata_big.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime" // Added for determining script location
)

var times = flag.Int("n", 10, "duplication factor")

// getSourceFileDir returns the directory of the source file from which it's called.
func getSourceFileDir() string {
	// Caller(0) would be getSourceFileDir itself.
	// Caller(1) is the function that called getSourceFileDir (e.g., main).
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		log.Fatal("Failed to get caller information to determine source file directory")
	}
	return filepath.Dir(filename)
}

func main() {
	flag.Parse()

	// Determine the path to the metadata directory, relative to this script's file location.
	// scriptDir will be the absolute path to proj3/scripts/dup/
	scriptDir := getSourceFileDir()
	// Construct path to proj3/data/metadata/
	// scriptDir/../.. takes us to proj3/
	projectBasePath := filepath.Join(scriptDir, "..", "..")
	metadataDirPath := filepath.Join(projectBasePath, "data", "metadata")

	log.Printf("Attempting to open metadata directory: %s", metadataDirPath) // Log the resolved path

	in, err := os.Open(metadataDirPath)
	if err != nil {
		log.Fatalf("Error opening directory %s: %v\n", metadataDirPath, err)
	}
	defer in.Close()

	files, err := in.Readdir(0)
	if err != nil {
		log.Fatalf("Error reading directory %s: %v\n", metadataDirPath, err)
	}

	if len(files) == 0 {
		log.Printf("No files found in directory %s\n", metadataDirPath)
		return
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	id := 0
	for rep := 0; rep < *times; rep++ {
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if filepath.Ext(f.Name()) != ".json" { // Process only .json files
				continue
			}

			filePath := filepath.Join(metadataDirPath, f.Name())
			b, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("Error reading file %s: %v\n", filePath, err)
				continue
			}

			var m map[string]any
			err = json.Unmarshal(b, &m)
			if err != nil {
				log.Printf("Error unmarshaling JSON from file %s: %v\n", filePath, err)
				continue
			}

			// Ensure photo_id exists and is a string before trying to format it
			photoIDInterface, ok := m["photo_id"]
			if !ok {
				log.Printf("photo_id not found in file %s\n", filePath)
				continue
			}
			photoIDStr, ok := photoIDInterface.(string)
			if !ok {
				log.Printf("photo_id in file %s is not a string\n", filePath)
				continue
			}

			m["photo_id"] = fmt.Sprintf("%s_%d", photoIDStr, rep)
			enc, err := json.Marshal(m)
			if err != nil {
				log.Printf("Error marshaling JSON for file %s (rep %d): %v\n", filePath, rep, err)
				continue
			}

			_, err = w.Write(enc)
			if err != nil {
				log.Fatalf("Error writing to output: %v\n", err)
			}
			err = w.WriteByte('\n')
			if err != nil {
				log.Fatalf("Error writing newline to output: %v\n", err)
			}
			id++
		}
	}
	// The deferred w.Flush() will handle flushing.
	log.Printf("Processed %d entries in total.\n", id)
}
