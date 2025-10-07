package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileOperations_DetectPartialDownload(t *testing.T) {
	fileOps := NewFileOperations()

	t.Run("existing_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		outputPath := filepath.Join(tempDir, "test.zip")
		partPath := outputPath + ".part"

		// Create a partial file with some content
		testData := make([]byte, 1024)
		err = os.WriteFile(partPath, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create part file: %v", err)
		}

		exists, size, err := fileOps.DetectPartialDownload(outputPath)
		if err != nil {
			t.Fatalf("Failed to detect partial download: %v", err)
		}

		if !exists {
			t.Errorf("Expected partial download to be detected")
		}

		if size != 1024 {
			t.Errorf("Expected size 1024, got %d", size)
		}
	})

	t.Run("no_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		outputPath := filepath.Join(tempDir, "test.zip")

		exists, size, err := fileOps.DetectPartialDownload(outputPath)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if exists {
			t.Errorf("Expected no partial download to be detected")
		}

		if size != 0 {
			t.Errorf("Expected size 0, got %d", size)
		}
	})
}

func TestFileOperations_ValidatePartialFile(t *testing.T) {
	fileOps := NewFileOperations()

	t.Run("valid_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		partPath := filepath.Join(tempDir, "test.zip.part")
		expectedSize := int64(2048)

		// Create a valid partial file
		testData := make([]byte, 1024) // Smaller than expected size
		err = os.WriteFile(partPath, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create part file: %v", err)
		}

		err = fileOps.ValidatePartialFile(partPath, expectedSize)
		if err != nil {
			t.Errorf("Expected valid partial file, got error: %v", err)
		}
	})

	t.Run("oversized_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		partPath := filepath.Join(tempDir, "test.zip.part")
		expectedSize := int64(1024)

		// Create an oversized partial file
		testData := make([]byte, 2048) // Larger than expected size
		err = os.WriteFile(partPath, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create part file: %v", err)
		}

		err = fileOps.ValidatePartialFile(partPath, expectedSize)
		if err == nil {
			t.Errorf("Expected error for oversized partial file")
		}
	})

	t.Run("nonexistent_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		partPath := filepath.Join(tempDir, "nonexistent.zip.part")
		expectedSize := int64(1024)

		err = fileOps.ValidatePartialFile(partPath, expectedSize)
		if err == nil {
			t.Errorf("Expected error for nonexistent partial file")
		}
	})

	t.Run("readonly_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		partPath := filepath.Join(tempDir, "test.zip.part")
		expectedSize := int64(1024)

		// Create a readonly partial file
		testData := make([]byte, 512)
		err = os.WriteFile(partPath, testData, 0444) // Read-only
		if err != nil {
			t.Fatalf("Failed to create part file: %v", err)
		}

		err = fileOps.ValidatePartialFile(partPath, expectedSize)
		if err == nil {
			t.Errorf("Expected error for readonly partial file")
		}
	})
}

func TestFileOperations_CreatePartialFile(t *testing.T) {
	fileOps := NewFileOperations()

	t.Run("create_new_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		partPath := filepath.Join(tempDir, "test.zip.part")
		expectedSize := int64(2048)

		err = fileOps.CreatePartialFile(partPath, expectedSize)
		if err != nil {
			t.Fatalf("Failed to create partial file: %v", err)
		}

		// Verify file was created with correct size
		info, err := os.Stat(partPath)
		if err != nil {
			t.Fatalf("Failed to stat created file: %v", err)
		}

		if info.Size() != expectedSize {
			t.Errorf("Expected file size %d, got %d", expectedSize, info.Size())
		}
	})

	t.Run("truncate_existing_partial_file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		partPath := filepath.Join(tempDir, "test.zip.part")
		expectedSize := int64(1024)

		// Create existing file with different size
		existingData := make([]byte, 2048)
		err = os.WriteFile(partPath, existingData, 0644)
		if err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		err = fileOps.CreatePartialFile(partPath, expectedSize)
		if err != nil {
			t.Fatalf("Failed to truncate partial file: %v", err)
		}

		// Verify file was truncated to correct size
		info, err := os.Stat(partPath)
		if err != nil {
			t.Fatalf("Failed to stat truncated file: %v", err)
		}

		if info.Size() != expectedSize {
			t.Errorf("Expected file size %d after truncation, got %d", expectedSize, info.Size())
		}
	})

	t.Run("create_in_nonexistent_directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Try to create file in nonexistent subdirectory
		partPath := filepath.Join(tempDir, "nonexistent", "test.zip.part")
		expectedSize := int64(1024)

		err = fileOps.CreatePartialFile(partPath, expectedSize)
		if err == nil {
			t.Errorf("Expected error when creating file in nonexistent directory")
		}
	})
}

func TestFileOperations_ExistingMethods(t *testing.T) {
	fileOps := NewFileOperations()

	t.Run("ensure_dir", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testPath := filepath.Join(tempDir, "subdir", "test.txt")

		err = fileOps.EnsureDir(testPath)
		if err != nil {
			t.Fatalf("Failed to ensure directory: %v", err)
		}

		// Verify directory was created
		dirPath := filepath.Dir(testPath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Directory was not created: %s", dirPath)
		}
	})

	t.Run("file_exists", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testPath := filepath.Join(tempDir, "test.txt")

		// File should not exist initially
		if fileOps.FileExists(testPath) {
			t.Errorf("File should not exist initially")
		}

		// Create file
		err = os.WriteFile(testPath, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// File should exist now
		if !fileOps.FileExists(testPath) {
			t.Errorf("File should exist after creation")
		}
	})

	t.Run("get_file_size", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testPath := filepath.Join(tempDir, "test.txt")
		testData := make([]byte, 1024)

		err = os.WriteFile(testPath, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		size, err := fileOps.GetFileSize(testPath)
		if err != nil {
			t.Fatalf("Failed to get file size: %v", err)
		}

		if size != 1024 {
			t.Errorf("Expected file size 1024, got %d", size)
		}
	})

	t.Run("atomic_rename", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "terafetch_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		oldPath := filepath.Join(tempDir, "old.txt")
		newPath := filepath.Join(tempDir, "new.txt")
		testData := []byte("test content")

		err = os.WriteFile(oldPath, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		err = fileOps.AtomicRename(oldPath, newPath)
		if err != nil {
			t.Fatalf("Failed to rename file: %v", err)
		}

		// Verify old file is gone
		if fileOps.FileExists(oldPath) {
			t.Errorf("Old file should not exist after rename")
		}

		// Verify new file exists with correct content
		if !fileOps.FileExists(newPath) {
			t.Errorf("New file should exist after rename")
		}

		content, err := os.ReadFile(newPath)
		if err != nil {
			t.Fatalf("Failed to read renamed file: %v", err)
		}

		if string(content) != string(testData) {
			t.Errorf("File content mismatch after rename")
		}
	})
}