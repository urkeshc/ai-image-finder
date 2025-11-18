package textmatch

import (
	"math"
	"testing"

	// "fmt" // For debugging if needed

	"proj3-redesigned/internal/meta"
)

// simple smoke-test: query "icebergs in iceland" should rank Vatnajökull photo first.
func TestRank_IcebergPhotoTop(t *testing.T) {
	photos := []meta.PhotoMetadata{
		{
			PhotoID:              "PXdBkNF8rlk",
			PhotoDescription:     "Icebergs of Iceland's Vatnajokull",
			AiDescription:        "icebergs floating on body of water during daytime",
			PhotoLocationCountry: "Iceland", // Added for more complete photoText
			StatsDownloads:       43442,
		},
		{
			PhotoID:          "dummy_synonym_match",
			PhotoDescription: "A large automobile",
			AiDescription:    "",
			StatsDownloads:   100,
		},
		{
			PhotoID:          "dummy_orig_and_synonym",
			PhotoDescription: "A joyful canine and a glad pooch",
			AiDescription:    "",
			StatsDownloads:   200,
		},
		{
			PhotoID:          "dummy1_no_match",
			PhotoDescription: "Golden sunset over a tropical beach",
			AiDescription:    "silhouette of palm trees at dusk",
			StatsDownloads:   9000,
		},
	}

	queryIce := "icebergs in iceland" // Original query stems: "iceberg", "iceland"
	topIce := Rank(queryIce, photos, 1)
	if len(topIce) == 0 {
		t.Fatalf("Rank returned zero results for query %q", queryIce)
	}
	if topIce[0].PhotoID != "PXdBkNF8rlk" {
		t.Errorf("expected PXdBkNF8rlk on top for query %q, got %s", queryIce, topIce[0].PhotoID)
	}

	// Recalculate expected score for PXdBkNF8rlk with new logic:
	// Photo text: "Icebergs of Iceland's Vatnajokull icebergs floating on body of water during daytime Iceland"
	// Original photo stems (approx): "iceberg", "iceland", "vatnajokul", "float", "bodi", "water", "daytim" (7 unique from desc/AI + "iceland" from country is repeated)
	// So, photoOriginalsSet: {"iceberg", "iceland", "vatnajokul", "float", "bodi", "water", "daytim"} -> 7 tokens
	// numPhotoOriginalTokens = 7. Norm factor = log(1+7) = log(8) ~ 2.07944
	// Matches:
	// - "iceberg" (photo) matches "iceberg" (query original): +10.0
	// - "iceland" (photo) matches "iceland" (query original): +10.0
	// Total rawScore = 20.0
	expectedScoreIce := float32(20.0 / math.Log(1.0+7.0)) // ~9.6179
	scoreIce := Score(queryIce, topIce[0])
	if math.Abs(float64(scoreIce-expectedScoreIce)) > 1e-4 {
		t.Errorf("expected score %.4f for PXdBkNF8rlk with query %q, got %.4f", expectedScoreIce, queryIce, scoreIce)
	}

	queryCar := "big car"      // Original query stems: "big", "car"
	photoCarMatch := photos[1] // dummy_synonym_match: "A large automobile"
	// Photo text: "A large automobile"
	// Original photo stems: "larg", "automobil" (2 tokens)
	// numPhotoOriginalTokens = 2. Norm factor = log(1+2) = log(3) ~ 1.09861
	// Matches:
	// - "larg" (photo) vs "big" (query original): "larg" is synonym of "big". Score += 1.5
	// - "automobil" (photo) vs "car" (query original): "automobil" is synonym of "car". Score += 1.5
	// Total rawScore = 3.0
	expectedScoreCar := float32(3.0 / math.Log(1.0+2.0)) // ~2.7307
	scoreCar := Score(queryCar, photoCarMatch)
	if math.Abs(float64(scoreCar-expectedScoreCar)) > 1e-4 {
		t.Errorf("expected score %.4f for %s with query %q, got %.4f", expectedScoreCar, photoCarMatch.PhotoID, queryCar, scoreCar)
	}
}

// TestTokenization tests that the tokenization and stemming works as expected
// This tests the public Tokens function which includes synonyms.
func TestTokenization(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string // Expected stemmed tokens, including synonyms
	}{
		{"simple", "hello world", []string{"hello", "world"}},
		{"uppercase", "HELLO WORLD", []string{"hello", "world"}},
		{"punctuation", "Hello, World!", []string{"hello", "world"}},
		{"stopwords", "The quick brown fox", []string{"quick", "brown", "fox"}}, // "quick" is synonym of "fast"
		{"plurals", "dogs cats houses", []string{"dog", "cat", "hous"}},         // "hous" is stem of "house", "home" etc.
		{"ing_verbs", "running jumping swimming", []string{"run", "jump", "swim"}},
		{"possessive", "dog's cat's", []string{"dog", "cat"}},
		{"mixed", "Beautifully running foxes jump over lazy dogs", []string{"beautiful", "run", "fox", "jump", "over", "lazi", "dog", "pretti", "love", "gorgeou", "stun"}}, // Added synonyms for beautiful
		{"empty", "", nil},
		{"stopwords_only", "a an the of", nil},
		{"single_letters_filtered", "a b c d e f g", nil},
		{"icebergs_case", "Icebergs of Iceland", []string{"iceberg", "iceland"}},
		{"synonym_car", "a fast car", []string{"fast", "car", "quick", "rapid", "speedi", "auto", "automobil", "vehicl"}}, // "speedi" is stem of "speedy"
		{"synonym_house", "big house", []string{"big", "hous", "larg", "huge", "enorm", "home", "resid", "dwell"}},        // "resid" is stem of "residence"
		{"synonym_no_match", "unique word", []string{"uniqu", "word"}},
		{"synonym_in_stopwords", "a fast the auto", []string{"fast", "quick", "rapid", "speedi", "auto"}},         // "auto" is not a stopword, "the" is
		{"synonym_already_stemmed_key", "a fast vehicle", []string{"fast", "vehicl", "quick", "rapid", "speedi"}}, // "vehicle" is a synonym of "car"
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokens := Tokens(tc.input) // Public Tokens includes synonyms

			tokenMap := make(map[string]bool)
			for _, token := range tokens {
				tokenMap[token] = true
			}

			expectedMap := make(map[string]bool)
			for _, expToken := range tc.expected {
				expectedMap[expToken] = true
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("Input: %q\nExpected %d tokens (%v), got %d tokens (%v)",
					tc.input, len(tc.expected), tc.expected, len(tokens), tokens)
			}

			for _, expToken := range tc.expected {
				if !tokenMap[expToken] {
					t.Errorf("Input: %q\nExpected token %q not found in result. Got: %v, Expected: %v",
						tc.input, expToken, tokens, tc.expected)
				}
			}
			for _, gotToken := range tokens {
				if !expectedMap[gotToken] {
					t.Errorf("Input: %q\nUnexpected token %q found in result. Got: %v, Expected: %v",
						tc.input, gotToken, tokens, tc.expected)
				}
			}
		})
	}
}
