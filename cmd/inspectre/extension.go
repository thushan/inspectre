package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/thushan/inspectre/internal/core/config"
	"github.com/thushan/inspectre/internal/extensions"
	"github.com/urfave/cli/v2"
)

// initExtensionManager initializes the extension manager
func initExtensionManager(c *cli.Context) error {
	if extensionManager != nil {
		return nil
	}

	cfg, err := config.LoadConfig(c.String("config"))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := os.MkdirAll(cfg.PluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create extensions directory: %w", err)
	}

	extensionManager = extensions.NewManager(cfg.PluginsDir)

	if err := extensionManager.LoadExtensionsFromConfig(""); err != nil {
		return fmt.Errorf("failed to load extensions: %w", err)
	}

	return nil
}

// ExtensionCommands returns the CLI commands for extension management
func ExtensionCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "plugin",
			Usage: "Manage analysis plugins",
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "List available plugins",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "type",
							Usage: "Filter by plugin type (cli, python, golang)",
						},
						&cli.BoolFlag{
							Name:  "json",
							Usage: "Output in JSON format",
						},
					},
					Action: listPluginsAction,
				},
				{
					Name:      "enable",
					Usage:     "Enable a plugin",
					ArgsUsage: "<plugin-name>",
					Action:    enablePluginAction,
				},
				{
					Name:      "disable",
					Usage:     "Disable a plugin",
					ArgsUsage: "<plugin-name>",
					Action:    disablePluginAction,
				},
				{
					Name:      "info",
					Usage:     "Show plugin details",
					ArgsUsage: "<plugin-name>",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "json",
							Usage: "Output in JSON format",
						},
					},
					Action: pluginInfoAction,
				},
			},
		},
	}
}

// listPluginsAction lists available plugins
func listPluginsAction(c *cli.Context) error {
	if err := initExtensionManager(c); err != nil {
		return err
	}

	extensionList := extensionManager.ListExtensions()

	// Filter by type if requested
	if c.String("type") != "" {
		var filtered []*extensions.Extension
		for _, ext := range extensionList {
			if string(ext.Type) == c.String("type") {
				filtered = append(filtered, ext)
			}
		}
		extensionList = filtered
	}

	// JSON output
	if c.Bool("json") {
		output, err := json.MarshalIndent(extensionList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal extensions: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Table output
	if len(extensionList) == 0 {
		fmt.Println("No plugins found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tENABLED\tVERSION\tDESCRIPTION")

	for _, ext := range extensionList {
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n",
			ext.Name,
			ext.Type,
			ext.Enabled,
			ext.Version,
			ext.Description,
		)
	}

	w.Flush()
	return nil
}

// enablePluginAction enables a plugin
func enablePluginAction(c *cli.Context) error {
	if err := initExtensionManager(c); err != nil {
		return err
	}

	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}

	ext, err := extensionManager.GetExtension(name)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Enable the plugin
	ext.Enabled = true

	// Re-register the extension
	extensionManager.RegisterExtension(ext)

	// Save the configuration
	exts := extensionManager.ListExtensions()
	config := &extensions.Config{
		Version:    1,
		Extensions: make([]extensions.Extension, len(exts)),
	}

	for i, e := range exts {
		config.Extensions[i] = *e
	}

	if err := extensions.SaveConfig(config, ""); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("Plugin %s enabled\n", name)
	return nil
}

// disablePluginAction disables a plugin
func disablePluginAction(c *cli.Context) error {
	if err := initExtensionManager(c); err != nil {
		return err
	}

	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}

	ext, err := extensionManager.GetExtension(name)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Disable the plugin
	ext.Enabled = false

	// Re-register the extension
	extensionManager.RegisterExtension(ext)

	// Save the configuration
	exts := extensionManager.ListExtensions()
	config := &extensions.Config{
		Version:    1,
		Extensions: make([]extensions.Extension, len(exts)),
	}

	for i, e := range exts {
		config.Extensions[i] = *e
	}

	if err := extensions.SaveConfig(config, ""); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("Plugin %s disabled\n", name)
	return nil
}

// pluginInfoAction shows details for a plugin
func pluginInfoAction(c *cli.Context) error {
	if err := initExtensionManager(c); err != nil {
		return err
	}

	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}

	ext, err := extensionManager.GetExtension(name)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// JSON output
	if c.Bool("json") {
		output, err := json.MarshalIndent(ext, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal extension: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Detailed output
	fmt.Printf("Name:        %s\n", ext.Name)
	fmt.Printf("Type:        %s\n", ext.Type)
	fmt.Printf("Path:        %s\n", ext.Path)
	fmt.Printf("Description: %s\n", ext.Description)
	fmt.Printf("Version:     %s\n", ext.Version)
	fmt.Printf("Author:      %s\n", ext.Author)
	fmt.Printf("Enabled:     %v\n", ext.Enabled)

	if len(ext.Config) > 0 {
		fmt.Println("Configuration:")
		for k, v := range ext.Config {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}
