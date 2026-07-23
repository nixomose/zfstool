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
# For deps targets: privilege wrapper (script clears it when already root).
SUDO    ?= sudo
# Native GUI: CGO + GTK3 + WebKit2GTK — a standalone window (NOT your default browser).
# Browser fallback only if: CGO_ENABLED=0, or -tags browser_gui, or GOFLAGS pollutes tags.
# Clear GOFLAGS on build lines so a global GOFLAGS=-tags=browser_gui cannot force the browser UI.

.PHONY: all build build-headless build-browser install clean deb srpm rpm rpm-tree vendor \
	deps deps-headless deb-deps rpm-deps help

all: build

# OS packages for local builds (see scripts/install-deps.sh). SUDO= empty if root.
deps:
	SUDO='$(SUDO)' ./scripts/install-deps.sh build

deps-headless:
	SUDO='$(SUDO)' ./scripts/install-deps.sh headless

deb-deps:
	SUDO='$(SUDO)' ./scripts/install-deps.sh deb

rpm-deps:
	SUDO='$(SUDO)' ./scripts/install-deps.sh rpm

build:
	mkdir -p bin && GOFLAGS= CGO_ENABLED=1 go build -trimpath -buildmode=pie \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o bin/zfstool ./cmd/zfstool
	@echo "bin/zfstool: native window (WebKit). If a browser opens, run: GOFLAGS= CGO_ENABLED=1 go build -o bin/zfstool ./cmd/zfstool"

# Browser UI: no CGO / no WebKit link (opens a browser tab for the UI).
build-headless:
	mkdir -p bin && GOFLAGS= CGO_ENABLED=0 go build -trimpath -buildmode=pie \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o bin/zfstool ./cmd/zfstool

# Browser UI while keeping CGO enabled (e.g. other packages need CGO).
build-browser:
	mkdir -p bin && GOFLAGS= CGO_ENABLED=1 go build -trimpath -buildmode=pie -tags browser_gui \
		-ldflags '-s -w -X github.com/nixomose/zfstool/internal/version.Version=$(VERSION)' \
		-o bin/zfstool ./cmd/zfstool

install: build
	install -d '$(DESTDIR)$(BINDIR)'
	install -m0755 bin/zfstool '$(DESTDIR)$(BINDIR)/zfstool'
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

# --- Debian (binary package in parent directory) ---
deb:
	./scripts/build-deb.sh

# --- RPM ---
rpm-tree:
	mkdir -p build/rpm/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}

rpm: rpm-tree
	git archive --format=tar.gz --prefix=zfstool-$(RPMVER)/ \
		-o build/rpm/SOURCES/zfstool-$(RPMVER).tar.gz HEAD
	rpmbuild -bb packaging/rpm/zfstool.spec \
		--define '_topdir $(CURDIR)/build/rpm' \
		--define 'ver $(RPMVER)' \
		--define 'rel $(RPMREL)'
	@echo "RPM(s) under build/rpm/RPMS/*/"

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
	@echo '         build (native WebKit WINDOW — not your browser), build-headless, build-browser,'
	@echo '         install, clean, deb, rpm, srpm, vendor, help'
	@echo 'If a browser tab opens: you built the browser variant (CGO off, browser_gui tag, or stale GOFLAGS).'
	@echo 'Plain go build: GOFLAGS= CGO_ENABLED=1 go build ./cmd/zfstool  (same as make build)'
	@echo 'Variables: VERSION=$(VERSION) PREFIX=$(PREFIX) RPMVER=$(RPMVER) RPMREL=$(RPMREL) SUDO=$(SUDO)'
