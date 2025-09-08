package utils

import (
	"path/filepath"
	"strings"
)

// ShouldIgnore checks if a file path should be ignored based on patterns
func ShouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchPattern checks if a path matches a given pattern
func matchPattern(path, pattern string) bool {
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		dirPattern := strings.TrimSuffix(pattern, "/")
		// Check if the path is within this directory
		return strings.HasPrefix(path, dirPattern+"/") || path == dirPattern
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			// If pattern matching fails, fall back to string comparison
			return strings.Contains(path, strings.ReplaceAll(pattern, "*", ""))
		}
		return matched
	}

	// Exact match
	return path == pattern || strings.HasSuffix(path, "/"+pattern) || strings.Contains(path, "/"+pattern+"/")
}

// FilterPaths filters a list of paths based on include and exclude patterns
func FilterPaths(paths []string, excludePatterns, includePatterns []string) []string {
	var filtered []string

	for _, path := range paths {
		// Check exclude patterns first
		if len(excludePatterns) > 0 && ShouldIgnore(path, excludePatterns) {
			continue
		}

		// If include patterns are specified, path must match at least one
		if len(includePatterns) > 0 {
			included := false
			for _, pattern := range includePatterns {
				if matchPattern(path, pattern) {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		filtered = append(filtered, path)
	}

	return filtered
}
