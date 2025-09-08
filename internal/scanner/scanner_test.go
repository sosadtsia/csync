package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanDirectory(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "csync_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	testFiles := map[string]string{
		"file1.txt":                "content1",
		"file2.txt":                "content2",
		"subdir/file3.txt":         "content3",
		"subdir/file4.log":         "log content",
		"subdir2/file5.md":         "markdown content",
		"subdir2/nested/file6.txt": "nested content",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(tempDir, relPath)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Scan the directory
	files, err := ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Verify results
	expectedPaths := []string{
		"file1.txt",
		"file2.txt",
		"subdir",
		"subdir/file3.txt",
		"subdir/file4.log",
		"subdir2",
		"subdir2/file5.md",
		"subdir2/nested",
		"subdir2/nested/file6.txt",
	}

	if len(files) != len(expectedPaths) {
		t.Errorf("Expected %d files, got %d", len(expectedPaths), len(files))
	}

	foundPaths := make(map[string]bool)
	for _, file := range files {
		foundPaths[file.Path] = true

		// Verify absolute path exists
		if _, err := os.Stat(file.AbsolutePath); os.IsNotExist(err) {
			t.Errorf("Absolute path does not exist: %s", file.AbsolutePath)
		}

		// Verify size for files
		if !file.IsDir && file.Size == 0 {
			t.Errorf("File %s has zero size", file.Path)
		}

		// Verify MD5 hash for non-empty files
		if !file.IsDir && file.Size > 0 && file.MD5Hash == "" {
			t.Errorf("File %s missing MD5 hash", file.Path)
		}
	}

	for _, expectedPath := range expectedPaths {
		if !foundPaths[expectedPath] {
			t.Errorf("Expected path not found: %s", expectedPath)
		}
	}
}

func TestScannerWithPatterns(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "csync_pattern_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{
		"file1.txt",
		"file2.log",
		"temp.tmp",
		"important.md",
		".hidden",
		"subdir/file3.txt",
		"subdir/temp.tmp",
		"logs/app.log",
		"logs/error.log",
	}

	for _, relPath := range testFiles {
		fullPath := filepath.Join(tempDir, relPath)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	tests := []struct {
		name            string
		ignorePatterns  []string
		includePatterns []string
		expectedPaths   []string
	}{
		{
			name:           "ignore temp files",
			ignorePatterns: []string{"*.tmp"},
			expectedPaths:  []string{"file1.txt", "file2.log", "important.md", ".hidden", "subdir", "subdir/file3.txt", "logs", "logs/app.log", "logs/error.log"},
		},
		{
			name:           "ignore log files",
			ignorePatterns: []string{"*.log"},
			expectedPaths:  []string{"file1.txt", "temp.tmp", "important.md", ".hidden", "subdir", "subdir/file3.txt", "subdir/temp.tmp", "logs"},
		},
		{
			name:           "ignore logs directory",
			ignorePatterns: []string{"logs/"},
			expectedPaths:  []string{"file1.txt", "file2.log", "temp.tmp", "important.md", ".hidden", "subdir", "subdir/file3.txt", "subdir/temp.tmp"},
		},
		{
			name:            "include only txt files",
			includePatterns: []string{"*.txt"},
			expectedPaths:   []string{"file1.txt", "subdir", "subdir/file3.txt", "logs"},
		},
		{
			name:            "include md and log files",
			includePatterns: []string{"*.md", "*.log"},
			expectedPaths:   []string{"file2.log", "important.md", "subdir", "logs", "logs/app.log", "logs/error.log"},
		},
		{
			name:           "ignore tmp and hidden files",
			ignorePatterns: []string{"*.tmp", ".*"},
			expectedPaths:  []string{"file1.txt", "file2.log", "important.md", "subdir", "subdir/file3.txt", "logs", "logs/app.log", "logs/error.log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(tt.ignorePatterns, tt.includePatterns)
			files, err := scanner.Scan(tempDir)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			foundPaths := make(map[string]bool)
			for _, file := range files {
				foundPaths[file.Path] = true
			}

			for _, expectedPath := range tt.expectedPaths {
				if !foundPaths[expectedPath] {
					t.Errorf("Expected path not found: %s", expectedPath)
				}
			}

			// Check for unexpected paths
			for foundPath := range foundPaths {
				found := false
				for _, expectedPath := range tt.expectedPaths {
					if foundPath == expectedPath {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected path found: %s", foundPath)
				}
			}
		})
	}
}

func TestFilterByPatterns(t *testing.T) {
	// Create test files
	files := []FileInfo{
		{Path: "file1.txt", IsDir: false, Size: 100},
		{Path: "file2.log", IsDir: false, Size: 200},
		{Path: "temp.tmp", IsDir: false, Size: 50},
		{Path: "docs", IsDir: true},
		{Path: "docs/readme.md", IsDir: false, Size: 300},
		{Path: "src", IsDir: true},
		{Path: "src/main.go", IsDir: false, Size: 1000},
		{Path: ".git", IsDir: true},
		{Path: ".git/config", IsDir: false, Size: 150},
	}

	tests := []struct {
		name            string
		ignorePatterns  []string
		includePatterns []string
		expectedCount   int
		expectedPaths   []string
	}{
		{
			name:          "no filters",
			expectedCount: 9,
			expectedPaths: []string{"file1.txt", "file2.log", "temp.tmp", "docs", "docs/readme.md", "src", "src/main.go", ".git", ".git/config"},
		},
		{
			name:           "ignore temp files",
			ignorePatterns: []string{"*.tmp"},
			expectedCount:  8,
			expectedPaths:  []string{"file1.txt", "file2.log", "docs", "docs/readme.md", "src", "src/main.go", ".git", ".git/config"},
		},
		{
			name:           "ignore .git directory",
			ignorePatterns: []string{".git/"},
			expectedCount:  7,
			expectedPaths:  []string{"file1.txt", "file2.log", "temp.tmp", "docs", "docs/readme.md", "src", "src/main.go"},
		},
		{
			name:            "include only go files",
			includePatterns: []string{"*.go"},
			expectedCount:   3, // includes directories
			expectedPaths:   []string{"docs", "src", "src/main.go"},
		},
		{
			name:            "include txt and md files",
			includePatterns: []string{"*.txt", "*.md"},
			expectedCount:   4, // includes directories
			expectedPaths:   []string{"file1.txt", "docs", "docs/readme.md", "src"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterByPatterns(files, tt.ignorePatterns, tt.includePatterns)

			if len(filtered) != tt.expectedCount {
				t.Errorf("Expected %d files, got %d", tt.expectedCount, len(filtered))
			}

			foundPaths := make(map[string]bool)
			for _, file := range filtered {
				foundPaths[file.Path] = true
			}

			for _, expectedPath := range tt.expectedPaths {
				if !foundPaths[expectedPath] {
					t.Errorf("Expected path not found: %s", expectedPath)
				}
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	scanner := NewScanner(nil, nil)

	tests := []struct {
		pattern  string
		path     string
		isDir    bool
		expected bool
	}{
		{"*.txt", "file.txt", false, true},
		{"*.txt", "file.log", false, false},
		{"*.txt", "subdir/file.txt", false, true},
		{"temp*", "temp.txt", false, true},
		{"temp*", "temporary.txt", false, true},
		{"temp*", "file.temp", false, false},
		{"logs/", "logs", true, true},
		{"logs/", "logs", false, false},
		{"logs/", "logs/app.log", false, true},
		{".git", ".git", true, true},
		{".git", ".git", false, true},
		{".git", ".gitignore", false, false},
		{"sub*/", "subdir", true, true},
		{"sub*/", "subdir/file.txt", false, true},
		{"**/temp", "temp", false, true},
		{"**/temp", "dir/temp", false, true},
		{"**/temp", "dir/subdir/temp", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			result := scanner.matchPattern(tt.pattern, tt.path, tt.isDir)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q, %v) = %v, expected %v",
					tt.pattern, tt.path, tt.isDir, result, tt.expected)
			}
		})
	}
}

func TestCalculateMD5(t *testing.T) {
	// Create temporary file
	tempDir, err := os.MkdirTemp("", "csync_md5_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	content := "Hello, World!"

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	scanner := NewScanner(nil, nil)
	hash, err := scanner.calculateMD5(testFile)
	if err != nil {
		t.Fatalf("calculateMD5 failed: %v", err)
	}

	// Expected MD5 hash of "Hello, World!"
	expected := "65a8e27d8879283831b664bd8b7f0ad4"
	if hash != expected {
		t.Errorf("Expected hash %s, got %s", expected, hash)
	}
}

func TestFileInfoFields(t *testing.T) {
	// Create temporary file
	tempDir, err := os.MkdirTemp("", "csync_fileinfo_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	content := "test content"

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get file info to compare timestamps
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	files, err := ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Find our test file in the results
	var testFileInfo *FileInfo
	for _, file := range files {
		if file.Path == "test.txt" {
			testFileInfo = &file
			break
		}
	}

	if testFileInfo == nil {
		t.Fatal("Test file not found in scan results")
	}

	// Verify fields
	if testFileInfo.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", testFileInfo.Path)
	}

	if testFileInfo.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), testFileInfo.Size)
	}

	if testFileInfo.IsDir {
		t.Error("File should not be marked as directory")
	}

	if testFileInfo.MD5Hash == "" {
		t.Error("MD5 hash should not be empty")
	}

	// Check that timestamps are approximately correct (within 1 second)
	if testFileInfo.ModTime.Sub(info.ModTime()).Abs() > time.Second {
		t.Errorf("Modification time mismatch: expected %v, got %v",
			info.ModTime(), testFileInfo.ModTime)
	}
}
