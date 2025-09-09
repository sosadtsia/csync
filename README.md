# csync

A cloud drive synchronization tool written in that syncs local folders to Google Drive and pCloud.

## Features

- **Multi-provider support**: Sync to Google Drive and pCloud simultaneously
- **Concurrent uploads**: Configurable concurrent file transfers for optimal performance
- **Smart sync**: MD5-based file comparison to avoid unnecessary uploads
- **Pattern filtering**: Include/exclude files using glob patterns
- **Dry run mode**: Preview changes before execution
- **Retry logic**: Automatic retry with exponential backoff
- **Progress tracking**: Real-time sync progress reporting
- **Daemon mode**: Run as background service with configurable sync intervals
- **File watching**: Real-time sync when files change (polling-based)
- **Standard library focused**: Minimal external dependencies, leveraging Go's powerful standard library

## Installation

### Prerequisites

- Go 1.25 or later
- Google Drive API credentials (for Google Drive sync)
- pCloud account (for pCloud sync)

### Build from source

```bash
git clone https://github.com/svosadtsia/csync.git
cd csync
go build -o csync ./cmd/csync
```

### Install via go install

```bash
go install github.com/svosadtsia/csync/cmd/csync@latest
```

### Google Drive Setup

1. Visit the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Drive API
4. Create credentials (OAuth 2.0 Client ID) for a desktop application
5. Download the credentials file and save it as `credentials.json`
6. Update the configuration file with the correct path

### pCloud Setup

1. Sign up for a [pCloud account](https://pcloud.com/)
2. Update the configuration file with your username and password
3. Optionally specify a folder ID to sync to a specific folder

## Usage

### Basic Usage

```bash
# One-time sync
csync -source /path/to/folder -provider gdrive

# Sync to specific provider only
csync -source ./documents -provider pcloud

# Sync to both providers
csync -source ./photos -provider all

# Dry run to preview changes
csync -source ./test -provider gdrive -dry-run

# Verbose logging
csync -source ./logs -provider all -verbose
```

### Daemon Mode

```bash
# Start daemon for continuous sync
csync -source ./documents -provider gdrive -daemon

# Start daemon with custom interval
csync -source ./photos -provider all -daemon -interval 10m

# Start daemon with file watching (real-time sync)
csync -source ./workspace -provider gdrive -daemon -watch

# Daemon control commands
csync -start -source ./docs -provider gdrive    # Start daemon
csync -stop                                     # Stop daemon
csync -status                                   # Show daemon status
csync -reload                                   # Reload configuration

# Custom daemon settings
csync -daemon -source ./data -provider all \
      -interval 1h -watch \
      -pid-file /var/run/csync.pid \
      -log-file /var/log/csync.log
```

### Advanced Usage

```bash
# Use custom configuration file
csync -config /path/to/custom-config.json

# Sync to multiple providers
csync -providers gdrive,pcloud -source ./documents

# Combine options
csync -source ./photos -providers gdrive -config ./photo-sync.json -verbose
```

### Command Line Options

| Option | Default | Description |
|--------|---------|-------------|
| `-config` | `csync.json` | Path to configuration file |
| `-source` | *required* | Local directory to sync |
| `-provider` | *required* | Cloud provider: `gdrive`, `pcloud`, or `all` |
| `-dry-run` | `false` | Show what would be synced without making changes |
| `-verbose` | `false` | Enable verbose logging |
| `-workers` | `0` | Max concurrent workers (0 = use config) |
| `-init` | `false` | Initialize configuration file with defaults |

### Daemon Mode Options

| Option | Default | Description |
|--------|---------|-------------|
| `-daemon` | `false` | Run as daemon (background service) |
| `-start` | `false` | Start daemon |
| `-stop` | `false` | Stop daemon |
| `-status` | `false` | Show daemon status |
| `-reload` | `false` | Reload daemon configuration |
| `-interval` | `5m` | Sync interval for daemon mode |
| `-watch` | `false` | Enable file watching for real-time sync |
| `-pid-file` | `csync.pid` | PID file location |
| `-log-file` | `csync.log` | Log file location |

## Pattern Filtering

### Ignore Patterns

Use glob patterns to exclude files and directories:

```json
{
  "general": {
    "ignore_patterns": [
      ".git/",           // Ignore .git directory
      "*.tmp",           // Ignore temporary files
      "node_modules/",   // Ignore node_modules
      "**/.DS_Store",    // Ignore .DS_Store files anywhere
      "logs/*.log"       // Ignore log files in logs directory
    ]
  }
}
```

### Include Patterns

When specified, only files matching these patterns are synced:

```json
{
  "general": {
    "include_patterns": [
      "*.jpg",           // Only JPG files
      "*.png",           // Only PNG files
      "documents/*.pdf"  // Only PDFs in documents folder
    ]
  }
}
```

## Performance Tuning

### Concurrency

Adjust `max_concurrency` based on your network and system capabilities:

```json
{
  "general": {
    "max_concurrency": 10,  // Higher for better networks
    "retry_attempts": 5,    // More retries for unreliable connections
    "chunk_size_bytes": 16777216  // 16MB chunks for large files
  }
}
```

### Recommendations

- **Local network**: `max_concurrency: 10-20`
- **Home broadband**: `max_concurrency: 5-10`
- **Mobile/limited**: `max_concurrency: 2-5`

## Error Handling

csync implements comprehensive error handling:

- **Automatic retry**: Failed uploads are retried with exponential backoff
- **Non-retryable errors**: Authentication and quota errors are not retried
- **Graceful degradation**: Continues processing other files even if some fail
- **Detailed logging**: Verbose mode provides detailed error information

## Security

- **OAuth 2.0**: Secure authentication with Google Drive
- **Token storage**: Access tokens stored securely with restricted permissions
- **Credential files**: Configuration files use restrictive file permissions (0600)
- **No plaintext storage**: Sensitive data encrypted at rest where possible

## Development

### Project Structure

```
csync/
├── cmd/csync/           # CLI application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── scanner/         # Directory scanning and filtering
│   └── sync/            # Cloud provider implementations
├── go.mod               # Go module definition
└── README.md            # This file
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/scanner

# Verbose test output
go test -verbose ./...
```

### Building

```bash
# Build for current platform
go build -o csync ./cmd/csync

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o csync-linux-amd64 ./cmd/csync
GOOS=windows GOARCH=amd64 go build -o csync-windows-amd64.exe ./cmd/csync
GOOS=darwin GOARCH=amd64 go build -o csync-darwin-amd64 ./cmd/csync
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/new-feature`
3. Make your changes and add tests
4. Run tests: `go test ./...`
5. Run linter: `golangci-lint run`
6. Commit your changes: `git commit -am 'Add new feature'`
7. Push to the branch: `git push origin feature/new-feature`
8. Create a Pull Request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- Create an issue for bug reports or feature requests
- Check existing issues before creating new ones
- Provide detailed information including:
  - Operating system
  - Go version
  - Configuration (remove sensitive data)
  - Error messages
  - Steps to reproduce

## Vision & Roadmap

For detailed project vision, goals, and roadmap, see [VISION.md](VISION.md).
