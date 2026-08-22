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

FROM alpine:3.21
RUN apk add --no-cache ffmpeg su-exec
COPY --from=backend /onyx /onyx
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
	CMD if [ "$ONYX_TLS" = "true" ]; then \
		wget --no-verbose --no-check-certificate --tries=1 --spider https://localhost:8080/api/health; \
	else \
		wget --no-verbose --tries=1 --spider http://localhost:8080/api/health; \
	fi || exit 1
ENTRYPOINT ["/entrypoint.sh"]
