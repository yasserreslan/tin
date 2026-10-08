# `database/sql` package design

Issue #921 places the generic client package at `toolchain/std/database/sql`.
It owns driver registration, `DB` lifecycle, generic rows and results, and the
pool policy. Drivers implement the public structural shapes in that package.
The PostgreSQL adapter lives beside the existing PostgreSQL client in
`packages/postgres`; it translates generic arguments and rows at the boundary
and uses the existing protocol client rather than duplicating the wire
implementation.

The package does not take an explicit context argument. Every operation runs
inside Tin's current task boundary, as the `task` package does. Acquisition and
wire waits already observe cancellation and the earliest enclosing deadline.
An explicit database timeout is an additional upper bound: use the earlier of
the task deadline and the configured timeout, and return the task's cancellation
fault when that boundary stops the operation. Closing a `DB` prevents new
operations and waits for checked-out connections to be returned before closing
them. Pool capacity is process-wide, following the existing clients' cap
contract, while per-core idle connections remain local to their owning core.

Driver names are process-wide and immutable after registration. Registering an
empty or already registered name fails, matching Go's one-name-one-driver rule.
