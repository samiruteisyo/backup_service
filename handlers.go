package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	http.ServeFileFS(w, r, webFS, "web/login.html")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := loadConfig()

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if creds.Username != config.AuthUser || hashPassword(creds.Password) != hashPassword(config.AuthPass) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token := createSession()
	setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := getSessionToken(r)
	destroySession(token)
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := loadConfig()
	composePaths := discoverServices(config)
	backupPath := getBackupPath()

	type projectSummary struct {
		Name        string      `json:"name"`
		DBType      string      `json:"db_type"`
		HasBuild    bool        `json:"has_build"`
		GitStatus   *GitStatus  `json:"git_status"`
		LastBackup  *time.Time  `json:"last_backup"`
		LastDeploy  *Deployment `json:"last_deploy"`
		BackupCount int         `json:"backup_count"`
		TotalSize   int64       `json:"total_size"`
	}

	var summaries []projectSummary

	for _, cp := range composePaths {
		project := parseComposeFile(cp)
		if project == nil {
			continue
		}

		summary := projectSummary{
			Name:     project.Name,
			HasBuild: project.HasBuild,
		}

		if project.Database != nil {
			summary.DBType = string(project.Database.Type)
		}

		gitStatus, err := getGitStatus(project.ProjectDir)
		if err == nil {
			summary.GitStatus = gitStatus
		}

		backupDir := filepath.Join(backupPath, project.Name)
		entries, err := os.ReadDir(backupDir)
		if err == nil {
			var lastMod time.Time
			for _, e := range entries {
				if e.IsDir() || !isBackupFile(e.Name()) {
					continue
				}
				info, _ := e.Info()
				if info != nil {
					if info.ModTime().After(lastMod) {
						lastMod = info.ModTime()
					}
					summary.BackupCount++
					summary.TotalSize += info.Size()
				}
			}
			if !lastMod.IsZero() {
				summary.LastBackup = &lastMod
			}
		}

		deployments := loadDeployments(project.Name, backupPath)
		if len(deployments) > 0 {
			summary.LastDeploy = &deployments[len(deployments)-1]
		}

		summaries = append(summaries, summary)
	}

	writeJSON(w, http.StatusOK, summaries)
}

func handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	config := loadConfig()

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/")
	projectName := pathParts[0]

	if projectName == "" {
		writeError(w, http.StatusBadRequest, "missing project name")
		return
	}

	project := findProject(projectName, config)
	if project == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	action := ""
	if len(pathParts) > 1 {
		action = pathParts[1]
	}

	switch r.Method {
	case "GET":
		switch action {
		case "status":
			handleGitStatus(w, r, project)
		case "":
			handleProjectInfo(w, r, project, config)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}

	case "POST":
		switch action {
		case "backup":
			handleBackup(w, r, project, config)
		case "restore":
			handleRestore(w, r, project, config)
		case "deploy":
			handleDeploy(w, r, project, config)
		case "rollback":
			handleRollback(w, r, project, config)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}

	case "DELETE":
		switch action {
		case "backup":
			handleDeleteBackup(w, r, project, config)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func findProject(name string, config *Config) *Project {
	composePaths := discoverServices(config)
	for _, cp := range composePaths {
		p := parseComposeFile(cp)
		if p != nil && p.Name == name {
			return p
		}
	}
	return nil
}

func handleProjectInfo(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	backupPath := getBackupPath()
	backupDir := filepath.Join(backupPath, project.Name)

	type backupEntry struct {
		File      string    `json:"file"`
		Size      int64     `json:"size"`
		Timestamp time.Time `json:"timestamp"`
		SHA       string    `json:"sha"`
		Branch    string    `json:"branch"`
	}

	var backups []backupEntry
	entries, err := os.ReadDir(backupDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !isBackupFile(e.Name()) {
				continue
			}
			info, _ := e.Info()
			if info == nil {
				continue
			}

			name := e.Name()
			ts := extractTimestampFromFilename(name)
			sha, branch := "", ""
			if ts != "" {
				meta := loadBackupMetaFromArchive(backupDir, name)
				if meta != nil {
					sha = shortSHA(meta.SHA)
					branch = meta.Branch
				}
			}

			backups = append(backups, backupEntry{
				File:      name,
				Size:      info.Size(),
				Timestamp: info.ModTime(),
				SHA:       sha,
				Branch:    branch,
			})
		}
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].Timestamp.After(backups[j].Timestamp)
		})
	}

	deployments := loadDeployments(project.Name, backupPath)

	activityPath := filepath.Join(backupDir, "activity.json")
	var activities []Activity
	if data, err := os.ReadFile(activityPath); err == nil {
		json.Unmarshal(data, &activities)
	}
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.After(activities[j].Timestamp)
	})

	gitStatus, _ := getGitStatus(project.ProjectDir)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":     project,
		"git_status":  gitStatus,
		"backups":     backups,
		"deployments": deployments,
		"activity":    activities,
	})
}

func loadBackupMetaFromArchive(backupDir string, archiveName string) *BackupMeta {
	archivePath := filepath.Join(backupDir, archiveName)

	tmpDir, err := os.MkdirTemp("", "meta-read-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	if err := extractMetaFromArchive(archivePath, tmpDir); err != nil {
		return nil
	}

	return loadMetaFromDir(tmpDir)
}

func extractMetaFromArchive(archivePath string, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if header.Name != "meta.json" {
			continue
		}

		outFile, err := os.OpenFile(filepath.Join(destDir, "meta.json"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		buf := make([]byte, 32*1024)
		for {
			n, err := tr.Read(buf)
			if err != nil || n == 0 {
				outFile.Close()
				break
			}
			outFile.Write(buf[:n])
		}
		return nil
	}

	return fmt.Errorf("meta.json not found in archive")
}

func handleGitStatus(w http.ResponseWriter, r *http.Request, project *Project) {
	status, err := getGitStatus(project.ProjectDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get git status")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func handleBackup(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	logActivity(project.Name, "backup", "Manual backup triggered", "running")

	backupPath := getBackupPath()
	result := backupProject(project, backupPath)

	logActivity(project.Name, "backup", result.Message, result.Status)

	status := result.Status
	if status == "skipped" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "skipped",
			"message": result.Message,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  status,
		"results": []interface{}{result},
	})
}

func handleRestore(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	var body struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Timestamp == "" {
		writeError(w, http.StatusBadRequest, "missing timestamp")
		return
	}

	result := restoreProject(project, getBackupPath(), body.Timestamp)

	if result.Success {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusBadRequest, result)
	}
}

func handleDeploy(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	result := deployProject(project, getBackupPath())

	if result.Success {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusBadRequest, result)
	}
}

func handleRollback(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	var body struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.SHA == "" {
		writeError(w, http.StatusBadRequest, "missing sha")
		return
	}

	result := rollbackProject(project, getBackupPath(), body.SHA)

	if result.Success {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusBadRequest, result)
	}
}

func handleDeleteBackup(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	var body struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Timestamp == "" {
		writeError(w, http.StatusBadRequest, "missing timestamp")
		return
	}

	backupPath := getBackupPath()
	backupDir := filepath.Join(backupPath, project.Name)

	backupFile := filepath.Join(backupDir, fmt.Sprintf("backup_%s.tar.gz", body.Timestamp))

	if err := os.Remove(backupFile); err != nil {
		writeError(w, http.StatusNotFound, "no backup file found for that timestamp")
		return
	}

	logActivity(project.Name, "delete", fmt.Sprintf("Deleted backup %s", body.Timestamp), "success")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"deleted": filepath.Base(backupFile),
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	backupPath := getBackupPath()

	pathParts := strings.TrimPrefix(r.URL.Path, "/api/download/")
	parts := strings.SplitN(pathParts, "/", 2)
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid download path")
		return
	}

	projectName := parts[0]
	fileName := parts[1]

	if projectName == "" || fileName == "" {
		writeError(w, http.StatusBadRequest, "missing project or file")
		return
	}

	if strings.Contains(fileName, "..") || strings.Contains(projectName, "..") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	filePath := filepath.Join(backupPath, projectName, fileName)
	if _, err := os.Stat(filePath); err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	if stat != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	}
	io.Copy(w, f)
}
