# Inspectre

**Inspectre** is a modular and extensible repository analysis tool built in Go. It orchestrates the process of connecting to Git repositories, performing deep analysis, and generating comprehensive reports.

## Features

- Clone and analyse repositories from GitHub, GitLab, and Bitbucket
- Support for direct Git URLs
- Built-in analysers for file statistics and Git metadata
- Extensible plugin system supporting CLI tools, Python scripts, and Go plugins
- Task management with detailed logging
- Storage and query capabilities for analysis results

## Installation

```bash
# Clone the repository
git clone https://github.com/thushan/inspectre.git
cd inspectre

# Build the executable
go build -o inspectre main.go

# Make plugins executable
chmod +x plugins/*.sh
chmod +x plugins/*.py
```

## Configuration

Inspectre uses configuration files stored in the `configs` directory:

- `repositories.json` - Repository configuration
- `extensions.json` - Plugin configuration

### Repository Configuration

```json
{
  "version": 1,
  "repositories": [
    {
      "name": "repo1",
      "url": "https://github.com/user/repo1.git",
      "type": "github",
      "auth": {
        "username": "your-github-username",
        "token": "${GITHUB_TOKEN}"
      }
    }
  ]
}
```

Environment variables in the form `${VAR_NAME}` will be automatically replaced.

## Usage

### Analyzing a Repository

```bash
# Analyse a configured repository
./inspectre run repo1

# Analyse any Git repository by URL
./inspectre run https://github.com/username/repo.git

# Specify output format
./inspectre run repo1 --output json
```

### Managing Tasks

```bash
# List running tasks
./inspectre ps

# Show all tasks (including completed)
./inspectre ps --all

# Show only failed tasks
./inspectre ps --failed

# View task logs
./inspectre logs <task-id>

# Follow task logs (similar to tail -f)
./inspectre logs <task-id> --follow
```

### Managing Repositories

```bash
# List configured repositories
./inspectre repos

# Show repositories in JSON format
./inspectre repos --json
```

### Managing Plugins

```bash
# List all plugins
./inspectre plugin list

# Filter plugins by type
./inspectre plugin list --type cli
./inspectre plugin list --type python
./inspectre plugin list --type golang

# Enable a plugin
./inspectre plugin enable <plugin-name>

# Disable a plugin
./inspectre plugin disable <plugin-name>

# View plugin details
./inspectre plugin info <plugin-name>
```

### Querying Results

```bash
# Run a query on analysis results
./inspectre query --sql "SELECT * FROM metrics LIMIT 10"

# Output query results as JSON
./inspectre query --sql "SELECT * FROM metrics WHERE name = 'commit_count'" --output json
```

## Extension System

Inspectre supports three types of extensions:

1. **CLI Tools** - Shell scripts or executables that analyse repositories
2. **Python Scripts** - Python scripts for more complex analysis
3. **Go Plugins** - Native Go extensions compiled as plugins

### Extension Configuration

Extensions are configured in `configs/extensions.json`:

```json
{
  "version": 1,
  "extensions": [
    {
      "name": "file_counter",
      "type": "cli",
      "path": "plugins/file_counter.sh",
      "description": "Counts files by type",
      "version": "1.0.0",
      "author": "Inspectre Team",
      "config": {
        "exclude_dirs": ".git,node_modules,vendor"
      },
      "enabled": true
    }
  ]
}
```

### Creating Extensions

Extensions receive the repository path as the first argument and can access configuration via environment variables prefixed with `INSPECTRE_CONFIG_`.

Extensions should output metrics in JSON format:

```json
[
  {
    "name": "metric_name",
    "value": 42,
    "labels": {
      "category": "example"
    },
    "timestamp": "2023-04-16T12:34:56Z"
  }
]
```

#### CLI Extensions

CLI extensions are simple executables or scripts that can be written in any language. They should:

1. Accept a repository path as the first command-line argument
2. Read configuration from environment variables prefixed with `INSPECTRE_CONFIG_`
3. Output metrics in JSON format to stdout

Example shell script:

```bash
#!/bin/bash
# Simple file counter
REPO_PATH="$1"
echo "["
echo "  {"
echo "    \"name\": \"file_count\","
echo "    \"value\": $(find $REPO_PATH -type f | wc -l),"
echo "    \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\""
echo "  }"
echo "]"
```

#### Python Extensions

Python extensions offer more flexibility and can use Python libraries:

```python
#!/usr/bin/env python3
import os
import sys
import json
from datetime import datetime

repo_path = sys.argv[1]
file_count = sum(1 for _ in os.popen(f"find {repo_path} -type f"))

metrics = [{
    "name": "file_count",
    "value": file_count,
    "timestamp": datetime.utcnow().isoformat() + "Z"
}]

print(json.dumps(metrics, indent=2))
```

#### Go Plugin Extensions

Go plugins require building a shared object file:

```go
package main

import (
	"time"
	"github.com/thushan/inspectre/internal/core/analysis"
)

// Analyser is exported for the plugin system
var Analyser = &ExampleAnalyser{}

type ExampleAnalyser struct{}

func (a *ExampleAnalyser) Name() string {
	return "example_analyser"
}

func (a *ExampleAnalyser) Initialize(repoPath string, env map[string]string) error {
	return nil
}

func (a *ExampleAnalyser) Run() ([]analysis.Metric, error) {
	return []analysis.Metric{
		{
			Name:      "example_metric",
			Value:     42,
			Timestamp: time.Now(),
		},
	}, nil
}

func (a *ExampleAnalyser) Cleanup() error {
	return nil
}
```

Build with:
```bash
go build -buildmode=plugin -o plugins/example.so example_plugin.go
```

## Architecture

Inspectre is built with a modular architecture:

- **Core Orchestration Layer** - Handles repository connections, Git operations, and task management
- **Extension System** - Supports various extension types for analysis
- **Storage Layer** - Stores and retrieves analysis results

### Directory Structure

```
.
├── cmd
│   └── inspectre         # CLI commands
│       ├── core.go       # Core commands
│       ├── extension.go  # Extension commands
│       └── root.go       # Command registration
├── configs               # Configuration files
│   ├── extensions.json   # Extension configuration
│   └── repositories.json # Repository configuration  
├── internal              # Internal packages
│   ├── core              # Core functionality
│   │   ├── analysis      # Analysis framework
│   │   ├── config        # Configuration management
│   │   ├── repository    # Repository management
│   │   └── task          # Task management
│   ├── extensions        # Extension system
│   │   ├── cli           # CLI extension support
│   │   ├── golang        # Go plugin support
│   │   └── python        # Python extension support
│   └── storage           # Storage system
├── plugins               # Extensions directory
│   ├── file_counter.sh   # Example CLI extension
│   └── complexity.py     # Example Python extension
├── main.go               # Application entry point
└── readme.md             # Documentation
```

## Development

### Adding New Analysers

To add a built-in analyser:

1. Implement the `analysis.Analyser` interface
2. Register it in the task manager

### Adding New Extension Types

To add a new extension type:

1. Create a new package in `internal/extensions/`
2. Implement the extension loading and execution logic
3. Register the extension type in the extension manager

## Future Roadmap

- Implement DuckDB integration for better metrics storage and querying
- Add support for remote extension repositories
- Create a web UI for visualization and interaction
- Add performance analysis capabilities
- Implement security scanning extensions
- Support for analyzing multiple repositories with comparison capabilities

## Contributing

Contributions are welcome! Please feel free to submit pull requests.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License