package system

import (
	"fmt"
	"syscall"
)

// CheckAvailableSpace returns the available disk space in bytes for the given path
func CheckAvailableSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to get disk space for %s: %w", path, err)
	}
	// Available blocks * block size = available bytes
	return stat.Bavail * uint64(stat.Bsize), nil
}

// HasSufficientSpace checks if the path has enough space for the required bytes
// It adds a 10% buffer to account for filesystem overhead and metadata
func HasSufficientSpace(path string, requiredBytes uint64) (bool, uint64, error) {
	available, err := CheckAvailableSpace(path)
	if err != nil {
		return false, 0, err
	}

	// Require 10% buffer for safety
	required := uint64(float64(requiredBytes) * 1.1)

	return available >= required, available, nil
}

// GetDiskUsage returns total, used, and available disk space for a path
func GetDiskUsage(path string) (total, used, available uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get disk usage for %s: %w", path, err)
	}

	total = stat.Blocks * uint64(stat.Bsize)
	available = stat.Bavail * uint64(stat.Bsize)
	used = total - (stat.Bfree * uint64(stat.Bsize))

	return total, used, available, nil
}

// DiskUsagePercent returns the percentage of disk space used (0-100)
func DiskUsagePercent(path string) (float64, error) {
	total, used, _, err := GetDiskUsage(path)
	if err != nil {
		return 0, err
	}

	if total == 0 {
		return 0, nil
	}

	return (float64(used) / float64(total)) * 100, nil
}

// IsLowDiskSpace returns true if disk usage is above the threshold percentage
func IsLowDiskSpace(path string, thresholdPercent float64) (bool, error) {
	usage, err := DiskUsagePercent(path)
	if err != nil {
		return false, err
	}
	return usage >= thresholdPercent, nil
}
