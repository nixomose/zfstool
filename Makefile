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
# Default desktop UI: native GTK (needs CGO + libgtk-3). Override: make build-headless
GTK_TAGS ?= gtk3

.PHONY: all build build-headless install clean deb srpm rpm rpm-tree vendor \
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
	mkdir -p bin
	CGO_ENABLED=1 go build -trimpath -buildmode=pie -tags '$(GTK_TAGS)' \
		-ldflags '-s -w -X zfstool/internal/version.Version=$(VERSION)' \
		-o bin/zfstool ./cmd/zfstool

# Static binary, browser-based UI when you run zfstool with no subcommand (no GTK/CGO).
build-headless:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -buildmode=pie \
		-ldflags '-s -w -X zfstool/internal/version.Version=$(VERSION)' \
		-o bin/zfstool ./cmd/zfstool

install: build
	install -d '$(DESTDIR)$(BINDIR)'
	install -m0755 bin/zfstool '$(DESTDIR)$(BINDIR)/zfstool'

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
	@echo 'Targets: deps, deps-headless, deb-deps, rpm-deps, build (default, GTK+CGO),'
	@echo '         build-headless, install, clean, deb, rpm, srpm, vendor, help'
	@echo 'Variables: VERSION=$(VERSION) PREFIX=$(PREFIX) RPMVER=$(RPMVER) RPMREL=$(RPMREL)'
	@echo '           GTK_TAGS=$(GTK_TAGS) SUDO=$(SUDO)'
