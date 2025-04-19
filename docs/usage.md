# Inspectre Usage Guide

This guide is designed to help you understand how to use Inspectre.

## Getting Started

Inspectre is portable, there's no real installation required in the traditional sense.

### Installation

```bash
# Clone the repository
git clone https://github.com/thushan/inspectre.git
cd inspectre

# Build the executable
go build -o inspectre main.go

# Make plugins executable
chmod +x plugins/*.sh plugins/*.py
```

## Command Reference

### Global Options

```
inspectre [global options] command [command options] [arguments...]

Global Options:
  --no-colour, --nc         Disable coloured output
  --quiet, -q               Minimal output mode
  --verbose, -vv            Detailed output
  --config, -c FILE         Specify custom configuration file
  --output, -o FORMAT       Output format (table, json, text)
```

### Repository Analysis Commands

#### Run Analysis
```bash
# Analyse a configured repository
inspectre run <repository-name>

# Analyse a repository by URL
inspectre run https://github.com/thushan/smash.git

# Specify output format
inspectre run repo-name --output json
```

Options:
- `--no-wait, -n`: Run task in background
- `--output, -o`: Output format (table, json, text)

#### Generate Insights
```bash
# Generate detailed insights
inspectre insights <repository-name-or-url>

# Save insights to a file
inspectre insights repo-name --file insights.json

# Specify output format
inspectre insights repo-name --output markdown
```

Options:
- `--output, -o`: Output format (json, markdown, text)
- `--file, -f`: Output file path
- `--force`: Re-analyse even if insights exist

### Task Management

#### List Tasks
```bash
# Show running tasks
inspectre ps

# Show all tasks
inspectre ps --all

# Show only failed tasks
inspectre ps --failed
```

Options:
- `--all`: Include completed tasks
- `--failed`: Show only failed tasks
- `--output, -o`: Output format (table, json)

#### View Task Logs
```bash
# View logs for a task
inspectre logs <task-id>

# Follow logs in real-time
inspectre logs <task-id> --follow
```

Options:
- `--follow, -f`: Continuously stream logs

#### Cancel a Task
```bash
# Cancel a running task
inspectre cancel <task-id>

# Cancel without confirmation
inspectre cancel <task-id> --yes
```

### Repository Management

#### List Repositories
```bash
# Show configured repositories
inspectre repos

# Output as JSON
inspectre repos --output json
```

### Plugin Management

#### List Plugins
```bash
# Show all plugins
inspectre plugin list

# Filter by type
inspectre plugin list --type cli
inspectre plugin list --type python
inspectre plugin list --type golang
```

#### Manage Plugins
```bash
# Enable a plugin
inspectre plugin enable <plugin-name>

# Disable a plugin
inspectre plugin disable <plugin-name>

# View plugin details
inspectre plugin info <plugin-name>
```

### Querying Results

```bash
# Query analyser results
inspectre query --sql "SELECT * FROM metrics LIMIT 10"

# Output as JSON
inspectre query --sql "SELECT * FROM metrics" --output json
```

## Configuration

### Repository Configuration
Location: `configs/repositories.json`

```json
{
  "repositories": [
    {
      "name": "my-project",
      "url": "https://github.com/username/project.git",
      "type": "github",
      "auth": {
        "token": "${GITHUB_TOKEN}"
      }
    }
  ]
}
```

### Extension Configuration
Location: `configs/extensions.json`

```json
{
  "extensions": [
    {
      "name": "file_counter",
      "type": "cli",
      "path": "plugins/file-counter.sh",
      "enabled": true,
      "config": {
        "exclude_dirs": ".git,node_modules,vendor"
      }
    }
  ]
}
```

## Environment Variables

- `GITHUB_TOKEN`: Authentication for GitHub repositories
- `INSPECTRE_CONFIG_*`: Plugin-specific configurations
- `NO_COLOUR`: Disable coloured output
- `VERBOSE`: Enable verbose logging

## Troubleshooting

- Check logs in task directory
- Verify repository and plugin configurations
- Ensure all dependencies are installed
- Use `--verbose` flag for detailed diagnostics

## Examples

### Analyse Multiple Repositories
```bash
# Analyse multiple repositories in sequence
inspectre run repo1 repo2 repo3
```

### Advanced Querying
```bash
# Complex metric analyser
inspectre query --sql "
  SELECT language, AVG(complexity) as avg_complexity 
  FROM metrics 
  WHERE name LIKE '%complexity%' 
  GROUP BY language
"
```

## Best Practices

- Keep plugins updated
- Use environment variables for sensitive data
- Regularly review and clean up task history
- Monitor system resources during large analyses