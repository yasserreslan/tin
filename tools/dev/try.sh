#!/bin/sh
# Usage: tools/dev/try.sh COMPILER FILE.tin — compile with COMPILER and run, printing a backtrace on compiler crash.
cd "$(dirname "$0")/../.." || exit 1
c=$1; f=$2; shift 2
TIN_ROOT=$PWD "$c" -o bin/t "$f"; rc=$?
if [ $rc -ge 128 ]; then
  o=$(TIN_ROOT=$PWD perl -e 'alarm shift; exec @ARGV' 60 lldb --batch -o "command script import tools/dev/tinbt.py" -o run -k "tinbt 30" -k quit -- "$c" -o bin/t "$f" </dev/null 2>&1) || echo "lldb timeout"
  printf '%s\n' "$o" | sed -n '/stop reason/,$p'
  exit $rc
fi
[ $rc -eq 0 ] && bin/t "$@"
