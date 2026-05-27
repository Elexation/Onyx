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

## Hardware video acceleration

Onyx transcodes non-browser-native video on demand into an HLS ABR ladder
(2160p/1440p/1080p/720p/480p, capped by source height). By default it
probes the host for hardware encoders on startup and uses the first one
that works; when none are available it falls back to libx264.

Supported encoders: NVIDIA NVENC, Intel Quick Sync (QSV), Linux VAAPI,
AMD AMF.

Environment variables:

- `ONYX_HWACCEL` — `auto` (default), `nvenc`, `qsv`, `vaapi`, `amf`, or
  `none` to force software. Unavailable forced encoders fall back to
  software with a warning in the logs.
- `ONYX_MAX_TRANSCODE_HEIGHT` — caps the highest rung produced. One of
  `480`, `720`, `1080`, `1440`, `2160` (default). Lowering this saves
  CPU/GPU for hosts that will never serve 4K.

GPU passthrough in Docker is commented out in `docker-compose.yml`;
uncomment the block matching your hardware. NVIDIA needs the
nvidia-container-toolkit; Intel/AMD need `/dev/dri/renderD128` exposed
to the container with the host `render` group GID.

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
