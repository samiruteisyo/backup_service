package main

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

type backupFile struct {
	name  string
	path  string
	mtime time.Time
	size  int64
}

func rotateBackups(config *Config, serviceName string) *RotationResult {
	serviceDir := filepath.Join(getBackupPath(), serviceName)

	var kept, deleted int

	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return &RotationResult{Service: serviceName}
	}

	var backups []backupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isBackupFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{
			name:  entry.Name(),
			path:  filepath.Join(serviceDir, entry.Name()),
			mtime: info.ModTime(),
			size:  info.Size(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].mtime.After(backups[j].mtime)
	})

	for i, b := range backups {
		if i < config.MaxBackups {
			kept++
			continue
		}
		if err := os.Remove(b.path); err != nil {
			kept++
		} else {
			deleted++
		}
	}

	return &RotationResult{
		Service: serviceName,
		Kept:    kept,
		Deleted: deleted,
	}
}

func rotateAllBackups(config *Config, serviceNames []string) []*RotationResult {
	results := make([]*RotationResult, len(serviceNames))
	for i, name := range serviceNames {
		results[i] = rotateBackups(config, name)
	}
	return results
}
