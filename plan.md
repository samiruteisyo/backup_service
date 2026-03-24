# Backup Service — Implementation Plan

## Overview

Go-based operations panel (like Envoyer) that runs as a single binary on the host. Discovers sibling projects in `/home/sameer/`, parses their docker-compose files, and provides **backup**, **restore**, and **deploy/rollback** functionality via a web UI. Caddy reverse-proxies to it.

---

## Phase 1 — Project Scaffolding ~~DONE~~

- Initialize Go module (`go.mod`) with dependencies:
  - `gopkg.in/yaml.v3` — compose file parsing
  - `github.com/robfig/cron/v3` — scheduling
  - Stdlib for everything else (`archive/tar`, `compress/gzip`, `net/http`, `embed`, `os/exec`, `crypto/sha256`, `crypto/rand`)
- Create directory structure:
  ```
  ├── main.go
  ├── config.go
  ├── types.go
  ├── discover.go
  ├── parser.go
  ├── restore.go
  ├── deploy.go
  ├── storage.go
  ├── server.go
  ├── handlers.go
  ├── middleware.go
  ├── backup/
  │   ├── database.go
  │   └── files.go
  ├── web/
  │   ├── index.html
  │   └── login.html
  └── backup-service.service
  ```
- Remove old TypeScript/Bun files (`package.json`, `tsconfig.json`, `Dockerfile`, `docker-compose.yml`, `src/`, `node_modules/`)

---

## Phase 2 — Types & Config ~~DONE~~

- Define all structs in `types.go`:
  - `Config` — scan path, backup path, schedule, retention, auth credentials, web port
  - `Project` — name, compose path, project dir, database info, bind mounts
  - `DatabaseInfo` — type, container name, service name, credentials
  - `BindMount` — source path, container path
  - `BackupResult` — service name, type, file path, size, timestamp, status
  - `Deployment` — sha, branch, timestamp, status, message
  - `Activity` — type (backup/restore/deploy/rollback), timestamp, message, status
- `config.go` — load from env vars with sensible defaults (`SCAN_PATH=/home/sameer`, `BACKUP_PATH=/home/sameer/backups`, `WEB_PORT=8090`)

---

## Phase 3 — Discovery & Compose Parser ~~DONE~~

- `discover.go` — scan `SCAN_PATH` for directories with compose files (`docker-compose.yml`, `compose.yml`, `compose.yaml`), skip `SKIP_DIRS`
- `parser.go` — parse compose YAML:
  - Detect databases by image name (postgres, mysql, mariadb, mongo)
  - Extract credentials from service environment + project `.env` (resolve `${VAR}` references)
  - Extract bind mounts from volumes section (filter out named volumes, resolve relative paths)
  - Detect which services have `build:` directives (needed for deploy)
  - Resolve container names (explicit or generated `project-service-1`)

---

## Phase 4 — Backup Engine ~~DONE~~

- `backup_database.go` — for each project with a database:
  - Check container is running (skip with warning if not)
  - PostgreSQL: `docker exec <container> pg_dump -U <user> <db> | gzip`
  - MySQL/MariaDB: `docker exec <container> mysqldump/mariadb-dump -u <user> -p'<pass>' <db> | gzip`
  - MongoDB: `docker exec <container> mongodump --db <db> --username <user> --password '<pass>' --archive | gzip`
  - Save to `BACKUP_PATH/<project>/db_<timestamp>.sql.gz` (or `.archive.gz` for mongo)
- `backup_files.go` — for each project with bind mounts:
  - Get git-tracked files via `git ls-files`, exclude them (always keep `.env` and compose file)
  - Skip: `node_modules`, `.git`, `vendor`, `__pycache__`, `.cache`, `.next`, `dist`, `build`
  - Create `.tar.gz` at `BACKUP_PATH/<project>/files_<timestamp>.tar.gz`
- `storage.go` — retention rotation:
  - Keep all backups from last `RETENTION_DAYS` days
  - Keep one per week for last `RETENTION_WEEKS` weeks
  - Delete the rest

---

## Phase 5 — Restore Engine ~~DONE~~

- `restore.go` — restore a project to a specific backup point:
  - Accept backup ID (timestamp) — automatically pairs `db_*` and `files_*` with same timestamp
  - Stop services: `docker compose down` in project dir
  - Restore files: extract `files_*.tar.gz` into project directory
  - Restore database: pipe dump into container via `docker exec`:
    - PostgreSQL: `gzip -dc <dump> | docker exec -i <container> psql -U <user> <db>`
    - MySQL: `gzip -dc <dump> | docker exec -i <container> mysql -u <user> -p'<pass>' <db>`
    - MongoDB: `docker cp <dump> <container>:/tmp && docker exec <container> mongorestore --db <db> --archive=/tmp/dump`
  - Restart services: `docker compose up -d`
  - Log activity entry
  - Auto-backup before restore (safety net)

---

## Phase 6 — Deploy & Rollback Engine ~~DONE~~

- `deploy.go` — deploy latest changes:
  - Record current git SHA (pre-deploy snapshot)
  - `git fetch origin`
  - `git pull`
  - `docker compose build` (only for services with `build:` directives)
  - `docker compose up -d`
  - Log deployment with new SHA, branch, timestamp
  - Save deployment history to `BACKUP_PATH/<project>/deployments.json`
- Rollback:
  - Select target SHA from deployment history
  - `git reset --hard <sha>`
  - `docker compose build` + `docker compose up -d`
  - Log rollback
- Git status endpoint:
  - Current branch, current SHA, commits ahead/behind remote
  - Used by web UI to show "updates available" indicator

---

## Phase 7 — Web Server & Auth ~~DONE~~

- `server.go` — HTTP server on `WEB_PORT`:
  - Cookie-based session auth (SHA256 hashed passwords, random session tokens, 24h TTL)
  - Serve embedded static files (`go:embed web/`)
  - Session cleanup interval
- `middleware.go`:
  - Auth middleware — redirect to `/login` for unauthenticated requests to `/api/*` and non-login pages
  - Request logging
- `handlers.go` — all API route handlers:
  - `POST /api/login` — validate credentials, set session cookie
  - `POST /api/logout` — clear session
  - `GET /api/projects` — list all discovered projects with status summary
  - `GET /api/projects/:name` — project detail (backups list, deployments, activity, git status)
  - `POST /api/projects/:name/backup` — trigger backup for one project
  - `POST /api/projects/:name/restore` — restore from backup point (body: `{timestamp}`)
  - `POST /api/projects/:name/deploy` — deploy latest (body: `{branch?}`)
  - `POST /api/projects/:name/rollback` — rollback to SHA (body: `{sha}`)
  - `GET /api/download/:project/:file` — download backup file
  - `GET /api/projects/:name/status` — git branch, SHA, ahead/behind

---

## Phase 8 — Web UI ~~DONE~~

- `web/login.html` — login page (same style as current, dark theme)
- `web/index.html` — full dashboard:
  - **Top bar**: stats (projects, total backups, total size), "Run All Backups" button, logout
  - **Project cards**: name, DB type badge, last backup time, last deploy time, git branch, pending updates indicator
  - **Expanded project view** (click to expand):
    - **Backups tab**: table with type, file, size, date, download button, restore button
    - **Deployments tab**: history with SHA, branch, timestamp, rollback button
    - **Activity tab**: timeline of all operations
  - **Modals**:
    - Restore confirmation — "Restore [project] to [timestamp]?"
    - Deploy confirmation — shows commits since last deploy
    - Rollback confirmation — "Rollback to [SHA]?"
  - Vanilla HTML/CSS/JS, dark theme, responsive, no build step

---

## Phase 9 — Entry Point & Scheduling ~~DONE~~

- `main.go` — CLI args and orchestration:
  - `--manual` — run backup for all projects and exit
  - `--dry-run` — discover and log what would be backed up
  - `--restore <project> <timestamp>` — restore from CLI
  - No flags — start cron scheduler + web server
- Cron scheduler using `robfig/cron`:
  - Run all backups on `SCHEDULE` (default `0 3 * * *`)
- Metadata persistence:
  - `deployments.json` per project — deployment history
  - `activity.json` per project — operation log
  - JSON file locking for concurrent access

---

## Phase 10 — Systemd & Caddy Integration ~~DONE~~

- `backup-service.service` — systemd unit file:
  - `After=network.target docker.service`
  - `EnvironmentFile=/home/sameer/backup_service/.env`
  - Auto-restart on failure
- Caddy integration skipped — service runs directly on `WEB_PORT` (default 8090)

---

## Phase 11 — Testing & Verification ~~DONE~~

- `go build` — verify compilation
- `--dry-run` — verify project discovery and compose parsing
- `--manual` — verify full backup cycle for all projects
- Verify backup contents (tar.gz has expected files, sql.gz is valid dump)
- Verify rotation deletes old backups correctly
- Test web UI: login, view projects, trigger backup, download backup
- Test restore: restore a project from backup, verify services restart
- Test deploy: git pull + rebuild, verify deployment logged
- Test rollback: revert to previous SHA, verify services restart
- Enable systemd service, verify it starts on boot and runs on schedule

---

## Current Environment

| Directory | Compose | Database | Bind Mounts | Has Build |
|-----------|---------|----------|-------------|-----------|
| `caddy` | Yes | None | None | No |
| `fudousan` | Yes | None | `./public`, `./includes` | Yes (nginx) |
| `mail_service` | Yes | PostgreSQL 18.3 | `./uploads`, `.`, `.env`, `./database/schema.sql` | Yes (web, worker) |
| `teisyophp` | Yes | None | `./public`, `./includes`, `.env` | Yes (nginx, php) |
