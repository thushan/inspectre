# Inspectre

**Inspectre** is a modular and extensible repository analysis tool built in Go. It orchestrates the process of connecting to Git repositories, performing deep analysis, and generating comprehensive reports.

## Key Features

- **Repository Integration**: Connect to GitHub, GitLab, and Bitbucket repositories
- **Extensible Architecture**: Support for CLI tools, Python scripts, and Go plugins
- **Intelligent Analysis**: File tree analysis with LLM-powered insights
- **Git Metadata Analysis**: Track commit history, branch health, and contributor patterns
- **Metrics Storage**: DuckDB integration for powerful querying of analysis results
- **Task Management**: Monitor and control ongoing analysis operations

## Architecture

Inspectre is designed as a modular system with a core orchestration layer and pluggable components:

- **Core Engine**: Repository management, environment setup, and artifact coordination
- **Extension System**: Interfaces for CLI, Python, and Go-based analysis tools
- **Artifact Store**: Shared data exchange between analysis components
- **Task Manager**: Control and monitor long-running processes
- **Storage Layer**: Persistent storage of metrics and analysis results

## Usage

```bash
# Run analysis on a repository
inspectre run https://github.com/username/repo

# Start daemon mode with API
inspectre daemon --port 8080

# List repositories
inspectre show repositories

# View running tasks
inspectre ps

# Get repository report
inspectre report myrepo --format=html
```

## Installation

```bash
# From source
git clone https://github.com/thushan/inspectre.git
cd inspectre
go build

# Via Go
go install github.com/thushan/inspectre@latest
```


## License

[MIT License](LICENSE)