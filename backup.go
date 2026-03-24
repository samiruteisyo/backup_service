package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func backupProject(project *Project, backupPath string) *BackupResult {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	archivePath := filepath.Join(backupPath, project.Name, fmt.Sprintf("backup_%s.tar.gz", timestamp))

	if project.Database == nil && len(project.BindMounts) == 0 {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "full",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "skipped",
			Message:     "Nothing to backup",
		}
	}

	tmpDir, err := os.MkdirTemp("", "backup-"+project.Name+"-")
	if err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "full",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to create temp dir: %v", err),
		}
	}
	defer os.RemoveAll(tmpDir)

	writeBackupMeta(project, tmpDir, timestamp)

	var parts []string
	var totalSize int64

	if project.Database != nil {
		dbPath, err := dumpDatabase(project, tmpDir)
		if err != nil {
			return &BackupResult{
				ServiceName: project.Name,
				Type:        "full",
				FilePath:    archivePath,
				SizeBytes:   0,
				Timestamp:   time.Now(),
				Status:      "error",
				Message:     fmt.Sprintf("Database backup failed: %v", err),
			}
		}
		if stat, err := os.Stat(dbPath); err == nil {
			totalSize += stat.Size()
			parts = append(parts, "database")
		}
	}

	if len(project.BindMounts) > 0 {
		_, size, err := createFilesArchive(project, tmpDir)
		if err == nil {
			totalSize += size
			parts = append(parts, "files")
		}
	}

	if len(parts) == 0 {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "full",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "skipped",
			Message:     "Nothing to backup",
		}
	}

	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "full",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to create directory: %v", err),
		}
	}

	if err := bundleBackupArchive(archivePath, tmpDir); err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "full",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to create archive: %v", err),
		}
	}

	stat, err := os.Stat(archivePath)
	if err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "full",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to stat archive: %v", err),
		}
	}

	return &BackupResult{
		ServiceName: project.Name,
		Type:        "full",
		FilePath:    archivePath,
		SizeBytes:   stat.Size(),
		Timestamp:   time.Now(),
		Status:      "success",
		Message:     fmt.Sprintf("Backup created (%s)", strings.Join(parts, " + ")),
	}
}

func bundleBackupArchive(archivePath string, tmpDir string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			continue
		}
		header.Name = entry.Name()

		if err := tw.WriteHeader(header); err != nil {
			continue
		}

		if !info.Mode().IsRegular() {
			continue
		}

		file, err := os.Open(filepath.Join(tmpDir, entry.Name()))
		if err != nil {
			continue
		}

		buf := make([]byte, 32*1024)
		for {
			n, err := file.Read(buf)
			if err != nil {
				file.Close()
				break
			}
			if n == 0 {
				file.Close()
				break
			}
			if _, err := tw.Write(buf[:n]); err != nil {
				file.Close()
				break
			}
		}
	}

	return nil
}
