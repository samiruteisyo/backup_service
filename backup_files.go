package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"__pycache__":  true,
	".cache":       true,
	".next":        true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	".nyc_output":  true,
}

func backupFiles(project *Project, backupPath string) *BackupResult {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	archivePath := filepath.Join(backupPath, project.Name, fmt.Sprintf("files_%s.tar.gz", timestamp))

	saveBackupMeta(project, backupPath, timestamp)

	if len(project.BindMounts) == 0 {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "files",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "skipped",
			Message:     "No bind mounts found",
		}
	}

	projectDir := project.ProjectDir
	trackedFiles := getGitTrackedFiles(projectDir)

	allFiles := make(map[string]string)
	composeFilePath := filepath.Join(projectDir, project.ComposeFile)

	if info, err := os.Stat(composeFilePath); err == nil && !info.IsDir() {
		rel := project.ComposeFile
		if shouldIncludeFile(rel, trackedFiles) {
			allFiles[rel] = composeFilePath
		}
	}

	for _, mount := range project.BindMounts {
		if !strings.HasPrefix(mount.Source, projectDir) {
			continue
		}

		if info, err := os.Stat(mount.Source); err != nil {
			continue
		} else if !info.IsDir() {
			rel, _ := filepath.Rel(projectDir, mount.Source)
			if shouldIncludeFile(rel, trackedFiles) {
				allFiles[rel] = mount.Source
			}
			continue
		}

		collectFilesRecursive(mount.Source, projectDir, trackedFiles, allFiles)
	}

	if len(allFiles) == 0 {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "files",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "skipped",
			Message:     "No files to backup (all tracked by git)",
		}
	}

	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "files",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to create directory: %v", err),
		}
	}

	sizeBytes, err := createTarGz(archivePath, allFiles)
	if err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "files",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to create archive: %v", err),
		}
	}

	return &BackupResult{
		ServiceName: project.Name,
		Type:        "files",
		FilePath:    archivePath,
		SizeBytes:   sizeBytes,
		Timestamp:   time.Now(),
		Status:      "success",
		Message:     fmt.Sprintf("%d files backed up", len(allFiles)),
	}
}

func getGitTrackedFiles(projectDir string) map[string]bool {
	cmd := exec.Command("git", "-C", projectDir, "ls-files")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	tracked := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			tracked[line] = true
		}
	}
	return tracked
}

func shouldIncludeFile(relPath string, trackedFiles map[string]bool) bool {
	if relPath == "" || relPath == "." {
		return false
	}

	parts := strings.Split(relPath, "/")
	for _, part := range parts {
		if skipDirs[part] {
			return false
		}
	}

	if trackedFiles != nil && trackedFiles[relPath] {
		if strings.HasSuffix(relPath, ".env") {
			return true
		}
		return false
	}

	return true
}

func collectFilesRecursive(dir, projectDir string, trackedFiles map[string]bool, allFiles map[string]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if skipDirs[entry.Name()] {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			collectFilesRecursive(fullPath, projectDir, trackedFiles, allFiles)
			continue
		}

		rel, err := filepath.Rel(projectDir, fullPath)
		if err != nil {
			continue
		}

		if shouldIncludeFile(rel, trackedFiles) {
			if _, exists := allFiles[rel]; !exists {
				allFiles[rel] = fullPath
			}
		}
	}
}

func createTarGz(archivePath string, files map[string]string) (int64, error) {
	f, err := os.Create(archivePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for relPath, absPath := range files {
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			continue
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			continue
		}

		if !info.Mode().IsRegular() {
			continue
		}

		file, err := os.Open(absPath)
		if err != nil {
			continue
		}

		if _, err := copyN(tw, file, info.Size()); err != nil {
			file.Close()
			continue
		}
		file.Close()
	}

	if err := gw.Close(); err != nil {
		return 0, err
	}

	stat, err := os.Stat(archivePath)
	if err != nil {
		return 0, err
	}

	return stat.Size(), nil
}

func copyN(dst *tar.Writer, src *os.File, size int64) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	remaining := size
	for remaining > 0 {
		if int64(len(buf)) > remaining {
			buf = buf[:remaining]
		}
		n, err := src.Read(buf)
		if err != nil {
			return written, err
		}
		if n == 0 {
			break
		}
		wn, err := dst.Write(buf[:n])
		written += int64(wn)
		remaining -= int64(n)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
