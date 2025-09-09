# Configuration Examples

This directory contains clean, organized configuration examples for `csync` to help you get started quickly. All examples use the new secure approach with environment variables for credentials.

## 🔐 Security Best Practices

### Environment Variables (Recommended)
**Never store credentials in config files.** Use environment variables instead:

```bash
# Set environment variables
export PCLOUD_USERNAME="your-email@example.com"
export PCLOUD_PASSWORD="your-secure-password"

# Run csync with secure config
./csync -config examples/csync-secure.json -provider pcloud
```

### Quick Setup
```bash
# Use the setup script
./scripts/setup-credentials.sh

# Or manually copy example
cp examples/env-example .env
# Edit .env with your credentials
source .env
```

### Production Deployment
- Use secret management services (AWS Secrets Manager, HashiCorp Vault)
- Set environment variables in your deployment system
- Never commit `.env` files to version control
- Rotate credentials regularly

## 📁 Destination Paths

You can specify where files should be synced to using **destination paths** (recommended) or **folder IDs**:

### Using Destination Paths (Recommended)
```json
{
  "google_drive": {
    "destination_path": "/backups/documents"
  },
  "pcloud": {
    "destination_path": "/sync/photos"
  }
}
```

**Path Examples:**
- `/backups/documents` → Creates/uses `/backups/documents/` folder
- `/photos/2024` → Creates/uses `/photos/2024/` folder
- `/` or `""` → Uses root folder

## 📋 Available Examples

### 1. `csync-secure.json` - Secure Configuration ⭐🔐
**RECOMMENDED** for production use - credentials via environment variables.

**Use case**: Secure deployment without credentials in config files
**Security**: Uses environment variables for sensitive data
**Features**: Clean config, destination paths, daemon ready

### 2. `csync-minimal.json` - Minimal Configuration
The absolute minimum configuration needed to run csync.

**Use case**: Getting started quickly with basic sync
**Features**: Simple setup, basic ignore patterns
**Note**: Still includes placeholder credentials - use with env vars

### 3. `csync-daemon.json` - Production Daemon
Optimized for server/production daemon deployment.

**Use case**: Background service, server backups, automated sync
**Features**: Advanced settings, comprehensive ignore patterns, system paths
**Security**: No hardcoded credentials

### 4. `csync-photos.json` - Media/Photos Sync
Specialized configuration for photo and media file synchronization.

**Use case**: Photo backup, media archival, large file sync
**Features**: Media file patterns, large chunk sizes, high concurrency
**Formats**: Supports RAW, video, and common image formats

### 5. `csync-development.json` - Development Environment
Tailored for developers syncing source code and projects.

**Use case**: Code backup, project sync, development workflow
**Features**: Development ignore patterns, verbose logging, fast sync
**Excludes**: Build artifacts, dependencies, cache files

## 🚀 Configuration Structure

### Required Sections
- **`google_drive`**: Google Drive API configuration
- **`pcloud`**: pCloud API configuration including `source_path`
- **`general`**: Core sync settings including `source_path`

### Optional Section
- **`optional`**: Advanced features (daemon mode, logging, etc.)
  - **`daemon`**: Background service settings
  - **`logging`**: Log file and verbosity settings
  - **`advanced`**: Performance and behavior tweaks

## 🔧 Usage Examples

### Basic Sync
```bash
# Set credentials via environment
export PCLOUD_USERNAME="user@example.com"
export PCLOUD_PASSWORD="password"

# Run sync
./csync -config examples/csync-secure.json -provider pcloud
```

### Daemon Mode
```bash
# Start daemon
./csync -config examples/csync-daemon.json -daemon -provider all

# Check status
./csync -status

# Stop daemon
./csync -stop
```

### Development Workflow
```bash
# Verbose sync for debugging
./csync -config examples/csync-development.json -provider pcloud -v

# Dry run to see what would sync
./csync -config examples/csync-development.json -provider pcloud -dry-run
```

## 🔄 Migration from Old Format

If you have old configuration files with credentials, migrate them:

1. **Remove credentials** from JSON files
2. **Set environment variables** using `./scripts/setup-credentials.sh`
3. **Update paths** from `folder_id` to `destination_path`
4. **Add `source_path`** to the `general` section
5. **Move optional settings** to the `optional` section

## 📖 Additional Resources

- **Main README**: `../README.md` - Complete documentation
- **Security Script**: `../scripts/setup-credentials.sh` - Credential setup helper
- **Environment Example**: `env-example` - Template for `.env` file

---

**Security Reminder**: Always use environment variables for credentials. Never commit sensitive information to version control.
