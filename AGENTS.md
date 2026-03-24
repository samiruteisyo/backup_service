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
| `config.go` | Env var config (`SCAN_PATH`, `BACKUP_PATH`, `WEB_PORT`, etc.) |
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

## API Routes

- `POST /api/login`, `POST /api/logout`
- `GET /api/projects`, `GET /api/projects/:name`
- `POST /api/projects/:name/backup`, `DELETE /api/projects/:name/backup`
- `POST /api/projects/:name/restore`
- `POST /api/projects/:name/deploy`, `POST /api/projects/:name/rollback`
- `GET /api/download/:project/:file`, `GET /api/projects/:name/status`

## Conventions

- MUST ask about git commit and push after EVERY file modification
- Config defaults: `SCAN_PATH=/home/sameer`, `BACKUP_PATH=/home/sameer/backups`, `WEB_PORT=8090`
- Backups stored at `BACKUP_PATH/<project>/db_<timestamp>.sql.gz` and `files_<timestamp>.tar.gz`
- Metadata stored alongside as `<timestamp>.json`
- Compose parser handles both list (`- KEY=val`) and map (`KEY: val`) env syntax
- Env var resolution supports `${VAR:-default}` and `${VAR:?required}` syntax
