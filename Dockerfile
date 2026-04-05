FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/build ./web/build
RUN CGO_ENABLED=0 go build -o /onyx ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ffmpeg
COPY --from=backend /onyx /onyx
EXPOSE 8080
CMD ["/onyx"]
