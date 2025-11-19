# AI Image Finder (Parallel Image Search Engine)

This project is an **AI-powered image finder** that lets you search your photo library using **natural language** (e.g. *“the mountain-top photos I took on my 2021 holiday in Switzerland”*), instead of simple keyword tags.

It was developed as the final project for **MPCS 52060 – Parallel Programming** at UChicago, with a strong focus on **parallel architectures, scalability, and performance engineering**.

---

## 🌍 Problem & Motivation

Modern photo apps (e.g. iOS Photos) can handle simple queries like *“Barcelona”* or *“credit card”*, but struggle with richer, contextual searches:

> *“The picture of a fjord I took in Norway in 2018”*  
> *“The photo of me standing in front of a waterfall, I think it was during my Iceland trip”*

This project explores how to:
- Parse **free-form user queries** into structured metadata (location, date, camera, description) using an LLM.
- Efficiently search at the scale of **millions of images**, where a sequential scan would be far too slow.
- Use **parallelism** (BSP, pipelining, work-stealing) to make this kind of search responsive and scalable.

Although the current version uses metadata + AI descriptions (from the Unsplash dataset), the architecture is designed to later plug in a **full vision model** that directly inspects image pixels.

---

## 🧠 High-Level Approach

1. **Query Parsing (LLM-assisted)**  
   - A user types a free-form request (e.g. *“I’m looking for the picture of a fjord I took in Norway in 2018”*).  
   - A small Python helper (`query_parser.py`) calls an LLM to extract structured metadata into a JSON object:  
     - `photo_location_country`, `photo_location_city`  
     - `photo_description`  
     - `photo_submitted_at` / year  
     - photographer, camera, etc.

2. **Fast Metadata Filtering (1st pass)**  
   - Go code in `internal/meta` loads Unsplash-style photo metadata (JSON / JSONL).  
   - It filters out clearly irrelevant images by matching fields like:
     - country, city  
     - year / date  
     - photographer / camera  
   - This pass is extremely fast (< 5 ms for thousands of entries).

3. **Textual Ranking (2nd pass)**  
   - For the remaining candidates, `internal/textmatch` scores each image using:
     - tokenization + stemming (Porter)  
     - stopword removal  
     - synonym expansion (precomputed GloVe neighbours)  
   - The score is based on token overlap between the **user query** and the **AI-generated caption** (`ai_description`) for each photo.

4. **Parallel Ranking (core of the project)**  
   The ranking stage is deliberately treated as a heavy “stand-in” for a vision model and is parallelized using three patterns:
   - **Sequential baseline**: single-threaded `Rank`.
   - **BSP (Bulk-Synchronous Parallel)**:
     - Split the photo set into `T` chunks, one per worker.
     - Each worker maintains a local top-K heap.
     - A barrier synchronizes workers before a final reduce/merge.
   - **Pipeline**:
     - Producer → scoring workers → collector pattern, connected by buffered channels.
     - Overlaps stages and provides natural load balancing.
   - **Work-Stealing (WS)**:
     - Per-worker deques (Chase-Lev style) of tasks.
     - Idle workers steal work from others.
     - Designed to handle skewed workloads (e.g. some photos are “heavier” to score).

---

## 🏗️ Project Structure

Key directories (see `./documentation/project_structure.txt` for full detail):

- `cmd/`
  - `finder/` – Main interactive CLI. Flags:
    - `--mode`: `seq`, `bsp`, `pipeline`, `ws`
    - `--duplicated N`: synthetic dataset size
  - `bench/` – Benchmark runner for all modes / thread counts. Outputs CSV into `results/`.
  - `ranker/` – Standalone ranking tool focused on the token-overlap scorer.

- `internal/`
  - `meta/` – Load and filter metadata (`loader.go`, `filter.go`).
  - `query/` – Glue to Python LLM parser; merges query history, manages metadata filters.
  - `textmatch/` – Tokenization, stemming, synonym expansion, scoring, and top-K heap.
  - `parallel/`
    - `bsp/` – Barrier + BSP implementation.
    - `pipeline/` – Channel-based pipeline scorer.
    - `ws/` – Work-stealing runtime with lock-free deques.
  - `util/` – Flags, timers, progress display, error helpers.

- `data/`
  - `metadata/` – JSON metadata for a 1k image subset.
  - (Large synthetic metadata and images are **not** in the repo; see “Data & Setup”.)

- `scripts/`
  - `get_image_sample.py` – Download a sample of Unsplash images + metadata.
  - `dup/dup.go` – Duplicate metadata to simulate up to 25M image entries.
  - `run_all.sh` – Run benchmarks across modes and thread counts.
  - `plot_speedup.py` – Generate speedup charts from CSV results into `plots/`.

---

## 📊 Performance & Parallelism

The project evaluates how different parallel patterns scale on synthetic datasets up to **tens of millions** of entries (simulating a large photo library or a production-scale CV inference pipeline):

- **Dataset sizes**: 100K, 1M, 25M (configurable via `--duplicated N`).
- **Modes**: `seq`, `bsp`, `pipeline`, `ws`.
- **Metrics**: ranking time only (filtering and LLM parsing excluded).

Key observations (example behaviour on large datasets):

- All parallel modes achieve substantial speedups vs. the sequential baseline.
- BSP is simple and performs well up to ~8 threads before barrier costs show.
- Pipeline slightly underperforms at low thread counts (channel overhead), but scales nicely when stages overlap.
- Work-Stealing provides the best robustness when scoring costs are non-uniform, achieving around ~8× speedup at higher thread counts in the experiments.

The overarching takeaway: **parallelizing the ranking kernel is essential** once you reach millions of images, and the choice of concurrency pattern matters for load balance and overheads.

---

## 🛠️ Installation & Setup (Short Version)

Detailed instructions live in `./documentation/run.txt`. High-level steps:

1. **Prerequisites**
   - Go ≥ 1.19  
   - Python 3.x  
   - Python packages (see `requirements.txt` in the repo root):  
     `openai`, `pandas`, `matplotlib`, `seaborn`, `numpy`
   - Set your OpenAI API key as an environment variable:
     ```bash
     export OPENAI_API_KEY="your-key-here"
     ```
     (On Windows PowerShell: `$env:OPENAI_API_KEY="your-key-here"`)

2. **Data**
   - Download a small sample of images + metadata:
     ```bash
     cd proj3/scripts
     python get_image_sample.py N   # e.g. N = 100 or 1000
     ```
   - Optionally generate large synthetic metadata:
     ```bash
     cd proj3/scripts/dup
     go run dup.go 25000000    # simulate 25M entries
     ```

---

## 🚀 Future Directions

While this version uses metadata + AI descriptions as a proxy for visual understanding, the design intentionally isolates the scoring kernel so it can be replaced by a real vision model:
    - Swap the token-overlap scorer for a multimodal model (e.g. “Does this image match the user’s description?” per image).
    - Reuse the same parallel runtime (BSP, pipeline, work-stealing) to keep latencies reasonable, even when each image requires a heavy CV inference.
    - Extend filters to integrate EXIF data, face recognition, and richer contextual cues.

This codebase is therefore both:
    - A practical prototype for AI-assisted photo search, and
    - A parallel systems case study in designing and benchmarking concurrent architectures on realistic, data-heavy workloads.