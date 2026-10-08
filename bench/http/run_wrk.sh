#!/bin/sh
# Usage: bench/http/run_wrk.sh CORES WRK_THREADS CONNS SECONDS ROUNDS [--base-api PATH --head-api PATH]
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root" && exec sh tools/ci/tin.sh bench/http/run_wrk.tin "$@"
