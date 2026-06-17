.PHONY: dev build clean

dev:
	-@command -v taskkill >/dev/null && taskkill //im onyx.exe //f >/dev/null 2>&1; true
	@trap 'kill $$(jobs -p) 2>/dev/null; exit 0' INT TERM; \
		air & \
		(cd frontend && npm run dev -- --host 0.0.0.0) & \
		wait

build:
	cd frontend && npm run build
	rm -rf web/build
	cp -r frontend/build web/build
	CGO_ENABLED=0 go build -o onyx ./cmd/server

clean:
	rm -rf onyx onyx.exe tmp web/build
