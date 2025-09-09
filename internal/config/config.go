package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the application configuration
type Config struct {
	GoogleDrive GoogleDriveConfig `json:"google_drive"`
	PCloud      PCloudConfig      `json:"pcloud"`
	General     GeneralConfig     `json:"general"`
	Optional    *OptionalConfig   `json:"optional,omitempty"`
}

// GoogleDriveConfig contains Google Drive API configuration
type GoogleDriveConfig struct {
	// Required fields
	CredentialsPath string   `json:"credentials_path"`
	TokenPath       string   `json:"token_path"`
	Scopes          []string `json:"scopes,omitempty"`

	// Optional fields - specify either folder_id OR destination_path
	FolderID        string            `json:"folder_id,omitempty"`        // Specific folder ID
	DestinationPath string            `json:"destination_path,omitempty"` // Folder path like "/backups/documents"
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// PCloudConfig contains pCloud API configuration
type PCloudConfig struct {
	// Required fields - can be set via environment variables
	Username string `json:"username,omitempty"` // Can use PCLOUD_USERNAME env var
	Password string `json:"password,omitempty"` // Can use PCLOUD_PASSWORD env var
	APIHost  string `json:"api_host,omitempty"`

	// Optional fields - specify either folder_id OR destination_path
	FolderID        string `json:"folder_id,omitempty"`        // Specific folder ID
	DestinationPath string `json:"destination_path,omitempty"` // Folder path like "/backups/photos"
}

// GeneralConfig contains general application settings
type GeneralConfig struct {
	// Required/Core settings
	SourcePath     string   `json:"source_path"` // Local directory to sync from
	MaxConcurrency int      `json:"max_concurrency"`
	RetryAttempts  int      `json:"retry_attempts"`
	ChunkSizeBytes int64    `json:"chunk_size_bytes"`
	IgnorePatterns []string `json:"ignore_patterns"`

	// Optional settings
	IncludePatterns []string `json:"include_patterns,omitempty"`
}

// OptionalConfig contains all optional/advanced features
type OptionalConfig struct {
	// Daemon mode settings
	Daemon *DaemonConfig `json:"daemon,omitempty"`

	// Logging settings
	Logging *LoggingConfig `json:"logging,omitempty"`

	// Advanced sync settings
	Advanced *AdvancedConfig `json:"advanced,omitempty"`
}

// DaemonConfig contains daemon-specific settings
type DaemonConfig struct {
	Enabled      bool   `json:"enabled"`
	SyncInterval string `json:"sync_interval"`
	WatchMode    bool   `json:"watch_mode"`
	PidFile      string `json:"pid_file"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	LogFile  string `json:"log_file,omitempty"`
	LogLevel string `json:"log_level,omitempty"`
	Verbose  bool   `json:"verbose,omitempty"`
}

// AdvancedConfig contains advanced sync settings
type AdvancedConfig struct {
	SkipExisting    bool     `json:"skip_existing,omitempty"`
	DeleteRemoved   bool     `json:"delete_removed,omitempty"`
	PreserveModTime bool     `json:"preserve_mod_time,omitempty"`
	CustomUserAgent string   `json:"custom_user_agent,omitempty"`
	ExcludeFolders  []string `json:"exclude_folders,omitempty"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		GoogleDrive: GoogleDriveConfig{
			CredentialsPath: "credentials.json",
			TokenPath:       "token.json",
			// Scopes will use defaults in the code
		},
		PCloud: PCloudConfig{
			// APIHost will use defaults in the code
		},
		General: GeneralConfig{
			SourcePath:     "", // Must be specified by user
			MaxConcurrency: 5,
			RetryAttempts:  3,
			ChunkSizeBytes: 8 * 1024 * 1024, // 8MB
			IgnorePatterns: []string{
				".git/",
				".DS_Store",
				"Thumbs.db",
				"*.tmp",
				"*.temp",
			},
		},
		// Optional section is omitted by default - will be nil
	}
}

// MinimalConfig returns a minimal configuration example
func MinimalConfig() *Config {
	return &Config{
		GoogleDrive: GoogleDriveConfig{
			CredentialsPath: "credentials.json",
			TokenPath:       "token.json",
			Scopes: []string{
				"https://www.googleapis.com/auth/drive.file",
			},
		},
		PCloud: PCloudConfig{
			Username: "your-email@example.com",
			Password: "your-password",
			APIHost:  "https://api.pcloud.com",
		},
		General: GeneralConfig{
			SourcePath:     "./documents", // Example source path
			MaxConcurrency: 5,
			RetryAttempts:  3,
			ChunkSizeBytes: 8 * 1024 * 1024,
			IgnorePatterns: []string{
				".git/",
				"*.tmp",
			},
		},
		// No optional section needed for basic usage
	}
}

// DaemonModeConfig returns a configuration optimized for daemon mode
func DaemonModeConfig() *Config {
	cfg := DefaultConfig()
	cfg.Optional = &OptionalConfig{
		Daemon: &DaemonConfig{
			Enabled:      true,
			SyncInterval: "5m",
			WatchMode:    false,
			PidFile:      "csync.pid",
		},
		Logging: &LoggingConfig{
			LogFile:  "csync.log",
			LogLevel: "info",
			Verbose:  false,
		},
		Advanced: &AdvancedConfig{
			SkipExisting:    true,
			DeleteRemoved:   false,
			PreserveModTime: true,
		},
	}
	return cfg
}

// Load reads configuration from a file, creating defaults if it doesn't exist
func Load(path string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config file
		cfg := DefaultConfig()
		if err := cfg.Save(path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		fmt.Printf("Created default configuration file: %s\n", path)
		fmt.Println("Please update the configuration with your API credentials before running csync.")
		return cfg, nil
	}

	// Read existing config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing values
	defaultCfg := DefaultConfig()
	if cfg.General.MaxConcurrency == 0 {
		cfg.General.MaxConcurrency = defaultCfg.General.MaxConcurrency
	}
	if cfg.General.RetryAttempts == 0 {
		cfg.General.RetryAttempts = defaultCfg.General.RetryAttempts
	}
	if cfg.General.ChunkSizeBytes == 0 {
		cfg.General.ChunkSizeBytes = defaultCfg.General.ChunkSizeBytes
	}

	// Apply environment variable overrides for sensitive data
	cfg.applyEnvOverrides()

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides for sensitive data
func (c *Config) applyEnvOverrides() {
	// pCloud credentials
	if username := os.Getenv("PCLOUD_USERNAME"); username != "" {
		c.PCloud.Username = username
	}
	if password := os.Getenv("PCLOUD_PASSWORD"); password != "" {
		c.PCloud.Password = password
	}

	// Google Drive credentials path (can be overridden)
	if credsPath := os.Getenv("GOOGLE_CREDENTIALS_PATH"); credsPath != "" {
		c.GoogleDrive.CredentialsPath = credsPath
	}
	if tokenPath := os.Getenv("GOOGLE_TOKEN_PATH"); tokenPath != "" {
		c.GoogleDrive.TokenPath = tokenPath
	}
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.General.MaxConcurrency <= 0 {
		return fmt.Errorf("max_concurrency must be greater than 0")
	}

	if c.General.RetryAttempts < 0 {
		return fmt.Errorf("retry_attempts must be non-negative")
	}

	if c.General.ChunkSizeBytes <= 0 {
		return fmt.Errorf("chunk_size_bytes must be greater than 0")
	}

	return nil
}

// IsDaemonMode returns true if daemon mode is enabled
func (c *Config) IsDaemonMode() bool {
	return c.Optional != nil && c.Optional.Daemon != nil && c.Optional.Daemon.Enabled
}

// GetSyncInterval returns the sync interval or default
func (c *Config) GetSyncInterval() string {
	if c.Optional != nil && c.Optional.Daemon != nil && c.Optional.Daemon.SyncInterval != "" {
		return c.Optional.Daemon.SyncInterval
	}
	return "5m" // default
}

// IsWatchMode returns true if file watching is enabled
func (c *Config) IsWatchMode() bool {
	return c.Optional != nil && c.Optional.Daemon != nil && c.Optional.Daemon.WatchMode
}

// GetPidFile returns the PID file path or default
func (c *Config) GetPidFile() string {
	if c.Optional != nil && c.Optional.Daemon != nil && c.Optional.Daemon.PidFile != "" {
		return c.Optional.Daemon.PidFile
	}
	return "csync.pid" // default
}

// GetLogFile returns the log file path or empty string
func (c *Config) GetLogFile() string {
	if c.Optional != nil && c.Optional.Logging != nil {
		return c.Optional.Logging.LogFile
	}
	return "" // no logging by default
}
