package commands

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
)

func queryAction(c *cli.Context) error {
	display := createDisplay(c)

	if err := setup(c.String("config")); err != nil {
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}

	query := c.String("sql")
	outputFormat := c.String("output")

	// Start spinner while executing query
	spinner := display.StartSpinner(fmt.Sprintf("Executing query: %s", query))

	results, err := storageManager.QueryMetrics(query)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Query failed: %v", err))
		return fmt.Errorf("query failed: %v", err)
	}

	if len(results) == 0 {
		spinner.Info("No results found")
		return nil
	}

	spinner.Success(fmt.Sprintf("Query returned %d results", len(results)))

	// Format output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to marshal results to JSON: %v", err))
			return fmt.Errorf("failed to marshal results to JSON: %v", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Create table data
	var headers []string
	if len(results) > 0 {
		for key := range results[0] {
			headers = append(headers, key)
		}
	}

	rows := make([][]string, 0, len(results))
	for _, row := range results {
		var values []string
		for _, key := range headers {
			val := row[key]
			values = append(values, fmt.Sprintf("%v", val))
		}
		rows = append(rows, values)
	}

	// Print table
	display.PrintResultTable(headers, rows)
	return nil
}
