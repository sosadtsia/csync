package watcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/pkg/utils"
)

// FileEvent represents a file system event
type FileEvent struct {
	Name string    // File path
	Op   Operation // Operation type
	Time time.Time // Event timestamp
}

// Operation represents the type of file operation
type Operation string

const (
	Create Operation = "CREATE"
	Write  Operation = "WRITE"
	Remove Operation = "REMOVE"
	Rename Operation = "RENAME"
	Chmod  Operation = "CHMOD"
)

// FileWatcher watches for file system changes
type FileWatcher struct {
	config      *config.Config
	watchPaths  map[string]bool
	events      chan FileEvent
	errors      chan error
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
	debounceMap map[string]time.Time
	debounce    time.Duration
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(cfg *config.Config) (*FileWatcher, error) {
	return &FileWatcher{
		config:      cfg,
		watchPaths:  make(map[string]bool),
		events:      make(chan FileEvent, 100),
		errors:      make(chan error, 10),
		stopChan:    make(chan struct{}),
		debounceMap: make(map[string]time.Time),
		debounce:    2 * time.Second, // Debounce events for 2 seconds
	}, nil
}

// AddPath adds a path to watch for changes
func (fw *FileWatcher) AddPath(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if fw.watchPaths[absPath] {
		return nil // Already watching this path
	}

	// Check if path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	fw.watchPaths[absPath] = true
	log.Printf("Added watch path: %s", absPath)

	// Start watching this path
	fw.wg.Add(1)
	go fw.watchPath(absPath)

	return nil
}

// RemovePath removes a path from watching
func (fw *FileWatcher) RemovePath(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if !fw.watchPaths[absPath] {
		return nil // Not watching this path
	}

	delete(fw.watchPaths, absPath)
	log.Printf("Removed watch path: %s", absPath)

	return nil
}

// Events returns the events channel
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Errors returns the errors channel
func (fw *FileWatcher) Errors() <-chan error {
	return fw.errors
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() {
	close(fw.stopChan)
	fw.wg.Wait()
	close(fw.events)
	close(fw.errors)
}

// watchPath watches a specific path for changes using polling
// Note: This is a simplified implementation using polling since Go's standard library
// doesn't include file system notifications. For production use, consider using
// a third-party library like fsnotify.
func (fw *FileWatcher) watchPath(path string) {
	defer fw.wg.Done()

	// Keep track of file states
	fileStates := make(map[string]os.FileInfo)

	// Initial scan
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip ignored files
		relPath, _ := filepath.Rel(path, filePath)
		if utils.ShouldIgnore(relPath, fw.config.General.IgnorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		fileStates[filePath] = info
		return nil
	})

	if err != nil {
		fw.errors <- fmt.Errorf("initial scan failed for %s: %w", path, err)
		return
	}

	ticker := time.NewTicker(1 * time.Second) // Poll every second
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopChan:
			return
		case <-ticker.C:
			fw.checkForChanges(path, fileStates)
		}
	}
}

// checkForChanges checks for file system changes by comparing current state with previous state
func (fw *FileWatcher) checkForChanges(basePath string, fileStates map[string]os.FileInfo) {
	currentStates := make(map[string]os.FileInfo)

	// Scan current state
	err := filepath.Walk(basePath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip ignored files
		relPath, _ := filepath.Rel(basePath, filePath)
		if utils.ShouldIgnore(relPath, fw.config.General.IgnorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		currentStates[filePath] = info
		return nil
	})

	if err != nil {
		fw.errors <- fmt.Errorf("scan failed for %s: %w", basePath, err)
		return
	}

	// Check for new or modified files
	for filePath, currentInfo := range currentStates {
		if previousInfo, exists := fileStates[filePath]; exists {
			// File existed before, check if modified
			if !currentInfo.ModTime().Equal(previousInfo.ModTime()) ||
				currentInfo.Size() != previousInfo.Size() {
				fw.sendEvent(FileEvent{
					Name: filePath,
					Op:   Write,
					Time: time.Now(),
				})
			}
		} else {
			// New file
			fw.sendEvent(FileEvent{
				Name: filePath,
				Op:   Create,
				Time: time.Now(),
			})
		}
	}

	// Check for deleted files
	for filePath := range fileStates {
		if _, exists := currentStates[filePath]; !exists {
			fw.sendEvent(FileEvent{
				Name: filePath,
				Op:   Remove,
				Time: time.Now(),
			})
		}
	}

	// Update file states
	for filePath, info := range currentStates {
		fileStates[filePath] = info
	}

	// Remove deleted files from tracking
	for filePath := range fileStates {
		if _, exists := currentStates[filePath]; !exists {
			delete(fileStates, filePath)
		}
	}
}

// sendEvent sends an event with debouncing
func (fw *FileWatcher) sendEvent(event FileEvent) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Debounce events for the same file
	if lastTime, exists := fw.debounceMap[event.Name]; exists {
		if time.Since(lastTime) < fw.debounce {
			return // Skip this event due to debouncing
		}
	}

	fw.debounceMap[event.Name] = event.Time

	select {
	case fw.events <- event:
	default:
		// Channel is full, drop the event
		log.Printf("Warning: Event channel full, dropping event for %s", event.Name)
	}
}

// String returns a string representation of the operation
func (op Operation) String() string {
	return string(op)
}

// WatchConfig represents configuration for file watching
type WatchConfig struct {
	Recursive      bool          // Watch subdirectories recursively
	PollInterval   time.Duration // Polling interval for changes
	DebounceTime   time.Duration // Debounce time for events
	IgnorePatterns []string      // Patterns to ignore
}

// DefaultWatchConfig returns default watch configuration
func DefaultWatchConfig() WatchConfig {
	return WatchConfig{
		Recursive:    true,
		PollInterval: 1 * time.Second,
		DebounceTime: 2 * time.Second,
		IgnorePatterns: []string{
			".git/",
			".DS_Store",
			"Thumbs.db",
			"*.tmp",
			"*.temp",
			"*.swp",
			"*~",
		},
	}
}
