package util

import (
	"flag"
	"os"
	"runtime"
	"testing"
)

func TestFlags_Parse(t *testing.T) {
	// Store original os.Args and restore it afterwards.
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Reset the flag package's command line to a new empty set.
	// This is important to avoid "flag redefined" panics and to clear parsed state.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError) // Use PanicOnError for testing

	// Re-initialize our package's flags on the new command line.
	// This is necessary because the global flag variables (DBPath, etc.)
	// were initialized with the original flag.CommandLine.
	DBPath = flag.String("db", "data/metadata", "metadata dir")
	Limit = flag.Int("limit", 30, "preview rows")
	TopK = flag.Int("topk", 5, "rank-list length")
	Threads = flag.Int("threads", runtime.NumCPU(), "worker count")

	// Define test cases
	tests := []struct {
		name          string
		args          []string
		expectedDB    string
		expectedLimit int
		expectedTopK  int
		expectedThr   int
	}{
		{
			name:          "default values",
			args:          []string{"cmd"},
			expectedDB:    "data/metadata",
			expectedLimit: 30,
			expectedTopK:  5,
			expectedThr:   runtime.NumCPU(),
		},
		{
			name:          "custom values",
			args:          []string{"cmd", "-db", "/tmp/db", "-limit", "50", "-topk", "10", "-threads", "4"},
			expectedDB:    "/tmp/db",
			expectedLimit: 50,
			expectedTopK:  10,
			expectedThr:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			Parse() // This calls flag.Parse() on the reset flag.CommandLine

			if *DBPath != tt.expectedDB {
				t.Errorf("DBPath: expected %s, got %s", tt.expectedDB, *DBPath)
			}
			if *Limit != tt.expectedLimit {
				t.Errorf("Limit: expected %d, got %d", tt.expectedLimit, *Limit)
			}
			if *TopK != tt.expectedTopK {
				t.Errorf("TopK: expected %d, got %d", tt.expectedTopK, *TopK)
			}
			if *Threads != tt.expectedThr {
				t.Errorf("Threads: expected %d, got %d", tt.expectedThr, *Threads)
			}

			// Reset CommandLine again for the next test case to ensure flags are fresh
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError)
			DBPath = flag.String("db", "data/metadata", "metadata dir")
			Limit = flag.Int("limit", 30, "preview rows")
			TopK = flag.Int("topk", 5, "rank-list length")
			Threads = flag.Int("threads", runtime.NumCPU(), "worker count")
		})
	}
}
