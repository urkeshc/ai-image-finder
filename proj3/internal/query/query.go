package query

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time" // Added for timing
)

// Query struct now includes metadata field to store the extracted metadata
type Query struct {
	Message  string                 `json:"message"`
	Metadata map[string]interface{} `json:"metadata"`
}

// getQueryParserScriptPath determines the absolute path to query_parser.py
// relative to the directory of this source file (query.go).
func getQueryParserScriptPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get caller information to determine query_parser.py path")
	}
	// dir will be the absolute path to the internal/query directory
	dir := filepath.Dir(filename)
	scriptPath := filepath.Join(dir, "query_parser.py")
	return scriptPath, nil
}

func Parse(userInput string) (Query, time.Duration, error) {
	startTime := time.Now() // Start timer
	pythonCmd := "python"
	if runtime.GOOS != "windows" {
		pythonCmd = "python3"
	}

	scriptPath, err := getQueryParserScriptPath()
	if err != nil {
		return Query{}, 0, fmt.Errorf("could not get query parser script path: %w", err)
	}

	cmd := exec.Command(pythonCmd, scriptPath, userInput)
	out, err := cmd.Output()
	duration := time.Since(startTime) // Calculate duration
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Println("Python stderr:", string(exitErr.Stderr))
		}
		return Query{}, duration, err
	}

	// fmt.Println("Raw output from Python:")
	// fmt.Println(string(out))

	var q Query
	err = json.Unmarshal(out, &q)
	if err != nil {
		fmt.Println("JSON unmarshal error:", err)
		if len(out) > 50 {
			fmt.Printf("First 50 chars: %q\n", out[:50])
		} else {
			fmt.Printf("All output: %q\n", out)
		}
	}
	return q, duration, err
}

// MergeMetadata updates old map by replacing keys with non-nil values from newMap.
func MergeMetadata(oldMap, newMap map[string]interface{}) map[string]interface{} {
	for k, v := range newMap {
		if v != nil {
			oldMap[k] = v
		}
	}
	return oldMap
}

func ParseWithHistory(prevQ Query, newInput string) (Query, time.Duration, error) {
	startTime := time.Now() // Start timer
	pythonCmd := "python"
	if runtime.GOOS != "windows" {
		pythonCmd = "python3"
	}

	scriptPath, err := getQueryParserScriptPath()
	if err != nil {
		return Query{}, 0, fmt.Errorf("could not get query parser script path: %w", err)
	}

	// Convert only the previous metadata map to JSON
	prevMetadataJSON, err := json.Marshal(prevQ.Metadata)
	if err != nil {
		return Query{}, 0, fmt.Errorf("failed to marshal previous metadata: %w", err)
	}

	// Pass as argv[1] = prevMetadataJSON_string, argv[2] = newUserText
	cmd := exec.Command(pythonCmd, scriptPath, string(prevMetadataJSON), newInput)
	out, err := cmd.Output()
	duration := time.Since(startTime) // Calculate duration
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Println("Python stderr:", string(exitErr.Stderr))
		}
		return Query{}, duration, err
	}

	// fmt.Println("Raw output from Python:")
	// fmt.Println(string(out))

	var qMerged Query
	err = json.Unmarshal(out, &qMerged)
	if err != nil {
		fmt.Println("JSON unmarshal error:", err)
	}
	return qMerged, duration, err
}
