package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analysis"
)

// FormatOptions represents formatting options
type FormatOptions struct {
	NoColor bool
	Format  string // "table", "json", "text"
}

// FormatResults formats analysis results for display
func FormatResults(results []*analysis.Result, options FormatOptions) string {
	switch options.Format {
	case "json":
		return formatResultsAsJSON(results)
	case "text":
		return formatResultsAsText(results, options.NoColor)
	default:
		return formatResultsAsTable(results, options.NoColor)
	}
}

// formatResultsAsJSON formats results as JSON
func formatResultsAsJSON(results []*analysis.Result) string {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting results: %v", err)
	}
	return string(data)
}

// formatResultsAsText formats results as human-readable text
func formatResultsAsText(results []*analysis.Result, noColor bool) string {
	var builder strings.Builder

	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n")
		}

		// Format header
		header := fmt.Sprintf("Results from %s (%s)", result.AnalyserName, result.Repository)
		if noColor {
			builder.WriteString(header + "\n")
			builder.WriteString(strings.Repeat("-", len(header)) + "\n")
		} else {
			builder.WriteString(pterm.Bold.Sprint(header) + "\n")
			builder.WriteString(pterm.Bold.Sprint(strings.Repeat("-", len(header))) + "\n")
		}

		// Format run details
		duration := result.EndTime.Sub(result.StartTime)
		builder.WriteString(fmt.Sprintf("Duration: %s\n", formatDuration(duration)))

		if !result.Success {
			errMsg := fmt.Sprintf("Failed: %s", result.Error)
			if noColor {
				builder.WriteString(errMsg + "\n\n")
			} else {
				builder.WriteString(pterm.Red(errMsg) + "\n\n")
			}
			continue
		}

		builder.WriteString(fmt.Sprintf("Metrics: %d\n\n", len(result.Metrics)))

		// Group metrics by name
		metricsByName := make(map[string][]analysis.Metric)
		for _, metric := range result.Metrics {
			metricsByName[metric.Name] = append(metricsByName[metric.Name], metric)
		}

		// Format metrics
		for name, metrics := range metricsByName {
			if noColor {
				builder.WriteString(fmt.Sprintf("%s:\n", name))
			} else {
				builder.WriteString(fmt.Sprintf("%s:\n", pterm.Bold.Sprint(name)))
			}

			for _, metric := range metrics {
				valueStr := formatValue(metric.Value)

				// Build label string if present
				labelStr := ""
				if len(metric.Labels) > 0 {
					labelParts := make([]string, 0, len(metric.Labels))
					for k, v := range metric.Labels {
						labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
					}
					labelStr = fmt.Sprintf(" [%s]", strings.Join(labelParts, ", "))
				}

				// Add key if present
				keyStr := ""
				if metric.Key != "" {
					keyStr = fmt.Sprintf(" (%s)", metric.Key)
				}

				builder.WriteString(fmt.Sprintf("  %s%s%s\n", valueStr, keyStr, labelStr))
			}
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// formatResultsAsTable formats results as a table
func formatResultsAsTable(results []*analysis.Result, noColor bool) string {
	var builder strings.Builder

	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n")
		}

		// Format header
		header := fmt.Sprintf("Results from %s (%s)", result.AnalyserName, result.Repository)
		if noColor {
			builder.WriteString(header + "\n")
			builder.WriteString(strings.Repeat("-", len(header)) + "\n")
		} else {
			builder.WriteString(pterm.Bold.Sprint(header) + "\n")
			builder.WriteString(pterm.Bold.Sprint(strings.Repeat("-", len(header))) + "\n")
		}

		// Format run details
		duration := result.EndTime.Sub(result.StartTime)
		builder.WriteString(fmt.Sprintf("Duration: %s\n", formatDuration(duration)))

		if !result.Success {
			errMsg := fmt.Sprintf("Failed: %s", result.Error)
			if noColor {
				builder.WriteString(errMsg + "\n\n")
			} else {
				builder.WriteString(pterm.Red(errMsg) + "\n\n")
			}
			continue
		}

		builder.WriteString(fmt.Sprintf("Metrics: %d\n\n", len(result.Metrics)))

		// Group metrics by name
		metricsByName := make(map[string][]analysis.Metric)
		for _, metric := range result.Metrics {
			metricsByName[metric.Name] = append(metricsByName[metric.Name], metric)
		}

		// Format each metric group as a table
		for name, metrics := range metricsByName {
			if len(metrics) == 1 && len(metrics[0].Labels) == 0 {
				// Simple metric with no labels, format as key-value
				metric := metrics[0]
				builder.WriteString(fmt.Sprintf("%s: %s\n", name, formatValue(metric.Value)))
				continue
			}

			if noColor {
				builder.WriteString(fmt.Sprintf("%s:\n", name))
			} else {
				builder.WriteString(fmt.Sprintf("%s:\n", pterm.Bold.Sprint(name)))
			}

			// Build table data
			var tableData [][]string
			var headers []string

			// Determine columns based on first metric
			hasKey := metrics[0].Key != ""
			labelKeys := make([]string, 0)
			for k := range metrics[0].Labels {
				labelKeys = append(labelKeys, k)
			}

			// Build headers
			headers = append(headers, "Value")
			if hasKey {
				headers = append(headers, "Key")
			}
			headers = append(headers, labelKeys...)

			// Build rows
			for _, metric := range metrics {
				row := make([]string, 0, len(headers))
				row = append(row, formatValue(metric.Value))

				if hasKey {
					row = append(row, metric.Key)
				}

				for _, labelKey := range labelKeys {
					row = append(row, metric.Labels[labelKey])
				}

				tableData = append(tableData, row)
			}

			// Format table
			tableStr := formatTable(headers, tableData, noColor)
			builder.WriteString(tableStr)
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// formatTable creates a formatted table
func formatTable(headers []string, data [][]string, noColor bool) string {
	if noColor {
		var builder strings.Builder

		// Format headers
		headerRow := "| " + strings.Join(headers, " | ") + " |"
		builder.WriteString(headerRow + "\n")

		// Format separator
		separators := make([]string, len(headers))
		for i, header := range headers {
			separators[i] = strings.Repeat("-", len(header))
		}
		separatorRow := "| " + strings.Join(separators, " | ") + " |"
		builder.WriteString(separatorRow + "\n")

		// Format data rows
		for _, row := range data {
			dataRow := "| " + strings.Join(row, " | ") + " |"
			builder.WriteString(dataRow + "\n")
		}

		return builder.String()
	} else {
		// Use pterm.TableData for colored output
		tableData := make(pterm.TableData, 0, len(data)+1)
		tableData = append(tableData, headers)

		for _, row := range data {
			tableData = append(tableData, row)
		}

		// Render table to string
		table, _ := pterm.DefaultTable.WithHasHeader().WithData(tableData).Srender()
		return table
	}
}

// formatValue formats a value for display
func formatValue(value interface{}) string {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%.2f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case string:
		return v
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		return fmt.Sprintf("%x", v)
	default:
		if v == nil {
			return "(nil)"
		}
		// Try to marshal complex types to JSON
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.2f sec", d.Seconds())
	} else {
		minutes := d / time.Minute
		seconds := (d % time.Minute) / time.Second
		return fmt.Sprintf("%d min %d sec", minutes, seconds)
	}
}

// FormatMetric formats a single metric for display
func FormatMetric(metric analysis.Metric, noColor bool) string {
	var builder strings.Builder

	valueStr := formatValue(metric.Value)

	name := metric.Name
	if !noColor {
		name = pterm.Bold.Sprint(name)
	}

	builder.WriteString(fmt.Sprintf("%s: %s", name, valueStr))

	// Add key if present
	if metric.Key != "" {
		builder.WriteString(fmt.Sprintf(" (%s)", metric.Key))
	}

	// Add labels if present
	if len(metric.Labels) > 0 {
		labelParts := make([]string, 0, len(metric.Labels))
		for k, v := range metric.Labels {
			labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
		}
		labelStr := strings.Join(labelParts, ", ")
		builder.WriteString(fmt.Sprintf(" [%s]", labelStr))
	}

	return builder.String()
}

// ShowSuccessMessage displays a success message
func ShowSuccessMessage(message string) {
	pterm.Success.Println(message)
}

// ShowInfoMessage displays an info message
func ShowInfoMessage(message string) {
	pterm.Info.Println(message)
}

// ShowWarningMessage displays a warning message
func ShowWarningMessage(message string) {
	pterm.Warning.Println(message)
}

// ShowErrorMessage displays an error message
func ShowErrorMessage(message string) {
	pterm.Error.Println(message)
}

// ShowProgressBar creates and returns a progress bar
func ShowProgressBar(total int, title string) *pterm.ProgressbarPrinter {
	pb, _ := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithRemoveWhenDone(true).
		Start()
	return pb
}

// ShowSpinner creates and returns a spinner
func ShowSpinner(text string) *pterm.SpinnerPrinter {
	spinner, _ := pterm.DefaultSpinner.
		WithText(text).
		Start()
	return spinner
}
