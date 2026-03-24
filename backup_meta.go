package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func writeBackupMeta(project *Project, tmpDir string, timestamp string) error {
	gitStatus, err := getGitStatus(project.ProjectDir)
	if err != nil {
		gitStatus = nil
	}

	meta := BackupMeta{
		Timestamp: timestamp,
	}
	if gitStatus != nil {
		meta.SHA = gitStatus.SHA
		meta.Branch = gitStatus.Branch
	}

	metaPath := filepath.Join(tmpDir, "meta.json")

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

func loadBackupMeta(backupPath string, projectName string, timestamp string) *BackupMeta {
	metaPath := filepath.Join(backupPath, projectName, timestamp+".json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil
	}

	var meta BackupMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}

	return &meta
}

func extractTimestampFromFilename(filename string) string {
	timestamp := filename

	if len(timestamp) > 7 && timestamp[:7] == "backup_" {
		timestamp = timestamp[7:]
	}

	if len(timestamp) > 7 && timestamp[len(timestamp)-7:] == ".tar.gz" {
		timestamp = timestamp[:len(timestamp)-7]
	}

	return timestamp
}

func isBackupFile(filename string) bool {
	return len(filename) > 7 && filename[:7] == "backup_" &&
		len(filename) > 7 && filename[len(filename)-7:] == ".tar.gz"
}
