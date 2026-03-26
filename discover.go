package main

import (
	"os"
	"path/filepath"
)

var composeFiles = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

func discoverServices(config *Config) []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	scanDir := filepath.Dir(exe)
	parentDir := filepath.Dir(scanDir)
	selfDir := filepath.Base(scanDir)

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil
	}

	var services []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == selfDir {
			continue
		}

		fullPath := filepath.Join(parentDir, entry.Name())
		for _, cf := range composeFiles {
			composePath := filepath.Join(fullPath, cf)
			if _, err := os.Stat(composePath); err == nil {
				services = append(services, composePath)
				break
			}
		}
	}

	return services
}
