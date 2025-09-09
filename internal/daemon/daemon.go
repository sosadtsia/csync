package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/internal/sync"
	"github.com/svosadtsia/csync/internal/watcher"
)

// Daemon represents a background sync daemon
type Daemon struct {
	config      *config.Config
	syncManager *sync.Manager
	watcher     *watcher.FileWatcher
	pidFile     string
	logFile     string
	interval    time.Duration
	stopChan    chan struct{}
}

// NewDaemon creates a new daemon instance
func NewDaemon(cfg *config.Config, syncManager *sync.Manager) (*Daemon, error) {
	interval, err := time.ParseDuration(cfg.GetSyncInterval())
	if err != nil {
		return nil, fmt.Errorf("invalid sync interval %s: %w", cfg.GetSyncInterval(), err)
	}

	daemon := &Daemon{
		config:      cfg,
		syncManager: syncManager,
		pidFile:     cfg.GetPidFile(),
		logFile:     cfg.GetLogFile(),
		interval:    interval,
		stopChan:    make(chan struct{}),
	}

	// Initialize file watcher if watch mode is enabled
	if cfg.IsWatchMode() {
		watcher, err := watcher.NewFileWatcher(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create file watcher: %w", err)
		}
		daemon.watcher = watcher
	}

	return daemon, nil
}

// Start starts the daemon process
func (d *Daemon) Start(ctx context.Context, sourcePath, provider string) error {
	// Setup daemon logging
	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Write PID file
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer d.removePIDFile()

	log.Printf("Starting csync daemon (PID: %d)", os.Getpid())
	log.Printf("Sync interval: %s", d.interval)
	log.Printf("Source: %s", sourcePath)
	log.Printf("Provider: %s", provider)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Start file watcher if enabled
	if d.watcher != nil {
		log.Println("Starting file watcher for real-time sync")
		go d.runFileWatcher(ctx, sourcePath, provider)
	}

	// Start periodic sync
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	// Perform initial sync
	log.Println("Performing initial sync...")
	if err := d.performSync(ctx, sourcePath, provider); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down daemon")
			return ctx.Err()

		case <-d.stopChan:
			log.Println("Stop signal received, shutting down daemon")
			return nil

		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				log.Println("SIGHUP received, reloading configuration")
				if err := d.reloadConfig(); err != nil {
					log.Printf("Failed to reload config: %v", err)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("%s received, shutting down daemon gracefully", sig)
				return nil
			}

		case <-ticker.C:
			log.Println("Starting scheduled sync...")
			if err := d.performSync(ctx, sourcePath, provider); err != nil {
				log.Printf("Scheduled sync failed: %v", err)
			}
		}
	}
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	close(d.stopChan)
	if d.watcher != nil {
		d.watcher.Stop()
	}
}

// performSync executes a sync operation
func (d *Daemon) performSync(ctx context.Context, sourcePath, provider string) error {
	start := time.Now()
	log.Printf("Starting sync operation (provider: %s)", provider)

	// Show destination paths
	switch provider {
	case "gdrive":
		if d.config.GoogleDrive.DestinationPath != "" {
			log.Printf("Google Drive destination: %s", d.config.GoogleDrive.DestinationPath)
		}
	case "pcloud":
		if d.config.PCloud.DestinationPath != "" {
			log.Printf("pCloud destination: %s", d.config.PCloud.DestinationPath)
		}
	case "all":
		if d.config.GoogleDrive.DestinationPath != "" {
			log.Printf("Google Drive destination: %s", d.config.GoogleDrive.DestinationPath)
		}
		if d.config.PCloud.DestinationPath != "" {
			log.Printf("pCloud destination: %s", d.config.PCloud.DestinationPath)
		}
	}

	var err error
	switch provider {
	case "gdrive":
		err = d.syncManager.SyncToGoogleDrive(ctx, sourcePath, false)
	case "pcloud":
		err = d.syncManager.SyncToPCloud(ctx, sourcePath, false)
	case "all":
		// Sync to both providers
		if gdriveErr := d.syncManager.SyncToGoogleDrive(ctx, sourcePath, false); gdriveErr != nil {
			log.Printf("Google Drive sync failed: %v", gdriveErr)
			err = gdriveErr
		}
		if pcloudErr := d.syncManager.SyncToPCloud(ctx, sourcePath, false); pcloudErr != nil {
			log.Printf("pCloud sync failed: %v", pcloudErr)
			if err == nil {
				err = pcloudErr
			}
		}
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	duration := time.Since(start)
	if err != nil {
		log.Printf("Sync completed with errors in %v: %v", duration, err)
		return err
	}

	log.Printf("Sync completed successfully in %v", duration)
	return nil
}

// runFileWatcher runs the file watcher for real-time sync
func (d *Daemon) runFileWatcher(ctx context.Context, sourcePath, provider string) {
	if d.watcher == nil {
		return
	}

	// Add the source path to watch
	if err := d.watcher.AddPath(sourcePath); err != nil {
		log.Printf("Failed to add watch path %s: %v", sourcePath, err)
		return
	}

	// Listen for file events
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-d.watcher.Events():
			log.Printf("File event: %s %s", event.Op, event.Name)
			// Debounce file events to avoid excessive syncing
			time.Sleep(1 * time.Second)
			if err := d.performSync(ctx, sourcePath, provider); err != nil {
				log.Printf("File watcher sync failed: %v", err)
			}
		case err := <-d.watcher.Errors():
			log.Printf("File watcher error: %v", err)
		}
	}
}

// setupLogging configures logging for daemon mode
func (d *Daemon) setupLogging() error {
	if d.logFile == "" {
		return nil // Use default logging
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(d.logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(d.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Set log output to file
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return nil
}

// writePIDFile writes the process ID to a file
func (d *Daemon) writePIDFile() error {
	if d.pidFile == "" {
		return nil
	}

	pidDir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	pid := os.Getpid()
	pidStr := strconv.Itoa(pid)

	if err := os.WriteFile(d.pidFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// removePIDFile removes the PID file
func (d *Daemon) removePIDFile() {
	if d.pidFile != "" {
		os.Remove(d.pidFile)
	}
}

// reloadConfig reloads the daemon configuration
func (d *Daemon) reloadConfig() error {
	// Note: In a more sophisticated implementation, you might want to
	// reload the config from file and update the daemon settings
	log.Println("Configuration reload requested (not implemented yet)")
	return nil
}

// IsRunning checks if a daemon is already running by checking the PID file
func IsRunning(pidFile string) (bool, int, error) {
	if pidFile == "" {
		return false, 0, nil
	}

	// Check if PID file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false, 0, nil
	}

	// Read PID from file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return false, 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	// Check if process is still running
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, pid, nil
	}

	return true, pid, nil
}

// StopDaemon stops a running daemon by sending SIGTERM
func StopDaemon(pidFile string) error {
	running, pid, err := IsRunning(pidFile)
	if err != nil {
		return err
	}

	if !running {
		return fmt.Errorf("daemon is not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
	}

	log.Printf("Sent SIGTERM to daemon process %d", pid)
	return nil
}
