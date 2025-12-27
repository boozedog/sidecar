package filebrowser

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

// QuickOpenMatch represents a file matching the fuzzy query.
type QuickOpenMatch struct {
	Path        string       // Relative path from root
	Name        string       // Base filename
	Score       int          // Match score (higher = better)
	MatchRanges []MatchRange // Ranges for highlighting matched chars
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

	// Bonus for shorter paths (prefer shallow files)
	score += 50 / (len(target) + 1)

	// Bonus for filename match vs path match
	lastSlash := strings.LastIndex(target, "/")
	if lastSlash == -1 {
		lastSlash = 0
	}
	filename := target[lastSlash:]
	if strings.Contains(strings.ToLower(filename), queryLower) {
		score += 20 // Bonus for matching in filename portion
	}

	return score, ranges
}

// isWordSeparator returns true for characters that start word boundaries.
func isWordSeparator(r rune) bool {
	return r == '/' || r == '_' || r == '-' || r == '.' || r == ' '
}

// FuzzySort sorts matches by score descending, then path length ascending.
func FuzzySort(matches []QuickOpenMatch) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return len(matches[i].Path) < len(matches[j].Path)
	})
}

// FuzzyFilter filters and scores files against a query.
// Returns top maxResults matches sorted by score.
func FuzzyFilter(files []string, query string, maxResults int) []QuickOpenMatch {
	if query == "" {
		// Return first maxResults files sorted by path length
		var matches []QuickOpenMatch
		for i, f := range files {
			if i >= maxResults {
				break
			}
			name := f
			if idx := strings.LastIndex(f, "/"); idx != -1 {
				name = f[idx+1:]
			}
			matches = append(matches, QuickOpenMatch{
				Path:  f,
				Name:  name,
				Score: 100 / (len(f) + 1), // Prefer shorter paths
			})
		}
		FuzzySort(matches)
		return matches
	}

	var matches []QuickOpenMatch
	for _, f := range files {
		score, ranges := FuzzyMatch(query, f)
		if score > 0 {
			name := f
			if idx := strings.LastIndex(f, "/"); idx != -1 {
				name = f[idx+1:]
			}
			matches = append(matches, QuickOpenMatch{
				Path:        f,
				Name:        name,
				Score:       score,
				MatchRanges: ranges,
			})
		}
	}

	FuzzySort(matches)

	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	return matches
}
