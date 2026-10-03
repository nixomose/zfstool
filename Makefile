# zfstool — build, Debian binary package, RPM binary package
#
# Debian: requires debhelper, golang-go (>= 1.22), dpkg-dev. First build may run
#         `go mod download` (network) unless you run `make vendor` and adjust debian/rules.
# RPM:    requires rpm-build, golang >= 1.22, git (to create the source tarball).

VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//' || echo 0.1.0-dev)
RPMREL  ?= 1
# RPM Version field must not contain Debian revision (e.g. "0.1.0" from "0.1.0-1")
RPMVER  ?= $(firstword $(subst -, ,$(VERSION)))

PREFIX  ?= /usr/local
BINDIR  ?= $(PREFIX)/bin
OUTPUT  ?= bin/zfstool
ARM64_OUTPUT ?= bin/zfstool-linux-arm64
# For deps targets: privilege wrapper (script clears it when already root).
SUDO    ?= sudo
# Native GUI: CGO + GTK3 + WebKit2GTK — a standalone window (NOT your default browser).
# Browser fallback only if: CGO_ENABLED=0, or -tags browser_gui, or GOFLAGS pollutes tags.
# Clear GOFLAGS on build lines so a global GOFLAGS=-tags=browser_gui cannot force the browser UI.

.PHONY: all build build-headless build-browser build-arm64 build-arm64-native \
	check-arm64 check-arm64-host install clean deb deb-amd64 deb-arm64 srpm rpm rpm-arm64 \
	rpm-tree vendor deps deps-headless deb-deps rpm-deps check-gui-deps help

all: build

# OS packages for local builds (see scripts/install-deps.sh). SUDO= empty if root.
deps:
	SUDO='$(SUDO)' ./scripts/install-deps.sh build

# Native GUI needs gtk+-3.0 and webkit2gtk-4.1 (see third_party/webview_go).
# If they are missing, install them the same way as `make deps`.
# SKIP_GUI_DEPS=1 skips the auto-install (package builds / already-prepared chroots).
check-gui-deps:
ifeq ($(SKIP_GUI_DEPS),1)
	@true
else
	@if ! command -v pkg-config >/dev/null 2>&1 || ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then \
		echo 'zfstool: GTK 3 / WebKit2GTK 4.1 not found; installing build deps (make deps)…'; \
		$(MAKE) deps; \
		if ! command -v pkg-config >/dev/null 2>&1 || ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then \
			echo 'zfstool: still missing gtk+-3.0 or webkit2gtk-4.1.' >&2; \
			echo 'Debian/Ubuntu 24.04+: libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config' >&2; \
			echo 'Ubuntu 22.04 uses webkit2gtk-4.0 — see third_party/README.md' >&2; \
			exit 1; \
		fi; \
	fi
endif

deps-headless:
	SUDO='$(SUDO)' ./scripts/install-deps.sh headless

deb-deps:
	SUDO='$(SUDO)' ./scripts/install-deps.sh deb

rpm-deps:
	SUDO='$(SUDO)' ./scripts/install-deps.sh rpm

build: check-gui-deps
	mkdir -p '$(dir $(OUTPUT))' && GOFLAGS= CGO_ENABLED=1 go build -trimpath -buildmode=pie \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o '$(OUTPUT)' ./cmd/zfstool
	@echo "$(OUTPUT): native window (WebKit). If a browser opens, run: GOFLAGS= CGO_ENABLED=1 go build -o $(OUTPUT) ./cmd/zfstool"

# Browser UI: no CGO / no WebKit link (opens a browser tab for the UI).
build-headless:
	mkdir -p '$(dir $(OUTPUT))' && GOFLAGS= CGO_ENABLED=0 go build -trimpath -buildmode=pie \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o '$(OUTPUT)' ./cmd/zfstool

# Browser UI while keeping CGO enabled (e.g. other packages need CGO).
build-browser:
	mkdir -p '$(dir $(OUTPUT))' && GOFLAGS= CGO_ENABLED=1 go build -trimpath -buildmode=pie -tags browser_gui \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o '$(OUTPUT)' ./cmd/zfstool

# Portable Raspberry Pi / Linux ARM64 build. CGO is deliberately disabled so
# this can be cross-compiled on any Go host without an ARM64 C toolchain. The
# resulting binary contains every command; `gui` opens the UI in a browser.
build-arm64:
	mkdir -p '$(dir $(ARM64_OUTPUT))' && GOFLAGS= GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -trimpath -buildmode=pie \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o '$(ARM64_OUTPUT)' ./cmd/zfstool
	@$(MAKE) --no-print-directory check-arm64

check-arm64:
	@go version -m '$(ARM64_OUTPUT)' | grep -q 'GOOS=linux'
	@go version -m '$(ARM64_OUTPUT)' | grep -q 'GOARCH=arm64'
	@echo "$(ARM64_OUTPUT): Linux ARM64 (browser UI)"

# The embedded WebKit window requires ARM64 GTK/WebKit libraries. Build this
# variant on the Pi (or another ARM64 Linux host) after `make deps`.
check-arm64-host:
	@if [ "$$(go env GOHOSTOS)/$$(go env GOHOSTARCH)" != linux/arm64 ]; then \
		echo 'This target needs a native Linux ARM64 host (for example, a 64-bit Raspberry Pi OS).' >&2; \
		echo 'Use `make build-arm64` here, or run this target on the Pi.' >&2; \
		exit 1; \
	fi

build-arm64-native: check-arm64-host check-gui-deps
	$(MAKE) --no-print-directory build GOOS=linux GOARCH=arm64 OUTPUT='$(ARM64_OUTPUT)'

install: build
	install -d '$(DESTDIR)$(BINDIR)'
	install -m0755 '$(OUTPUT)' '$(DESTDIR)$(BINDIR)/zfstool'
	install -D -m0644 deploy/zfstool.desktop '$(DESTDIR)$(PREFIX)/share/applications/zfstool.desktop'
	@for sz in 16x16 24x24 32x32 48x48 64x64 128x128 256x256 512x512; do \
		install -D -m0644 deploy/icons/hicolor/$$sz/apps/zfstool.png \
			'$(DESTDIR)$(PREFIX)/share/icons/hicolor/'$$sz'/apps/zfstool.png'; \
	done
	install -D -m0644 deploy/icons/hicolor/scalable/apps/zfstool.svg \
		'$(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/zfstool.svg'

clean:
	rm -rf bin build/rpm
	rm -f zfstool

# --- Debian (both binary packages in parent directory) ---
deb:
	./scripts/build-deb.sh -aamd64
	./scripts/build-deb.sh -aarm64

deb-amd64:
	./scripts/build-deb.sh -aamd64

deb-arm64:
	./scripts/build-deb.sh -aarm64

# --- RPM ---
rpm-tree:
	mkdir -p build/rpm/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}

rpm: rpm-tree
	git archive --format=tar.gz --prefix=zfstool-$(RPMVER)/ \
		-o build/rpm/SOURCES/zfstool-$(RPMVER).tar.gz HEAD
	rpmbuild -bb packaging/rpm/zfstool.spec \
		--define '_topdir $(CURDIR)/build/rpm' \
		--define 'ver $(RPMVER)' \
		--define 'rel $(RPMREL)' $(if $(RPMTARGET),--target '$(RPMTARGET)')
	@echo "RPM(s) under build/rpm/RPMS/*/"

rpm-arm64: check-arm64-host
	$(MAKE) --no-print-directory rpm RPMTARGET=aarch64

srpm: rpm-tree
	git archive --format=tar.gz --prefix=zfstool-$(RPMVER)/ \
		-o build/rpm/SOURCES/zfstool-$(RPMVER).tar.gz HEAD
	rpmbuild -bs packaging/rpm/zfstool.spec \
		--define '_topdir $(CURDIR)/build/rpm' \
		--define 'ver $(RPMVER)' \
		--define 'rel $(RPMREL)'
	@echo "SRPM under build/rpm/SRPMS/"

vendor:
	go mod vendor

help:
	@echo 'Targets: deps, deps-headless, deb-deps, rpm-deps,'
	@echo '         build (installs GTK/WebKit if missing, then native WebKit WINDOW),'
	@echo '         build-headless, build-browser, build-arm64 (cross-build, browser UI),'
	@echo '         build-arm64-native (run on ARM64 for WebKit UI), install, clean,'
	@echo '         deb (amd64 + arm64), deb-amd64, deb-arm64, rpm, rpm-arm64, srpm, vendor, help'
	@echo 'If a browser tab opens: you built the browser variant (CGO off, browser_gui tag, or stale GOFLAGS).'
	@echo 'Plain go build: GOFLAGS= CGO_ENABLED=1 go build ./cmd/zfstool  (same as make build)'
	@echo 'Variables: VERSION=$(VERSION) OUTPUT=$(OUTPUT) ARM64_OUTPUT=$(ARM64_OUTPUT) PREFIX=$(PREFIX)'
	@echo '           RPMVER=$(RPMVER) RPMREL=$(RPMREL) SUDO=$(SUDO)'
