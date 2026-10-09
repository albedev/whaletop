VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

PREFIX ?= $(HOME)/.local

.PHONY: build install uninstall run test vet dump licenses cross release-check release-snapshot clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/whaletop ./cmd/whaletop

# installs into $(PREFIX)/bin (default ~/.local/bin, no sudo needed) plus the `wtop` alias.
# An existing wtop that is not our symlink (e.g. the PyPI web-server tool) is never overwritten.
install: build
	install -d $(PREFIX)/bin
	install -m 755 bin/whaletop $(PREFIX)/bin/whaletop
	@if [ -e $(PREFIX)/bin/wtop ] && [ "$$(readlink $(PREFIX)/bin/wtop)" != "whaletop" ]; then \
		echo "warning: $(PREFIX)/bin/wtop already exists and is not whaletop: alias not installed"; \
	else ln -sfn whaletop $(PREFIX)/bin/wtop && echo "alias: $(PREFIX)/bin/wtop -> whaletop"; fi

uninstall:
	rm -f $(PREFIX)/bin/whaletop
	@if [ "$$(readlink $(PREFIX)/bin/wtop)" = "whaletop" ]; then rm -f $(PREFIX)/bin/wtop; fi

run: build
	./bin/whaletop

test:
	go test ./...

vet:
	go vet ./...

# render one frame of each view without a TTY (needs a running daemon)
dump: build
	./bin/whaletop --dump 160x45

# list linked modules with their license type
licenses:
	@bash -c 'shopt -s nullglob; for m in $$(go list -deps -f "{{if .Module}}{{.Module.Path}}{{end}}" ./cmd/whaletop | sort -u | grep -v albedev/whaletop); do \
		d=$$(go list -m -f "{{.Dir}}" $$m); fs=($$d/LICENSE* $$d/COPYING*); \
		l=$$(head -c 1500 "$${fs[0]}" | grep -oE "MIT License|Permission is hereby granted|Apache License|Redistribution and use" | head -1); \
		echo "$$m | $$l"; done | sed "s/Permission is hereby granted/MIT/; s/Redistribution and use/BSD/"'

cross:
	for t in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do \
		CGO_ENABLED=0 GOOS=$${t%/*} GOARCH=$${t#*/} go build -trimpath -ldflags "$(LDFLAGS)" -o bin/whaletop-$${t%/*}-$${t#*/} ./cmd/whaletop || exit 1; \
	done

# GoReleaser (go install github.com/goreleaser/goreleaser/v2@latest): validate config / full local dry run into dist/
GORELEASER ?= $(shell command -v goreleaser || echo $(shell go env GOPATH)/bin/goreleaser)

release-check:
	HOMEBREW_TAP_DEPLOY_KEY=dry-run $(GORELEASER) check

release-snapshot:
	HOMEBREW_TAP_DEPLOY_KEY=dry-run $(GORELEASER) release --snapshot --clean

clean:
	rm -rf bin dist
