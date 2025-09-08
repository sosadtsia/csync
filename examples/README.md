# Configuration Examples

This directory contains various configuration examples for `csync` to help you get started quickly with different use cases. The configurations are structured to clearly separate **required** and **optional** parameters.

## Configuration Structure

### Required Sections
- **`google_drive`**: Google Drive API configuration
- **`pcloud`**: pCloud API configuration
- **`general`**: Core sync settings

### Optional Section
- **`optional`**: Advanced features (daemon mode, logging, etc.)

## Available Examples

### 1. `csync-minimal.json` - Minimal Configuration ⭐
The absolute minimum configuration needed to run csync.

**Use case**: Getting started quickly with basic sync
**Features**:
- Only required fields
- No optional features
- Clean and simple

```json
{
  "google_drive": {
    "credentials_path": "credentials.json",
    "token_path": "token.json",
    "scopes": ["https://www.googleapis.com/auth/drive.file"]
  },
  "pcloud": {
    "username": "your-email@example.com",
    "password": "your-password",
    "api_host": "https://api.pcloud.com"
  },
  "general": {
    "max_concurrency": 5,
    "retry_attempts": 3,
    "chunk_size_bytes": 8388608,
    "ignore_patterns": [".git/", "*.tmp"]
  }
}
```

**Usage**:
```bash
csync -config examples/csync-minimal.json -source ./documents -provider gdrive
```

### 2. `csync-basic.json` - Basic Configuration
Standard configuration with common optional features.

**Use case**: Personal file sync with some customization
**Features**:
- Basic Google Drive and pCloud setup
- Standard ignore patterns
- Optional metadata for file tagging

**Usage**:
```bash
csync -config examples/csync-basic.json -source ./documents -provider gdrive
```

### 3. `csync-daemon.json` - Production Daemon
Full-featured configuration for production daemon mode.

**Use case**: Server/production environment with continuous sync
**Features**:
- Daemon mode enabled in `optional` section
- Advanced logging configuration
- Production-optimized settings
- Document-focused include patterns

```json
{
  // ... required sections ...
  "optional": {
    "daemon": {
      "enabled": true,
      "sync_interval": "15m",
      "watch_mode": true,
      "pid_file": "/var/run/csync.pid"
    },
    "logging": {
      "log_file": "/var/log/csync.log",
      "log_level": "info"
    },
    "advanced": {
      "skip_existing": true,
      "preserve_mod_time": true
    }
  }
}
```

**Usage**:
```bash
# Start daemon
csync -config examples/csync-daemon.json -source /data/documents -provider all -daemon

# Control daemon
csync -config examples/csync-daemon.json -status
csync -config examples/csync-daemon.json -stop
```

### 4. `csync-development.json` - Development Environment
Configured for development and testing.

**Use case**: Development environment with frequent changes
**Features**:
- Development-optimized settings
- Local paths for PID and log files
- Development-focused ignore patterns

### 5. `csync-photos.json` - Media/Photo Backup
Optimized for syncing photos and media files.

**Use case**: Photo and video backup
**Features**:
- Media-specific include patterns
- Large chunk size for big files
- Photo-specific ignore patterns

## Configuration Reference

### Required Fields

#### Google Drive (Required)
```json
{
  "credentials_path": "credentials.json",    // OAuth2 credentials file (required)
  "token_path": "token.json",               // OAuth2 token storage (required)
  "scopes": ["https://www.googleapis.com/auth/drive.file"]  // API scopes (required)
}
```

#### pCloud (Required)
```json
{
  "username": "your-email@example.com",     // pCloud username (required)
  "password": "your-password",              // pCloud password (required)
  "api_host": "https://api.pcloud.com"     // API endpoint (required)
}
```

#### General Settings (Required)
```json
{
  "max_concurrency": 5,                     // Concurrent uploads (required)
  "retry_attempts": 3,                      // Retry failed uploads (required)
  "chunk_size_bytes": 8388608,              // Upload chunk size (required)
  "ignore_patterns": ["*.tmp", ".git/"]    // Files to ignore (required)
}
```

### Optional Fields

#### Google Drive (Optional)
```json
{
  "folder_id": "1ABC...XYZ",                // Target folder ID (empty = root)
  "metadata": {                             // Custom metadata for uploads
    "source": "csync",
    "version": "1.0.0"
  }
}
```

#### pCloud (Optional)
```json
{
  "folder_id": "12345"                      // Target folder ID (empty = root)
}
```

#### General Settings (Optional)
```json
{
  "include_patterns": ["*.pdf", "*.docx"]   // Files to include (if specified)
}
```

#### Optional Section
```json
{
  "optional": {
    "daemon": {
      "enabled": true,                      // Enable daemon mode
      "sync_interval": "5m",               // Sync interval
      "watch_mode": false,                 // Real-time file watching
      "pid_file": "csync.pid"              // PID file location
    },
    "logging": {
      "log_file": "csync.log",             // Log file location
      "log_level": "info",                 // Log level (debug, info, warn, error)
      "verbose": false                     // Verbose output
    },
    "advanced": {
      "skip_existing": true,               // Skip files that already exist
      "delete_removed": false,             // Delete files removed locally
      "preserve_mod_time": true,           // Preserve modification times
      "custom_user_agent": "MyApp/1.0",   // Custom User-Agent header
      "exclude_folders": ["temp/", "cache/"]  // Additional folder exclusions
    }
  }
}
```

## Migration from Old Format

If you have an old configuration, here's how to migrate:

**Old Format:**
```json
{
  "general": {
    "daemon_mode": true,
    "sync_interval": "5m",
    "watch_mode": false,
    "pid_file": "csync.pid",
    "log_file": "csync.log"
  }
}
```

**New Format:**
```json
{
  "general": {
    // only core settings here
  },
  "optional": {
    "daemon": {
      "enabled": true,
      "sync_interval": "5m",
      "watch_mode": false,
      "pid_file": "csync.pid"
    },
    "logging": {
      "log_file": "csync.log"
    }
  }
}
```

## Getting Started

1. **Start simple**: Copy `csync-minimal.json` for basic usage
2. **Add features**: Copy sections from other examples as needed
3. **Update credentials**: Add your API credentials
4. **Test first**: Always test with `-dry-run` flag
5. **Add optional features**: Only add the `optional` section when you need advanced features

## Benefits of New Structure

✅ **Clear separation**: Required vs. optional parameters
✅ **Easier to start**: Minimal config has only essential fields
✅ **Better organization**: Related settings grouped together
✅ **Future-proof**: Easy to add new optional features
✅ **Self-documenting**: Structure shows what's necessary vs. nice-to-have

For more information, see the main [README.md](../README.md) in the project root.
