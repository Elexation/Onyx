FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
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
RUN apk add --no-cache ffmpeg
COPY --from=backend /onyx /onyx
RUN addgroup -S onyx && adduser -S onyx -G onyx
USER onyx
EXPOSE 8080
CMD ["/onyx"]
