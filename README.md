# Onyx

Self-hosted, single-admin file browser and sharing platform.

Go backend, SvelteKit frontend, SQLite storage. Dark mode only. Ships as a single binary or Docker container.

## Quick Start

```bash
docker compose up -d
```

Open `http://localhost:8080`. To use a different port:

```bash
ONYX_PORT=3000 docker compose up -d
```

## Development

**Prerequisites:** Go 1.24+, Node.js 22+

```bash
# Install dependencies
go mod tidy
cd frontend && npm install && cd ..
go install github.com/air-verse/air@latest

# Run backend (hot reload on :8080)
make dev-backend

# Run frontend (Vite on :5173, proxies /api to :8080)
make dev-frontend

# Production build
make build
```
