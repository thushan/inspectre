package inspectre

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thushan/inspectre/internal/version"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   version.Name,
	Short: version.Description,
	Long:  `Inspectre is a modular and extensible code analysis tool that serves as an orchestrator for repository analysis.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./configs/inspectre.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		workDir, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		configDir := filepath.Join(workDir, "configs")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("inspectre")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("inspectre")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		extended, _ := cmd.Flags().GetBool("extended")
		version.PrintVersionInfo(extended, nil)
	},
}

func init() {
	versionCmd.Flags().BoolP("extended", "e", false, "Display extended version information")
}
