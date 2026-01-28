// Package conversations provides content search execution for cross-conversation search.
package conversations

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/adapter"
)

const (
	// searchTimeout is the max duration for the entire search operation.
	searchTimeout = 30 * time.Second
	// maxVisibleMatches is the visible match limit for UX (td-8e1a2b)
	maxVisibleMatches = 100
	// maxTotalMatches is the global match limit for early termination.
	maxTotalMatches = 500
	// debounceDelay is the delay before executing search after input.
	debounceDelay = 200 * time.Millisecond
)

// searchConcurrency returns the concurrency limit based on CPU count (td-80cbe1).
// We use NumCPU() to scale with the machine, with a minimum of 4 and max of 16.
// Limits explained:
//   - Min 4: Ensures reasonable parallelism even on low-end hardware
//   - Max 16: Prevents excessive file descriptor usage and memory pressure
//     from too many concurrent session file reads
func searchConcurrency() int {
	n := runtime.NumCPU()
	if n < 4 {
		return 4
	}
	if n > 16 {
		return 16
	}
	return n
}

// RunContentSearch executes search across all sessions using their adapters.
// Returns a tea.Cmd that produces ContentSearchResultsMsg when complete.
// Search runs in parallel with concurrency limit, timeout, and match cap.
// Performance optimizations (td-80cbe1):
//   - Concurrency scales with CPU count
//   - Sessions sorted by UpdatedAt (recent first for early results)
//   - Empty sessions skipped
func RunContentSearch(query string, sessions []adapter.Session,
	adapters map[string]adapter.Adapter, opts adapter.SearchOptions, epoch uint64) tea.Cmd {
	return func() tea.Msg {
		if query == "" {
			return ContentSearchResultsMsg{Epoch: epoch, Results: nil}
		}

		// Performance: sort sessions by UpdatedAt descending before searching (td-80cbe1)
		// This prioritizes recent sessions and improves perceived performance
		sortedSessions := make([]adapter.Session, len(sessions))
		copy(sortedSessions, sessions)
		sort.Slice(sortedSessions, func(i, j int) bool {
			return sortedSessions[i].UpdatedAt.After(sortedSessions[j].UpdatedAt)
		})

		var results []SessionSearchResult
		var mu sync.Mutex
		var wg sync.WaitGroup
		concurrency := searchConcurrency()
		sem := make(chan struct{}, concurrency)

		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()

		totalMatches := 0
		done := make(chan struct{})

	sessionLoop:
		for _, session := range sortedSessions {
			// Performance: skip sessions with no messages (td-80cbe1)
			if session.MessageCount == 0 {
				continue
			}

			// Check if we've hit the match limit
			mu.Lock()
			if totalMatches >= maxTotalMatches {
				mu.Unlock()
				break sessionLoop
			}
			mu.Unlock()

			// Check context cancellation
			select {
			case <-ctx.Done():
				break sessionLoop
			default:
			}

			wg.Add(1)
			go func(s adapter.Session) {
				defer wg.Done()

				// Acquire semaphore or bail on context cancel
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}

				// Get adapter for this session
				adp, ok := adapters[s.AdapterID]
				if !ok || adp == nil {
					return
				}

				// Check if adapter supports search
				searcher, ok := adp.(adapter.MessageSearcher)
				if !ok {
					return
				}

				// Execute search
				matches, err := searcher.SearchMessages(s.ID, query, opts)
				if err != nil || len(matches) == 0 {
					return
				}

				matchCount := countMatches(matches)

				mu.Lock()
				results = append(results, SessionSearchResult{
					Session:   s,
					Messages:  matches,
					Collapsed: false,
				})
				totalMatches += matchCount
				mu.Unlock()
			}(session)
		}

		// Wait for all goroutines in a separate goroutine
		go func() {
			wg.Wait()
			close(done)
		}()

		// Wait for completion or context timeout
		select {
		case <-done:
		case <-ctx.Done():
		}

		// Sort results by session UpdatedAt descending (most recent first)
		sort.Slice(results, func(i, j int) bool {
			return results[i].Session.UpdatedAt.After(results[j].Session.UpdatedAt)
		})

		// Count total matches and cap visible results (td-8e1a2b)
		totalFound := totalMatches
		truncated := totalMatches > maxVisibleMatches

		// Truncate results to maxVisibleMatches
		if truncated {
			visibleCount := 0
			truncatedResults := make([]SessionSearchResult, 0, len(results))
			for _, sr := range results {
				if visibleCount >= maxVisibleMatches {
					break
				}
				// Count matches in this session
				sessionMatches := 0
				for _, mm := range sr.Messages {
					sessionMatches += len(mm.Matches)
				}
				if visibleCount+sessionMatches <= maxVisibleMatches {
					// Include whole session
					truncatedResults = append(truncatedResults, sr)
					visibleCount += sessionMatches
				} else {
					// Need to truncate within this session
					remaining := maxVisibleMatches - visibleCount
					truncatedSession := SessionSearchResult{
						Session:   sr.Session,
						Collapsed: sr.Collapsed,
					}
					for _, mm := range sr.Messages {
						if remaining <= 0 {
							break
						}
						if len(mm.Matches) <= remaining {
							truncatedSession.Messages = append(truncatedSession.Messages, mm)
							remaining -= len(mm.Matches)
						} else {
							// Truncate matches within message
							truncatedMsg := adapter.MessageMatch{
								MessageID:  mm.MessageID,
								MessageIdx: mm.MessageIdx,
								Role:       mm.Role,
								Timestamp:  mm.Timestamp,
								Model:      mm.Model,
								Matches:    mm.Matches[:remaining],
							}
							truncatedSession.Messages = append(truncatedSession.Messages, truncatedMsg)
							remaining = 0
						}
					}
					if len(truncatedSession.Messages) > 0 {
						truncatedResults = append(truncatedResults, truncatedSession)
					}
					break
				}
			}
			results = truncatedResults
		}

		// Include query in results for staleness validation (td-5b9928)
		return ContentSearchResultsMsg{
			Epoch:        epoch,
			Results:      results,
			Query:        query,
			TotalMatches: totalFound,
			Truncated:    truncated,
		}
	}
}

// countMatches returns total ContentMatch count across messages.
func countMatches(matches []adapter.MessageMatch) int {
	count := 0
	for _, m := range matches {
		count += len(m.Matches)
	}
	return count
}

// scheduleContentSearch returns a tea.Cmd that triggers search after debounce.
// The returned Tick sends ContentSearchDebounceMsg after debounceDelay.
func scheduleContentSearch(query string, version int) tea.Cmd {
	return tea.Tick(debounceDelay, func(t time.Time) tea.Msg {
		return ContentSearchDebounceMsg{
			Version: version,
			Query:   query,
		}
	})
}
