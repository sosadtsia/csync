# csync

A cloud drive synchronization tool written in that syncs local folders to Google Drive and pCloud.

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

#### Detailed Google Drive Authorization Process

**Step 1: Create Google Cloud Project**
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Sign in with your Google account (same one used for Google Drive)
3. Click "Select a project" → "New Project"
4. Enter project name (e.g., "Personal Drive Sync")
5. Click "Create"

**Step 2: Enable Google Drive API**
1. In left sidebar: "APIs & Services" → "Library"
2. Search for "Google Drive API"
3. Click "Google Drive API" → Click "Enable"

**Step 3: Configure OAuth Consent Screen**
1. Go to "APIs & Services" → "OAuth consent screen"
2. Select "External" user type
3. Fill required fields:
   - App name: "csync" (or preferred name)
   - User support email: your email
   - Developer contact: your email
4. Click "Save and Continue" through all steps
5. Optional: Add yourself as a test user, or publish the app

**Step 4: Create OAuth Credentials**
1. Go to "APIs & Services" → "Credentials"
2. Click "+ Create Credentials" → "OAuth client ID"
3. Application type: "Desktop application"
4. Name: "csync-desktop" (or preferred name)
5. **Important**: Under "Authorized redirect URIs", add exactly:
   ```
   http://localhost
   ```
6. Click "Create"

**Step 5: Download Credentials**
1. Click "Download JSON" button
2. Save file as `credentials.json` in your csync project directory

**Step 6: First-Time Authorization**
When you run csync for the first time:

1. **Run csync**: `./csync -s ./test -p gdrive -d`
2. **Copy the authorization URL** that csync displays:
   ```
   Go to the following link in your browser then type the authorization code:
   https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=...
   ```
3. **Visit the URL in your browser**
4. **Sign in** with your Google account (same one for Google Drive access)
5. **Grant permissions**: Click "Allow" to let csync access your Google Drive
6. **Browser redirect**: You'll be redirected to `http://localhost` with an error page - **this is expected!**
7. **Extract authorization code**: Look at the URL in your browser's address bar:
   ```
   http://localhost/?state=state-token&code={AUTH_CODE}&scope=https://www.googleapis.com/auth/drive.file
   ```

   **The authorization code is the long string after `code=` and before `&scope`**:
   ```
   AUTH_CODE
   ```

8. **Copy only the code part** (without `code=` or anything after it)
9. **Return to terminal** where csync is waiting
10. **Paste the authorization code** and press **Enter**

**Example of the complete flow:**

```bash
$ ./csync -s ./documents -p gdrive
Go to the following link in your browser then type the authorization code:
https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=643805...

# After visiting URL, signing in, and getting redirected:
# Browser shows: http://localhost/?state=state-token&code=4/0AVMBsJj...&scope=...
# You copy AUTH_CODE
# This is the authorization code and it is the long string after `code=` and before `&scope`
# You paste it into the terminal and press Enter
Saving credential file to: token.json
→ README.md (1.2 KB)
→ document.txt (856 bytes)
Sync completed successfully!
```

csync will then:
- Exchange the authorization code for access and refresh tokens
- Save `token.json` for future use (no more browser interaction needed!)
- Complete the sync operation
- Upload files to your specified destination path (e.g., `/0-test/documents/`)

**Future Runs**: csync automatically uses the saved `token.json` - no browser interaction needed!

### pCloud Setup

1. Sign up for a [pCloud account](https://pcloud.com/)
2. Update the configuration file with your username and password
3. Optionally specify a folder ID to sync to a specific folder

## Usage

### Basic Usage

```bash
# One-time sync (using short flags)
csync -s /path/to/folder -p gdrive

# Sync to specific provider only
csync -s ./documents -p pcloud

# Sync to both providers
csync -s ./photos -p all

# Run regular sync in background (detached from terminal)
csync -s ./documents -p gdrive -b

# Background sync with logging
csync -s ./documents -p all -b -l ./sync.log

# Sync with logging to file
csync -s ./documents -p gdrive -l /var/log/csync.log

# Start background daemon for continuous sync
csync -s ./documents -p gdrive -daemon -b

# Background daemon with real-time sync and logging
csync -s ./workspace -p all -daemon -b -watch -interval 1h -l ./logs/csync.log

# Dry run to preview changes
csync -s ./test -p gdrive -d

# Verbose logging (shows detailed information)
csync -s ./logs -p all -v

# Debug logging (shows very detailed troubleshooting information)
csync -s ./logs -p all --debug

# Combine verbose and debug with log file
csync -s ./logs -p all -v --debug -l ./debug.log

# Long form flags still work
csync -source ./documents -provider gdrive -dry-run -verbose -log-file ./sync.log
```

### Daemon Mode

```bash
# Start daemon for continuous sync (using short flags)
csync -s ./documents -p gdrive -daemon

# Start daemon in background (detached from terminal)
csync -s ./documents -p gdrive -daemon -background
csync -s ./documents -p gdrive -daemon -b  # Short flag

# Start daemon with logging to file
csync -s ./documents -p gdrive -daemon -l /var/log/csync.log

# Start daemon with custom interval
csync -s ./photos -p all -daemon -interval 10m

# Start daemon with file watching (real-time sync)
csync -s ./workspace -p gdrive -daemon -watch

# Combined: background daemon with real-time sync and logging
csync -s ./workspace -p all -daemon -background -watch -interval 1h -l ./logs/csync.log

# Production setup with all features
csync -s ./production-data -p all -daemon -b -watch -interval 30m \
      -v -l /var/log/csync.log -pid-file /var/run/csync.pid

# Daemon control commands
csync -start -s ./docs -p gdrive    # Start daemon
csync -stop                         # Stop daemon
csync -status                       # Show daemon status
csync -reload                       # Reload configuration

# Custom daemon settings with verbose logging
csync -daemon -s ./data -p all -v \
      -interval 1h -watch -background \
      -pid-file /var/run/csync.pid \
      -log-file /var/log/csync.log
```

## Scheduling Daemon Mode

csync provides flexible scheduling options for continuous synchronization. Choose the approach that best fits your needs:

### 1. **Interval-Based Scheduling** (Time-based sync)

Sync at regular time intervals regardless of file changes:

```bash
# Every 5 minutes (default)
csync -s ./documents -p gdrive -daemon -b -l ./sync.log

# Every 10 minutes
csync -s ./documents -p all -daemon -b -interval 10m -l ./sync.log

# Every hour
csync -s ./backups -p gdrive -daemon -b -interval 1h -l ./hourly.log

# Every 6 hours
csync -s ./archives -p pcloud -daemon -b -interval 6h -l ./daily.log

# Daily sync (24 hours)
csync -s ./photos -p all -daemon -b -interval 24h -l ./daily-sync.log
```

**Interval Format Examples:**
- `30s` - Every 30 seconds
- `5m` - Every 5 minutes
- `1h` - Every hour
- `12h` - Every 12 hours
- `24h` - Daily

### 2. **Real-Time Scheduling** (File watching)

Sync immediately when files change:

```bash
# Instant sync on file changes
csync -s ./active-project -p gdrive -daemon -b -watch -l ./realtime.log

# Watch mode with both providers
csync -s ./workspace -p all -daemon -b -watch -l ./watch.log

# Real-time + periodic backup (recommended)
csync -s ./important-docs -p all -daemon -b -watch -interval 2h -l ./hybrid.log
```

### 3. **Hybrid Scheduling** (Best of both worlds)

Combine real-time sync with periodic backups:

```bash
# Real-time sync + hourly backup
csync -s ./workspace -p all -daemon -b \
      -watch -interval 1h \
      -l /var/log/csync-hybrid.log

# Real-time sync + daily full sync
csync -s ~/Documents -p all -daemon -b \
      -watch -interval 24h \
      -v -l /var/log/csync-daily.log
```

### 4. **Production Scheduling Setup**

Complete setup for production environments:

```bash
# Create directories
sudo mkdir -p /var/log/csync /var/run

# Start production daemon
csync -s ~/critical-data -p all -daemon -b \
      -watch -interval 30m \
      -v -l /var/log/csync/production.log \
      -pid-file /var/run/csync.pid

# Verify daemon is running
csync -status

# Monitor logs
tail -f /var/log/csync/production.log
```

### 5. **Multiple Scheduled Daemons**

Run different schedules for different directories:

```bash
# Fast sync for active work
csync -s ~/active-projects -p gdrive -daemon -b \
      -watch -interval 5m \
      -l /var/log/csync-work.log \
      -pid-file /var/run/csync-work.pid

# Daily backup for archives
csync -s ~/archives -p pcloud -daemon -b \
      -interval 24h \
      -l /var/log/csync-archive.log \
      -pid-file /var/run/csync-archive.pid
```

### 6. **Daemon Management Commands**

```bash
# Start specific daemon configuration
csync -start -s ./docs -p gdrive -interval 15m

# Check daemon status
csync -status

# Stop running daemon
csync -stop

# Reload configuration (applies new settings)
csync -reload

# View daemon logs
tail -f /var/log/csync.log

# Check daemon process
ps aux | grep csync
```

### 7. **Schedule Examples by Use Case**

| Use Case | Recommended Schedule | Command |
|----------|---------------------|---------|
| **Active Development** | Real-time + 1h backup | `csync -s ./code -p gdrive -daemon -b -watch -interval 1h -l ./dev.log` |
| **Document Backup** | Every 30 minutes | `csync -s ~/Documents -p all -daemon -b -interval 30m -l ./docs.log` |
| **Photo Backup** | Daily sync | `csync -s ~/Photos -p pcloud -daemon -b -interval 24h -l ./photos.log` |
| **Critical Data** | Real-time + 2h backup | `csync -s ~/critical -p all -daemon -b -watch -interval 2h -l ./critical.log` |
| **Archive Storage** | Weekly sync | `csync -s ~/archives -p pcloud -daemon -b -interval 168h -l ./archive.log` |

### 8. **Troubleshooting Scheduled Daemons**

```bash
# Check if daemon is running
csync -status
ps aux | grep csync

# Debug daemon issues
csync -s ./test -p gdrive -daemon -v --debug -l ./debug.log

# Test scheduling without daemon
csync -s ./test -p gdrive -interval 1m -v  # Runs once, shows what daemon would do

# Stop stuck daemon
csync -stop
# Or manually: kill $(cat /var/run/csync.pid)
```

### 9. **Systemd Integration** (Linux)

For system-level scheduling, create a systemd service:

```bash
# Create service file: /etc/systemd/system/csync.service
[Unit]
Description=csync daemon
After=network.target

[Service]
Type=simple
User=your-username
ExecStart=/usr/local/bin/csync -s /home/user/data -p all -daemon -b -watch -interval 1h -l /var/log/csync.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target

# Enable and start service
sudo systemctl enable csync
sudo systemctl start csync
sudo systemctl status csync
```

**Key Benefits of Daemon Scheduling:**
- ✅ **Automatic sync** - No manual intervention needed
- ✅ **Persistent** - Continues after reboot (with systemd)
- ✅ **Flexible timing** - From seconds to days
- ✅ **Real-time option** - Instant sync on file changes
- ✅ **Multiple targets** - Different schedules for different folders
- ✅ **Reliable** - Automatic retry and error handling

### Advanced Usage

```bash
# Use custom configuration file (short flag)
csync -c /path/to/custom-config.json -s ./docs -p gdrive

# Combine short and long flags
csync -s ./photos -p gdrive -config ./photo-sync.json -verbose

# Initialize config and then use it
csync -i                           # Create default csync.json
csync -p gdrive                    # Use config file (no -s needed if set in config)

# Multiple workers for faster sync
csync -s ./large-folder -p all -w 10 -v
```

### Command Line Options

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `-config` | `-c` | `csync.json` | Path to configuration file |
| `-source` | `-s` | *required* | Local directory to sync |
| `-provider` | `-p` | *required* | Cloud provider: `gdrive`, `pcloud`, or `all` |
| `-dry-run` | `-d` | `false` | Show what would be synced without making changes |
| `-verbose` | `-v` | `false` | Enable verbose logging with detailed output |
| `-debug` | | `false` | Enable detailed debug logging for troubleshooting |
| `-background` | `-b` | `false` | Run in background (works for both regular sync and daemon mode) |
| `-workers` | `-w` | `0` | Max concurrent workers (0 = use config) |
| `-init` | `-i` | `false` | Initialize configuration file with defaults |
| `-log-file` | `-l` | | Write logs to specified file (works for both regular and daemon mode) |

### Daemon Mode Options

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `-daemon` | | `false` | Run as daemon (background service) |
| `-start` | | `false` | Start daemon |
| `-stop` | | `false` | Stop daemon |
| `-status` | | `false` | Show daemon status |
| `-reload` | | `false` | Reload daemon configuration |
| `-background` | `-b` | `false` | Run daemon detached from terminal |
| `-interval` | | `5m` | Sync interval for daemon mode |
| `-watch` | | `false` | Enable file watching for real-time sync |
| `-pid-file` | | `csync.pid` | PID file location |
| `-log-file` | | `csync.log` | Log file location |

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


### Quick Start

```bash
# 1. Initialize minimal config
csync -i

# 2. Edit csync.json - add your paths

# 3. Setup Google Drive credentials (one time)
# Follow detailed guide in "Google Drive Setup" section above

# 4. Start syncing
csync -p gdrive              # Clean output
csync -p all -v              # Verbose output
csync -s ./docs -p gdrive -d # Preview changes
```

## Logging

csync provides flexible logging options for both regular sync operations and daemon mode.

### Log Destinations

| Mode | Default Output | With `-l logfile` | With `-b` (background) |
|------|---------------|-------------------|------------------------|
| **Regular Sync** | Terminal/Console | File + Terminal | File only (if `-l` specified) or `/dev/null` |
| **Daemon Mode** | Terminal/Console | File + Terminal | File only (if `-l` specified) or `/dev/null` |
| **Background Daemon** | File (if `-l` specified) or `/dev/null` | File only | File only |

### Log File Examples

```bash
# Regular sync with log file
csync -s ./docs -p gdrive -l ./sync.log

# Regular sync in background with log file
csync -s ./docs -p gdrive -b -l ./sync.log

# Daemon with log file (visible in terminal too)
csync -s ./docs -p gdrive -daemon -l /var/log/csync.log

# Background daemon with log file (no terminal output)
csync -s ./docs -p gdrive -daemon -b -l /var/log/csync.log

# View live logs
tail -f /var/log/csync.log
```

### Log Levels

- **Default**: Essential operations and errors
- **Verbose (`-v`)**: Detailed operational information
- **Debug (`--debug`)**: Very detailed troubleshooting information

```bash
# Debug logging to file for troubleshooting
csync -s ./problematic-folder -p all --debug -l ./debug.log
```

### Production Logging Setup

```bash
# Create log directory
sudo mkdir -p /var/log/csync
sudo chown $USER:$USER /var/log/csync

# Start production daemon with logging
csync -s ~/important-data -p all -daemon -b \
      -watch -interval 1h \
      -v -l /var/log/csync/sync.log \
      -pid-file /var/run/csync.pid

# Monitor logs
tail -f /var/log/csync/sync.log

# Rotate logs (add to crontab)
# 0 0 * * 0 /usr/sbin/logrotate /etc/logrotate.d/csync
```
