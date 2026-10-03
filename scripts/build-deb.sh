#!/bin/sh
# Build a Debian binary package (see debian/). Output: ../zfstool_<ver>_<arch>.deb
# Extra arguments are passed to dpkg-buildpackage (for example, -aarm64).
set -eu
cd "$(dirname "$0")/.."

BUILD_ARCH=$(dpkg-architecture -qDEB_BUILD_ARCH)
HOST_ARCH=$BUILD_ARCH
SKIP_DEPS=0
for ARG in "$@"; do
	case "$ARG" in
	-a*) HOST_ARCH=${ARG#-a} ;;
	-d) SKIP_DEPS=1 ;;
	esac
done

# Cross-builds intentionally use CGO=0, so foreign GTK/WebKit development
# packages are not needed even though they remain native build dependencies.
if [ "$HOST_ARCH" != "$BUILD_ARCH" ] && [ "$SKIP_DEPS" -eq 0 ]; then
	set -- "$@" -d
fi

exec dpkg-buildpackage -us -uc -b "$@"
