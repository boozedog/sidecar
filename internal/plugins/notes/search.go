package notes

import (
	"sort"
	"strings"
	"unicode"
)

// NoteMatch represents a note matching the search query.
type NoteMatch struct {
	Note  Note
	Score int // Higher = better match
}

// FuzzyMatchNote scores how well query matches a note's title and content.
// Returns score (0 = no match). Scoring:
//   - Consecutive matches: bonus per consecutive char
//   - Word start matches: large bonus
//   - Title match bonus: prefer title matches over content
//   - Shorter text: small bonus
func FuzzyMatchNote(query string, note Note) int {
	if query == "" {
		return 0
	}

	queryLower := strings.ToLower(query)

	// Check title first (prefer title matches)
	title := note.Title
	if title == "" {
		// Use first line of content as title
		lines := strings.SplitN(note.Content, "\n", 2)
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			title = strings.TrimSpace(lines[0])
		}
	}

	// Score title match
	titleScore := fuzzyScore(queryLower, strings.ToLower(title))
	if titleScore > 0 {
		titleScore += 50 // Big bonus for title match
	}

	// Score content match
	contentScore := fuzzyScore(queryLower, strings.ToLower(note.Content))

	// Return the best score
	if titleScore > contentScore {
		return titleScore
	}
	return contentScore
}

// fuzzyScore computes fuzzy match score between query and target.
func fuzzyScore(query, target string) int {
	qi := 0 // query index
	score := 0
	consecutive := 0

	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if target[ti] == query[qi] {
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
			consecutive = 0
		}
	}

	// Did we match all query characters?
	if qi < len(query) {
		return 0 // Incomplete match
	}

	// Bonus for shorter targets (prefer more concise matches)
	score += 20 / (len(target) + 1)

	return score
}

// isWordSeparator returns true for characters that start word boundaries.
func isWordSeparator(r rune) bool {
	return r == ' ' || r == '_' || r == '-' || r == '.' || r == '/' || r == '\n' || r == '\t'
}

// ExactTitleMatch returns true if query matches a note's title exactly (case-insensitive).
func ExactTitleMatch(query string, note Note) bool {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	if queryLower == "" {
		return false
	}

	title := note.Title
	if title == "" {
		// Use first line of content as title
		lines := strings.SplitN(note.Content, "\n", 2)
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			title = strings.TrimSpace(lines[0])
		}
	}

	return strings.ToLower(strings.TrimSpace(title)) == queryLower
}

// FilterNotes filters and scores notes against a query.
// Returns matches sorted by score descending.
func FilterNotes(notes []Note, query string) []NoteMatch {
	if query == "" {
		// Return all notes as matches with default score
		var matches []NoteMatch
		for _, note := range notes {
			matches = append(matches, NoteMatch{
				Note:  note,
				Score: 100, // Default score for unfiltered
			})
		}
		return matches
	}

	var matches []NoteMatch
	for _, note := range notes {
		score := FuzzyMatchNote(query, note)
		if score > 0 {
			matches = append(matches, NoteMatch{
				Note:  note,
				Score: score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

// FindExactTitleMatch returns the note with exact title match, or nil if none.
func FindExactTitleMatch(notes []Note, query string) *Note {
	for i := range notes {
		if ExactTitleMatch(query, notes[i]) {
			return &notes[i]
		}
	}
	return nil
}
