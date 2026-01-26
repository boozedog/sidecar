package tty

import (
	"strings"
)

// Terminal mode escape sequences
const (
	BracketedPasteEnable  = "\x1b[?2004h" // ESC[?2004h - app enables bracketed paste
	BracketedPasteDisable = "\x1b[?2004l" // ESC[?2004l - app disables bracketed paste
	BracketedPasteStart   = "\x1b[200~"   // ESC[200~ - start of pasted content
	BracketedPasteEnd     = "\x1b[201~"   // ESC[201~ - end of pasted content

	MouseModeEnable1000  = "\x1b[?1000h"
	MouseModeEnable1002  = "\x1b[?1002h"
	MouseModeEnable1003  = "\x1b[?1003h"
	MouseModeEnable1006  = "\x1b[?1006h"
	MouseModeEnable1015  = "\x1b[?1015h"
	MouseModeDisable1000 = "\x1b[?1000l"
	MouseModeDisable1002 = "\x1b[?1002l"
	MouseModeDisable1003 = "\x1b[?1003l"
	MouseModeDisable1006 = "\x1b[?1006l"
	MouseModeDisable1015 = "\x1b[?1015l"
)

// DetectBracketedPasteMode checks captured output to determine if the app has
// enabled bracketed paste mode. Looks for the most recent occurrence of either
// the enable (ESC[?2004h) or disable (ESC[?2004l) sequence.
func DetectBracketedPasteMode(output string) bool {
	enableIdx := strings.LastIndex(output, BracketedPasteEnable)
	disableIdx := strings.LastIndex(output, BracketedPasteDisable)
	// If enable was found more recently than disable, bracketed paste is enabled
	return enableIdx > disableIdx
}

// DetectMouseReportingMode checks captured output to determine if the app has
// enabled mouse reporting. Looks for the most recent occurrence of enable vs disable
// sequences across all mouse mode types.
func DetectMouseReportingMode(output string) bool {
	enableSeqs := []string{
		MouseModeEnable1000,
		MouseModeEnable1002,
		MouseModeEnable1003,
		MouseModeEnable1006,
		MouseModeEnable1015,
	}
	disableSeqs := []string{
		MouseModeDisable1000,
		MouseModeDisable1002,
		MouseModeDisable1003,
		MouseModeDisable1006,
		MouseModeDisable1015,
	}

	latestEnable := -1
	for _, seq := range enableSeqs {
		if idx := strings.LastIndex(output, seq); idx > latestEnable {
			latestEnable = idx
		}
	}

	latestDisable := -1
	for _, seq := range disableSeqs {
		if idx := strings.LastIndex(output, seq); idx > latestDisable {
			latestDisable = idx
		}
	}

	return latestEnable > latestDisable
}
