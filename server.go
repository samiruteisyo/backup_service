package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed dist
var distFS embed.FS

func startServer(config *Config) {
	startSessionCleanup()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", handleLogout)
	mux.HandleFunc("/api/projects", handleProjects)
	mux.HandleFunc("/api/projects/", handleProjectDetail)

	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}

	fs := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		if _, err := distFS.Open("dist" + r.URL.Path); err != nil {
			http.ServeFileFS(w, r, distFS, "dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	chain := loggingMiddleware(corsMiddleware(authMiddleware(mux)))

	addr := fmt.Sprintf(":%d", config.WebPort)
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, chain); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
