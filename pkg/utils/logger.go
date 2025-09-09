package utils

import (
	"fmt"
	"log"
	"os"
)

var (
	verboseMode   bool
	cleanLogger   *log.Logger
	verboseLogger *log.Logger
)

func init() {
	// Clean logger without file/line info for regular output
	cleanLogger = log.New(os.Stderr, "", log.LstdFlags)
	// Verbose logger with file/line info for detailed output
	verboseLogger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
}

// SetVerbose sets the verbose logging mode
func SetVerbose(v bool) {
	verboseMode = v
}

// LogInfo logs an info message (always shown)
func LogInfo(format string, args ...interface{}) {
	if verboseMode {
		verboseLogger.Printf(format, args...)
	} else {
		cleanLogger.Printf(format, args...)
	}
}

// LogVerbose logs a verbose message (only shown in verbose mode)
func LogVerbose(format string, args ...interface{}) {
	if verboseMode {
		verboseLogger.Printf("[VERBOSE] "+format, args...)
	}
}

// LogDebug logs a debug message (only shown in verbose mode)
func LogDebug(format string, args ...interface{}) {
	if verboseMode {
		verboseLogger.Printf("[DEBUG] "+format, args...)
	}
}

// LogError logs an error message (always shown)
func LogError(format string, args ...interface{}) {
	if verboseMode {
		verboseLogger.Printf("[ERROR] "+format, args...)
	} else {
		cleanLogger.Printf("Error: "+format, args...)
	}
}

// Print logs a simple message without timestamp (for clean output)
func Print(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
