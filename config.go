package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func loadConfig() *Config {
	skipDirs := getEnv("SKIP_DIRS", "backup_service")
	dirs := strings.Split(skipDirs, ",")
	for i := range dirs {
		dirs[i] = strings.TrimSpace(dirs[i])
	}

	return &Config{
		ScanPath:       getEnv("SCAN_PATH", "/home/sameer"),
		Schedule:       getEnv("SCHEDULE", "0 3 * * *"),
		RetentionDays:  getEnvInt("RETENTION_DAYS", 7),
		RetentionWeeks: getEnvInt("RETENTION_WEEKS", 4),
		SkipDirs:       dirs,
		WebPort:        getEnvInt("WEB_PORT", 8090),
		AuthUser:       getEnv("AUTH_USER", "admin"),
		AuthPass:       getEnv("AUTH_PASS", "changeme"),
	}
}

func getBackupPath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "backups")
	}
	return filepath.Join(filepath.Dir(exe), "backups")
}
