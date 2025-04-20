package inspectre

import (
	"github.com/spf13/cobra"
	"github.com/thushan/inspectre/internal/core/task"
)

var runCmd = &cobra.Command{
	Use:   "run [repo]",
	Short: "Run analysis on a repository",
	Long: `The run command clones the repository and executes all 
enabled analysis plugins against it.

You can specify a repository by name (if configured) or by URL:

  inspectre run repo1
  inspectre run https://github.com/username/repo.git`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoTarget := args[0]
		outputFormat, _ := cmd.Flags().GetString("output")
		taskManager := task.NewManager()
		taskManager.Run(repoTarget, outputFormat)
	},
}

var insightsCmd = &cobra.Command{
	Use:   "insights [repo]",
	Short: "Generate insights for a repository",
	Long: `The insights command performs deep analysis on 
a repository to extract patterns, architecture, and insights.

You can specify a repository by name (if configured) or by URL:

  inspectre insights repo1
  inspectre insights https://github.com/username/repo.git`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoTarget := args[0]
		format, _ := cmd.Flags().GetString("format")
		outputFile, _ := cmd.Flags().GetString("output")
		taskManager := task.NewManager()
		taskManager.GenerateInsights(repoTarget, format, outputFile)
	},
}

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running tasks",
	Long: `The ps command shows all active and recently completed tasks.
It provides information about task status, duration, and results.`,
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		failed, _ := cmd.Flags().GetBool("failed")
		format, _ := cmd.Flags().GetString("format")
		taskManager := task.NewManager()
		taskManager.ListTasks(all, failed, format)
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [task-id]",
	Short: "Show logs for a specific task",
	Long: `The logs command retrieves and displays the logs for a specific task.
You can specify the task ID to view its logs.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskID := args[0]
		follow, _ := cmd.Flags().GetBool("follow")
		taskManager := task.NewManager()
		taskManager.ShowLogs(taskID, follow)
	},
}

func init() {
	// Run command
	runCmd.Flags().StringP("output", "o", "text", "Output format (text, json, table)")
	rootCmd.AddCommand(runCmd)

	// Insights command
	insightsCmd.Flags().StringP("format", "f", "markdown", "Format for insights (markdown, html, json)")
	insightsCmd.Flags().StringP("output", "o", "", "Output file for the insights")
	rootCmd.AddCommand(insightsCmd)

	// PS command
	psCmd.Flags().BoolP("all", "a", false, "Show all tasks, including completed ones")
	psCmd.Flags().BoolP("failed", "f", false, "Show only failed tasks")
	psCmd.Flags().StringP("format", "o", "table", "Output format (table, json)")
	rootCmd.AddCommand(psCmd)

	// Logs command
	logsCmd.Flags().BoolP("follow", "f", false, "Follow task logs in real-time")
	rootCmd.AddCommand(logsCmd)
}
