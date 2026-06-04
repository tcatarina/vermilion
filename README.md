# Vermilion

Vermilion is a self-hosted Kanban board for [Redmine](https://www.redmine.org/). It connects to your existing Redmine instance via API key and presents issues as draggable cards across status columns. Board data is cached in Postgres; Redmine remains the source of truth.

## Stack

- **Backend** — Go with `chi` router, `pgx/v5`, board cache in PostgreSQL
- **Frontend** — Vue 3, TypeScript, Vite, Pinia, Tailwind CSS v4, Headless UI
- **Database** — PostgreSQL 16
- **Auth** — Redmine API key passed per-request (nothing stored server-side)
- **Dev** — Docker Compose with hot reload (`air` for Go, Vite HMR for Vue)

## Features

- Kanban board with drag-and-drop status changes
- Filter by assignee and version
- Saved presets (named filters: project + assignees + versions + visible columns)
- Issue detail modal with comments
- Board cache with force-refresh option
- AMOLED dark theme

## Getting started

The only host requirement is Docker with the Compose plugin.

```sh
git clone https://github.com/tcatarina/vermilion.git
cd vermilion

# Copy env defaults (edit if needed)
cp .env.example .env

# Start all services
docker compose up --build -d
```

Open **http://localhost:5173**, go to Settings (⚙), and enter your Redmine URL and API key.

## Services

| Service            | URL                   | Description              |
|--------------------|-----------------------|--------------------------|
| Frontend           | http://localhost:5173 | Vue SPA (Vite dev server)|
| Backend            | http://localhost:20030| Go JSON API              |
| Vermilion Postgres | localhost:5433        | App database             |

## Development

```sh
# Backend tests
cd backend && go test ./...

# Hot reload is automatic inside Docker:
# - Backend: air watches Go files and rebuilds on save
# - Frontend: Vite HMR updates the browser on save
```

## Repository layout

```
backend/      Go service (chi, pgx, redmine client)
frontend/     Vue 3 + TypeScript SPA
deploy/       Deployment notes
LICENSE       Apache License 2.0
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
