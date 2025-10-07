package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileOperations provides file system utilities
type FileOperations struct{}

// NewFileOperations creates a new FileOperations instance
func NewFileOperations() *FileOperations {
	return &FileOperations{}
}

// EnsureDir creates directory if it doesn't exist
func (f *FileOperations) EnsureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// FileExists checks if a file exists
func (f *FileOperations) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileSize returns the size of a file
func (f *FileOperations) GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// AtomicRename performs an atomic file rename operation
func (f *FileOperations) AtomicRename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// DetectPartialDownload checks if a partial download exists and returns its size
func (f *FileOperations) DetectPartialDownload(outputPath string) (bool, int64, error) {
	partPath := outputPath + ".part"
	
	info, err := os.Stat(partPath)
	if os.IsNotExist(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	
	return true, info.Size(), nil
}

// ValidatePartialFile checks if a partial file is valid for resuming
func (f *FileOperations) ValidatePartialFile(partPath string, expectedSize int64) error {
	info, err := os.Stat(partPath)
	if err != nil {
		return err
	}
	
	// Check if partial file size is reasonable
	if info.Size() > expectedSize {
		return fmt.Errorf("partial file size (%d) exceeds expected size (%d)", info.Size(), expectedSize)
	}
	
	// Check if file is readable and writable
	file, err := os.OpenFile(partPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("cannot access partial file: %w", err)
	}
	file.Close()
	
	return nil
}

// CreatePartialFile creates or truncates a partial download file
func (f *FileOperations) CreatePartialFile(partPath string, size int64) (err error) {
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create partial file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	
	// Pre-allocate file space
	if err := file.Truncate(size); err != nil {
		return fmt.Errorf("failed to allocate file space: %w", err)
	}
	
	return nil
}