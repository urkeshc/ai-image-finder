package meta

import (
	"bufio"
	"bytes" // Added for BOM check
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PhotoMetadata struct {
	PhotoID                string  `json:"photo_id"`
	PhotoSubmittedAt       string  `json:"photo_submitted_at"`
	PhotoLocationCountry   string  `json:"photo_location_country"`
	PhotoLocationCity      string  `json:"photo_location_city"`
	PhotoLocationLatitude  float64 `json:"photo_location_latitude"`
	PhotoLocationLongitude float64 `json:"photo_location_longitude"`
	PhotographerUsername   string  `json:"photographer_username"`
	PhotographerFirstName  string  `json:"photographer_first_name"`
	PhotographerLastName   string  `json:"photographer_last_name"`
	PhotoDescription       string  `json:"photo_description"`
	AiDescription          string  `json:"ai_description"`
	ExifCameraMake         string  `json:"exif_camera_make"`
	ExifCameraModel        string  `json:"exif_camera_model"`
	StatsDownloads         int     `json:"stats_downloads"` // Added for ranking tie-breaking
}

// LoadMetadata walks the given metadataDir, reads each .json file, and unmarshals into PhotoMetadata.
func LoadMetadata(metadataDir string) ([]PhotoMetadata, error) {
	var results []PhotoMetadata

	err := filepath.Walk(metadataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %q: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}

		data, readErr := os.ReadFile(path) // Changed from ioutil.ReadFile
		if readErr != nil {
			return fmt.Errorf("failed to read file %s: %w", path, readErr)
		}

		var pm PhotoMetadata
		if unmarshalErr := json.Unmarshal(data, &pm); unmarshalErr != nil {
			// If parsing fails, skip this file
			return nil
		}
		results = append(results, pm)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed walking metadata directory: %w", err)
	}

	return results, nil
}

// UTF8BOM is the byte order mark for UTF-8
var UTF8BOM = []byte{0xEF, 0xBB, 0xBF}

// LoadMetadataFromJSONL reads a JSON Lines file, where each line is a JSON object
// representing a PhotoMetadata. It skips a leading UTF-8 BOM if present.
func LoadMetadataFromJSONL(filePath string) ([]PhotoMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file %s: %w", filePath, err)
	}
	defer file.Close()

	// Use a bufio.Reader to allow peeking for BOM and then pass to scanner
	br := bufio.NewReader(file)

	// Check for UTF-8 BOM
	// Peek the first 3 bytes. If an error occurs (e.g. file is too short),
	// it's not a BOM we can identify, so proceed.
	bomCand, err := br.Peek(3)
	if err == nil && bytes.Equal(bomCand, UTF8BOM) {
		_, _ = br.Discard(3) // Skip the BOM if matched
	}
	// If err != nil from Peek, or if bytes don't match, br remains as is (no discard).

	var results []PhotoMetadata
	// Pass the bufio.Reader (which may have had BOM skipped) to the scanner
	scanner := bufio.NewScanner(br)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 { // Skip empty lines
			continue
		}

		var pm PhotoMetadata
		if unmarshalErr := json.Unmarshal(line, &pm); unmarshalErr != nil {
			// Include the problematic line content for easier debugging, careful with very long lines
			problematicLine := string(line)
			if len(problematicLine) > 100 { // Truncate if too long
				problematicLine = problematicLine[:100] + "..."
			}
			return nil, fmt.Errorf("failed to unmarshal JSON from line %d in %s (content: %q): %w", lineNumber, filePath, problematicLine, unmarshalErr)
		}
		results = append(results, pm)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading JSONL file %s: %w", filePath, err)
	}

	return results, nil
}
