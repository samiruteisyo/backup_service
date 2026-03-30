package main

import (
	"fmt"
	"log"
	"net/http"

	"embed"
)

//go:embed dist
var frontendFiles embed.FS

func startServer(config *Config) {
	startSessionCleanup()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", handleLogout)
	mux.HandleFunc("/api/projects", handleProjects)
	mux.HandleFunc("/api/projects/", handleProjectDetail)

	mux.Handle("/", http.FileServer(http.FS(frontendFiles)))

	chain := loggingMiddleware(corsMiddleware(authMiddleware(mux)))

	addr := fmt.Sprintf(":%d", config.WebPort)
	log.Printf("Starting API server on %s", addr)
	if err := http.ListenAndServe(addr, chain); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
