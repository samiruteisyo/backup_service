package main

import (
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
			DBType:   string(project.Database.Type),
			HasBuild: project.HasBuild,
		}

		if project.Database != nil {
			summary.DBType = string(project.Database.Type)
		}

		gitStatus, err := getGitStatus(project.ProjectDir)
		if err == nil {
			summary.GitStatus = gitStatus
		}

		backupDir := filepath.Join(config.BackupPath, project.Name)
		entries, err := os.ReadDir(backupDir)
		if err == nil {
			var lastMod time.Time
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, _ := e.Info()
				if info != nil {
					if info.ModTime().After(lastMod) {
						lastMod = info.ModTime()
					}
					if strings.HasPrefix(e.Name(), "db_") || strings.HasPrefix(e.Name(), "files_") {
						summary.BackupCount++
						summary.TotalSize += info.Size()
					}
				}
			}
			if !lastMod.IsZero() {
				summary.LastBackup = &lastMod
			}
		}

		deployments := loadDeployments(project.Name, config.BackupPath)
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
	backupDir := filepath.Join(config.BackupPath, project.Name)

	type backupEntry struct {
		Type      string    `json:"type"`
		File      string    `json:"file"`
		Size      int64     `json:"size"`
		Timestamp time.Time `json:"timestamp"`
	}

	var backups []backupEntry
	entries, err := os.ReadDir(backupDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "db_") && !strings.HasPrefix(name, "files_") {
				continue
			}
			info, _ := e.Info()
			if info == nil {
				continue
			}

			bType := "database"
			if strings.HasPrefix(name, "files_") {
				bType = "files"
			}

			backups = append(backups, backupEntry{
				Type:      bType,
				File:      name,
				Size:      info.Size(),
				Timestamp: info.ModTime(),
			})
		}
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].Timestamp.After(backups[j].Timestamp)
		})
	}

	deployments := loadDeployments(project.Name, config.BackupPath)

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
		"project":    project,
		"git_status": gitStatus,
		"backups":    backups,
		"deployments": deployments,
		"activity":   activities,
	})
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

	var results []*BackupResult

	if project.Database != nil {
		result := backupDatabase(project, config.BackupPath)
		results = append(results, result)
		logActivity(project.Name, "backup", result.Message, result.Status)
	}

	if len(project.BindMounts) > 0 {
		result := backupFiles(project, config.BackupPath)
		results = append(results, result)
		logActivity(project.Name, "backup", result.Message, result.Status)
	}

	if len(results) == 0 {
		logActivity(project.Name, "backup", "Nothing to backup", "skipped")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "skipped",
			"message": "nothing to backup",
		})
		return
	}

	allSuccess := true
	for _, r := range results {
		if r.Status != "success" && r.Status != "skipped" {
			allSuccess = false
		}
	}

	status := "success"
	if !allSuccess {
		status = "partial"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  status,
		"results": results,
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

	result := restoreProject(project, config.BackupPath, body.Timestamp)

	if result.Success {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusBadRequest, result)
	}
}

func handleDeploy(w http.ResponseWriter, r *http.Request, project *Project, config *Config) {
	result := deployProject(project, config.BackupPath)

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

	result := rollbackProject(project, config.BackupPath, body.SHA)

	if result.Success {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusBadRequest, result)
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	config := loadConfig()

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

	filePath := filepath.Join(config.BackupPath, projectName, fileName)
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
