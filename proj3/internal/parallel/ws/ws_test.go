package ws

import (
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/textmatch"
	"reflect"
	"runtime"
	"testing"
)

// smallFakePhotos builds a tiny slice for testing.
func smallFakePhotos() []meta.PhotoMetadata {
	return []meta.PhotoMetadata{
		{PhotoID: "A", PhotoDescription: "foo bar baz", AiDescription: "object foo", StatsDownloads: 1},
		{PhotoID: "B", PhotoDescription: "foo qux", AiDescription: "item foo", StatsDownloads: 10},
		{PhotoID: "C", PhotoDescription: "baz quux", AiDescription: "detail baz", StatsDownloads: 100},
		{PhotoID: "D", PhotoDescription: "foo only", AiDescription: "another foo", StatsDownloads: 5},
		{PhotoID: "E", PhotoDescription: "nothing relevant", AiDescription: "empty", StatsDownloads: 20},
		{PhotoID: "F", PhotoDescription: "another foo example", AiDescription: "foo detail", StatsDownloads: 15},
		{PhotoID: "G", PhotoDescription: "baz and foo", AiDescription: "item baz foo", StatsDownloads: 25},
	}
}

func ids(photos []meta.PhotoMetadata) []string {
	if photos == nil {
		return nil
	}
	out := make([]string, len(photos))
	for i, p := range photos {
		out[i] = p.PhotoID
	}
	return out
}

func TestRankWS_EmptyInputs(t *testing.T) {
	if res, dur := RankWS("foo", smallFakePhotos(), 0, 2); res != nil {
		t.Errorf("Expected nil result when topK=0, got %v", ids(res))
	} else if dur < 0 {
		t.Errorf("Expected non-negative duration, got %v", dur)
	}

	if res, dur := RankWS("foo", []meta.PhotoMetadata{}, 3, 2); res != nil {
		t.Errorf("Expected nil result for empty photos, got %v", ids(res))
	} else if dur < 0 {
		t.Errorf("Expected non-negative duration, got %v", dur)
	}
}

func TestRankWS_BasicOrder(t *testing.T) {
	photos := smallFakePhotos()
	topK := 3
	workers := 2
	queryString := "foo"

	expected := textmatch.Rank(queryString, photos, topK)
	got, duration := RankWS(queryString, photos, topK, workers)
	t.Logf("RankWS with query %q, %d workers, topK %d took %v", queryString, workers, topK, duration)

	if got == nil && len(expected) > 0 {
		t.Fatalf("RankWS returned nil, but expected %d results (%v).", len(expected), ids(expected))
	}

	if len(got) != len(expected) {
		t.Fatalf("Expected %d results, got %d. Expected IDs: %v, Got IDs: %v", len(expected), len(got), ids(expected), ids(got))
	}
	if !reflect.DeepEqual(ids(got), ids(expected)) {
		t.Errorf("Order mismatch.\nExpected: %v\nGot:      %v", ids(expected), ids(got))
	}
}

func TestRankWS_MoreWorkersThanPhotos(t *testing.T) {
	photos := smallFakePhotos()[:3] // Use only 3 photos
	topK := 2
	workers := 5 // More workers than photos
	queryString := "foo"

	expected := textmatch.Rank(queryString, photos, topK)
	got, duration := RankWS(queryString, photos, topK, workers)
	t.Logf("RankWS (more workers than photos) with query %q, %d workers, topK %d took %v", queryString, workers, topK, duration)

	if got == nil && len(expected) > 0 {
		t.Fatalf("RankWS returned nil, but expected %d results (%v).", len(expected), ids(expected))
	}
	if len(got) != len(expected) {
		t.Fatalf("Expected %d results, got %d. Expected IDs: %v, Got IDs: %v", len(expected), len(got), ids(expected), ids(got))
	}
	if !reflect.DeepEqual(ids(got), ids(expected)) {
		t.Errorf("Order mismatch.\nExpected: %v\nGot:      %v", ids(expected), ids(got))
	}
}

func TestRankWS_SingleWorker(t *testing.T) {
	photos := smallFakePhotos()
	topK := 3
	workers := 1 // Single worker (sequential essentially)
	queryString := "baz"

	expected := textmatch.Rank(queryString, photos, topK)
	got, duration := RankWS(queryString, photos, topK, workers)
	t.Logf("RankWS (single worker) with query %q, %d worker, topK %d took %v", queryString, workers, topK, duration)

	if len(got) != len(expected) {
		t.Fatalf("Expected %d results, got %d. Expected IDs: %v, Got IDs: %v", len(expected), len(got), ids(expected), ids(got))
	}
	if !reflect.DeepEqual(ids(got), ids(expected)) {
		t.Errorf("Order mismatch.\nExpected: %v\nGot:      %v", ids(expected), ids(got))
	}
}

// BenchmarkRankWS (Optional, can be added later for performance comparison)
func BenchmarkRankWS(b *testing.B) {
	numPhotos := 100000
	benchPhotos := make([]meta.PhotoMetadata, numPhotos)
	for i := 0; i < numPhotos; i++ {
		benchPhotos[i] = meta.PhotoMetadata{PhotoID: string(rune('A' + (i % 26))), PhotoDescription: "benchmark photo item", StatsDownloads: i}
	}
	queryString := "benchmark item"
	topK := 10
	numWorkers := runtime.NumCPU()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RankWS(queryString, benchPhotos, topK, numWorkers)
	}
}
