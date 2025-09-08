package scanner

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo represents metadata about a file to be synced
type FileInfo struct {
	Path         string    // Relative path from sync root
	AbsolutePath string    // Absolute path on filesystem
	Size         int64     // File size in bytes
	ModTime      time.Time // Last modification time
	IsDir        bool      // Whether this is a directory
	MD5Hash      string    // MD5 hash of file content (empty for directories)
}

// Scanner handles directory scanning with pattern matching
type Scanner struct {
	ignorePatterns  []string
	includePatterns []string
}

// NewScanner creates a new scanner with pattern filters
func NewScanner(ignorePatterns, includePatterns []string) *Scanner {
	return &Scanner{
		ignorePatterns:  ignorePatterns,
		includePatterns: includePatterns,
	}
}

// ScanDirectory scans a directory and returns file information
func ScanDirectory(rootPath string) ([]FileInfo, error) {
	scanner := NewScanner(nil, nil)
	return scanner.Scan(rootPath)
}

// Scan performs the directory scan with configured patterns
func (s *Scanner) Scan(rootPath string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing %s: %w", path, err)
		}

		// Get relative path from root
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Apply ignore patterns
		if s.shouldIgnore(relPath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply include patterns (if specified)
		if !s.shouldInclude(relPath, info.IsDir()) {
			if info.IsDir() {
				return nil // Don't skip directory, but don't include it
			}
			return nil
		}

		fileInfo := FileInfo{
			Path:         filepath.ToSlash(relPath), // Use forward slashes for consistency
			AbsolutePath: path,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IsDir:        info.IsDir(),
		}

		// Calculate MD5 hash for files (not directories)
		if !info.IsDir() && info.Size() > 0 {
			hash, err := s.calculateMD5(path)
			if err != nil {
				// Log warning but continue processing
				fmt.Printf("Warning: Failed to calculate MD5 for %s: %v\n", path, err)
			} else {
				fileInfo.MD5Hash = hash
			}
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return files, nil
}

// shouldIgnore checks if a path should be ignored based on patterns
func (s *Scanner) shouldIgnore(relPath string, isDir bool) bool {
	for _, pattern := range s.ignorePatterns {
		if matched := s.matchPattern(pattern, relPath, isDir); matched {
			return true
		}
	}
	return false
}

// shouldInclude checks if a path should be included based on patterns
func (s *Scanner) shouldInclude(relPath string, isDir bool) bool {
	// If no include patterns specified, include everything
	if len(s.includePatterns) == 0 {
		return true
	}

	// For directories, always include them to allow traversal
	if isDir {
		return true
	}

	// Check if file matches any include pattern
	for _, pattern := range s.includePatterns {
		if matched := s.matchPattern(pattern, relPath, isDir); matched {
			return true
		}
	}

	return false
}

// matchPattern performs pattern matching using filepath.Match and custom logic
func (s *Scanner) matchPattern(pattern, path string, isDir bool) bool {
	// Convert to forward slashes for consistent matching
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	// Handle directory-specific patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		if !isDir {
			return false
		}
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Try exact match first
	if path == pattern {
		return true
	}

	// Try filepath.Match for shell-style patterns
	if matched, err := filepath.Match(pattern, path); err == nil && matched {
		return true
	}

	// Check if path starts with pattern (for directory matching)
	if strings.HasPrefix(path, pattern+"/") {
		return true
	}

	// Check if any parent directory matches the pattern
	dir := filepath.Dir(path)
	for dir != "." && dir != "/" {
		if matched, err := filepath.Match(pattern, filepath.Base(dir)); err == nil && matched {
			return true
		}
		dir = filepath.Dir(dir)
	}

	return false
}

// calculateMD5 computes MD5 hash of a file
func (s *Scanner) calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file for hashing: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// FilterByPatterns applies ignore and include patterns to a list of files
func FilterByPatterns(files []FileInfo, ignorePatterns, includePatterns []string) []FileInfo {
	scanner := NewScanner(ignorePatterns, includePatterns)
	var filtered []FileInfo

	for _, file := range files {
		if scanner.shouldIgnore(file.Path, file.IsDir) {
			continue
		}
		if !scanner.shouldInclude(file.Path, file.IsDir) {
			continue
		}
		filtered = append(filtered, file)
	}

	return filtered
}
