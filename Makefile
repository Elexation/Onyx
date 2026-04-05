.PHONY: dev-backend dev-frontend build docker clean

dev-backend:
	air

dev-frontend:
	cd frontend && npm run dev

build:
	cd frontend && npm run build
	cp -r frontend/build web/build
	CGO_ENABLED=0 go build -o onyx ./cmd/server

docker:
	docker compose build

clean:
	rm -rf onyx tmp web/build
