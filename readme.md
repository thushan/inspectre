# Inspectre

**Inspectre** is a modular and extensible repository analysis tool built in Go. It provides comprehensive insights into software projects through intelligent, multi-dimensional code analysis.

## 🚀 Features

- 🔍 Multi-language repository analysis
- 🧩 Extensible plugin system
- 📊 Comprehensive metrics generation
- 🤖 AI-powered code insights
- 🚢 Supports GitHub, GitLab, and Bitbucket
- 💻 CLI and extension-based analysis

## Installation

```bash
# Clone the repository
git clone https://github.com/thushan/inspectre.git
cd inspectre

# Build the executable
go build -o inspectre main.go

# Make plugins executable
chmod +x plugins/*.sh plugins/*.py
```

## Quick Start

### Analyse a Repository

```bash
# Analyse a configured repository
./inspectre run repo1

# Analyse any Git repository by URL
./inspectre run https://github.com/username/repo.git

# Generate detailed insights
./inspectre insights repo1
```

## Configuration

Inspectre uses configuration files stored in the `configs` directory:

### Repository Configuration (`repositories.json`)

```json
{
  "repositories": [
    {
      "name": "my-project",
      "url": "https://github.com/user/repo1.git",
      "type": "github",
      "auth": {
        "token": "${GITHUB_TOKEN}"
      }
    }
  ]
}
```

### Extension Configuration (`extensions.json`)

```json
{
  "extensions": [
    {
      "name": "file_counter",
      "type": "cli",
      "path": "plugins/file-counter.sh",
      "description": "Counts files by type",
      "enabled": true,
      "config": {
        "exclude_dirs": ".git,node_modules,vendor"
      }
    }
  ]
}
```

## CLI Commands

Here are some basic commands and operations, for indepth documentation, see [usage](./docs/usage.md).

### Basic Analysis
```bash
# Run repository analyser
inspectre run <repo-url>

# Generate insights
inspectre insights <repo-url>

# List tasks
inspectre ps

# View task logs
inspectre logs <task-id>
```

### Plugin Management
```bash
# List plugins
inspectre plugin list

# Enable a plugin
inspectre plugin enable <plugin-name>

# Disable a plugin
inspectre plugin disable <plugin-name>
```

## Extension Types

Inspectre supports three extension types:

1. **CLI Tools**: Simple executable scripts
    - Lightweight analysis
    - Easy to create
    - Minimal dependencies

2. **Python Scripts**: Complex analysis with library support
    - Full Python ecosystem
    - Rich metric generation
    - Advanced processing capabilities

3. **Go Plugins**: Native performance plugins
    - Compile-time type safety
    - Direct Go interface
    - Zero-overhead abstractions

### Creating an Extension

Example Python extension:
```python
#!/usr/bin/env python3
import os
import sys
import json
from datetime import datetime

repo_path = sys.argv[1]
metrics = [{
    "name": "file_count",
    "value": len(os.listdir(repo_path)),
    "timestamp": datetime.utcnow().isoformat() + "Z"
}]

print(json.dumps(metrics))
```

## Architecture

Inspectre uses a sophisticated, event-driven architecture:

- Dynamic worker pool
- Concurrent analysis
- Extensible plugin framework
- Advanced error handling

For a more detailed overview, see [Technical Overview](docs/technical.md)

## Performance & Scalability

- Adaptive worker pool
- Concurrent analysis
- Intelligent resource management
- Configurable parallelism

## Roadmap

- Enhanced LLM integration
- More analysis techniques
- Advanced security scanning
- Distributed analysis support
- Machine learning insights

## Contributing

Contributions are welcome!

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

For issues and feature requests, please use GitHub Issues.

## License

MIT License

## Authors

Developed by Thushan Fernando and contributors.