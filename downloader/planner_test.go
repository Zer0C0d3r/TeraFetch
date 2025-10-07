package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"terafetch/internal"
)

func TestDownloadPlanner_CalculateSegments(t *testing.T) {
	planner := NewDownloadPlanner()

	tests := []struct {
		name         string
		fileSize     int64
		threadCount  int
		expectedSegs int
		description  string
	}{
		{
			name:         "small_file_single_thread",
			fileSize:     500 * 1024, // 500KB
			threadCount:  8,
			expectedSegs: 1,
			description:  "Small files should use single thread",
		},
		{
			name:         "large_file_multi_thread",
			fileSize:     100 * 1024 * 1024, // 100MB
			threadCount:  8,
			expectedSegs: 8,
			description:  "Large files should use requested threads",
		},
		{
			name:         "medium_file_limited_threads",
			fileSize:     5 * 1024 * 1024, // 5MB
			threadCount:  8,
			expectedSegs: 5,
			description:  "Medium files should limit threads to maintain min segment size",
		},
		{
			name:         "zero_file_size",
			fileSize:     0,
			threadCount:  4,
			expectedSegs: 0,
			description:  "Zero file size should return empty segments",
		},
		{
			name:         "negative_threads",
			fileSize:     10 * 1024 * 1024,
			threadCount:  -1,
			expectedSegs: 1,
			description:  "Negative thread count should default to 1",
		},
		{
			name:         "excessive_threads",
			fileSize:     1000 * 1024 * 1024, // 1GB
			threadCount:  50,
			expectedSegs: 32,
			description:  "Thread count should be capped at maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := planner.CalculateSegments(tt.fileSize, tt.threadCount)
			
			if len(segments) != tt.expectedSegs {
				t.Errorf("Expected %d segments, got %d. %s", tt.expectedSegs, len(segments), tt.description)
			}

			// Verify segment properties for non-empty results
			if len(segments) > 0 && tt.fileSize > 0 {
				// Check that segments cover the entire file
				totalCovered := int64(0)
				for i, seg := range segments {
					if seg.Index != i {
						t.Errorf("Segment %d has incorrect index %d", i, seg.Index)
					}
					if seg.Start < 0 || seg.End < seg.Start {
						t.Errorf("Segment %d has invalid range: %d-%d", i, seg.Start, seg.End)
					}
					if seg.Completed {
						t.Errorf("New segment %d should not be marked as completed", i)
					}
					if seg.Retries != 0 {
						t.Errorf("New segment %d should have 0 retries, got %d", i, seg.Retries)
					}
					totalCovered += seg.End - seg.Start + 1
				}
				
				if totalCovered != tt.fileSize {
					t.Errorf("Segments don't cover entire file: covered %d, expected %d", totalCovered, tt.fileSize)
				}

				// Check minimum segment size for multi-threaded downloads
				if len(segments) > 1 {
					for i, seg := range segments[:len(segments)-1] { // All except last
						segSize := seg.End - seg.Start + 1
						if segSize < MinSegmentSize {
							t.Errorf("Segment %d size %d is below minimum %d", i, segSize, MinSegmentSize)
						}
					}
				}
			}
		})
	}
}

func TestDownloadPlanner_PlanDownload(t *testing.T) {
	planner := NewDownloadPlanner()

	tests := []struct {
		name        string
		meta        *internal.FileMetadata
		config      *internal.DownloadConfig
		expectError bool
		description string
	}{
		{
			name: "valid_plan",
			meta: &internal.FileMetadata{
				Filename:  "test.zip",
				Size:      10 * 1024 * 1024, // 10MB
				DirectURL: "https://example.com/test.zip",
			},
			config: &internal.DownloadConfig{
				Threads: 4,
			},
			expectError: false,
			description: "Valid metadata and config should succeed",
		},
		{
			name:        "nil_metadata",
			meta:        nil,
			config:      &internal.DownloadConfig{Threads: 4},
			expectError: true,
			description: "Nil metadata should return error",
		},
		{
			name: "nil_config",
			meta: &internal.FileMetadata{
				Filename: "test.zip",
				Size:     1024,
			},
			config:      nil,
			expectError: true,
			description: "Nil config should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := planner.PlanDownload(tt.meta, tt.config)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none. %s", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v. %s", err, tt.description)
				}
				if segments == nil {
					t.Errorf("Expected segments but got nil. %s", tt.description)
				}
			}
		})
	}
}

func TestDownloadPlanner_ResumeMetadata(t *testing.T) {
	planner := NewDownloadPlanner()
	
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	outputPath := filepath.Join(tempDir, "test.zip")
	
	// Test metadata
	meta := &internal.FileMetadata{
		Filename:  "test.zip",
		Size:      5 * 1024 * 1024, // 5MB
		DirectURL: "https://example.com/test.zip",
		ShareID:   "test123",
		Timestamp: time.Now(),
	}
	
	segments := []internal.SegmentInfo{
		{Index: 0, Start: 0, End: 2621439, Completed: true, Retries: 0},
		{Index: 1, Start: 2621440, End: 5242879, Completed: false, Retries: 1},
	}

	t.Run("save_and_load_metadata", func(t *testing.T) {
		// Save metadata
		err := planner.SaveResumeMetadata(outputPath, meta, segments)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}
		
		// Load metadata
		loadedData, err := planner.LoadResumeMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to load metadata: %v", err)
		}
		
		// Verify loaded data
		if loadedData.FileMetadata.Filename != meta.Filename {
			t.Errorf("Filename mismatch: expected %s, got %s", meta.Filename, loadedData.FileMetadata.Filename)
		}
		if loadedData.FileMetadata.Size != meta.Size {
			t.Errorf("Size mismatch: expected %d, got %d", meta.Size, loadedData.FileMetadata.Size)
		}
		if len(loadedData.Segments) != len(segments) {
			t.Errorf("Segment count mismatch: expected %d, got %d", len(segments), len(loadedData.Segments))
		}
		
		for i, seg := range loadedData.Segments {
			if seg.Index != segments[i].Index {
				t.Errorf("Segment %d index mismatch: expected %d, got %d", i, segments[i].Index, seg.Index)
			}
			if seg.Completed != segments[i].Completed {
				t.Errorf("Segment %d completion mismatch: expected %t, got %t", i, segments[i].Completed, seg.Completed)
			}
		}
	})

	t.Run("update_segment_progress", func(t *testing.T) {
		// Update segment 1 to completed
		err := planner.UpdateSegmentProgress(outputPath, 1, true)
		if err != nil {
			t.Fatalf("Failed to update segment progress: %v", err)
		}
		
		// Load and verify
		loadedData, err := planner.LoadResumeMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to load metadata after update: %v", err)
		}
		
		if !loadedData.Segments[1].Completed {
			t.Errorf("Segment 1 should be marked as completed")
		}
	})

	t.Run("increment_segment_retries", func(t *testing.T) {
		originalRetries := segments[1].Retries
		
		// Increment retries for segment 1
		err := planner.IncrementSegmentRetries(outputPath, 1)
		if err != nil {
			t.Fatalf("Failed to increment segment retries: %v", err)
		}
		
		// Load and verify
		loadedData, err := planner.LoadResumeMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to load metadata after retry increment: %v", err)
		}
		
		expectedRetries := originalRetries + 1
		if loadedData.Segments[1].Retries != expectedRetries {
			t.Errorf("Segment 1 retries: expected %d, got %d", expectedRetries, loadedData.Segments[1].Retries)
		}
	})

	t.Run("is_download_complete", func(t *testing.T) {
		// Test incomplete download
		incompleteSegments := []internal.SegmentInfo{
			{Completed: true},
			{Completed: false},
		}
		if planner.IsDownloadComplete(incompleteSegments) {
			t.Errorf("Should not be complete with incomplete segments")
		}
		
		// Test complete download
		completeSegments := []internal.SegmentInfo{
			{Completed: true},
			{Completed: true},
		}
		if !planner.IsDownloadComplete(completeSegments) {
			t.Errorf("Should be complete with all segments completed")
		}
		
		// Test empty segments
		if planner.IsDownloadComplete([]internal.SegmentInfo{}) {
			t.Errorf("Empty segments should not be considered complete")
		}
	})

	t.Run("cleanup_metadata", func(t *testing.T) {
		// Cleanup metadata
		err := planner.CleanupResumeMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to cleanup metadata: %v", err)
		}
		
		// Verify file is gone
		metadataPath := outputPath + ResumeMetadataExt
		if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
			t.Errorf("Metadata file should be deleted")
		}
	})
}

func TestDownloadPlanner_PlanResumeDownload(t *testing.T) {
	planner := NewDownloadPlanner()
	
	originalMeta := &internal.FileMetadata{
		Filename: "test.zip",
		Size:     1024,
	}
	
	resumeData := &internal.ResumeMetadata{
		FileMetadata: originalMeta,
		Segments: []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 511, Completed: true},
			{Index: 1, Start: 512, End: 1023, Completed: false},
		},
	}
	
	config := &internal.DownloadConfig{
		Threads:    2,
		ResumeData: resumeData,
	}

	t.Run("successful_resume", func(t *testing.T) {
		currentMeta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
		}
		
		segments, err := planner.PlanDownload(currentMeta, config)
		if err != nil {
			t.Fatalf("Failed to plan resume download: %v", err)
		}
		
		if len(segments) != 2 {
			t.Errorf("Expected 2 segments, got %d", len(segments))
		}
		
		if segments[0].Completed != true || segments[1].Completed != false {
			t.Errorf("Segment completion status not preserved")
		}
	})

	t.Run("size_mismatch", func(t *testing.T) {
		currentMeta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     2048, // Different size
		}
		
		_, err := planner.PlanDownload(currentMeta, config)
		if err == nil {
			t.Errorf("Expected error for size mismatch")
		}
	})

	t.Run("filename_mismatch", func(t *testing.T) {
		currentMeta := &internal.FileMetadata{
			Filename: "different.zip", // Different filename
			Size:     1024,
		}
		
		_, err := planner.PlanDownload(currentMeta, config)
		if err == nil {
			t.Errorf("Expected error for filename mismatch")
		}
	})
}

func TestDownloadPlanner_ResumeDetection(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("detect_resumable_download", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		partPath := outputPath + ".part"
		
		// Create metadata and part file
		meta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
			DirectURL: "https://example.com/test.zip",
		}
		segments := []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 511, Completed: true},
			{Index: 1, Start: 512, End: 1023, Completed: false},
		}
		
		err = planner.SaveResumeMetadata(outputPath, meta, segments)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}
		
		// Create part file
		err = os.WriteFile(partPath, make([]byte, 1024), 0644)
		if err != nil {
			t.Fatalf("Failed to create part file: %v", err)
		}
		
		// Test detection
		resumeData, err := planner.DetectResumableDownload(outputPath)
		if err != nil {
			t.Fatalf("Failed to detect resumable download: %v", err)
		}
		
		if resumeData == nil {
			t.Errorf("Expected resumable download to be detected")
		}
		
		if resumeData.FileMetadata.Size != 1024 {
			t.Errorf("Expected file size 1024, got %d", resumeData.FileMetadata.Size)
		}
	})

	t.Run("no_resume_data", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		
		resumeData, err := planner.DetectResumableDownload(outputPath)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		if resumeData != nil {
			t.Errorf("Expected no resume data, got %v", resumeData)
		}
	})

	t.Run("oversized_part_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		partPath := outputPath + ".part"
		
		// Create metadata for 1KB file
		meta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
		}
		segments := []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 1023, Completed: false},
		}
		
		err = planner.SaveResumeMetadata(outputPath, meta, segments)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}
		
		// Create oversized part file (2KB)
		err = os.WriteFile(partPath, make([]byte, 2048), 0644)
		if err != nil {
			t.Fatalf("Failed to create part file: %v", err)
		}
		
		// Should cleanup and return nil
		resumeData, err := planner.DetectResumableDownload(outputPath)
		if err == nil {
			t.Errorf("Expected error for oversized part file")
		}
		
		if resumeData != nil {
			t.Errorf("Expected no resume data for oversized part file")
		}
	})
}

func TestDownloadPlanner_ResumeCompatibility(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("compatible_resume_data", func(t *testing.T) {
		resumeData := &internal.ResumeMetadata{
			FileMetadata: &internal.FileMetadata{
				Filename: "test.zip",
				Size:     1024,
			},
			LastUpdate: time.Now().Add(-1 * time.Hour), // 1 hour ago
		}
		
		currentMeta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
		}
		
		err := planner.ValidateResumeCompatibility(resumeData, currentMeta)
		if err != nil {
			t.Errorf("Expected compatible resume data, got error: %v", err)
		}
	})

	t.Run("size_mismatch", func(t *testing.T) {
		resumeData := &internal.ResumeMetadata{
			FileMetadata: &internal.FileMetadata{
				Filename: "test.zip",
				Size:     1024,
			},
			LastUpdate: time.Now(),
		}
		
		currentMeta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     2048, // Different size
		}
		
		err := planner.ValidateResumeCompatibility(resumeData, currentMeta)
		if err == nil {
			t.Errorf("Expected error for size mismatch")
		}
	})

	t.Run("old_resume_data", func(t *testing.T) {
		resumeData := &internal.ResumeMetadata{
			FileMetadata: &internal.FileMetadata{
				Filename: "test.zip",
				Size:     1024,
			},
			LastUpdate: time.Now().Add(-8 * 24 * time.Hour), // 8 days ago
		}
		
		currentMeta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
		}
		
		err := planner.ValidateResumeCompatibility(resumeData, currentMeta)
		if err == nil {
			t.Errorf("Expected error for old resume data")
		}
	})
}

func TestDownloadPlanner_NetworkRecovery(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("network_interruption_recovery", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		
		// Create initial metadata
		meta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
		}
		segments := []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 511, Completed: true, Retries: 0},
			{Index: 1, Start: 512, End: 1023, Completed: false, Retries: 2},
		}
		
		err = planner.SaveResumeMetadata(outputPath, meta, segments)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}
		
		// Simulate network interruption recovery
		networkErr := fmt.Errorf("connection reset by peer")
		err = planner.RecoverFromNetworkInterruption(outputPath, 1, networkErr)
		if err != nil {
			t.Fatalf("Failed to recover from network interruption: %v", err)
		}
		
		// Verify retry count was incremented
		resumeData, err := planner.LoadResumeMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to load metadata: %v", err)
		}
		
		if resumeData.Segments[1].Retries != 3 {
			t.Errorf("Expected retry count 3, got %d", resumeData.Segments[1].Retries)
		}
	})

	t.Run("max_retries_exceeded", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		
		// Create metadata with segment at max retries
		meta := &internal.FileMetadata{
			Filename: "test.zip",
			Size:     1024,
		}
		segments := []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 1023, Completed: false, Retries: 5}, // At max retries
		}
		
		err = planner.SaveResumeMetadata(outputPath, meta, segments)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}
		
		// Should fail when max retries exceeded
		networkErr := fmt.Errorf("connection timeout")
		err = planner.RecoverFromNetworkInterruption(outputPath, 0, networkErr)
		if err == nil {
			t.Errorf("Expected error when max retries exceeded")
		}
	})
}

func TestDownloadPlanner_ProgressCalculation(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("calculate_resume_progress", func(t *testing.T) {
		segments := []internal.SegmentInfo{
			{Start: 0, End: 511, Completed: true},     // 512 bytes completed
			{Start: 512, End: 1023, Completed: false}, // 512 bytes incomplete
		}
		
		progress := planner.CalculateResumeProgress(segments)
		expected := 50.0 // 50% complete
		
		if progress != expected {
			t.Errorf("Expected progress %.1f%%, got %.1f%%", expected, progress)
		}
	})

	t.Run("all_completed", func(t *testing.T) {
		segments := []internal.SegmentInfo{
			{Start: 0, End: 511, Completed: true},
			{Start: 512, End: 1023, Completed: true},
		}
		
		progress := planner.CalculateResumeProgress(segments)
		expected := 100.0
		
		if progress != expected {
			t.Errorf("Expected progress %.1f%%, got %.1f%%", expected, progress)
		}
	})

	t.Run("none_completed", func(t *testing.T) {
		segments := []internal.SegmentInfo{
			{Start: 0, End: 511, Completed: false},
			{Start: 512, End: 1023, Completed: false},
		}
		
		progress := planner.CalculateResumeProgress(segments)
		expected := 0.0
		
		if progress != expected {
			t.Errorf("Expected progress %.1f%%, got %.1f%%", expected, progress)
		}
	})

	t.Run("empty_segments", func(t *testing.T) {
		progress := planner.CalculateResumeProgress([]internal.SegmentInfo{})
		expected := 0.0
		
		if progress != expected {
			t.Errorf("Expected progress %.1f%% for empty segments, got %.1f%%", expected, progress)
		}
	})
}

func TestDownloadPlanner_IncompleteSegments(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("get_incomplete_segments", func(t *testing.T) {
		segments := []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 511, Completed: true},
			{Index: 1, Start: 512, End: 767, Completed: false},
			{Index: 2, Start: 768, End: 1023, Completed: false},
		}
		
		incomplete := planner.GetIncompleteSegments(segments)
		
		if len(incomplete) != 2 {
			t.Errorf("Expected 2 incomplete segments, got %d", len(incomplete))
		}
		
		if incomplete[0].Index != 1 || incomplete[1].Index != 2 {
			t.Errorf("Incorrect incomplete segments returned")
		}
	})

	t.Run("all_complete", func(t *testing.T) {
		segments := []internal.SegmentInfo{
			{Index: 0, Start: 0, End: 511, Completed: true},
			{Index: 1, Start: 512, End: 1023, Completed: true},
		}
		
		incomplete := planner.GetIncompleteSegments(segments)
		
		if len(incomplete) != 0 {
			t.Errorf("Expected 0 incomplete segments, got %d", len(incomplete))
		}
	})
}

func TestDownloadPlanner_EdgeCases(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("invalid_segment_index", func(t *testing.T) {
		// Create temporary directory and metadata
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		meta := &internal.FileMetadata{Filename: "test.zip", Size: 1024}
		segments := []internal.SegmentInfo{{Index: 0, Start: 0, End: 1023}}
		
		err = planner.SaveResumeMetadata(outputPath, meta, segments)
		if err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}
		
		// Try to update invalid segment index
		err = planner.UpdateSegmentProgress(outputPath, 5, true)
		if err == nil {
			t.Errorf("Expected error for invalid segment index")
		}
		
		err = planner.IncrementSegmentRetries(outputPath, -1)
		if err == nil {
			t.Errorf("Expected error for negative segment index")
		}
	})

	t.Run("missing_metadata_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "nonexistent.zip")
		
		_, err = planner.LoadResumeMetadata(outputPath)
		if err == nil {
			t.Errorf("Expected error for missing metadata file")
		}
		
		err = planner.UpdateSegmentProgress(outputPath, 0, true)
		if err == nil {
			t.Errorf("Expected error for missing metadata file")
		}
	})

	t.Run("corrupted_metadata_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		outputPath := filepath.Join(tempDir, "test.zip")
		metadataPath := outputPath + ResumeMetadataExt
		
		// Write invalid JSON
		err = os.WriteFile(metadataPath, []byte("invalid json"), 0644)
		if err != nil {
			t.Fatalf("Failed to write corrupted metadata: %v", err)
		}
		
		_, err = planner.LoadResumeMetadata(outputPath)
		if err == nil {
			t.Errorf("Expected error for corrupted metadata file")
		}
	})
}

// TestDownloadPlanner_SegmentationEdgeCases tests comprehensive segmentation edge cases
func TestDownloadPlanner_SegmentationEdgeCases(t *testing.T) {
	planner := NewDownloadPlanner()

	tests := []struct {
		name            string
		fileSize        int64
		threadCount     int
		expectedSegs    int
		minSegmentSize  int64
		maxSegmentSize  int64
		description     string
	}{
		{
			name:           "exactly_min_segment_size",
			fileSize:       MinSegmentSize, // 1MB
			threadCount:    1,
			expectedSegs:   1,
			minSegmentSize: MinSegmentSize,
			maxSegmentSize: MinSegmentSize,
			description:    "File exactly at minimum segment size",
		},
		{
			name:           "just_below_min_segment_size",
			fileSize:       MinSegmentSize - 1, // 1MB - 1 byte
			threadCount:    8,
			expectedSegs:   1,
			minSegmentSize: MinSegmentSize - 1,
			maxSegmentSize: MinSegmentSize - 1,
			description:    "File just below minimum segment size should use single thread",
		},
		{
			name:           "exactly_two_min_segments",
			fileSize:       2 * MinSegmentSize, // 2MB
			threadCount:    8,
			expectedSegs:   2,
			minSegmentSize: MinSegmentSize,
			maxSegmentSize: MinSegmentSize,
			description:    "File exactly divisible by min segment size",
		},
		{
			name:           "odd_file_size_division",
			fileSize:       (2 * MinSegmentSize) + 1, // 2MB + 1 byte
			threadCount:    2,
			expectedSegs:   2,
			minSegmentSize: MinSegmentSize,
			maxSegmentSize: MinSegmentSize + 1,
			description:    "Odd file size should distribute extra bytes to last segment",
		},
		{
			name:           "very_large_file",
			fileSize:       1024 * MinSegmentSize, // 1GB
			threadCount:    32,
			expectedSegs:   32,
			minSegmentSize: 32 * MinSegmentSize,
			maxSegmentSize: 32 * MinSegmentSize,
			description:    "Very large file with maximum threads",
		},
		{
			name:           "thread_count_exceeds_possible_segments",
			fileSize:       3 * MinSegmentSize, // 3MB
			threadCount:    10,
			expectedSegs:   3,
			minSegmentSize: MinSegmentSize,
			maxSegmentSize: MinSegmentSize,
			description:    "Thread count should be limited by minimum segment size",
		},
		{
			name:           "single_byte_file",
			fileSize:       1,
			threadCount:    4,
			expectedSegs:   1,
			minSegmentSize: 1,
			maxSegmentSize: 1,
			description:    "Single byte file should use one segment",
		},
		{
			name:           "thread_count_zero",
			fileSize:       10 * MinSegmentSize,
			threadCount:    0,
			expectedSegs:   1,
			minSegmentSize: 10 * MinSegmentSize,
			maxSegmentSize: 10 * MinSegmentSize,
			description:    "Zero thread count should default to 1",
		},
		{
			name:           "thread_count_exceeds_max",
			fileSize:       100 * MinSegmentSize, // 100MB
			threadCount:    50,                   // Exceeds MaxThreads (32)
			expectedSegs:   32,
			minSegmentSize: 3 * MinSegmentSize + MinSegmentSize/8, // ~3.125MB
			maxSegmentSize: 4 * MinSegmentSize,                    // 4MB
			description:    "Thread count should be capped at maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := planner.CalculateSegments(tt.fileSize, tt.threadCount)
			
			if len(segments) != tt.expectedSegs {
				t.Errorf("Expected %d segments, got %d. %s", tt.expectedSegs, len(segments), tt.description)
			}

			if len(segments) == 0 {
				return // No further validation needed for empty segments
			}

			// Verify segment coverage and properties
			var totalCovered int64
			for i, seg := range segments {
				// Check segment index
				if seg.Index != i {
					t.Errorf("Segment %d has incorrect index %d", i, seg.Index)
				}

				// Check segment range validity
				if seg.Start < 0 || seg.End < seg.Start {
					t.Errorf("Segment %d has invalid range: %d-%d", i, seg.Start, seg.End)
				}

				// Check initial state
				if seg.Completed {
					t.Errorf("New segment %d should not be marked as completed", i)
				}
				if seg.Retries != 0 {
					t.Errorf("New segment %d should have 0 retries, got %d", i, seg.Retries)
				}

				// Calculate segment size
				segmentSize := seg.End - seg.Start + 1
				totalCovered += segmentSize

				// Check segment size constraints (except for last segment which may be smaller)
				if i < len(segments)-1 {
					if segmentSize < tt.minSegmentSize {
						t.Errorf("Segment %d size %d is below expected minimum %d", i, segmentSize, tt.minSegmentSize)
					}
					if segmentSize > tt.maxSegmentSize {
						t.Errorf("Segment %d size %d exceeds expected maximum %d", i, segmentSize, tt.maxSegmentSize)
					}
				}

				// Check segment continuity (no gaps or overlaps)
				if i > 0 {
					prevEnd := segments[i-1].End
					if seg.Start != prevEnd+1 {
						t.Errorf("Gap or overlap between segment %d (end: %d) and segment %d (start: %d)", 
							i-1, prevEnd, i, seg.Start)
					}
				}
			}

			// Verify total coverage
			if totalCovered != tt.fileSize {
				t.Errorf("Segments don't cover entire file: covered %d, expected %d", totalCovered, tt.fileSize)
			}

			// Verify first segment starts at 0
			if segments[0].Start != 0 {
				t.Errorf("First segment should start at 0, got %d", segments[0].Start)
			}

			// Verify last segment ends at file size - 1
			lastSeg := segments[len(segments)-1]
			if lastSeg.End != tt.fileSize-1 {
				t.Errorf("Last segment should end at %d, got %d", tt.fileSize-1, lastSeg.End)
			}
		})
	}
}

// TestDownloadPlanner_OptimalThreadCalculation tests thread optimization logic
func TestDownloadPlanner_OptimalThreadCalculation(t *testing.T) {
	planner := NewDownloadPlanner()

	tests := []struct {
		name            string
		fileSize        int64
		requestedThreads int
		expectedThreads int
		description     string
	}{
		{
			name:            "small_file_thread_reduction",
			fileSize:        MinSegmentSize / 2, // 512KB
			requestedThreads: 8,
			expectedThreads: 1,
			description:     "Small files should reduce to single thread",
		},
		{
			name:            "medium_file_partial_reduction",
			fileSize:        3 * MinSegmentSize, // 3MB
			requestedThreads: 8,
			expectedThreads: 3,
			description:     "Medium files should reduce threads to maintain min segment size",
		},
		{
			name:            "large_file_no_reduction",
			fileSize:        100 * MinSegmentSize, // 100MB
			requestedThreads: 8,
			expectedThreads: 8,
			description:     "Large files should use requested threads",
		},
		{
			name:            "negative_threads",
			fileSize:        10 * MinSegmentSize,
			requestedThreads: -5,
			expectedThreads: 1,
			description:     "Negative thread count should default to 1",
		},
		{
			name:            "zero_threads",
			fileSize:        10 * MinSegmentSize,
			requestedThreads: 0,
			expectedThreads: 1,
			description:     "Zero thread count should default to 1",
		},
		{
			name:            "excessive_threads",
			fileSize:        1000 * MinSegmentSize, // 1GB
			requestedThreads: 100,
			expectedThreads: MaxThreads, // 32
			description:     "Excessive thread count should be capped",
		},
		{
			name:            "exactly_max_threads",
			fileSize:        100 * MinSegmentSize,
			requestedThreads: MaxThreads,
			expectedThreads: MaxThreads,
			description:     "Exactly max threads should be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the internal method through CalculateSegments
			segments := planner.CalculateSegments(tt.fileSize, tt.requestedThreads)
			actualThreads := len(segments)
			
			if actualThreads != tt.expectedThreads {
				t.Errorf("Expected %d threads, got %d. %s", tt.expectedThreads, actualThreads, tt.description)
			}
		})
	}
}

// TestDownloadPlanner_SegmentBoundaryConditions tests boundary conditions for segments
func TestDownloadPlanner_SegmentBoundaryConditions(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("segment_boundaries_no_gaps", func(t *testing.T) {
		fileSize := int64(10 * MinSegmentSize) // 10MB
		threadCount := 4
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		// Verify no gaps between segments
		for i := 1; i < len(segments); i++ {
			prevEnd := segments[i-1].End
			currentStart := segments[i].Start
			
			if currentStart != prevEnd+1 {
				t.Errorf("Gap between segments %d and %d: prev ends at %d, current starts at %d", 
					i-1, i, prevEnd, currentStart)
			}
		}
	})

	t.Run("segment_boundaries_no_overlaps", func(t *testing.T) {
		fileSize := int64(7*MinSegmentSize + MinSegmentSize/2) // 7.5MB (odd size)
		threadCount := 3
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		// Verify no overlaps between segments
		for i := 1; i < len(segments); i++ {
			prevEnd := segments[i-1].End
			currentStart := segments[i].Start
			
			if currentStart <= prevEnd {
				t.Errorf("Overlap between segments %d and %d: prev ends at %d, current starts at %d", 
					i-1, i, prevEnd, currentStart)
			}
		}
	})

	t.Run("last_segment_gets_remainder", func(t *testing.T) {
		fileSize := int64(10*MinSegmentSize + 12345) // 10MB + 12345 bytes
		threadCount := 4
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		// Calculate expected segment size (without remainder)
		baseSegmentSize := fileSize / int64(threadCount)
		remainder := fileSize % int64(threadCount)
		
		// Verify last segment includes remainder
		lastSegment := segments[len(segments)-1]
		expectedLastSegmentSize := baseSegmentSize + remainder
		actualLastSegmentSize := lastSegment.End - lastSegment.Start + 1
		
		if actualLastSegmentSize != expectedLastSegmentSize {
			t.Errorf("Last segment size incorrect: expected %d, got %d", 
				expectedLastSegmentSize, actualLastSegmentSize)
		}
	})

	t.Run("single_thread_covers_entire_file", func(t *testing.T) {
		fileSize := int64(MinSegmentSize / 4) // 256KB (small file)
		threadCount := 1
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		if len(segments) != 1 {
			t.Errorf("Expected 1 segment for single thread, got %d", len(segments))
		}
		
		segment := segments[0]
		if segment.Start != 0 {
			t.Errorf("Single segment should start at 0, got %d", segment.Start)
		}
		if segment.End != fileSize-1 {
			t.Errorf("Single segment should end at %d, got %d", fileSize-1, segment.End)
		}
	})
}

// TestDownloadPlanner_SegmentSizeConsistency tests segment size consistency
func TestDownloadPlanner_SegmentSizeConsistency(t *testing.T) {
	planner := NewDownloadPlanner()

	t.Run("uniform_segment_sizes", func(t *testing.T) {
		fileSize := int64(8 * MinSegmentSize) // 8MB (evenly divisible)
		threadCount := 4
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		expectedSegmentSize := fileSize / int64(threadCount)
		
		// All segments should be the same size for evenly divisible files
		for i, seg := range segments {
			actualSize := seg.End - seg.Start + 1
			if actualSize != expectedSegmentSize {
				t.Errorf("Segment %d size %d differs from expected %d", i, actualSize, expectedSegmentSize)
			}
		}
	})

	t.Run("segment_size_variance", func(t *testing.T) {
		fileSize := int64(10*MinSegmentSize + 1234) // 10MB + 1234 bytes (not evenly divisible)
		threadCount := 3
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		baseSegmentSize := fileSize / int64(threadCount)
		
		// First segments should have base size
		for i := 0; i < len(segments)-1; i++ {
			actualSize := segments[i].End - segments[i].Start + 1
			if actualSize != baseSegmentSize {
				t.Errorf("Segment %d size %d differs from expected base size %d", i, actualSize, baseSegmentSize)
			}
		}
		
		// Last segment should have base size + remainder
		lastSegment := segments[len(segments)-1]
		remainder := fileSize % int64(threadCount)
		expectedLastSize := baseSegmentSize + remainder
		actualLastSize := lastSegment.End - lastSegment.Start + 1
		
		if actualLastSize != expectedLastSize {
			t.Errorf("Last segment size %d differs from expected %d", actualLastSize, expectedLastSize)
		}
	})

	t.Run("minimum_segment_size_enforcement", func(t *testing.T) {
		// File size that would create segments smaller than minimum if using requested threads
		fileSize := int64(2*MinSegmentSize + MinSegmentSize/2) // 2.5MB
		threadCount := 8 // Would create segments < 1MB
		
		segments := planner.CalculateSegments(fileSize, threadCount)
		
		// Should reduce to fewer threads to maintain minimum segment size
		expectedThreads := int(fileSize / MinSegmentSize) // Should be 2
		if len(segments) != expectedThreads {
			t.Errorf("Expected %d threads to maintain min segment size, got %d", expectedThreads, len(segments))
		}
		
		// Verify all segments (except possibly the last) meet minimum size
		for i := 0; i < len(segments)-1; i++ {
			segmentSize := segments[i].End - segments[i].Start + 1
			if segmentSize < MinSegmentSize {
				t.Errorf("Segment %d size %d is below minimum %d", i, segmentSize, MinSegmentSize)
			}
		}
	})
}