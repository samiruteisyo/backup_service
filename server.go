package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

//go:embed web
var webFS embed.FS

func startServer(config *Config) {
	startSessionCleanup()

	mux := http.NewServeMux()

	mux.HandleFunc("/login", handleLoginPage)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", handleLogout)

	mux.HandleFunc("/api/projects", handleProjects)
	mux.HandleFunc("/api/projects/", handleProjectDetail)

	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	chain := loggingMiddleware(corsMiddleware(authMiddleware(mux)))

	addr := fmt.Sprintf(":%d", config.WebPort)
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, chain); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
