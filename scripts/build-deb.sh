#!/bin/sh
# Build a Debian binary package (see debian/). Output: ../zfstool_<ver>_<arch>.deb
# Extra arguments are passed to dpkg-buildpackage (for example, -aarm64).
set -e
cd "$(dirname "$0")/.."
exec dpkg-buildpackage -us -uc -b "$@"
