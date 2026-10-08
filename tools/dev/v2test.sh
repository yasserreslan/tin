#!/bin/sh
# Compile and run all strict tests; stdout and exit status are both required.
cd "$(dirname "$0")/../.." || exit 1
compiler=${1:-bin/tinc}
sh tools/ci/tin.sh suite "$compiler" || exit $?
# Diagnostic codes: the compiler, toolchain/docs/ERRORS.md and the .err files agree; examples compile as shown.
sh tools/ci/tin.sh diagnostics_check "$compiler" || exit $?
toolchain/tests/darwin.sh "$compiler" || exit $?
# Tinland: the portable parts (ui layout, project, terminal screen) everywhere, the editor and window on macOS
sh products/tinland/tests/run.sh "$compiler" || exit $?
exec toolchain/tests/edition1.sh "$compiler"
