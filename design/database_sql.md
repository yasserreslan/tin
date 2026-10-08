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
operations, closes idle connections and closes checked-out connections when
they are returned. Pool capacity is process-wide, following the existing
clients' cap contract, while per-core idle connections remain local to their
owning core. The PostgreSQL adapter requires `SetMaxOpen` and `SetMaxIdle` before
first use because its existing client fixes those limits when the per-core pool
is first created.

Driver names are immutable after registration on each core. Registering an empty
or already registered name fails with Go's registration fault text. Tin's
mutable package globals are per-core, so the current registry is also per-core;
a process-wide registry of mutable driver values needs a shared driver lifecycle
contract that this patch does not provide. This remains an open requirement in
issue #921.
