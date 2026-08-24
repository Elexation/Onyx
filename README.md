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

## Environment Variables

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `ONYX_PORT` | `8080` | Port the server listens on. In Docker, also set the host-side port mapping. |
| `ONYX_HTTPS` | `false` | Enable built-in HTTPS. Auto-generates a self-signed cert on first run. |
| `ONYX_TLS_CERT` | *(auto)* | Path to a custom TLS certificate. Must be set with `ONYX_TLS_KEY`. |
| `ONYX_TLS_KEY` | *(auto)* | Path to a custom TLS private key. Must be set with `ONYX_TLS_CERT`. |
| `ONYX_DOMAIN` | *(unset)* | Canonical hostname. Requests with a different `Host` header (e.g. an IP address) are 301-redirected to this domain. |
| `ONYX_HTTPS_REDIRECT` | `false` | Start a separate HTTP listener that redirects to HTTPS. Requires `ONYX_HTTPS=true`. |
| `ONYX_HTTPS_REDIRECT_PORT` | `80` | Port for the HTTP redirect listener. Map it in `ports:` (e.g. `"80:80"`). |
| `ONYX_TRUSTED_PROXY` | `false` | Trust `X-Forwarded-Proto` and `X-Real-IP` headers. Enable when behind a reverse proxy (nginx, Caddy, Traefik). Also omits the internal port from domain redirects. |
| `ONYX_REQUIRE_HTTPS` | `false` | Force `Secure` flag on cookies and emit HSTS headers regardless of connection type. Use when TLS is terminated at the proxy, not by Onyx. |

### Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `ONYX_DATA` | `data` | Root directory for user files. |
| `ONYX_CONFIG` | `config` | Directory for the database and TLS certificates. |
| `ONYX_CACHE` | `.cache` | Directory for thumbnails, transcode output, and upload staging. |

### Transcoding

| Variable | Default | Description |
|----------|---------|-------------|
| `ONYX_HWACCEL` | `auto` | Hardware encoder preference: `auto`, `nvenc`, `qsv`, `vaapi`, `amf`, or `none` (software). |
| `ONYX_MAX_TRANSCODE_HEIGHT` | `2160` | Cap the highest ABR rung. One of `480`, `720`, `1080`, `1440`, `2160`. |

### Docker

| Variable | Default | Description |
|----------|---------|-------------|
| `PUID` | `1000` | UID the server process runs as inside the container. |
| `PGID` | `1000` | GID the server process runs as inside the container. |

### Advanced

| Variable | Default | Description |
|----------|---------|-------------|
| `ONYX_VERSION_RETENTION_INTERVAL` | `24h` | How often the version retention sweep runs. |

## Hardware video acceleration

Onyx transcodes non-browser-native video on demand into an HLS ABR ladder
(2160p/1440p/1080p/720p/480p, capped by source height). By default it
probes the host for hardware encoders on startup and uses the best one
available; when none are found it falls back to libx264 (software).

If the GPU isn't ready at startup (common after a reboot — the GPU driver
may initialize after the container starts), Onyx re-probes every 30 seconds
for up to 5 minutes. When a hardware encoder becomes available, it
upgrades automatically — no restart needed.

Encoder priority: NVENC > QSV > VAAPI > AMF > libx264.

### NVIDIA (NVENC)

1. Install [nvidia-container-toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) on the host
2. Register the runtime with Docker:
   ```bash
   sudo nvidia-ctk runtime configure --runtime=docker
   sudo systemctl restart docker
   ```
3. Uncomment the NVIDIA block in `docker-compose.yml`

### Intel (QSV) / AMD (VAAPI/AMF)

Uncomment the Intel/AMD block in `docker-compose.yml`. Replace the group
GID with your host's `render` group (`getent group render`).

### Configuration

See `ONYX_HWACCEL` and `ONYX_MAX_TRANSCODE_HEIGHT` in the environment
variables table above.

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

## Data Protection

`.versions/` and `.trash/` protect against accidental edits and deletions.
They are not ransomware protection — they live on the same disk with the same
permissions as your data.

For resilience against filesystem-level threats, use host-level protections:

- **ZFS/Btrfs snapshots** — read-only, inaccessible to userspace processes
- **Pull-based offsite backups** — backup server pulls from you; compromised host can't reach the backup target
- **S3/B2 with Object Lock** — immutable retention windows that even the account owner can't override
