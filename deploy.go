package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DeployResult struct {
	Success    bool      `json:"success"`
	Message    string    `json:"message"`
	OldSHA     string    `json:"old_sha"`
	NewSHA     string    `json:"new_sha"`
	Branch     string    `json:"branch"`
	Timestamp  time.Time `json:"timestamp"`
}

type GitStatus struct {
	Branch string `json:"branch"`
	SHA    string `json:"sha"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
	Clean  bool   `json:"clean"`
}

func deployProject(project *Project, backupPath string) *DeployResult {
	projectDir := project.ProjectDir
	deployDir := filepath.Join(backupPath, project.Name)
	os.MkdirAll(deployDir, 0755)

	logActivity(project.Name, "deploy", "Starting deploy", "running")

	oldSHA, err := runGitCmd(projectDir, "rev-parse", "HEAD")
	if err != nil {
		msg := fmt.Sprintf("Failed to get current SHA: %v", err)
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	branch, err := runGitCmd(projectDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		msg := fmt.Sprintf("Failed to get current branch: %v", err)
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	status, err := getGitStatus(projectDir)
	if err != nil {
		msg := fmt.Sprintf("Failed to check git status: %v", err)
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	if !status.Clean {
		msg := "Working directory has uncommitted changes — aborting deploy"
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	logActivity(project.Name, "deploy", "Running git fetch", "running")
	if out, err := exec.Command("git", "-C", projectDir, "fetch", "origin").CombinedOutput(); err != nil {
		msg := fmt.Sprintf("git fetch failed: %v\n%s", err, string(out))
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	logActivity(project.Name, "deploy", "Running git pull", "running")
	if out, err := exec.Command("git", "-C", projectDir, "pull", "origin", branch).CombinedOutput(); err != nil {
		msg := fmt.Sprintf("git pull failed: %v\n%s", err, string(out))
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	newSHA, err := runGitCmd(projectDir, "rev-parse", "HEAD")
	if err != nil {
		msg := fmt.Sprintf("Failed to get new SHA: %v", err)
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	if strings.TrimSpace(newSHA) == strings.TrimSpace(oldSHA) {
		msg := "Already up to date — nothing to deploy"
		logActivity(project.Name, "deploy", msg, "success")
		return &DeployResult{
			Success:   true,
			Message:   msg,
			OldSHA:    strings.TrimSpace(oldSHA),
			NewSHA:    strings.TrimSpace(newSHA),
			Branch:    strings.TrimSpace(branch),
			Timestamp: time.Now(),
		}
	}

	if project.HasBuild {
		logActivity(project.Name, "deploy", "Running docker compose build", "running")
		cmd := exec.Command("docker", "compose", "-f", project.ComposePath, "build")
		if out, err := cmd.CombinedOutput(); err != nil {
			msg := fmt.Sprintf("docker compose build failed: %v\n%s", err, string(out))
			logActivity(project.Name, "deploy", msg, "error")
			return &DeployResult{Success: false, Message: msg, OldSHA: strings.TrimSpace(oldSHA), Timestamp: time.Now()}
		}
	}

	logActivity(project.Name, "deploy", "Running docker compose up -d", "running")
	cmd := exec.Command("docker", "compose", "-f", project.ComposePath, "up", "-d")
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := fmt.Sprintf("docker compose up failed: %v\n%s", err, string(out))
		logActivity(project.Name, "deploy", msg, "error")
		return &DeployResult{Success: false, Message: msg, OldSHA: strings.TrimSpace(oldSHA), Timestamp: time.Now()}
	}

	shortNew := shortSHA(newSHA)
	shortOld := shortSHA(oldSHA)
	msg := fmt.Sprintf("Deployed %s: %s → %s", project.Name, shortOld, shortNew)
	logActivity(project.Name, "deploy", msg, "success")

	deployment := Deployment{
		SHA:       strings.TrimSpace(newSHA),
		Branch:    strings.TrimSpace(branch),
		Timestamp: time.Now(),
		Status:    "success",
		Message:   msg,
	}
	saveDeployment(project.Name, backupPath, deployment)

	return &DeployResult{
		Success:   true,
		Message:   msg,
		OldSHA:    strings.TrimSpace(oldSHA),
		NewSHA:    strings.TrimSpace(newSHA),
		Branch:    strings.TrimSpace(branch),
		Timestamp: time.Now(),
	}
}

func rollbackProject(project *Project, backupPath, targetSHA string) *DeployResult {
	projectDir := project.ProjectDir
	deployDir := filepath.Join(backupPath, project.Name)
	os.MkdirAll(deployDir, 0755)

	logActivity(project.Name, "rollback", fmt.Sprintf("Starting rollback to %s", shortSHA(targetSHA)), "running")

	oldSHA, err := runGitCmd(projectDir, "rev-parse", "HEAD")
	if err != nil {
		msg := fmt.Sprintf("Failed to get current SHA: %v", err)
		logActivity(project.Name, "rollback", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	branch, err := runGitCmd(projectDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		msg := fmt.Sprintf("Failed to get current branch: %v", err)
		logActivity(project.Name, "rollback", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	if out, err := exec.Command("git", "-C", projectDir, "reset", "--hard", targetSHA).CombinedOutput(); err != nil {
		msg := fmt.Sprintf("git reset --hard failed: %v\n%s", err, string(out))
		logActivity(project.Name, "rollback", msg, "error")
		return &DeployResult{Success: false, Message: msg, Timestamp: time.Now()}
	}

	if project.HasBuild {
		logActivity(project.Name, "rollback", "Rebuilding containers after rollback", "running")
		cmd := exec.Command("docker", "compose", "-f", project.ComposePath, "build")
		if out, err := cmd.CombinedOutput(); err != nil {
			msg := fmt.Sprintf("docker compose build failed: %v\n%s", err, string(out))
			logActivity(project.Name, "rollback", msg, "error")
			return &DeployResult{Success: false, Message: msg, OldSHA: strings.TrimSpace(oldSHA), Timestamp: time.Now()}
		}
	}

	logActivity(project.Name, "rollback", "Restarting containers", "running")
	cmd := exec.Command("docker", "compose", "-f", project.ComposePath, "up", "-d")
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := fmt.Sprintf("docker compose up failed: %v\n%s", err, string(out))
		logActivity(project.Name, "rollback", msg, "error")
		return &DeployResult{Success: false, Message: msg, OldSHA: strings.TrimSpace(oldSHA), Timestamp: time.Now()}
	}

	msg := fmt.Sprintf("Rolled back %s: %s → %s", project.Name, shortSHA(oldSHA), shortSHA(targetSHA))
	logActivity(project.Name, "rollback", msg, "success")

	deployment := Deployment{
		SHA:       strings.TrimSpace(targetSHA),
		Branch:    strings.TrimSpace(branch),
		Timestamp: time.Now(),
		Status:    "rollback",
		Message:   msg,
	}
	saveDeployment(project.Name, backupPath, deployment)

	return &DeployResult{
		Success:   true,
		Message:   msg,
		OldSHA:    strings.TrimSpace(oldSHA),
		NewSHA:    strings.TrimSpace(targetSHA),
		Branch:    strings.TrimSpace(branch),
		Timestamp: time.Now(),
	}
}

func getGitStatus(projectDir string) (*GitStatus, error) {
	sha, err := runGitCmd(projectDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	branch, err := runGitCmd(projectDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}

	aheadBehind, err := runGitCmd(projectDir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return nil, err
	}

	parts := strings.Fields(strings.TrimSpace(aheadBehind))
	var ahead, behind int
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &ahead)
		fmt.Sscanf(parts[1], "%d", &behind)
	}

	statusOut, err := runGitCmd(projectDir, "status", "--porcelain")
	clean := err == nil && strings.TrimSpace(statusOut) == ""

	return &GitStatus{
		Branch: strings.TrimSpace(branch),
		SHA:    strings.TrimSpace(sha),
		Ahead:  ahead,
		Behind: behind,
		Clean:  clean,
	}, nil
}

func loadDeployments(projectName, backupPath string) []Deployment {
	deployPath := filepath.Join(backupPath, projectName, "deployments.json")

	data, err := os.ReadFile(deployPath)
	if err != nil {
		return []Deployment{}
	}

	var deployments []Deployment
	if err := json.Unmarshal(data, &deployments); err != nil {
		return []Deployment{}
	}
	return deployments
}

func saveDeployment(projectName, backupPath string, deployment Deployment) {
	deployments := loadDeployments(projectName, backupPath)
	deployments = append(deployments, deployment)

	deployPath := filepath.Join(backupPath, projectName, "deployments.json")
	os.MkdirAll(filepath.Dir(deployPath), 0755)

	out, _ := json.MarshalIndent(deployments, "", "  ")
	os.WriteFile(deployPath, out, 0644)
}

func runGitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func shortSHA(sha string) string {
	s := strings.TrimSpace(sha)
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
