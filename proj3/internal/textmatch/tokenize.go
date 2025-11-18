package textmatch

import (
	"embed"
	"encoding/json"
	"log"
	"regexp"
	"strings"

	"github.com/reiver/go-porterstemmer"
)

var (
	//go:embed data/synonym_map.json data/glove_neighbors_15k.json
	synFiles embed.FS // Embeds synonym files from the data directory

	loadedSynonymMap map[string][]string
)

// stopwords is a list of common words to ignore during tokenization.
// This list is similar to the one used in internal/meta/filter.go.
var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true,
	"of": true, "in": true, "on": true, "at": true, "for": true, "to": true, "by": true, "with": true,
	"picture": true, "photo": true, "image": true, "photograph": true, "view": true,
	// Consider adding more or making this configurable if needed.
	// Words specific to photography context that might not add value for matching:
	"camera": true, "lens": true, "shot": true, "taken": true, "exposure": true,
	// Common prepositions and conjunctions
	"and": true, "but": true, "or": true, "as": true, "if": true, "it": true, "its": true,
	"this": true, "that": true, "these": true, "those": true, "my": true, "your": true,
	"he": true, "she": true, "him": true, "her": true, "they": true, "them": true,
	"i": true, "you": true, "me": true, "us": true, "we": true,
	// Common verbs
	"be": true, "have": true, "do": true, "say": true, "get": true, "make": true, "go": true,
	"know": true, "take": true, "see": true, "come": true, "think": true, "look": true,
	"want": true, "give": true, "use": true, "find": true, "tell": true, "ask": true,
	"work": true, "seem": true, "feel": true, "try": true, "leave": true, "call": true,
}

// wordRegex matches sequences of Unicode letters.
var wordRegex = regexp.MustCompile(`\pL+`)

// init is called once when the package is loaded.
// It loads and unmarshals the synonym maps from the embedded JSON files
// and merges them into loadedSynonymMap.
func init() {
	loadedSynonymMap = make(map[string][]string)

	// Load and process the original synonym_map.json
	synData, err := synFiles.ReadFile("data/synonym_map.json")
	if err != nil {
		log.Fatalf("Failed to read embedded data/synonym_map.json: %v", err)
	}

	var baseSynonyms map[string][]string
	err = json.Unmarshal(synData, &baseSynonyms)
	if err != nil {
		log.Fatalf("Failed to unmarshal data/synonym_map.json: %v", err)
	}

	for key, values := range baseSynonyms {
		loadedSynonymMap[key] = append(loadedSynonymMap[key], values...)
	}

	// Load and process glove_neighbors_15k.json
	gloveData, err := synFiles.ReadFile("data/glove_neighbors_15k.json")
	if err != nil {
		log.Fatalf("Failed to read embedded data/glove_neighbors_15k.json: %v", err)
	}

	var gloveSynonyms map[string][]string
	err = json.Unmarshal(gloveData, &gloveSynonyms)
	if err != nil {
		log.Fatalf("Failed to unmarshal data/glove_neighbors_15k.json: %v", err)
	}

	for key, values := range gloveSynonyms {
		// Append new synonyms, ensuring no duplicates if that's desired,
		// but for now, simple append as per "appended to existing keys".
		// To avoid duplicates, one would need to check if a synonym already exists in loadedSynonymMap[key].
		loadedSynonymMap[key] = append(loadedSynonymMap[key], values...)
	}
}

// getStemmedTokensSet is the core tokenizer, returning a set of unique stemmed tokens.
// If expandSynonyms is true, synonyms from loadedSynonymMap will also be included.
func getStemmedTokensSet(s string, expandSynonyms bool) map[string]struct{} {
	if s == "" {
		return make(map[string]struct{}) // Return empty map, not nil
	}

	lowerS := strings.ToLower(s)
	rawWords := wordRegex.FindAllString(lowerS, -1)

	setT := make(map[string]struct{})
	for _, word := range rawWords {
		if !stopwords[word] && len(word) > 1 {
			stemmedWord := porterstemmer.StemString(word)
			setT[stemmedWord] = struct{}{} // Add original stemmed word

			if expandSynonyms {
				if synonyms, found := loadedSynonymMap[word]; found { // word is unstemmed here
					for _, syn := range synonyms {
						if !stopwords[syn] && len(syn) > 1 {
							stemmedSyn := porterstemmer.StemString(syn)
							setT[stemmedSyn] = struct{}{}
						}
					}
				}
			}
		}
	}
	return setT
}

// Tokens converts a string to a slice of lower-case, stemmed words,
// including synonyms, and deduplicated.
func Tokens(s string) []string {
	tokenSet := getStemmedTokensSet(s, true)
	if len(tokenSet) == 0 {
		return nil
	}
	tokens := make([]string, 0, len(tokenSet))
	for w := range tokenSet {
		tokens = append(tokens, w)
	}
	return tokens
}
