package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Column struct {
	Header string
	Width  int // 0 = auto-size from content
	Value  func(row any) string
}

var headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

func RenderTable(w io.Writer, columns []Column, rows []any, noColor bool) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col.Header)
		if col.Width > 0 {
			widths[i] = col.Width
		}
	}
	for _, row := range rows {
		for i, col := range columns {
			if val := col.Value(row); len(val) > widths[i] && columns[i].Width == 0 {
				widths[i] = len(val)
			}
		}
	}

	// Print header
	headers := make([]string, len(columns))
	for i, col := range columns {
		padded := fmt.Sprintf("%-*s", widths[i], col.Header)
		if noColor {
			headers[i] = padded
		} else {
			headers[i] = headerStyle.Render(padded)
		}
	}
	fmt.Fprintln(w, strings.Join(headers, "  "))

	// Print rows
	for _, row := range rows {
		vals := make([]string, len(columns))
		for i, col := range columns {
			vals[i] = fmt.Sprintf("%-*s", widths[i], col.Value(row))
		}
		fmt.Fprintln(w, strings.Join(vals, "  "))
	}
}

func RenderJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func RenderCSV(w io.Writer, columns []Column, rows []any) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Header
	}
	writer.Write(headers)

	// Data rows
	for _, row := range rows {
		vals := make([]string, len(columns))
		for i, col := range columns {
			vals[i] = col.Value(row)
		}
		writer.Write(vals)
	}
}
