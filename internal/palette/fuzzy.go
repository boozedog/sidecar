package palette

import (
	"sort"
	"strings"
	"unicode"
)

// MatchRange represents a contiguous range of matched characters.
type MatchRange struct {
	Start int
	End   int
}

// FuzzyMatch scores how well query matches target string.
// Returns score (0 = no match) and ranges for highlighting.
// Scoring:
//   - Consecutive matches: bonus per consecutive char
//   - Word start matches (after /, _, -, .): large bonus
//   - Shorter targets: small bonus
func FuzzyMatch(query, target string) (int, []MatchRange) {
	if query == "" {
		return 0, nil
	}

	queryLower := strings.ToLower(query)
	targetLower := strings.ToLower(target)

	qi := 0 // query index
	score := 0
	consecutive := 0
	var ranges []MatchRange
	currentRange := MatchRange{Start: -1, End: -1}

	for ti := 0; ti < len(targetLower) && qi < len(queryLower); ti++ {
		if targetLower[ti] == queryLower[qi] {
			// Match found
			if currentRange.Start == -1 {
				currentRange.Start = ti
			}
			currentRange.End = ti + 1

			qi++
			consecutive++

			// Base score + consecutive bonus
			score += 1 + consecutive

			// Word start bonus (after separator or at start)
			if ti == 0 || isWordSeparator(rune(target[ti-1])) {
				score += 10
			}

			// Capital letter bonus (camelCase boundary)
			if ti > 0 && unicode.IsUpper(rune(target[ti])) && unicode.IsLower(rune(target[ti-1])) {
				score += 5
			}
		} else {
			// No match - close current range if open
			if currentRange.Start != -1 {
				ranges = append(ranges, currentRange)
				currentRange = MatchRange{Start: -1, End: -1}
			}
			consecutive = 0
		}
	}

	// Close final range
	if currentRange.Start != -1 {
		ranges = append(ranges, currentRange)
	}

	// Did we match all query characters?
	if qi < len(queryLower) {
		return 0, nil // Incomplete match
	}

	// Bonus for shorter strings (prefer concise names)
	score += 50 / (len(target) + 1)

	return score, ranges
}

// isWordSeparator returns true for characters that start word boundaries.
func isWordSeparator(r rune) bool {
	return r == '/' || r == '_' || r == '-' || r == '.' || r == ' '
}

// ScoreEntry scores a palette entry against a query.
// Combines scores from multiple fields with weighting.
func ScoreEntry(entry *PaletteEntry, query string) {
	if query == "" {
		entry.Score = 0
		entry.MatchRanges = nil
		return
	}

	// Score name (highest weight)
	nameScore, nameRanges := FuzzyMatch(query, entry.Name)

	// Score key
	keyScore, _ := FuzzyMatch(query, entry.Key)

	// Score description
	descScore, _ := FuzzyMatch(query, entry.Description)

	// Score category
	catScore, _ := FuzzyMatch(query, string(entry.Category))

	// Weighted combination: name 3x, key 2x, desc 1x, category 0.5x
	baseScore := nameScore*3 + keyScore*2 + descScore + catScore/2

	// Only apply layer boost if there's at least one match
	if baseScore > 0 {
		switch entry.Layer {
		case LayerCurrentMode:
			baseScore += 100
		case LayerPlugin:
			baseScore += 50
		}
	}

	entry.Score = baseScore

	// Use name match ranges for highlighting
	entry.MatchRanges = nameRanges
}

// SortEntries sorts entries by score descending, then by layer, then alphabetically.
func SortEntries(entries []PaletteEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		if entries[i].Layer != entries[j].Layer {
			return entries[i].Layer < entries[j].Layer
		}
		return entries[i].Name < entries[j].Name
	})
}

// FilterEntries filters and scores entries against a query.
// Returns entries sorted by relevance.
func FilterEntries(entries []PaletteEntry, query string) []PaletteEntry {
	if query == "" {
		// No filter - return all sorted by layer then name
		result := make([]PaletteEntry, len(entries))
		copy(result, entries)
		sort.Slice(result, func(i, j int) bool {
			if result[i].Layer != result[j].Layer {
				return result[i].Layer < result[j].Layer
			}
			return result[i].Name < result[j].Name
		})
		return result
	}

	var matched []PaletteEntry
	for _, e := range entries {
		entry := e // copy
		ScoreEntry(&entry, query)
		if entry.Score > 0 {
			matched = append(matched, entry)
		}
	}

	SortEntries(matched)
	return matched
}
