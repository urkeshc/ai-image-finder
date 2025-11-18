package meta

import (
	"path/filepath"
	"runtime"
	"testing"

	"proj3-redesigned/internal/query"
)

func Q(m map[string]any) query.Query { return query.Query{Metadata: m} }

func TestFilterPhotos_EndToEnd(t *testing.T) {
	// locate proj3/data/metadata
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // up to proj3
	metaDir := filepath.Join(root, "data", "metadata")

	photos, err := LoadMetadata(metaDir)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	if len(photos) == 0 {
		t.Fatalf("metadata dir empty – did you generate the sample corpus?")
	}

	bridgeQ := Q(map[string]any{
		"photo_description": "A bridge covered in snow",
		"exif_camera_make":  "Google",
		"exif_camera_model": "Pixel XL",
		"year":              2017,
	})

	alpsQ := Q(map[string]any{
		"photo_location_country":   "France",
		"photo_location_latitude":  45.0904,
		"photo_location_longitude": 6.0792,
		"year":                     2019,
	})

	iceQ := Q(map[string]any{
		"photo_location_country":   "Iceland",
		"photo_location_latitude":  64.9631,
		"photo_location_longitude": -19.0208,
		"year":                     2017,
	})

	benjaminChildQuery := Q(map[string]any{
		"photographer_first_name": "Benjamin",
		"photographer_last_name":  "Child",
		// "photo_description": "moon", // Optional: add description to make it more specific if needed
	})

	yosemiteQuery := Q(map[string]any{
		"photo_location_name":    "Yosemite National Park",
		"photo_location_country": "United States",
		// "photo_description": "mountains", // Example, AI might fill this
	})

	cases := []struct {
		name    string
		query   query.Query
		wantMin int
		// Optional: add wantMax or specific photo_id if test becomes flaky
	}{
		{"BridgeSnow", bridgeQ, 1},
		{"FrenchAlps", alpsQ, 1},
		{"IcelandParaglider", iceQ, 1},
		{"BenjaminChildPhotos", benjaminChildQuery, 1}, // Expect at least one photo by Benjamin Child
		{"YosemitePhotos", yosemiteQuery, 1},           // Expect at least one photo from Yosemite
	}

	for _, tc := range cases {
		// Test with FilterPhotos (used by CLI)
		resultsCli := FilterPhotos(tc.query, photos)
		if len(resultsCli) < tc.wantMin {
			t.Errorf("%s (FilterPhotos): expected ≥%d hits, got %d", tc.name, tc.wantMin, len(resultsCli))
		} else {
			t.Logf("%s (FilterPhotos): got %d hits (expected ≥%d)", tc.name, len(resultsCli), tc.wantMin)
		}
		// Optionally log some results from FilterPhotos
		// for i, r := range resultsCli {
		// 	if i < 2 { // Log first few
		// 		t.Logf("    CLI → %s", r.PhotoID)
		// 	}
		// }

		// Test with FilterPhotosWithReasons (original test function)
		resultsReasons := FilterPhotosWithReasons(tc.query, photos)
		if len(resultsReasons) < tc.wantMin {
			t.Errorf("%s (FilterPhotosWithReasons): expected ≥%d hits, got %d", tc.name, tc.wantMin, len(resultsReasons))
		}
		for _, r := range resultsReasons {
			t.Logf("%s (FilterPhotosWithReasons) → %s", tc.name, r.Photo.PhotoID)
			for _, reason := range r.Reasons {
				t.Logf("    ↳ %s", reason)
			}
		}
	}
}
