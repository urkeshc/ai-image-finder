package meta

import (
	"fmt"
	"math"
	"proj3-redesigned/internal/query"
	"regexp"
	"strings"
)

var countryNormalizationMap = map[string]string{
	"usa":   "United States",
	"us":    "United States",
	"u.s.":  "United States",
	"u.s.a": "United States",

	"ksa":   "Saudi Arabia",
	"saudi": "Saudi Arabia", // As per prompt request

	"uae": "United Arab Emirates",

	"uk":   "United Kingdom",
	"u.k.": "United Kingdom",
}

// normalizeCountryName returns a canonical name for a country if a known alias is provided,
// otherwise it returns the original country name. Comparisons should be case-insensitive.
func normalizeCountryName(country string) string {
	if country == "" {
		return ""
	}
	lowerCountry := strings.ToLower(strings.TrimSpace(country))
	if normalized, ok := countryNormalizationMap[lowerCountry]; ok {
		return normalized
	}
	return country // Return original for case-insensitive comparison or if already canonical/unknown
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true,
	"of": true, "in": true, "on": true, "at": true, "for": true, "to": true, "by": true, "with": true,
	"picture": true, "photo": true, "image": true, "photograph": true, "view": true,
	// Add more common words if needed, but be careful not to remove important descriptive terms.
}

// wordRegex to split by non-alphanumeric characters and get words
var wordRegex = regexp.MustCompile(`\b\w+\b`)

// matchesDescription checks if keywords from query description are in photo's descriptions.
func matchesDescription(q query.Query, p PhotoMetadata) bool {
	queryDescStr, ok := q.Metadata["photo_description"].(string)
	if !ok || queryDescStr == "" {
		return true // No description in query, so it's a pass.
	}

	// Extract keywords from query description
	rawQueryWords := wordRegex.FindAllString(strings.ToLower(queryDescStr), -1)
	var keywords []string
	for _, word := range rawQueryWords {
		if !stopwords[word] && len(word) > 2 { // Filter stopwords and short words
			// Avoid adding country names as keywords if they are handled by geo filter
			// This is a simplification; a more robust solution would involve named entity recognition
			// or checking against a list of all country names/aliases.
			// For now, we rely on the AI not putting redundant country info in description if it's also in location_country.
			isCountryTerm := false
			for _, normalizedCountry := range countryNormalizationMap {
				if strings.Contains(normalizedCountry, word) { // e.g. "states" in "United States"
					isCountryTerm = true
					break
				}
			}
			if !isCountryTerm {
				keywords = append(keywords, word)
			}
		}
	}

	if len(keywords) == 0 {
		// If query was e.g. "a picture of the USA", keywords might be empty after filtering.
		// In this case, if the query *only* contained stopwords/country, consider it a pass for description.
		// Or, if the original queryDescStr was not empty but keywords list is, it means the description
		// was effectively all stopwords or terms we decided to ignore.
		// This might need refinement based on AI behavior. For now, if no keywords, pass.
		return true
	}

	// photoText combines both PhotoDescription and AiDescription.
	photoText := strings.ToLower(p.PhotoDescription + " " + p.AiDescription)
	for _, kw := range keywords {
		if !strings.Contains(photoText, kw) {
			return false // All keywords must be present
		}
	}
	return true
}

// has reports whether the AI returned a non-nil value for key
func has(q query.Query, key string) bool {
	v, ok := q.Metadata[key]
	return ok && v != nil
}

// passesAllQueryCriteria consolidates all filter checks.
func passesAllQueryCriteria(q query.Query, p PhotoMetadata, useLatLonFilter bool, useDescriptionFilter bool) bool {
	// Geo filter
	if useLatLonFilter {
		if has(q, "photo_location_latitude") && has(q, "photo_location_longitude") {
			if !withinLatLon(q, p) {
				return false
			}
			// If lat/lon is primary, we might still want to check country/city if also specified for stricter matching.
			// For this helper, if lat/lon passes, we assume the geo part is fine for this level of strictness.
			// However, if country is also specified, it should also match.
			if has(q, "photo_location_city") || has(q, "photo_location_country") {
				if !matchesCityCountry(q, p) {
					return false // e.g. lat/lon is in France, but query also says country "Germany"
				}
			}
		} else if has(q, "photo_location_city") || has(q, "photo_location_country") {
			if !matchesCityCountry(q, p) {
				return false
			}
		}
	} else { // Not using query's lat/lon filter, rely on city/country if present in query
		if has(q, "photo_location_city") || has(q, "photo_location_country") {
			if !matchesCityCountry(q, p) {
				return false
			}
		}
	}

	// Date filters
	if has(q, "month") && !matchesMonth(q, p.PhotoSubmittedAt) {
		return false
	}
	if has(q, "year") && !matchesDate(q, p.PhotoSubmittedAt) {
		return false
	}

	// Photographer filter
	photographerCriteriaPresent := has(q, "photographer_username") || has(q, "photographer_first_name") || has(q, "photographer_last_name")
	if photographerCriteriaPresent && !matchesPhotographer(q, p) {
		return false
	}

	// Camera filter
	if (has(q, "exif_camera_make") || has(q, "exif_camera_model")) && !matchesCamera(q, p) {
		return false
	}

	// Description filter
	if useDescriptionFilter {
		if has(q, "photo_description") && !matchesDescription(q, p) {
			return false
		}
	}

	return true
}

// FilterPhotos applies your filtering logic based on the Query fields with fallbacks.
func FilterPhotos(q query.Query, photos []PhotoMetadata) []PhotoMetadata {
	var results []PhotoMetadata

	// Stage 1: Strict filtering (all criteria, including lat/lon and description if present)
	for _, p := range photos {
		if passesAllQueryCriteria(q, p, true, true) {
			results = append(results, p)
		}
	}
	if len(results) > 0 {
		return results
	}

	// Stage 2: Geo Fallback (Country/City + Other Attributes, including description, but ignoring query's specific lat/lon)
	// This triggers if Stage 1 failed AND the query actually had lat/lon.
	queryHadLatLon := has(q, "photo_location_latitude") && has(q, "photo_location_longitude")
	if queryHadLatLon {
		var geoFallbackResults []PhotoMetadata
		for _, p := range photos {
			// useLatLonFilter = false (ignore query's lat/lon, use city/country from query if available)
			// useDescriptionFilter = true (still try to match description)
			if passesAllQueryCriteria(q, p, false, true) {
				geoFallbackResults = append(geoFallbackResults, p)
			}
		}
		if len(geoFallbackResults) > 0 {
			return geoFallbackResults
		}
	}

	// Stage 3: Description Relaxed Fallback (Country/City + Other Attributes, NO description filter)
	// This triggers if Stage 1 & 2 failed AND the query had a description (implying description was too strict).
	queryHadDescription := has(q, "photo_description")
	if queryHadDescription {
		var descRelaxedFallbackResults []PhotoMetadata
		for _, p := range photos {
			// Determine if we should use the lat/lon filter based on whether the query originally had it.
			// If the query had lat/lon, we ignore it for this fallback (useLatLonFilter = false).
			// If the query did NOT have lat/lon, then city/country was the primary geo, so we respect that (useLatLonFilter = true, which in passesAllQueryCriteria means it will check city/country).
			useQueryLatLonInThisStage := false // For this stage, if lat/lon was in query, we are explicitly ignoring it.
			// The city/country check will happen regardless inside passesAllQueryCriteria if city/country is in query.
			if passesAllQueryCriteria(q, p, useQueryLatLonInThisStage, false) { // description filter is false
				descRelaxedFallbackResults = append(descRelaxedFallbackResults, p)
			}
		}
		if len(descRelaxedFallbackResults) > 0 {
			return descRelaxedFallbackResults
		}
	}

	// Stage 4: Date-Only Fallback
	queryHadDate := has(q, "year") || has(q, "month")
	if queryHadDate {
		var dateFallbackResults []PhotoMetadata
		for _, p := range photos {
			passesDateOnly := true
			if has(q, "month") && !matchesMonth(q, p.PhotoSubmittedAt) {
				passesDateOnly = false
			}
			if has(q, "year") && !matchesDate(q, p.PhotoSubmittedAt) {
				passesDateOnly = false
			}
			if passesDateOnly {
				dateFallbackResults = append(dateFallbackResults, p)
			}
		}
		if len(dateFallbackResults) > 0 {
			return dateFallbackResults
		}
	}

	return []PhotoMetadata{} // No results after all attempts
}

// FilterPhotos applies your filtering logic based on the Query fields.
func FilterPhotosOld(q query.Query, photos []PhotoMetadata) []PhotoMetadata {
	var out []PhotoMetadata
	for _, p := range photos {
		// 1) Lat/Lon filter if coords present
		if has(q, "photo_location_latitude") && has(q, "photo_location_longitude") {
			if !withinLatLon(q, p) {
				continue
			}
		} else if has(q, "photo_location_city") || has(q, "photo_location_country") {
			// 2) City/Country filter when coords absent
			if !matchesCityCountry(q, p) {
				continue
			}
		}

		// 3) Month+Year / Year-only
		if has(q, "month") {
			if !matchesMonth(q, p.PhotoSubmittedAt) {
				continue
			}
		}
		if has(q, "year") {
			if !matchesDate(q, p.PhotoSubmittedAt) {
				continue
			}
		}

		// 4) Photographer
		if has(q, "photographer_username") && !matchesPhotographer(q, p) {
			continue
		}

		// 5) Camera make/model
		if (has(q, "exif_camera_make") || has(q, "exif_camera_model")) && !matchesCamera(q, p) {
			continue
		}

		out = append(out, p)
	}

	// fallback: if nothing matched but user specified a date/year/month
	if len(out) == 0 && (has(q, "year") || has(q, "month") || has(q, "photo_submitted_at")) {
		for _, p := range photos {
			if has(q, "month") && !matchesMonth(q, p.PhotoSubmittedAt) {
				continue
			}
			if has(q, "year") && !matchesDate(q, p.PhotoSubmittedAt) {
				continue
			}
			out = append(out, p)
		}
	}

	return out
}

// matchesCityCountry
func matchesCityCountry(q query.Query, p PhotoMetadata) bool {
	queryCity, queryHasCity := q.Metadata["photo_location_city"].(string)
	photoCity := p.PhotoLocationCity

	// City check:
	// If query has a city:
	//   - If photo also has a city, they must match (case-insensitive).
	//   - If photo does NOT have a city, this specific city check is inconclusive (doesn't fail yet, depends on country).
	// If query does NOT have a city, this specific city check passes.
	if queryHasCity && queryCity != "" {
		if photoCity != "" { // Both query and photo have a city
			if !strings.EqualFold(queryCity, photoCity) {
				return false // Cities specified and they don't match
			}
		}
		// If query has city, but photo doesn't, we don't fail here.
		// The match might still be valid based on country.
		// Example: Query "Paris, France", Photo "France" (no city). This could be a broader match.
		// However, if the user is specific about a city, and the photo has no city,
		// it's arguably not a great match. For now, let's allow it if country matches.
		// A stricter version would be: if queryHasCity && photoCity == "", return false.
	}

	// Country check (must always pass if country is specified in query)
	if co, _ := q.Metadata["photo_location_country"].(string); co != "" {
		normalizedQueryCountry := normalizeCountryName(co)
		normalizedPhotoCountry := normalizeCountryName(p.PhotoLocationCountry)
		if !strings.EqualFold(normalizedQueryCountry, normalizedPhotoCountry) {
			return false // Countries specified and they don't match
		}
	}
	return true
}

// withinLatLon
func withinLatLon(q query.Query, p PhotoMetadata) bool {
	lat, _ := q.Metadata["photo_location_latitude"].(float64)
	lon, _ := q.Metadata["photo_location_longitude"].(float64)
	d := haversineDistance(lat, lon, p.PhotoLocationLatitude, p.PhotoLocationLongitude)
	return d <= 200.0
}

// matchesDate = year-only ±5
func matchesDate(q query.Query, submitted string) bool {
	py := parseYear(submitted)
	if yv, ok := q.Metadata["year"].(float64); ok {
		return math.Abs(float64(py)-yv) <= 5
	}
	if raw, ok := q.Metadata["photo_submitted_at"].(string); ok && len(raw) >= 4 {
		uy := parseYear(raw)
		return math.Abs(float64(py)-float64(uy)) <= 5
	}
	return true
}

// matchesMonth: same-year ±3-month; if year mismatch, fall back to true so year-only can apply
func matchesMonth(q query.Query, submitted string) bool {
	mf, mok := q.Metadata["month"].(float64)
	if !mok {
		return true
	}
	um := int(mf)
	pm := parseMonth(submitted)
	if pm == 0 {
		return false
	}
	if yf, yok := q.Metadata["year"].(float64); yok {
		uy := int(yf)
		py := parseYear(submitted)
		if py == uy {
			return absInt(pm-um) <= 3
		}
		// year mismatch → skip month filter, allow year-only to decide
		return true
	}
	// month-only: wrap around
	d := absInt(pm - um)
	return d <= 3 || d >= 9
}

// matchesPhotographer
func matchesPhotographer(q query.Query, p PhotoMetadata) bool {
	// Check username first if present in the query
	if queryUsernameVal, okUsernameStr := q.Metadata["photographer_username"].(string); has(q, "photographer_username") {
		if okUsernameStr { // It's a string
			if queryUsernameVal != "" { // Non-empty username query
				return strings.EqualFold(queryUsernameVal, p.PhotographerUsername)
			} else { // Empty username query ("")
				return p.PhotographerUsername == ""
			}
		} else { // Present but not a string (e.g. number, null if has() was different)
			return false // Cannot match if type is wrong
		}
	}

	// If username was not in query, evaluate based on first and/or last names.
	// All specified name parts must match.
	passesFirstNameCheck := true
	firstNameCriteriaConsidered := false
	if has(q, "photographer_first_name") {
		firstNameCriteriaConsidered = true
		if queryFirstNameVal, okFirstNameStr := q.Metadata["photographer_first_name"].(string); okFirstNameStr {
			if queryFirstNameVal != "" {
				passesFirstNameCheck = strings.EqualFold(queryFirstNameVal, p.PhotographerFirstName)
			} else { // Query for empty first name
				passesFirstNameCheck = (p.PhotographerFirstName == "")
			}
		} else { // Present but not a string
			passesFirstNameCheck = false
		}
	}

	passesLastNameCheck := true
	lastNameCriteriaConsidered := false
	if has(q, "photographer_last_name") {
		lastNameCriteriaConsidered = true
		if queryLastNameVal, okLastNameStr := q.Metadata["photographer_last_name"].(string); okLastNameStr {
			if queryLastNameVal != "" {
				passesLastNameCheck = strings.EqualFold(queryLastNameVal, p.PhotographerLastName)
			} else { // Query for empty last name
				passesLastNameCheck = (p.PhotographerLastName == "")
			}
		} else { // Present but not a string
			passesLastNameCheck = false
		}
	}

	if firstNameCriteriaConsidered || lastNameCriteriaConsidered {
		// If any first/last name criteria were considered, their combined success determines the match.
		// If only first name was considered, result is passesFirstNameCheck && true.
		// If only last name was considered, result is true && passesLastNameCheck.
		// If both, result is passesFirstNameCheck && passesLastNameCheck.
		return passesFirstNameCheck && passesLastNameCheck
	}

	// No photographer criteria were found in the query (username, first, or last name).
	// This function should ideally not be called by passesAllQueryCriteria if this is the case.
	// However, if it is, it means no filter applies.
	return true
}

// matchesCamera
func matchesCamera(q query.Query, p PhotoMetadata) bool {
	if mk, _ := q.Metadata["exif_camera_make"].(string); mk != "" {
		if !strings.EqualFold(mk, p.ExifCameraMake) {
			return false
		}
	}
	if md, _ := q.Metadata["exif_camera_model"].(string); md != "" {
		if !strings.EqualFold(md, p.ExifCameraModel) {
			return false
		}
	}
	return true
}

// haversineDistance
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * R * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// parseYear helper
func parseYear(ts string) int {
	var y int
	if len(ts) >= 4 {
		// If Sscan fails, y remains 0. We are accepting this default.
		// The linter warning was about an empty `if err != nil {}` block.
		// By removing the block, we acknowledge the error is not explicitly handled beyond Sscan's behavior.
		_, _ = fmt.Sscan(ts[:4], &y) // Explicitly ignore error for linter if default is OK
	}
	return y
}

// parseMonth helper
func parseMonth(ts string) int {
	var m int
	if len(ts) >= 7 {
		// If Sscan fails, m remains 0. We are accepting this default.
		_, _ = fmt.Sscan(ts[5:7], &m) // Explicitly ignore error for linter if default is OK
	}
	return m
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// FilterResult holds both the matched photo and the reasons for the match
type FilterResult struct {
	Photo   PhotoMetadata
	Reasons []string
}

// FilterPhotosWithReasons is similar to FilterPhotos but also returns
// the reasons why a photo matched.
func FilterPhotosWithReasons(q query.Query, photos []PhotoMetadata) []FilterResult {
	// 1) strict matching
	var strict []FilterResult
	for _, p := range photos {
		reasons := collectAllReasons(q, p)
		if len(reasons) > 0 {
			strict = append(strict, FilterResult{Photo: p, Reasons: reasons})
		}
	}
	if len(strict) > 0 {
		return strict
	}

	// 2) fallback: if user provided location but nothing matched, and user gave a date
	hasLoc := has(q, "photo_location_latitude") && has(q, "photo_location_longitude") ||
		has(q, "photo_location_city") || has(q, "photo_location_country")
	hasDate := has(q, "month") || has(q, "year") || has(q, "photo_submitted_at")
	if hasLoc && hasDate {
		var fallback []FilterResult
		for _, p := range photos {
			var reasons []string
			// month filter
			if has(q, "month") && matchesMonth(q, p.PhotoSubmittedAt) {
				mf, _ := q.Metadata["month"].(float64)
				reasons = append(reasons, fmt.Sprintf("Fallback: matched month %d", int(mf)))
			}
			// year filter
			if has(q, "year") && matchesDate(q, p.PhotoSubmittedAt) {
				yv, _ := q.Metadata["year"].(float64)
				reasons = append(reasons, fmt.Sprintf("Fallback: matched year %d", int(yv)))
			}
			if len(reasons) > 0 {
				fallback = append(fallback, FilterResult{Photo: p, Reasons: reasons})
			}
		}
		return fallback
	}

	// 3) nothing matched at all
	return nil
}

// collectAllReasons gathers EVERY passing test as a reason.
// A photo is kept if it gets ≥1 reason.
func collectAllReasons(q query.Query, photo PhotoMetadata) []string {
	var reasons []string

	// city/country
	if c := matchCityCountry(q, photo); c != "" {
		reasons = append(reasons, c)
	}
	// lat/lon
	if d := matchLatLon(q, photo); d != "" {
		reasons = append(reasons, d)
	}
	// year
	if y := matchYear(q, photo); y != "" {
		reasons = append(reasons, y)
	}
	// photographer
	if u := matchPhotographer(q, photo); u != "" {
		reasons = append(reasons, u)
	}
	// camera make/model
	if m := matchCamera(q, photo); m != "" {
		reasons = append(reasons, m)
	}

	return reasons
}

// matchCityCountry returns a reason if city or country match
func matchCityCountry(q query.Query, p PhotoMetadata) string {
	queryCity, queryHasCity := q.Metadata["photo_location_city"].(string)
	photoCity := p.PhotoLocationCity
	matchedCity := false

	if queryHasCity && queryCity != "" {
		if photoCity != "" {
			if strings.EqualFold(queryCity, photoCity) {
				matchedCity = true
			}
		} // Removed empty else block
	}

	if co, _ := q.Metadata["photo_location_country"].(string); co != "" {
		normalizedQueryCountry := normalizeCountryName(co)
		normalizedPhotoCountry := normalizeCountryName(p.PhotoLocationCountry)
		if strings.EqualFold(normalizedQueryCountry, normalizedPhotoCountry) {
			if matchedCity { // If city also matched
				return "Matched city: " + queryCity + " and country: " + co
			}
			return "Matched country: " + co // Only country matched
		}
	}
	// If only city matched (but country didn't, or country wasn't in query)
	if matchedCity {
		return "Matched city: " + queryCity
	}
	return ""
}

// matchLatLon returns reason if within 200km
func matchLatLon(q query.Query, p PhotoMetadata) string {
	if lat, lok := q.Metadata["photo_location_latitude"].(float64); lok {
		if lon, rok := q.Metadata["photo_location_longitude"].(float64); rok {
			dist := haversineDistance(lat, lon, p.PhotoLocationLatitude, p.PhotoLocationLongitude)
			if dist <= 200 {
				return "Lat/Lon match within 200km"
			}
		}
	}
	return ""
}

// matchYear returns reason if |photoYear - query.year| ≤ 5
func matchYear(q query.Query, p PhotoMetadata) string {
	if yv, ok := q.Metadata["year"].(float64); ok && yv > 0 {
		py := parseYear(p.PhotoSubmittedAt)
		if math.Abs(float64(py)-yv) <= 5 {
			return fmt.Sprintf("Matched year: %d ±5", int(yv))
		}
	}
	return ""
}

// matchPhotographer returns reason if photographer criteria match
func matchPhotographer(q query.Query, p PhotoMetadata) string {
	// Check username first
	if queryUsernameVal, okUsernameStr := q.Metadata["photographer_username"].(string); has(q, "photographer_username") {
		if okUsernameStr {
			if queryUsernameVal != "" { // Non-empty username query
				if strings.EqualFold(queryUsernameVal, p.PhotographerUsername) {
					return "Matched photographer username: " + queryUsernameVal
				}
				return "" // Username specified but does not match
			} else { // Empty username query
				if p.PhotographerUsername == "" {
					return "Matched empty photographer username"
				}
				return "" // Username specified as empty but photo's username is not empty
			}
		} else { // Present but not a string
			return "" // Type mismatch
		}
	}

	// If username was not in query, check first/last names
	firstNameReason := ""
	lastNameReason := ""
	firstNameCriteriaConsidered := false
	lastNameCriteriaConsidered := false

	if has(q, "photographer_first_name") {
		firstNameCriteriaConsidered = true
		if queryFirstNameVal, okFirstNameStr := q.Metadata["photographer_first_name"].(string); okFirstNameStr {
			if queryFirstNameVal != "" {
				if strings.EqualFold(queryFirstNameVal, p.PhotographerFirstName) {
					firstNameReason = "Matched photographer first name: " + queryFirstNameVal
				}
			} else { // Query for empty first name
				if p.PhotographerFirstName == "" {
					firstNameReason = "Matched empty photographer first name"
				}
			}
		}
	}

	if has(q, "photographer_last_name") {
		lastNameCriteriaConsidered = true
		if queryLastNameVal, okLastNameStr := q.Metadata["photographer_last_name"].(string); okLastNameStr {
			if queryLastNameVal != "" {
				if strings.EqualFold(queryLastNameVal, p.PhotographerLastName) {
					lastNameReason = "Matched photographer last name: " + queryLastNameVal
				}
			} else { // Query for empty last name
				if p.PhotographerLastName == "" {
					lastNameReason = "Matched empty photographer last name"
				}
			}
		}
	}

	if firstNameCriteriaConsidered && lastNameCriteriaConsidered { // Both first and last name were in query
		if firstNameReason != "" && lastNameReason != "" {
			return firstNameReason + "; " + lastNameReason
		}
	} else if firstNameCriteriaConsidered { // Only first name in query
		return firstNameReason
	} else if lastNameCriteriaConsidered { // Only last name in query
		return lastNameReason
	}

	return "" // No match or no relevant criteria
}

// matchCamera returns a reason for make or (non-generic) model matches
func matchCamera(q query.Query, p PhotoMetadata) string {
	if mk, _ := q.Metadata["exif_camera_make"].(string); mk != "" &&
		strings.EqualFold(mk, p.ExifCameraMake) {
		return "Matched camera make: " + mk
	}
	if md, _ := q.Metadata["exif_camera_model"].(string); len(md) > 3 &&
		!strings.EqualFold(md, "camera") &&
		strings.EqualFold(md, p.ExifCameraModel) {
		return "Matched camera model: " + md
	}
	return ""
}
