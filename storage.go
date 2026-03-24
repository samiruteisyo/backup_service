package main

import (
	"os"
	"path/filepath"
	"time"
)

type backupFile struct {
	name    string
	path    string
	mtime   time.Time
	size    int64
}

func rotateBackups(config *Config, serviceName string) *RotationResult {
	serviceDir := filepath.Join(config.BackupPath, serviceName)

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

	weeklyKept := make(map[string]bool)

	for _, b := range backups {
		ageDays := time.Since(b.mtime).Hours() / 24

		if ageDays <= float64(config.RetentionDays) {
			kept++
			continue
		}

		if ageDays <= float64(config.RetentionWeeks*7) {
			weekStart := getStartOfWeek(b.mtime)
			prefix := "db"
			if len(b.name) > 4 && b.name[:4] == "file" {
				prefix = "files"
			}
			weekKey := prefix + "_" + weekStart.Format("2006-01-02")

			if !weeklyKept[weekKey] {
				weeklyKept[weekKey] = true
				kept++
				continue
			}
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

func getStartOfWeek(t time.Time) time.Time {
	d := t
	for d.Weekday() != time.Sunday {
		d = d.AddDate(0, 0, -1)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}
