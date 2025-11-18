package bsp

import (
	"proj3-redesigned/internal/meta"
	"testing"
)

// smallFakePhotos builds a tiny slice with known scores:
// photo A: description="foo bar", downloads=1
// photo B: description="foo", downloads=10
func smallFakePhotos() []meta.PhotoMetadata {
	return []meta.PhotoMetadata{
		{PhotoID: "A", PhotoDescription: "foo bar", StatsDownloads: 1},
		{PhotoID: "B", PhotoDescription: "foo", StatsDownloads: 10},
		{PhotoID: "C", PhotoDescription: "baz", StatsDownloads: 100},
	}
}

func TestRankBSPConsistency(t *testing.T) {
	photos := smallFakePhotos()
	// sequential top-2 using textmatch.Rank
	want, _ := RankBSP("foo", photos, 2, 1) // 1 worker == sequential
	got, _ := RankBSP("foo", photos, 2, 2)  // 2 workers

	if len(got) != 2 || len(want) != 2 {
		t.Fatalf("expected 2 results, got %d and %d", len(want), len(got))
	}
	// compare IDs in order
	for i := range want {
		if want[i].PhotoID != got[i].PhotoID {
			t.Errorf("mismatch at pos %d: want %q got %q", i, want[i].PhotoID, got[i].PhotoID)
		}
	}
}

func BenchmarkRankBSP(b *testing.B) {
	// generate N dummy photos
	const N = 10000
	photos := make([]meta.PhotoMetadata, N)
	for i := 0; i < N; i++ {
		photos[i] = meta.PhotoMetadata{
			PhotoID:          string(rune('A' + (i % 26))),
			PhotoDescription: "lorem ipsum dolor sit amet",
			StatsDownloads:   i,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RankBSP("lorem ipsum", photos, 5, 4)
	}
}

func TestRankBSPEmptyOrInvalid(t *testing.T) {
	// k <= 0
	if res, dur := RankBSP("foo", smallFakePhotos(), 0, 4); res != nil {
		t.Errorf("expected nil result when k=0, got %v", res)
	} else if dur < 0 { // Allow 0s duration, but not negative
		t.Errorf("expected non-negative duration, got %v", dur)
	}

	// zero photos
	if res, dur := RankBSP("foo", []meta.PhotoMetadata{}, 2, 4); res != nil {
		t.Errorf("expected nil on empty photos, got %v", res)
	} else if dur < 0 { // Also check duration for this case
		t.Errorf("expected non-negative duration for empty photos, got %v", dur)
	}
}
