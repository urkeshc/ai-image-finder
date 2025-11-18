package pipeline

import (
	"proj3-redesigned/internal/meta"
	"proj3-redesigned/internal/query" // For query.Parse to check for errors
	"proj3-redesigned/internal/textmatch"
	"reflect"
	"testing"
)

// smallFakePhotos builds a tiny slice with known descriptions for testing.
func smallFakePhotos() []meta.PhotoMetadata {
	return []meta.PhotoMetadata{
		{PhotoID: "A", PhotoDescription: "foo bar baz", AiDescription: "object foo", StatsDownloads: 1},
		{PhotoID: "B", PhotoDescription: "foo qux", AiDescription: "item foo", StatsDownloads: 10},
		{PhotoID: "C", PhotoDescription: "baz quux", AiDescription: "detail baz", StatsDownloads: 100},
		{PhotoID: "D", PhotoDescription: "foo only", AiDescription: "another foo", StatsDownloads: 5},
		{PhotoID: "E", PhotoDescription: "nothing relevant", AiDescription: "empty", StatsDownloads: 20},
	}
}

// helper to extract PhotoIDs from a slice
func ids(photos []meta.PhotoMetadata) []string {
	out := make([]string, len(photos))
	for i, p := range photos {
		out[i] = p.PhotoID
	}
	return out
}

func TestRankPipeline_EmptyInputs(t *testing.T) {
	// Check if query.Parse itself is failing, which would cause RankPipeline to return nil.
	// This is a pre-condition check for other tests.
	_, _, err := query.Parse("test")
	queryParseFailed := err != nil
	if queryParseFailed {
		t.Logf("Warning: query.Parse failed with error: %v. Subsequent tests might be skipped or produce unexpected nils.", err)
	}

	// topK=0 should return nil
	if res := RankScoringPipeline("foo", smallFakePhotos(), 0, 2); res != nil {
		t.Errorf("Expected nil result when topK=0, got %v", ids(res))
	}
	// empty photo slice
	if res := RankScoringPipeline("foo", []meta.PhotoMetadata{}, 3, 2); res != nil {
		t.Errorf("Expected nil result for empty photos, got %v", ids(res))
	}
}

func TestRankPipeline_BasicOrder(t *testing.T) {
	photos := smallFakePhotos() // These are now considered "pre-filtered" candidates
	topK := 2
	scorers := 2
	queryString := "foo"

	// Sequential reference using textmatch.Rank on the same "pre-filtered" candidates
	expected := textmatch.Rank(queryString, photos, topK)

	// Pipeline result
	got := RankScoringPipeline(queryString, photos, topK, scorers)

	// query.Parse is no longer called by RankScoringPipeline, so qpErr check is removed here.
	// If 'got' is nil, it's because inputs were empty or topK was 0.
	if got == nil && len(expected) > 0 {
		t.Fatalf("RankScoringPipeline returned nil, but expected %d results (%v). Inputs: %d photos, topK: %d", len(expected), ids(expected), len(photos), topK)
	}

	if len(got) != len(expected) {
		t.Fatalf("Expected %d results, got %d. Expected IDs: %v, Got IDs: %v", len(expected), len(got), ids(expected), ids(got))
	}
	if !reflect.DeepEqual(ids(got), ids(expected)) {
		t.Errorf("Order mismatch.\nExpected: %v\nGot:      %v", ids(expected), ids(got))
	}
}

func TestRankPipeline_Filtering(t *testing.T) {
	// This test's original intent was to check the pipeline's filtering stage.
	// Since the filtering stage is removed from RankScoringPipeline,
	// this test needs to be re-thought or removed.
	// For now, let's adapt it to test scoring on a manually "pre-filtered" subset.

	photos := smallFakePhotos() // Full set
	queryString := "baz"
	topK := 2 // Expecting C and A from the "baz" query if all are scored

	// Manually "filter" for photos relevant to "baz" for this test's purpose
	// This simulates what would happen before calling RankScoringPipeline
	var preFilteredForBaz []meta.PhotoMetadata
	for _, p := range photos {
		if p.PhotoID == "A" || p.PhotoID == "C" { // A and C contain "baz"
			preFilteredForBaz = append(preFilteredForBaz, p)
		}
	}
	// Expected order after scoring "baz" on [A, C]: C then A
	// C: "baz quux" (downloads 100)
	// A: "foo bar baz" (downloads 1)
	expectedIDs := []string{"C", "A"}

	got := RankScoringPipeline(queryString, preFilteredForBaz, topK, 2)
	gotIDs := ids(got)

	if got == nil && len(expectedIDs) > 0 {
		t.Fatalf("RankScoringPipeline returned nil for query '%s' on pre-filtered set, but expected %d results (%v).", queryString, len(expectedIDs), expectedIDs)
	}

	if len(gotIDs) != len(expectedIDs) {
		t.Fatalf("Expected %d results for '%s' on pre-filtered set, got %d: %v. Expected IDs: %v", len(expectedIDs), queryString, len(gotIDs), gotIDs, expectedIDs)
	}
	for i, want := range expectedIDs {
		if gotIDs[i] != want {
			t.Errorf("At position %d for query '%s' on pre-filtered set: expected %s, got %s", i, queryString, want, gotIDs[i])
		}
	}
}
