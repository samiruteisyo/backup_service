# Backup Service

Go-based operations panel (like Envoyer) that runs as a single binary on the host.

## Stack

- Go (no Docker for the service itself), vanilla HTML/CSS/JS
- Cookie-based session auth, embedded web UI via `go:embed`
- Uses `docker exec` for database operations (databases run in containers)
- All code in `package main` (flat structure, no sub-packages)

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | CLI entry point, cron scheduler |
| `config.go` | Env var config (`SCHEDULE`, `WEB_PORT`, `AUTH_*`, etc.) |
| `types.go` | All structs (`Project`, `BackupMeta`, `Config`, etc.) |
| `discover.go` | Scan for docker-compose projects |
| `parser.go` | Parse compose YAML (handles list/map env syntax) |
| `backup_database.go` | DB backups via docker exec (pg_dump, mysqldump, mongodump) |
| `backup_files.go` | File backups as tar.gz |
| `backup_meta.go` | Backup metadata (git SHA, branch, timestamp) |
| `restore.go` | Restore from backup (extract files, restore DB, git checkout) |
| `deploy.go` | Deploy (git pull, compose build/up) & rollback (git reset --hard) |
| `storage.go` | Backup rotation/retention |
| `server.go` | HTTP server, embedded FS, routes |
| `handlers.go` | All API handlers |
| `middleware.go` | Auth middleware, session management |
| `build.sh` | Quick rebuild & restart (day-to-day use) |
| `install.sh` | Full install as systemd service (first-time setup) |

## Commands

```bash
./backup-service              # Start web server + cron scheduler
./backup-service --manual     # Run backups for all projects and exit
./backup-service --dry-run    # Discover projects, log what would be backed up
./backup-service --restore <project> <timestamp>  # Restore from CLI
./build.sh                    # Rebuild binary + restart service
./install.sh                  # Build + install systemd unit (first time)
```

## Service Management (systemd)

```bash
sudo systemctl restart backup-service  # Restart the service
sudo systemctl stop backup-service       # Stop the service
sudo systemctl start backup-service      # Start the service
sudo systemctl status backup-service     # Check status
sudo journalctl -u backup-service -f     # View live logs
sudo journalctl -u backup-service -n 50   # View last 50 log lines
```

## API Routes

- `POST /api/login`, `POST /api/logout`
- `GET /api/projects`, `GET /api/projects/:name`
- `POST /api/projects/:name/backup`, `DELETE /api/projects/:name/backup`
- `POST /api/projects/:name/restore`
- `POST /api/projects/:name/deploy`, `POST /api/projects/:name/rollback`
- `GET /api/download/:project/:file`, `GET /api/projects/:name/status`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WEB_PORT` | `8090` | Port for the web UI |
| `AUTH_USER` | `admin` | Web UI username |
| `AUTH_PASS` | `changeme` | Web UI password |

## Conventions

- MUST ask about git commit and push after EVERY file modification
- Discovery scans sibling directories of the binary, skipping its own directory
- Backup path resolves to `<binary_dir>/backups` (no env var, set by `getBackupPath()` in `config.go:41`)
- Backups stored at `<binary_dir>/backups/<project>/db_<timestamp>.sql.gz` and `files_<timestamp>.tar.gz`
- Max 5 backups retained per project (newest kept, oldest deleted)
- Automatic backups run daily at 3am (`0 3 * * *`)
- Metadata stored alongside as `<timestamp>.json`
- Compose parser handles both list (`- KEY=val`) and map (`KEY: val`) env syntax
- Env var resolution supports `${VAR:-default}` and `${VAR:?required}` syntax
