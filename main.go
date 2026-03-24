package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/robfig/cron/v3"
)

func main() {
	manual := flag.Bool("manual", false, "run backup for all projects and exit")
	dryRun := flag.Bool("dry-run", false, "discover projects and log what would be backed up")
	restoreProject := flag.String("restore", "", "restore a project (format: <project>:<timestamp>)")
	flag.Parse()

	config := loadConfig()

	if *restoreProject != "" {
		handleRestoreCLI(config, *restoreProject)
		return
	}

	if *dryRun {
		handleDryRun(config)
		return
	}

	if *manual {
		handleManualBackup(config)
		return
	}

	log.Printf("Backup service starting (port %d, schedule %s)", config.WebPort, config.Schedule)

	c := cron.New()
	_, err := c.AddFunc(config.Schedule, func() {
		log.Println("Scheduled backup starting...")
		runAllBackups(config)
	})
	if err != nil {
		log.Fatalf("Invalid cron schedule %q: %v", config.Schedule, err)
	}
	c.Start()

	startServer(config)
}

func handleRestoreCLI(config *Config, arg string) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fmt.Fprintf(os.Stderr, "Usage: --restore <project>:<timestamp>\n")
		os.Exit(1)
	}

	projectName := parts[0]
	timestamp := parts[1]

	project := findProject(projectName, config)
	if project == nil {
		fmt.Fprintf(os.Stderr, "Project '%s' not found\n", projectName)
		os.Exit(1)
	}

	fmt.Printf("Restoring %s to %s...\n", projectName, timestamp)
	result := restoreProject(project, getBackupPath(), timestamp)
	if result.Success {
		fmt.Printf("Success: %s\n", result.Message)
	} else {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", result.Message)
		os.Exit(1)
	}
}

func handleDryRun(config *Config) {
	composePaths := discoverServices(config)
	if len(composePaths) == 0 {
		fmt.Println("No projects found.")
		return
	}

	fmt.Printf("Discovered %d projects in %s:\n\n", len(composePaths), config.ScanPath)

	for _, cp := range composePaths {
		project := parseComposeFile(cp)
		if project == nil {
			continue
		}

		fmt.Printf("  %s\n", project.Name)
		fmt.Printf("    Compose: %s\n", project.ComposePath)

		if project.Database != nil {
			fmt.Printf("    Database: %s (%s)\n", project.Database.Type, project.Database.Credentials.Database)
		}

		if len(project.BindMounts) > 0 {
			fmt.Printf("    Mounts: %d\n", len(project.BindMounts))
			for _, m := range project.BindMounts {
				fmt.Printf("      %s -> %s\n", m.Source, m.ContainerPath)
			}
		}

		fmt.Printf("    Has build: %v\n", project.HasBuild)

		gitStatus, err := getGitStatus(project.ProjectDir)
		if err == nil {
			fmt.Printf("    Git: %s@%s (ahead %d, behind %d)\n", gitStatus.Branch, shortSHA(gitStatus.SHA), gitStatus.Ahead, gitStatus.Behind)
		}

		fmt.Println()
	}
}

func handleManualBackup(config *Config) {
	log.Println("Manual backup starting...")
	runAllBackups(config)
	log.Println("Manual backup complete. Exiting.")
}

func runAllBackups(config *Config) {
	composePaths := discoverServices(config)
	if len(composePaths) == 0 {
		log.Println("No projects found.")
		return
	}

	backupPath := getBackupPath()

	for _, cp := range composePaths {
		project := parseComposeFile(cp)
		if project == nil {
			continue
		}

		log.Printf("Backing up %s...", project.Name)

		result := backupProject(project, backupPath)
		log.Printf("  [%s] %s", result.Status, result.Message)

		rotateBackups(config, project.Name)
	}
}
