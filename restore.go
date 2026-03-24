package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RestoreResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Restored  []string  `json:"restored"`
	Timestamp time.Time `json:"timestamp"`
}

func restoreProject(project *Project, backupPath string, timestamp string) *RestoreResult {
	projectBackupDir := filepath.Join(backupPath, project.Name)
	backupFile := filepath.Join(projectBackupDir, fmt.Sprintf("backup_%s.tar.gz", timestamp))

	if !fileExists(backupFile) {
		return &RestoreResult{
			Success:   false,
			Message:   fmt.Sprintf("No backup found for timestamp '%s'", timestamp),
			Timestamp: time.Now(),
		}
	}

	logActivity(project.Name, "restore", fmt.Sprintf("Starting restore to %s", timestamp), "running")

	logActivity(project.Name, "restore", "Auto-backup before restore", "running")
	backupProject(project, backupPath)
	logActivity(project.Name, "restore", "Auto-backup complete", "success")

	tmpDir, err := os.MkdirTemp("", "restore-"+project.Name+"-")
	if err != nil {
		msg := fmt.Sprintf("Failed to create temp dir: %v", err)
		logActivity(project.Name, "restore", msg, "error")
		return &RestoreResult{Success: false, Message: msg, Timestamp: time.Now()}
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarGz(backupFile, tmpDir); err != nil {
		msg := fmt.Sprintf("Failed to extract backup archive: %v", err)
		logActivity(project.Name, "restore", msg, "error")
		return &RestoreResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	meta := loadMetaFromDir(tmpDir)

	composeDir := project.ProjectDir

	cmd := exec.Command("docker", "compose", "-f", project.ComposePath, "down")
	if output, err := cmd.CombinedOutput(); err != nil {
		msg := fmt.Sprintf("Failed to stop services: %v\n%s", err, string(output))
		logActivity(project.Name, "restore", msg, "error")
		return &RestoreResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	var restored []string

	filesArchive := filepath.Join(tmpDir, "files.tar.gz")
	if fileExists(filesArchive) {
		if err := extractTarGz(filesArchive, composeDir); err != nil {
			msg := fmt.Sprintf("Failed to extract files: %v", err)
			logActivity(project.Name, "restore", msg, "error")
			restartServices(project)
			return &RestoreResult{Success: false, Message: msg, Timestamp: time.Now()}
		}
		restored = append(restored, "files")
	}

	if meta != nil && meta.SHA != "" {
		logActivity(project.Name, "restore", fmt.Sprintf("Checking out git commit %s", shortSHA(meta.SHA)), "running")
		if out, err := exec.Command("git", "-C", composeDir, "checkout", meta.SHA).CombinedOutput(); err != nil {
			msg := fmt.Sprintf("git checkout %s failed: %v\n%s", shortSHA(meta.SHA), err, string(out))
			logActivity(project.Name, "restore", msg, "error")
			restartServices(project)
			return &RestoreResult{Success: false, Message: msg, Restored: restored, Timestamp: time.Now()}
		}
		restored = append(restored, fmt.Sprintf("git@%s", shortSHA(meta.SHA)))
	}

	dbFile := ""
	for _, name := range []string{"db.sql.gz", "db.archive.gz"} {
		if fileExists(filepath.Join(tmpDir, name)) {
			dbFile = filepath.Join(tmpDir, name)
			break
		}
	}
	if project.Database != nil && dbFile != "" {
		if err := restoreDatabase(project, dbFile); err != nil {
			msg := fmt.Sprintf("Failed to restore database: %v", err)
			logActivity(project.Name, "restore", msg, "error")
			restartServices(project)
			return &RestoreResult{Success: false, Message: msg, Timestamp: time.Now()}
		}
		restored = append(restored, "database")
	}

	if err := restartServices(project); err != nil {
		msg := fmt.Sprintf("Failed to restart services: %v", err)
		logActivity(project.Name, "restore", msg, "error")
		return &RestoreResult{Success: false, Message: msg, Restored: restored, Timestamp: time.Now()}
	}

	msg := fmt.Sprintf("Restored %s to %s (%s)", project.Name, timestamp, strings.Join(restored, ", "))
	if meta != nil {
		msg = fmt.Sprintf("Restored %s to %s (git %s, %s)", project.Name, timestamp, shortSHA(meta.SHA), strings.Join(restored, ", "))
	}
	logActivity(project.Name, "restore", msg, "success")

	return &RestoreResult{
		Success:   true,
		Message:   msg,
		Restored:  restored,
		Timestamp: time.Now(),
	}
}

func loadMetaFromDir(dir string) *BackupMeta {
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil
	}
	var meta BackupMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err != nil {
			break
		}

		target := filepath.Join(destDir, filepath.FromSlash(header.Name))

		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", filepath.Dir(target), err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := copyFromTar(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			outFile.Close()
		}
	}

	return nil
}

func copyFromTar(dst *os.File, src *tar.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, err := src.Read(buf)
		if err != nil {
			return written, err
		}
		if n == 0 {
			break
		}
		wn, err := dst.Write(buf[:n])
		written += int64(wn)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

func restoreDatabase(project *Project, dumpPath string) error {
	if project.Database == nil {
		return nil
	}

	db := project.Database
	resolvedName := resolveContainerName(db, project.Name)
	if resolvedName == "" {
		return fmt.Errorf("database container '%s' is not running", db.ServiceName)
	}
	db.ContainerName = resolvedName
	creds := db.Credentials

	switch db.Type {
	case DBPostgres:
		cmd := fmt.Sprintf("gzip -dc %s | docker exec -i %s psql -U %s %s", dumpPath, db.ContainerName, creds.User, creds.Database)
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("psql restore failed: %v\n%s", err, string(out))
		}

	case DBMySQL:
		cmd := fmt.Sprintf("gzip -dc %s | docker exec -i %s mysql -u %s -p'%s' %s", dumpPath, db.ContainerName, creds.User, creds.Password, creds.Database)
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mysql restore failed: %v\n%s", err, string(out))
		}

	case DBMariaDB:
		cmd := fmt.Sprintf("gzip -dc %s | docker exec -i %s mariadb -u %s -p'%s' %s", dumpPath, db.ContainerName, creds.User, creds.Password, creds.Database)
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mariadb restore failed: %v\n%s", err, string(out))
		}

	case DBMongo:
		tmpName := filepath.Base(dumpPath)
		cpCmd := exec.Command("docker", "cp", dumpPath, fmt.Sprintf("%s:/tmp/%s", db.ContainerName, tmpName))
		if out, err := cpCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("docker cp failed: %v\n%s", err, string(out))
		}
		restoreCmd := fmt.Sprintf("docker exec %s mongorestore --db %s --username %s --password '%s' --archive=/tmp/%s --drop",
			db.ContainerName, creds.Database, creds.User, creds.Password, tmpName)
		out, err := exec.Command("bash", "-c", restoreCmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mongorestore failed: %v\n%s", err, string(out))
		}

	default:
		return fmt.Errorf("unsupported database type: %s", db.Type)
	}

	return nil
}

func restartServices(project *Project) error {
	cmd := exec.Command("docker", "compose", "-f", project.ComposePath, "up", "-d")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, string(output))
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func logActivity(projectName, opType, message, status string) {
	activityDir := filepath.Join(getBackupPath(), projectName)
	os.MkdirAll(activityDir, 0755)

	activityPath := filepath.Join(activityDir, "activity.json")

	var activities []Activity
	data, err := os.ReadFile(activityPath)
	if err == nil {
		json.Unmarshal(data, &activities)
	}

	activities = append(activities, Activity{
		Type:      opType,
		Timestamp: time.Now(),
		Message:   message,
		Status:    status,
	})

	out, _ := json.MarshalIndent(activities, "", "  ")
	os.WriteFile(activityPath, out, 0644)
}
