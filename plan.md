# Frontend React Conversion - COMPLETED

## Overview

Converted the vanilla HTML/CSS/JS frontend to ReactJS. The Go backend remains unchanged.

## Completed Architecture

```
frontend/
├── index.html
├── package.json
├── vite.config.js
├── src/
│   ├── main.jsx
│   ├── App.jsx
│   ├── index.css
│   ├── api/
│   │   └── client.js
│   ├── components/
│   │   ├── Button.jsx / Button.css
│   │   ├── Card.jsx / Card.css
│   │   ├── Modal.jsx / Modal.css
│   │   ├── Toast.jsx / Toast.css
│   │   ├── Tabs.jsx / Tabs.css
│   │   ├── Badge.jsx / Badge.css
│   │   └── index.js
│   ├── pages/
│   │   ├── Login.jsx / Login.css
│   │   ├── Dashboard.jsx / Dashboard.css
│   │   └── index.js
│   └── hooks/
│       ├── useApi.js
│       ├── useAuth.jsx
│       └── index.js
```

## Components Built

| Component | Purpose |
|-----------|---------|
| `Login` | Login form, session handling |
| `Dashboard` | Main layout with header, stats, project list |
| `ProjectCard` | Expandable card per project with tabs |
| `BackupList` | Table of backups with restore/delete actions |
| `DeploymentList` | Table of deployments with rollback action |
| `ActivityLog` | Activity feed display |
| `StatCard` | Summary statistics display |
| `Modal` | Confirmation dialogs |
| `Toast` | Success/error notifications |
| `Button` | Variants: primary/secondary/danger/success/ghost |
| `Card` | Card, CardHeader, CardBody, CardFooter |
| `Tabs` | Tab switching with content panels |
| `Badge` | Status badges |

## State Management

- **React Context** for global auth state (`useAuth` hook)
- **Local state** (`useState`) for component-specific data
- **Custom hooks** for API calls with loading/error handling

## API Integration

Existing endpoints unchanged. API client in `frontend/src/api/client.js`:

```js
// Auth
POST /api/login { username, password }
POST /api/logout

// Projects
GET  /api/projects → Project[]
GET  /api/projects/:name → ProjectDetail
POST /api/projects/:name/backup
DELETE /api/projects/:name/backup/:timestamp
POST /api/projects/:name/restore { timestamp }
POST /api/projects/:name/deploy
POST /api/projects/:name/rollback { sha }
```

## Build & Embed Process

1. `npm run build` in `frontend/` → `dist/` folder
2. Build script copies `dist/` to project root
3. Go embeds `dist/` via `go:embed dist`
4. Binary serves React SPA with fallback to `index.html`

## Commands

```bash
./build.sh              # Build React + Go binary
./backup-service        # Run combined app

# Development
cd frontend && npm run dev   # React dev server (proxies /api to localhost:8090)
```

## Changes Made

| File | Change |
|------|--------|
| `server.go` | Embed `dist/`, SPA fallback handler |
| `handlers.go` | Removed `handleLoginPage` |
| `build.sh` | Added React build step |
| `.gitignore` | Added `dist/` |
| `web/` | **Deleted** (replaced by React) |

## Commits

- `62811f0` Phase 1: Initialize React frontend with Vite and component library
- `db76abf` Phase 2: Integrate React SPA with Go backend
