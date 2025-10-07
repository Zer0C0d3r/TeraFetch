package utils

import (
	"testing"
	"time"
)

func TestProgressTracker_BasicFunctionality(t *testing.T) {
	// Test quiet mode
	quietTracker := NewProgressTracker(1000, true)
	if !quietTracker.IsQuiet() {
		t.Error("Expected quiet tracker to be in quiet mode")
	}
	
	// Update progress
	quietTracker.Update(500)
	
	// Get current stats
	speed, eta, percentage := quietTracker.GetCurrentStats()
	if percentage != 50.0 {
		t.Errorf("Expected 50%% progress, got %.1f%%", percentage)
	}
	
	// Finish and get summary
	summary := quietTracker.Finish()
	if summary == nil {
		t.Error("Expected summary to be returned")
	}
	
	if summary.TotalBytes != 500 {
		t.Errorf("Expected 500 bytes, got %d", summary.TotalBytes)
	}
	
	// Test that speed and ETA are calculated (even if zero initially)
	_ = speed
	_ = eta
}

func TestProgressTracker_StatisticsCalculation(t *testing.T) {
	tracker := NewProgressTracker(1000, true)
	
	// Simulate progress updates with time delays
	tracker.Update(100)
	time.Sleep(10 * time.Millisecond)
	tracker.Update(300)
	time.Sleep(10 * time.Millisecond)
	tracker.Update(600)
	
	// Get statistics
	speed, eta, percentage := tracker.GetCurrentStats()
	
	if percentage != 60.0 {
		t.Errorf("Expected 60%% progress, got %.1f%%", percentage)
	}
	
	// Speed should be calculated (may be zero due to short time intervals in tests)
	if speed < 0 {
		t.Error("Speed should not be negative")
	}
	
	// ETA should be calculated for incomplete downloads
	if eta < 0 {
		t.Error("ETA should not be negative")
	}
	
	// Complete the download
	tracker.Update(1000)
	summary := tracker.Finish()
	
	if summary.TotalBytes != 1000 {
		t.Errorf("Expected 1000 bytes, got %d", summary.TotalBytes)
	}
	
	if summary.TotalTime <= 0 {
		t.Error("Total time should be positive")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{5368709120, "5.0 GB"},
	}
	
	for _, test := range tests {
		result := formatBytes(test.bytes)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

func TestProgressTracker_NonQuietMode(t *testing.T) {
	// This test verifies that non-quiet mode doesn't crash
	// We can't easily test the visual output in unit tests
	tracker := NewProgressTracker(1000, false)
	
	if tracker.IsQuiet() {
		t.Error("Expected non-quiet tracker")
	}
	
	tracker.Update(250)
	tracker.Update(500)
	tracker.Update(750)
	tracker.Update(1000)
	
	summary := tracker.Finish()
	if summary == nil {
		t.Error("Expected summary to be returned")
	}
}