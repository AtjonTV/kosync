//
// File:        internal/kosync/log_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogging(t *testing.T) {
	// Test basic logging functions (mostly for coverage as they wrap fiber log)
	LogInfo("Test info message")
	LogInfo("Test info message with arg: %s", "arg")
	LogError("Test error message")
	LogError("Test error message with arg: %v", 123)

	SetDebugLogging(false)
	LogDebug("This should not be logged")

	SetDebugLogging(true)
	LogDebug("This should be logged")
	LogDebug("This should be logged with arg: %d", 1)

	LogDebugUnchecked("Unchecked debug")
}

func TestKlog(t *testing.T) {
	k := NewKlog("test-tag")
	k.Info("Klog info")
	k.Error("Klog error")

	SetDebugLogging(false)
	k.Debug("Klog hidden debug")

	SetDebugLogging(true)
	k.Debug("Klog visible debug")
}

func TestSetLogOutput(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	writer, f := SetLogOutput(true, logFile)
	if writer == nil {
		t.Error("Expected writer, got nil")
	}
	if f == nil {
		t.Error("Expected file, got nil")
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	// Test failure to open file
	writer2, f2 := SetLogOutput(true, "/non/existent/path/to/log.log")
	if writer2 != os.Stdout {
		t.Error("Expected stdout on failure")
	}
	if f2 != nil {
		t.Error("Expected nil file on failure")
	}

	// Test no file logging
	writer3, f3 := SetLogOutput(false, "")
	if writer3 != os.Stdout {
		t.Error("Expected stdout when writeToFile is false")
	}
	if f3 != nil {
		t.Error("Expected nil file when writeToFile is false")
	}
}
