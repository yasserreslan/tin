# E120 diagnostic without a source position

Signature: `diagnostic-without-position:E120 NO_MAIN`.

Command: `TIN_ROOT=$PWD bin/tinc -S repro.tin`

Expected: a diagnostic with `file:line:col` and E code.

Actual:

```
error E120 NO_MAIN: no main function
```

Exit status: 1. This is the mutator's `diagnostic lacks file:line:col and E code` finding on macOS arm64. No compiler change is included here.
