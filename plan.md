# Frontend React Conversion Plan

## Overview

Convert the vanilla HTML/CSS/JS frontend to ReactJS while keeping Go backend unchanged. The embedded `web/` directory will be replaced with a React SPA built via Vite.

## Current State

- **Location**: `web/` directory with `index.html` and `login.html`
- **Tech**: Pure vanilla HTML, CSS (inline), JavaScript (inline)
- **Auth**: Cookie-based sessions
- **API**: REST endpoints returning JSON

## Target Architecture

```
frontend/
├── index.html
├── package.json
├── vite.config.js
├── src/
│   ├── main.jsx
│   ├── App.jsx
│   ├── index.css
│   ├── api/           # API client functions
│   │   └── client.js
│   ├── components/    # Reusable UI components
│   │   ├── Button.jsx
│   │   ├── Card.jsx
│   │   ├── Modal.jsx
│   │   ├── Toast.jsx
│   │   ├── Tabs.jsx
│   │   └── Badge.jsx
│   ├── pages/         # Route pages
│   │   ├── Login.jsx
│   │   └── Dashboard.jsx
│   └── hooks/         # Custom hooks
│       └── useApi.js
```

## Components to Build

| Component | Purpose |
|-----------|---------|
| `Login` | Login form, session handling |
| `Dashboard` | Main layout with header, stats, project list |
| `ProjectCard` | Expandable card per project with tabs |
| `ProjectTabs` | Backups, Deployments, Activity tabs |
| `BackupList` | Table of backups with restore/delete actions |
| `DeploymentList` | Table of deployments with rollback action |
| `ActivityLog` | Activity feed display |
| `StatCard` | Summary statistics display |
| `Modal` | Confirmation dialogs |
| `Toast` | Success/error notifications |

## State Management

- **React Context** for global auth state (user, login, logout)
- **Local state** (`useState`) for component-specific data
- **Custom hooks** for API calls with loading/error handling

## API Integration

Existing endpoints remain unchanged. API client module wraps `fetch()`:

```js
// Auth
POST /api/login { username, password } → { user }
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

1. Build React app: `npm run build` → `dist/` folder
2. Embed `dist/` into binary via updated `go:embed`
3. Serve from Go same as current (`http.FileServer`)

## Migration Steps

1. **Initialize Vite project** in `frontend/` directory
2. **Create component library** (Button, Card, Modal, Toast, Tabs, Badge)
3. **Build API client** with auth handling
4. **Implement Login page** with session check
5. **Build Dashboard** with stat cards
6. **Build ProjectCard** with expandable sections and tabs
7. **Implement all CRUD operations** (backup, restore, deploy, rollback, delete)
8. **Add styling** matching current dark theme
9. **Test in dev mode** with `vite --port 5173` proxying to Go server
10. **Build and embed** into Go binary
11. **Update `server.go`** to serve React app (index.html fallback for SPA routing)

## Go Server Updates Required

1. Change `go:embed web` to `go:embed dist`
2. Add SPA fallback handler for client-side routing
3. Update `install.sh`/`build.sh` to include React build step

## Post-Conversion Cleanup

- Remove `web/` directory
- Remove inline styles from converted components
- Add `frontend/` to `.gitignore`
- Update README with new dev workflow

## Time Estimate

| Phase | Tasks |
|-------|-------|
| Setup & Components | 1-2 hours |
| Pages & Logic | 2-3 hours |
| Testing & Polish | 1-2 hours |
| Go Integration | 30 min |
| **Total** | 5-8 hours |
