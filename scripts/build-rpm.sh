#!/bin/sh
# Build a binary RPM via git-archive + rpmbuild (see packaging/rpm/).
set -e
cd "$(dirname "$0")/.."
exec make rpm
