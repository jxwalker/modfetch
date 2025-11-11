package lockfile

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// LockFile represents an exclusive lock on a file
type LockFile struct {
	path string
	file *os.File
}

// Acquire creates and locks a lockfile at the given path
// Returns error if another process already holds the lock
func Acquire(path string) (*LockFile, error) {
	// Try to create the lock file exclusively
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists, check if process is still running
			return nil, handleExistingLock(path)
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	// Write our PID to the lock file
	pid := os.Getpid()
	if _, err := fmt.Fprintf(f, "%d\n", pid); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to write PID to lock file: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to sync lock file: %w", err)
	}

	return &LockFile{path: path, file: f}, nil
}

// handleExistingLock checks if an existing lock is stale
func handleExistingLock(path string) error {
	// Read the PID from the existing lock file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("lock file exists but cannot be read: %s\nRemove it manually if no other instance is running: rm %s", path, path)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("lock file contains invalid PID: %s\nRemove it manually if corrupted: rm %s", path, path)
	}

	// Check if the process is still running
	if processExists(pid) {
		return fmt.Errorf("modfetch is already running (PID %d)\nClose other instances or remove lock file if stale: %s", pid, path)
	}

	// Process is dead, remove stale lock
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("stale lock file found (PID %d not running) but cannot be removed: %w\nRemove manually: rm %s", pid, err, path)
	}

	// Retry acquisition
	return fmt.Errorf("stale lock detected and removed, please retry")
}

// processExists checks if a process with the given PID is running
func processExists(pid int) bool {
	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send a signal
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}

	// Check if error is "process not found"
	if err == syscall.ESRCH {
		return false
	}

	// Process exists but we don't have permission to signal it
	// Assume it exists
	return true
}

// Release releases the lock and removes the lock file
func (l *LockFile) Release() error {
	if l.file != nil {
		l.file.Close()
	}

	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	return nil
}

// Path returns the path to the lock file
func (l *LockFile) Path() string {
	return l.path
}
