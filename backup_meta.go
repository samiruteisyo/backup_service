package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func saveBackupMeta(project *Project, backupPath string, timestamp string) {
	gitStatus, err := getGitStatus(project.ProjectDir)
	if err != nil {
		return
	}

	meta := BackupMeta{
		SHA:       gitStatus.SHA,
		Branch:    gitStatus.Branch,
		Timestamp: timestamp,
	}

	metaPath := filepath.Join(backupPath, project.Name, timestamp+".json")
	os.MkdirAll(filepath.Dir(metaPath), 0755)

	data, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(metaPath, data, 0644)
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

	for _, prefix := range []string{"db_", "files_"} {
		if len(timestamp) > len(prefix) && timestamp[:len(prefix)] == prefix {
			timestamp = timestamp[len(prefix):]
			break
		}
	}

	for _, suffix := range []string{".sql.gz", ".archive.gz", ".tar.gz"} {
		if len(timestamp) > len(suffix) && timestamp[len(timestamp)-len(suffix):] == suffix {
			timestamp = timestamp[:len(timestamp)-len(suffix)]
			break
		}
	}

	return timestamp
}

func isMetaFile(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}

	if info.IsDir() {
		return false
	}

	_, err = time.Parse("2006-01-02T15-04-05Z", info.Name())
	return err == nil
}
