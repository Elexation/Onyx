FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* frontend/.npmrc ./
RUN npm ci
COPY frontend/ .
RUN mkdir -p /app/web
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/build ./web/build
COPY --from=frontend /app/web/csp_hash.go ./web/csp_hash.go
RUN CGO_ENABLED=0 go build -o /onyx ./cmd/server

FROM debian:bookworm-slim
RUN sed -i 's/Components: main/Components: main non-free non-free-firmware/' /etc/apt/sources.list.d/debian.sources && \
	apt-get update && apt-get install -y --no-install-recommends \
	ffmpeg \
	intel-media-va-driver-non-free \
	mesa-va-drivers \
	gosu \
	curl \
	&& rm -rf /var/lib/apt/lists/*
COPY --from=backend /onyx /onyx
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
	CMD if [ "$ONYX_HTTPS" = "true" ] || [ "$ONYX_TLS" = "true" ]; then \
		curl -fsk https://localhost:8080/api/health; \
	else \
		curl -fs http://localhost:8080/api/health; \
	fi || exit 1
ENTRYPOINT ["/entrypoint.sh"]
