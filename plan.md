# Backup Service — Implementation Plan

## Overview

A Docker-based backup service that discovers sibling projects in `/home/sameer/`, parses their docker-compose files, and backs up databases and non-git-tracked files.

---

## Phase 1 — Project Scaffolding

- Initialize `package.json` with Bun runtime
- Add dependencies: `yaml`, `node-cron`, `archiver`
- Add dev dependency: `@types/bun`, `@types/node`, `typescript`
- Create `tsconfig.json`
- Create directory structure:
  ```
  src/
  ├── index.ts
  ├── discover.ts
  ├── parser.ts
  ├── backup/
  │   ├── database.ts
  │   └── files.ts
  ├── storage.ts
  ├── config.ts
  └── types.ts
  ```

---

## Phase 2 — Types & Config

### `src/types.ts`
- `DiscoveredService` — project name, compose path, database info, bind mounts
- `DatabaseInfo` — type (postgres/mysql/mongo), container name, credentials
- `BindMount` — source path, container path
- `BackupResult` — service name, backup type (db/files), file path, size, timestamp, status

### `src/config.ts`
- Read from environment variables:
  - `SCAN_PATH` (default: `/source`)
  - `BACKUP_PATH` (default: `/backups`)
  - `SCHEDULE` (default: `0 3 * * *`)
  - `RETENTION_DAYS` (default: `7`)
  - `RETENTION_WEEKS` (default: `4`)
  - `SKIP_DIRS` (default: `backup_service`)
- Export a `Config` interface and `loadConfig()` function

---

## Phase 3 — Discovery

### `src/discover.ts`
- Scan `SCAN_PATH/*` for directories
- Skip directories listed in `SKIP_DIRS`
- For each directory, check for `docker-compose.yml`, `compose.yml`, or `compose.yaml`
- Return list of `DiscoveredService` with project name and compose file path

---

## Phase 4 — Compose Parser

### `src/parser.ts`
- Parse docker-compose YAML using `yaml` library
- For each service in the compose file:
  - **Database detection** — match image name against patterns:
    - `postgres:*` → PostgreSQL
    - `mysql:*` / `mariadb:*` → MySQL/MariaDB
    - `mongo:*` / `mongodb:*` → MongoDB
  - **Extract credentials** from `environment` section:
    - PostgreSQL: `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`
    - MySQL: `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`
    - MongoDB: `MONGO_INITDB_DATABASE`, `MONGO_INITDB_ROOT_USERNAME`, `MONGO_INITDB_ROOT_PASSWORD`
  - **Extract container_name** or generate from project name + service name
  - **Extract bind mounts** from `volumes` section (filter out named volumes)
- Also read `.env` file from project directory to resolve `${VAR}` references in credentials
- Return enriched `DiscoveredService[]` with database info and bind mounts

---

## Phase 5 — File Backup

### `src/backup/files.ts`
- For each discovered service with bind mounts:
  - Determine source directories on host (resolve relative paths from compose file location)
  - If project has `.git/`:
    - Run `git -C <project_dir> ls-files` to get tracked files
    - Exclude tracked files from backup (but always include `docker-compose.yml` / `compose.yml`)
  - Skip directories: `node_modules`, `.git`, `vendor`, `__pycache__`, `.cache`
  - Create `.tar.gz` archive at `BACKUP_PATH/<service_name>/files_<timestamp>.tar.gz`
  - Return `BackupResult`

---

## Phase 6 — Database Backup

### `src/backup/database.ts`
- For each discovered service with a database:
  - **PostgreSQL**: `docker exec <container> pg_dump -U <user> <dbname> | gzip > <backup_path>`
  - **MySQL/MariaDB**: `docker exec <container> mysqldump -u <user> -p<pass> <dbname> | gzip > <backup_path>`
  - **MongoDB**: `docker exec <container> mongodump --db <dbname> --archive --gzip > <backup_path>`
- Check if container is running before attempting backup (skip with warning if not)
- Save to `BACKUP_PATH/<service_name>/db_<timestamp>.sql.gz` (or `.archive.gz` for mongo)
- Return `BackupResult`

---

## Phase 7 — Storage & Rotation

### `src/storage.ts`
- Ensure `BACKUP_PATH/<service_name>/` directories exist
- After successful backup, run rotation:
  - List existing backups per service
  - **Daily retention**: Keep backups from the last `RETENTION_DAYS` days
  - **Weekly retention**: Keep one backup per week for the last `RETENTION_WEEKS` weeks
  - Delete backups that don't match either policy
- Log rotation summary (kept, deleted counts)

---

## Phase 8 — Entry Point & Scheduling

### `src/index.ts`
- Parse CLI args:
  - `--manual` — run backup immediately and exit
  - `--dry-run` — discover and log what would be backed up without actually backing up
  - No flags — start cron scheduler and run continuously
- Main flow:
  1. Load config
  2. Discover services
  3. Parse compose files
  4. For each service:
     a. Run file backup (if bind mounts exist)
     b. Run database backup (if database detected)
  5. Run rotation
  6. Log summary
- Use `node-cron` for scheduling when running as a service
- Use `console.log` for logging (Docker handles log collection)

---

## Phase 9 — Docker Setup

### `Dockerfile`
```dockerfile
FROM oven/bun:1
WORKDIR /app
COPY package.json bun.lockb ./
RUN bun install --frozen-lockfile
COPY src/ ./src/
CMD ["bun", "run", "src/index.ts"]
```

### `docker-compose.yml`
```yaml
services:
  backup:
    build: .
    container_name: backup_service
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /home/sameer:/source:ro
      - /home/sameer/backups:/backups
    environment:
      - SCHEDULE=0 3 * * *
      - RETENTION_DAYS=7
      - RETENTION_WEEKS=4
    networks:
      - caddy
```

---

## Phase 10 — Testing & Verification

- Run `--dry-run` to verify discovery works correctly
- Run `--manual` to verify full backup cycle
- Verify backup contents:
  - `.tar.gz` files contain expected untracked files
  - `.sql.gz` files are valid database dumps
- Verify rotation deletes old backups correctly
- Verify the service runs on schedule via Docker logs

---

## Current Environment

| Directory | Compose | Database | Bind Mounts |
|-----------|---------|----------|-------------|
| `caddy` | Yes | None | None |
| `fudousan` | Yes | None | `./public`, `./includes` |
| `mail_service` | Yes | PostgreSQL 18.3 | `./uploads`, `.`, `.env`, `./database/schema.sql` |
| `teisyophp` | Yes | None | `./public`, `./includes`, `.env` |
