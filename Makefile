.PHONY: web build install dist test dev-web clean

# The version stamped into the binary: the git tag, or the short commit.
VERSION ?= $(or $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//'),dev)
LDFLAGS  = -s -w -X main.Version=$(VERSION)
PREFIX  ?= $(HOME)/.sentinel

# Platforms `make dist` builds for. Windows is not listed: hook installation
# has not been tried there.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

# Build the cockpit into internal/ui/dist (embedded by go:embed).
web:
	cd web && npm install && npm run build

# Single self-contained binary: cockpit + proxy.
build: web
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/sentinel ./cmd/sentinel

# Copy the binary to $(PREFIX)/bin. Cursor's hooks.json stores its absolute
# path, so put it where it will stay before running `sentinel hook install`.
install: build
	mkdir -p $(PREFIX)/bin
	install -m 0755 bin/sentinel $(PREFIX)/bin/sentinel
	@echo "installed $(PREFIX)/bin/sentinel"

# One tar.gz per platform plus checksums.txt, in dist/.
dist: web
	rm -rf dist && mkdir -p dist
	for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; \
	  name=sentinel_$(VERSION)_$${os}_$${arch}; \
	  mkdir -p dist/$$name; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/sentinel ./cmd/sentinel || exit 1; \
	  cp README.md LICENSE dist/$$name/; \
	  tar -C dist -czf dist/$$name.tar.gz $$name; \
	  rm -rf dist/$$name; \
	done
	cd dist && shasum -a 256 *.tar.gz > checksums.txt
	@cat dist/checksums.txt

test:
	go vet ./...
	go test -race ./...
	cd web && npm run typecheck

# Vite dev server with hot reload; proxies /api and /ws to a running
# `sentinel mcp --ui --dev`, `sentinel serve --dev` or `sentinel ui --dev` on :8848.
dev-web:
	cd web && npm run dev

clean:
	rm -rf bin dist internal/ui/dist/* web/node_modules
	touch internal/ui/dist/.gitkeep
