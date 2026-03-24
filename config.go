package main

import (
	"os"
	"path/filepath"
	"strconv"
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
	return &Config{
		WebPort:    getEnvInt("WEB_PORT", 8090),
		AuthUser:   getEnv("AUTH_USER", "admin"),
		AuthPass:   getEnv("AUTH_PASS", "changeme"),
		MaxBackups: 5,
		Schedule:   "0 3 * * *",
	}
}

func getBackupPath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "backups")
	}
	return filepath.Join(filepath.Dir(exe), "backups")
}
