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
	entries, err := os.ReadDir(config.ScanPath)
	if err != nil {
		return nil
	}

	skipMap := make(map[string]bool, len(config.SkipDirs))
	for _, d := range config.SkipDirs {
		skipMap[d] = true
	}

	var services []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if skipMap[entry.Name()] {
			continue
		}

		fullPath := filepath.Join(config.ScanPath, entry.Name())
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
