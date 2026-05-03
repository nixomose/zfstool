#!/bin/sh
# Build a Debian binary package (see debian/). Output: ../zfstool_<ver>_<arch>.deb
set -e
cd "$(dirname "$0")/.."
exec dpkg-buildpackage -us -uc -b
