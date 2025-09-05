package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format represents an output format type.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ParseFormat converts a string to a Format, defaulting to table.
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	default:
		return FormatTable
	}
}

// Column defines a table column.
type Column struct {
	Header string
	Width  int
}

// Table renders data in a formatted table.
func Table(w io.Writer, columns []Column, rows [][]string) {
	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col.Header)
		if col.Width > widths[i] {
			widths[i] = col.Width
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerLine := ""
	separatorLine := ""
	for i, col := range columns {
		headerLine += fmt.Sprintf("%-*s", widths[i]+2, col.Header)
		separatorLine += strings.Repeat("─", widths[i]) + "  "
	}
	fmt.Fprintln(w, headerLine)
	fmt.Fprintln(w, separatorLine)

	// Print rows
	for _, row := range rows {
		line := ""
		for i, cell := range row {
			if i < len(widths) {
				line += fmt.Sprintf("%-*s", widths[i]+2, cell)
			}
		}
		fmt.Fprintln(w, line)
	}
}

// JSON renders data as formatted JSON.
func JSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// YAML renders data as YAML.
func YAML(w io.Writer, data interface{}) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(data)
}

// Render outputs data in the specified format.
func Render(w io.Writer, format Format, columns []Column, rows [][]string, rawData interface{}) error {
	switch format {
	case FormatJSON:
		return JSON(w, rawData)
	case FormatYAML:
		return YAML(w, rawData)
	default:
		Table(w, columns, rows)
		return nil
	}
}

// StatusColor returns an ANSI-colored status string.
func StatusColor(status string) string {
	upper := strings.ToUpper(status)
	switch {
	case upper == "ACTIVE" || upper == "RUNNING" || upper == "SUCCEEDED" || upper == "READY":
		return "\033[32m" + status + "\033[0m" // Green
	case upper == "CREATING" || upper == "UPDATING" || upper == "PROVISIONING":
		return "\033[33m" + status + "\033[0m" // Yellow
	case upper == "ERROR" || upper == "FAILED" || upper == "DELETING":
		return "\033[31m" + status + "\033[0m" // Red
	default:
		return status
	}
}
