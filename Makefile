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

.PHONY: all build install clean deb srpm rpm rpm-tree vendor help

all: build

build:
	mkdir -p bin
	go build -trimpath -buildmode=pie \
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
	@echo 'Targets: build (default), install, clean, deb, rpm, srpm, vendor, help'
	@echo 'Variables: VERSION=$(VERSION) PREFIX=$(PREFIX) RPMVER=$(RPMVER) RPMREL=$(RPMREL)'
