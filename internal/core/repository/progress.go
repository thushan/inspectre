package repository

import (
	"fmt"
	"regexp"
	"strings"
)

// Write implements io.Writer
func (pw *progressWriter) Write(p []byte) (n int, err error) {
	// Extract a readable progress message
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		// Try to parse progress information
		progress := CloneProgress{
			Message: msg,
		}

		// Look for counts/percentages in output
		// Example format: "Receiving objects:  67% (591/881)"
		re := regexp.MustCompile(`(\d+)%\s+\((\d+)/(\d+)\)`)
		if matches := re.FindStringSubmatch(msg); len(matches) >= 4 {
			// Extract current and total from match groups
			current, _ := fmt.Sscanf(matches[2], "%d", &progress.Current)
			total, _ := fmt.Sscanf(matches[3], "%d", &progress.Total)
			if current > 0 && total > 0 {
				progress.Current = int64(current)
				progress.Total = int64(total)
			}
		}

		// Send progress update (non-blocking)
		select {
		case pw.ch <- progress:
			// Progress update sent
		default:
			// Channel buffer full, skip this update
		}
	}
	return len(p), nil
}
