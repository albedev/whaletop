VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

PREFIX ?= $(HOME)/.local

.PHONY: build install run test vet dump licenses cross clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/dtop ./cmd/dtop

# installs into $(PREFIX)/bin (default ~/.local/bin, no sudo needed)
install: build
	install -d $(PREFIX)/bin
	install -m 755 bin/dtop $(PREFIX)/bin/dtop

run: build
	./bin/dtop

test:
	go test ./...

vet:
	go vet ./...

# render one frame of each view without a TTY (needs a running daemon)
dump: build
	./bin/dtop --dump 160x45

# list linked modules with their license type
licenses:
	@bash -c 'shopt -s nullglob; for m in $$(go list -deps -f "{{if .Module}}{{.Module.Path}}{{end}}" ./cmd/dtop | sort -u | grep -v albedev/dtop); do \
		d=$$(go list -m -f "{{.Dir}}" $$m); fs=($$d/LICENSE* $$d/COPYING*); \
		l=$$(head -c 1500 "$${fs[0]}" | grep -oE "MIT License|Permission is hereby granted|Apache License|Redistribution and use" | head -1); \
		echo "$$m | $$l"; done | sed "s/Permission is hereby granted/MIT/; s/Redistribution and use/BSD/"'

cross:
	for t in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do \
		CGO_ENABLED=0 GOOS=$${t%/*} GOARCH=$${t#*/} go build -trimpath -ldflags "$(LDFLAGS)" -o bin/dtop-$${t%/*}-$${t#*/} ./cmd/dtop || exit 1; \
	done

clean:
	rm -rf bin
