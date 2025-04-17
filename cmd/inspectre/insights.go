package commands

import (
	"encoding/json"
	"fmt"
	"github.com/thushan/inspectre/internal/core/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thushan/inspectre/internal/core/analysis"
	"github.com/thushan/inspectre/internal/core/repository"
	"github.com/urfave/cli/v2"
)

const PublicReadFilePerm = 0o644

// InsightsCommands returns CLI commands for generating insights
func InsightsCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "insights",
			Usage:     "Generate insights for a repository",
			ArgsUsage: "<repository-name-or-url>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "Output format (json, markdown, text)",
					Value:   "text",
				},
				&cli.StringFlag{
					Name:    "file",
					Aliases: []string{"f"},
					Usage:   "Output file path (default: stdout)",
				},
				&cli.StringFlag{
					Name:    "config",
					Aliases: []string{"c"},
					Usage:   "Custom path for configuration data",
				},
				&cli.BoolFlag{
					Name:  "force",
					Usage: "Force re-analysis even if insights exist",
				},
			},
			Action: insightsAction,
		},
	}
}

func insightsAction(c *cli.Context) error {
	if err := setup(c.String("config")); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	repoArg := c.Args().First()
	if repoArg == "" {
		return fmt.Errorf("repository name or URL is required")
	}

	task, err := taskManager.CreateTask(repoArg)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	outputFormat := c.String("output")
	outputFile := c.String("file")
	force := c.Bool("force")

	if err := os.MkdirAll(task.WorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	logPath := filepath.Join(task.WorkDir, "insights.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	logger := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fmt.Fprintln(logFile, message)
		fmt.Println(message) // Also print to stdout
	}

	logger("Starting insights generation for %s", repoArg)

	insightsPath := filepath.Join(task.WorkDir, "insights.json")
	if !force && utils.FileExists(insightsPath) {
		logger("Using cached insights from %s", insightsPath)
		return outputInsights(insightsPath, outputFormat, outputFile)
	}

	var repoPath string

	if filepath.IsAbs(repoArg) || utils.FileExists(repoArg) {
		// Local directory
		repoPath = repoArg
		logger("Using local repository at %s", repoPath)
	} else {
		// Clone the repository
		logger("Cloning repository %s to %s", repoArg, task.WorkDir)

		var repo *repository.Repository
		isURL := strings.HasPrefix(repoArg, "http") || strings.HasPrefix(repoArg, "git@")

		if isURL {
			// Create a temporary repository object
			repo = &repository.Repository{
				URL:  repoArg,
				Auth: repository.Auth{},
			}
		} else {
			repo, err = repoManager.GetRepository(repoArg)
			if err != nil {
				return fmt.Errorf("repository not found: %w", err)
			}
		}

		err = repoManager.Clone(repo, task.WorkDir)
		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		repoPath = task.WorkDir
		logger("Repository cloned successfully to %s", repoPath)
	}

	logger("Initializing LLM analyser")
	llmAnalyser := analysis.NewLLMAnalyser()

	err = llmAnalyser.Initialize(repoPath, map[string]string{})
	if err != nil {
		return fmt.Errorf("failed to initialize LLM analyser: %w", err)
	}

	logger("Generating insights. This may take a while...")
	insights, err := llmAnalyser.AnalyseRepository(repoPath)
	if err != nil {
		return fmt.Errorf("failed to generate insights: %w", err)
	}

	insightsJSON, err := json.MarshalIndent(insights, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal insights: %w", err)
	}

	if err := os.WriteFile(insightsPath, insightsJSON, PublicReadFilePerm); err != nil {
		return fmt.Errorf("failed to save insights: %w", err)
	}

	logger("Insights generated and saved to %s", insightsPath)

	return outputInsights(insightsPath, outputFormat, outputFile)
}

// outputInsights outputs insights in the specified format
func outputInsights(insightsPath, format, outputFile string) error {
	data, err := os.ReadFile(insightsPath)
	if err != nil {
		return fmt.Errorf("failed to read insights file: %w", err)
	}

	var insights analysis.RepoInsights
	if err := json.Unmarshal(data, &insights); err != nil {
		return fmt.Errorf("failed to parse insights: %w", err)
	}

	var output string
	switch format {
	case "json":
		output = string(data)
	case "markdown":
		output = formatMarkdown(&insights)
	case "text", "table":
		output = formatText(&insights)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Insights written to %s\n", outputFile)
	} else {
		fmt.Println(output)
	}

	return nil
}

// formatMarkdown formats insights as Markdown
func formatMarkdown(insights *analysis.RepoInsights) string {
	var sb strings.Builder

	sb.WriteString("# Repository Insights\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString(insights.Summary)
	sb.WriteString("\n\n")

	if len(insights.KeyFindings) > 0 {
		sb.WriteString("## Key Findings\n\n")
		for _, finding := range insights.KeyFindings {
			sb.WriteString(fmt.Sprintf("- %s\n", finding))
		}
		sb.WriteString("\n")
	}

	if len(insights.Recommendations) > 0 {
		sb.WriteString("## Recommendations\n\n")
		for _, rec := range insights.Recommendations {
			sb.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("## Architecture Score: %d/100\n\n", insights.ArchitectureScore))

	if len(insights.Languages) > 0 {
		sb.WriteString("## Languages\n\n")
		for lang, pct := range insights.Languages {
			sb.WriteString(fmt.Sprintf("- %s: %.1f%%\n", lang, pct))
		}
		sb.WriteString("\n")
	}

	if len(insights.Frameworks) > 0 {
		sb.WriteString("## Frameworks\n\n")
		for _, framework := range insights.Frameworks {
			sb.WriteString(fmt.Sprintf("- %s\n", framework))
		}
		sb.WriteString("\n")
	}

	if len(insights.SecurityConcerns) > 0 {
		sb.WriteString("## Security Concerns\n\n")
		for _, concern := range insights.SecurityConcerns {
			sb.WriteString(fmt.Sprintf("- %s\n", concern))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("*Generated on %s*\n", insights.GeneratedTimestamp.Format(time.RFC1123)))

	return sb.String()
}

// formatText formats insights as plain text
func formatText(insights *analysis.RepoInsights) string {
	var sb strings.Builder

	sb.WriteString("REPOSITORY INSIGHTS\n\n")
	sb.WriteString("Summary:\n")
	sb.WriteString(insights.Summary)
	sb.WriteString("\n\n")

	if len(insights.KeyFindings) > 0 {
		sb.WriteString("Key Findings:\n")
		for i, finding := range insights.KeyFindings {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, finding))
		}
		sb.WriteString("\n")
	}

	if len(insights.Recommendations) > 0 {
		sb.WriteString("Recommendations:\n")
		for i, rec := range insights.Recommendations {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Architecture Score: %d/100\n\n", insights.ArchitectureScore))

	if len(insights.Languages) > 0 {
		sb.WriteString("Languages:\n")
		for lang, pct := range insights.Languages {
			sb.WriteString(fmt.Sprintf("- %s: %.1f%%\n", lang, pct))
		}
		sb.WriteString("\n")
	}

	if len(insights.Frameworks) > 0 {
		sb.WriteString("Frameworks:\n")
		for _, framework := range insights.Frameworks {
			sb.WriteString(fmt.Sprintf("- %s\n", framework))
		}
		sb.WriteString("\n")
	}

	if len(insights.SecurityConcerns) > 0 {
		sb.WriteString("Security Concerns:\n")
		for _, concern := range insights.SecurityConcerns {
			sb.WriteString(fmt.Sprintf("- %s\n", concern))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Generated on %s\n", insights.GeneratedTimestamp.Format(time.RFC1123)))

	return sb.String()
}
