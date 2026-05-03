#!/bin/sh
# Install OS packages for zfstool Makefile targets.
# Usage: install-deps.sh build|headless|deb|rpm
# Environment: SUDO=sudo (default), empty if already root.

set -eu
MODE="${1:?usage: $0 build|headless|deb|rpm}"
SUDO="${SUDO:-sudo}"
if [ "$(id -u)" -eq 0 ]; then
	SUDO=""
fi
run() {
	if [ -n "$SUDO" ]; then
		$SUDO "$@"
	else
		"$@"
	fi
}

if command -v apt-get >/dev/null 2>&1; then
	export DEBIAN_FRONTEND=noninteractive
	run apt-get update -qq
	case "$MODE" in
	build)
		run apt-get install -y --no-install-recommends \
			golang-go gcc pkg-config libgtk-3-dev libc6-dev
		;;
	headless)
		run apt-get install -y --no-install-recommends golang-go
		;;
	deb)
		run apt-get install -y --no-install-recommends \
			build-essential dpkg-dev debhelper-compat golang-go gcc pkg-config \
			libgtk-3-dev libc6-dev
		;;
	rpm)
		echo "rpm-deps: use Fedora/RHEL (dnf/yum). On Debian/Ubuntu, only apt is supported." >&2
		exit 1
		;;
	esac
	exit 0
fi

if command -v dnf >/dev/null 2>&1; then
	case "$MODE" in
	build)
		run dnf install -y golang gcc gtk3-devel pkgconf-pkg-config
		;;
	headless)
		run dnf install -y golang
		;;
	deb)
		echo "deb-deps requires apt (Debian/Ubuntu)." >&2
		exit 1
		;;
	rpm)
		run dnf install -y rpm-build git golang gcc gtk3-devel pkgconf-pkg-config systemd-rpm-macros
		;;
	esac
	exit 0
fi

if command -v yum >/dev/null 2>&1; then
	case "$MODE" in
	build)
		run yum install -y golang gcc gtk3-devel pkgconfig
		;;
	headless)
		run yum install -y golang
		;;
	deb)
		echo "deb-deps requires apt (Debian/Ubuntu)." >&2
		exit 1
		;;
	rpm)
		run yum install -y rpm-build git golang gcc gtk3-devel pkgconfig systemd-rpm-macros
		;;
	esac
	exit 0
fi

if command -v zypper >/dev/null 2>&1; then
	case "$MODE" in
	build)
		run zypper --non-interactive install go gcc gtk3-devel pkg-config
		;;
	headless)
		run zypper --non-interactive install go
		;;
	deb)
		echo "deb-deps requires apt (Debian/Ubuntu)." >&2
		exit 1
		;;
	rpm)
		run zypper --non-interactive install rpm-build git go gcc gtk3-devel pkg-config \
			systemd-rpm-macros
		;;
	esac
	exit 0
fi

if command -v pacman >/dev/null 2>&1; then
	case "$MODE" in
	build)
		run pacman -S --needed --noconfirm go gcc pkgconf gtk3
		;;
	headless)
		run pacman -S --needed --noconfirm go
		;;
	deb)
		echo "deb-deps requires apt (Debian/Ubuntu)." >&2
		exit 1
		;;
	rpm)
		echo "rpm-deps: use Fedora/RHEL (dnf/yum)." >&2
		exit 1
		;;
	esac
	exit 0
fi

echo "install-deps.sh: unsupported OS (need apt-get, dnf, yum, zypper, or pacman)." >&2
echo "Install manually: Go >= 1.22, gcc, pkg-config, GTK 3 development headers." >&2
exit 1
