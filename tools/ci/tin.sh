#!/bin/sh
# Runs a CI check or tool written in Tin: `tools/ci/tin.sh NAME [ARGS...]` compiles tools/ci/NAME.tin (or, with a slash, the path
# given) with bin/tinc, which is built from the seed first when it is missing, and runs it from the repository root.
set -e
# checks that hold hundreds of connections need more descriptors than a Mac's default 256; the servers they start inherit the limit
ulimit -n 4096 2>/dev/null || true
cd "$(dirname "$0")/../.."
[ -x bin/tinc ] || make -s bin/tinc
name=$1
shift
case $name in
	*/*) source=$name ;;
	*) source=tools/ci/$name.tin ;;
esac
mkdir -p bin/ci/tools
exe=bin/ci/tools/$(basename "$source" .tin)
TIN_ROOT=$PWD bin/tinc -o "$exe" "$source"
exec "$exe" "$@"
