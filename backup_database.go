package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func backupDatabase(project *Project, backupPath string) *BackupResult {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	extension := "sql.gz"
	if project.Database.Type == DBMongo {
		extension = "archive.gz"
	}
	archivePath := filepath.Join(backupPath, project.Name, fmt.Sprintf("db_%s.%s", timestamp, extension))

	if project.Database == nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "database",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "skipped",
			Message:     "No database detected",
		}
	}

	db := project.Database
	resolvedName := resolveContainerName(db, project.Name)

	if resolvedName == "" {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "database",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "skipped",
			Message:     fmt.Sprintf("Database container '%s' is not running", db.ServiceName),
		}
	}

	db.ContainerName = resolvedName

	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "database",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to create directory: %v", err),
		}
	}

	dumpCmd := getDumpCommand(db)
	fullCmd := fmt.Sprintf("%s | gzip > %s", dumpCmd, archivePath)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "database",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Backup failed: %v\n%s", err, string(output)),
		}
	}

	stat, err := os.Stat(archivePath)
	if err != nil {
		return &BackupResult{
			ServiceName: project.Name,
			Type:        "database",
			FilePath:    archivePath,
			SizeBytes:   0,
			Timestamp:   time.Now(),
			Status:      "error",
			Message:     fmt.Sprintf("Failed to stat backup file: %v", err),
		}
	}

	return &BackupResult{
		ServiceName: project.Name,
		Type:        "database",
		FilePath:    archivePath,
		SizeBytes:   stat.Size(),
		Timestamp:   time.Now(),
		Status:      "success",
		Message:     fmt.Sprintf("%s database '%s' backed up from '%s'", db.Type, db.Credentials.Database, db.ContainerName),
	}
}

func resolveContainerName(db *DatabaseInfo, projectName string) string {
	candidates := []string{
		db.ContainerName,
		fmt.Sprintf("%s-%s-1", projectName, db.ServiceName),
		fmt.Sprintf("%s_%s_1", projectName, db.ServiceName),
		fmt.Sprintf("%s-%s", projectName, db.ServiceName),
		fmt.Sprintf("%s_%s", projectName, db.ServiceName),
	}

	for _, name := range candidates {
		cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", name)
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(output)) == "true" {
			return name
		}
	}

	return ""
}

func getDumpCommand(db *DatabaseInfo) string {
	creds := db.Credentials
	container := db.ContainerName

	switch db.Type {
	case DBPostgres:
		return fmt.Sprintf("docker exec %s pg_dump -U %s %s", container, creds.User, creds.Database)
	case DBMySQL:
		return fmt.Sprintf("docker exec %s mysqldump -u %s -p'%s' %s", container, creds.User, creds.Password, creds.Database)
	case DBMariaDB:
		return fmt.Sprintf("docker exec %s mariadb-dump -u %s -p'%s' %s", container, creds.User, creds.Password, creds.Database)
	case DBMongo:
		return fmt.Sprintf("docker exec %s mongodump --db %s --username %s --password '%s' --archive", container, creds.Database, creds.User, creds.Password)
	default:
		return ""
	}
}
