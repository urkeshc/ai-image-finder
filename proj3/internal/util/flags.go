package util

import (
	"flag"
	"runtime"
)

var (
	// Mode specifies the execution mode: "seq" (sequential) or "bsp" (Bulk Synchronous Parallel).
	Mode *string

	// Workers specifies the number of worker goroutines to use in BSP or WS mode.
	Workers *int

	// Scorers specifies the number of scorer worker goroutines for pipeline mode.
	Scorers *int

	// TopK is the number of top results to return from ranking. (Existing, moved here for centralization if preferred)
	TopK *int

	// Limit is the number of candidates to preview. (Existing, moved here for centralization if preferred)
	Limit *int
)

func init() {
	// Define flags. Default values are set here.
	Mode = flag.String("mode", "seq", "Execution mode: 'seq' for sequential, 'bsp' for Bulk Synchronous Parallel, 'pipeline' for pipelined scoring, or 'ws' for work-stealing.")
	Workers = flag.Int("workers", runtime.NumCPU(), "Number of workers for BSP or WS mode. Defaults to number of CPU cores.")
	Scorers = flag.Int("scorers", runtime.NumCPU(), "Number of scorer workers for pipeline mode. Defaults to number of CPU cores.")
	TopK = flag.Int("topk", 5, "Number of top results to return from AI ranking.")
	Limit = flag.Int("limit", 10, "Number of metadata-filtered candidates to preview when using 'see' option.")
}
