#!/bin/bash

# Script to run benchmarks for different modes, thread counts, and dataset sizes.
set -e # Exit immediately if a command exits with a non-zero status.

# Configuration
RESULTS_CSV="results.csv"
DATASET_PATH="data/metadata_big.jsonl" # Using duplicated data
BENCH_QUERY="landscape winter snow mountains" # Consistent query for benchmarks
BENCH_TOPK=10                             # Consistent TopK

# Dataset sizes to test (number of photo entries)
# Set to 0 to use the full dataset loaded by cmd/bench
SIZES=(0) # 0 means use all available photos from the dataset

# Modes to test
MODES=("seq" "bsp" "pipeline" "ws")

# Thread counts for parallel modes
# For 'seq' mode, threads will effectively be 1.
# For 'pipeline' mode, this corresponds to 'scorers'.
# For 'bsp' and 'ws' modes, this corresponds to 'workers'.
THREAD_COUNTS=(1 2 4 8 16) # Adjust as per your machine's capability

# --- Script Start ---
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")" # Assumes scripts is one level down from project root
BENCH_CMD_DIR="$PROJECT_ROOT/cmd/bench"
BENCH_RUNNER_NAME="bench_runner"
BENCH_RUNNER_PATH="$BENCH_CMD_DIR/$BENCH_RUNNER_NAME"

# Build the benchmark tool
echo "Building benchmark tool..."
# Ensure GOBIN is not set, or build output is predictable
(cd "$BENCH_CMD_DIR" && go build -o "$BENCH_RUNNER_NAME" main.go)

if [ ! -f "$BENCH_RUNNER_PATH" ]; then
    echo "Error: Benchmark runner executable not found at $BENCH_RUNNER_PATH. Build failed."
    exit 1
fi
echo "Benchmark tool built successfully: $BENCH_RUNNER_PATH"

# Prepare results CSV file
# Ensure the results directory exists
mkdir -p "$PROJECT_ROOT/results"
RESULTS_FILE_PATH="$PROJECT_ROOT/results/$RESULTS_CSV" # Place results.csv in project_root/results

if [ ! -f "$RESULTS_FILE_PATH" ] || ! grep -q "mode,threads,size,time_ms" "$RESULTS_FILE_PATH"; then
  echo "mode,threads,size,time_ms" > "$RESULTS_FILE_PATH"
  echo "Created/Reinitialized $RESULTS_FILE_PATH with header."
else
  echo "Appending results to existing $RESULTS_FILE_PATH."
fi


echo "Starting benchmark runs..."
echo "----------------------------------------------------"

for size in "${SIZES[@]}"; do
  echo "Dataset Size: $size"
  for mode in "${MODES[@]}"; do
    echo "  Mode: $mode"
    if [ "$mode" == "seq" ]; then
      # Sequential mode always runs with 1 "thread"
      threads=1
      echo "    Threads: $threads (Sequential)"
      "$BENCH_RUNNER_PATH" --mode "$mode" --threads "$threads" --size "$size" \
                           --query "$BENCH_QUERY" --topk "$BENCH_TOPK" --dataset "$PROJECT_ROOT/$DATASET_PATH" >> "$RESULTS_FILE_PATH"
      echo "      Done."
    else
      # Parallel modes
      for threads in "${THREAD_COUNTS[@]}"; do
        echo "    Threads: $threads"
        # For pipeline, --threads flag in bench_runner maps to scorers
        # For bsp/ws, --threads flag maps to workers
        "$BENCH_RUNNER_PATH" --mode "$mode" --threads "$threads" --size "$size" \
                             --query "$BENCH_QUERY" --topk "$BENCH_TOPK" --dataset "$PROJECT_ROOT/$DATASET_PATH" >> "$RESULTS_FILE_PATH"
        echo "      Done."
      done
    fi
    echo "  Mode $mode finished for size $size."
    echo "----------------------------------------"
  done
  echo "Dataset size $size finished."
  echo "========================================"
done

echo "All benchmark runs completed."
echo "Results saved to $RESULTS_FILE_PATH"
echo "You can now use scripts/plot_speedup.py to visualize the results."

exit 0
