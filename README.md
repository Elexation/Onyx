# Onyx

Self-hosted, single-admin file browser and sharing platform.

Go backend, SvelteKit frontend, SQLite storage. Dark mode only. Ships as a single binary or Docker container.

## Quick Start

### Docker Compose (recommended)

```bash
docker compose up -d
```

Open `http://localhost:8080`. To use a different port:

```bash
ONYX_PORT=3000 docker compose up -d
```

### Docker Run

```bash
docker run -d \
  --name onyx \
  -p 8080:8080 \
  -v ./config:/config \
  -v ./data:/srv \
  -v ./.versions:/.versions \
  -v ./.trash:/.trash \
  -v ./.cache:/.cache \
  -e ONYX_DATA=/srv \
  -e ONYX_CONFIG=/config \
  --restart always \
  onyx:latest
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

## Hardware Video Acceleration

Onyx transcodes non-browser-native video on demand into an HLS ABR ladder
(2160p/1440p/1080p/720p/480p, capped by source height). By default it
probes for hardware encoders on startup and uses the best available;
when none are found it falls back to libx264 (software).

If the GPU isn't ready at startup (common after a reboot — the driver may
initialize after the container starts), Onyx re-probes every 30 seconds
for up to 5 minutes and upgrades automatically — no restart needed.

Encoder priority: **NVENC > QSV > VAAPI > AMF > libx264**.

### Intel / AMD

Add one line to your compose file:

```yaml
devices:
  - /dev/dri:/dev/dri
```

Or with `docker run`:

```bash
docker run -d --device /dev/dri:/dev/dri ... onyx:latest
```

No host-side driver installation needed for most systems — the kernel
module (i915 for Intel, amdgpu for AMD) is auto-loaded, and the container
bundles the userspace VA-API drivers.

### NVIDIA

One-time host setup:

```bash
sudo apt install nvidia-container-toolkit
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

Then add to your compose file:

```yaml
runtime: nvidia
environment:
  - NVIDIA_VISIBLE_DEVICES=all
  - NVIDIA_DRIVER_CAPABILITIES=video,compute,utility
```

Or with `docker run`:

```bash
docker run -d --runtime=nvidia \
  -e NVIDIA_VISIBLE_DEVICES=all \
  -e NVIDIA_DRIVER_CAPABILITIES=video,compute,utility \
  ... onyx:latest
```

### Both GPUs

If your system has both (e.g. NVIDIA discrete + Intel iGPU), combine both
sections. Onyx will prefer NVENC (faster) and fall back to VAAPI if the
NVIDIA runtime isn't available.

### Configuration

Set `ONYX_HWACCEL` to force a specific encoder (`nvenc`, `qsv`, `vaapi`,
`amf`, `none`) or leave it as `auto` (default). See the environment
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

## License

Onyx is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0-only). See [LICENSE](LICENSE) for the full text.

Copyright (C) 2026 Elexation

Onyx is network-served software, so the AGPL requires that anyone interacting with a modified instance over a network be able to obtain its source. The canonical source is available at <https://github.com/Elexation/Onyx>.
