package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func dumpDatabase(project *Project, tmpDir string) (string, error) {
	if project.Database == nil {
		return "", fmt.Errorf("no database detected")
	}

	db := project.Database
	resolvedName := resolveContainerName(db, project.Name)

	if resolvedName == "" {
		return "", fmt.Errorf("database container '%s' is not running", db.ServiceName)
	}

	db.ContainerName = resolvedName

	extension := "sql.gz"
	if db.Type == DBMongo {
		extension = "archive.gz"
	}
	dumpPath := filepath.Join(tmpDir, "db."+extension)

	dumpCmd := getDumpCommand(db)
	fullCmd := fmt.Sprintf("%s | gzip > %s", dumpCmd, dumpPath)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("backup failed: %v\n%s", err, string(output))
	}

	return dumpPath, nil
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
