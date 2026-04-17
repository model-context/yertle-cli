package cmd

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// runeWidth returns the display width of a string, counting each rune as 1.
func runeWidth(s string) int {
	return len([]rune(s))
}

// getTerminalWidth returns the terminal width, defaulting to 120 if detection fails.
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 120
	}
	return width
}

// maxLineWidth returns the runeWidth of the longest line in the slice.
func maxLineWidth(lines []string) int {
	max := 0
	for _, l := range lines {
		if w := runeWidth(l); w > max {
			max = w
		}
	}
	return max
}

// renderSideBySide joins left and right panels with a gap between them.
// If the combined width exceeds termWidth, it falls back to vertical stacking.
func renderSideBySide(leftLines, rightLines []string, gap, termWidth int) []string {
	leftW := maxLineWidth(leftLines)
	rightW := maxLineWidth(rightLines)

	if leftW+gap+rightW > termWidth {
		// Vertical fallback: details on top, blank line, canvas below
		result := make([]string, 0, len(leftLines)+1+len(rightLines))
		result = append(result, leftLines...)
		result = append(result, "")
		result = append(result, rightLines...)
		return result
	}

	maxRows := len(leftLines)
	if len(rightLines) > maxRows {
		maxRows = len(rightLines)
	}

	separator := strings.Repeat(" ", gap)
	result := make([]string, maxRows)

	for i := 0; i < maxRows; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		// Pad left column to uniform width using rune-aware measurement
		padded := left + strings.Repeat(" ", leftW-runeWidth(left))
		if right != "" {
			result[i] = padded + separator + right
		} else {
			result[i] = left
		}
	}
	return result
}
