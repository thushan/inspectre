package commands

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
)

func reposAction(c *cli.Context) error {
	display := createDisplay(c)

	if err := setup(c.String("config")); err != nil {
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}

	outputFormat := c.String("output")

	// Start spinner while retrieving repositories
	spinner := display.StartSpinner("Retrieving repositories...")

	repos, err := repoManager.ListRepositories()
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to list repositories: %v", err))
		return fmt.Errorf("failed to list repositories: %v", err)
	}

	if len(repos) == 0 {
		spinner.Info("No repositories configured")
		return nil
	}

	spinner.Success(fmt.Sprintf("Found %d repositories", len(repos)))

	// Format output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(repos, "", "  ")
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to marshal repositories to JSON: %v", err))
			return fmt.Errorf("failed to marshal repositories to JSON: %v", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Create table data
	headers := []string{"NAME", "URL", "TYPE", "AUTH"}
	rows := make([][]string, 0, len(repos))

	for _, repo := range repos {
		authType := "none"
		if repo.Auth.Token != "" {
			authType = "token"
		} else if repo.Auth.Username != "" {
			authType = "user/pass"
		}

		rows = append(rows, []string{
			repo.Name,
			repo.URL,
			repo.Type,
			authType,
		})
	}

	// Print table
	display.PrintResultTable(headers, rows)
	return nil
}
