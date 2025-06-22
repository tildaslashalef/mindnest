# Mindnest

> [!WARNING]
> **⚠️ EXPERIMENTAL PROJECT - NOT ACTIVELY MAINTAINED ⚠️**
> 
> This project was an experimental side project and is **no longer actively maintained**. It may contain bugs, security vulnerabilities, or incomplete features. Use at your own risk and do not rely on it for production environments or critical workflows.


Mindnest is an LLM-powered code review tool designed to help developers improve code quality and maintainability. It's a self-hosted binary that runs locally on your machine, ensuring your code never leaves your environment.

## Features

- Self-hosted binary with no cloud dependencies required
- AI-powered code reviews using local LLMs (Ollama, Claude, Gemini)
- Context-aware code analysis with vector embeddings
- Git integration for reviewing staged changes, commits, or branch differences
- GitHub integration for submitting PR comments
- Intelligent code chunking and similarity search
- Privacy-first: all processing happens locally
- Zero-configuration: embedded migrations and automatic setup

## Installation

### Prerequisites

- Go 1.24 or later
- SQLite and Git
- [Ollama](https://github.com/ollama/ollama) (default LLM provider)
- Optional: [Claude](https://console.anthropic.com/), [Gemini](https://aistudio.google.com/apikey), or [GitHub](https://github.com) API keys

### Download Pre-built Binaries

Download from the [GitHub Releases](https://github.com/tildaslashalef/mindnest/releases) page:

```bash
# Download and install (Linux)
wget https://github.com/tildaslashalef/mindnest/releases/latest/download/mindnest-linux-amd64
chmod +x mindnest-linux-amd64
sudo mv mindnest-linux-amd64 /usr/local/bin/mindnest
```

> **Note:** Only Linux binaries include full [sqlite-vec](https://github.com/asg017/sqlite-vec) support. For macOS, build from source.

### Build from Source

```bash
git clone https://github.com/tildaslashalef/mindnest.git
cd mindnest
make build && make install
```

## Configuration

Mindnest features zero-configuration setup. On first run, it creates a `.mindnest` directory in your home folder with the database and sample configuration.

Customize your setup by editing `~/.mindnest/.env`:

```bash
# View sample configuration
cat ~/.mindnest/.env

# Edit configuration
vim ~/.mindnest/.env
```

### Ollama Setup

Install and configure Ollama for local LLM capabilities:

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Start Ollama
ollama serve

# Pull required models
ollama pull nomic-embed-text  # For embeddings
ollama pull gemma2           # For code reviews
```

## Usage

```bash
# Initialize Mindnest (first-time setup)
mindnest init

# Review staged changes (default)
cd /path/to/project && mindnest

# Review specific commit
mindnest --commit <commit-hash>

# Review branch differences
mindnest --branch <branch-name>

# Configure GitHub repository
mindnest ws -g <github-repo-url>

# List workspace issues
mindnest ws
```

## Development

The project is structured with clear separation between CLI commands, services, repositories, and external integrations. Key components include:

- **CLI Layer**: Review, workspace, and sync commands
- **Services**: Workspace, review, Git, LLM, RAG, and parser services
- **Data Layer**: SQLite with sqlite-vec for vector embeddings
- **External**: Ollama, Claude, Gemini, and GitHub integrations


# License

[MIT License](LICENSE)