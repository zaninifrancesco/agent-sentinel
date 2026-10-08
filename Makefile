.PHONY: web build test dev-web clean

# Build the cockpit into internal/ui/dist (embedded by go:embed).
web:
	cd web && npm install && npm run build

# Single self-contained binary: cockpit + proxy.
build: web
	go build -o bin/sentinel ./cmd/sentinel

test:
	go vet ./...
	go test -race ./...

# Vite dev server with hot reload; proxies /api and /ws to a running
# `sentinel mcp --ui` or `sentinel ui` on :8848.
dev-web:
	cd web && npm run dev

clean:
	rm -rf bin internal/ui/dist/* web/node_modules
	touch internal/ui/dist/.gitkeep
