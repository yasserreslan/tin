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
owning core.

Driver registration is process-wide. `Register` links each driver into a list in
process memory (shared state under a spin lock, each entry kept in the ingot
heap), so a driver registered on core 0 is found on every core. A name already
registered is refused with Go's text; an empty name is allowed, as in Go.

Pool limits change after first use, with Go's semantics where a core can apply
them:
- `SetMaxOpen(n)` sets the cap of the client's cap record (0: no cap). A cap
  that is raised wakes the waiters on the calling core. A cap that is lowered
  closes the calling core's idle connections above it; other cores do so when
  they next use the client, and a connection released while the process is over
  the cap is closed rather than kept. Clients that share a `MaxTotal` cap share
  the change.
- `SetMaxIdle(n)` sets how many idle connections each core keeps (0: none).
  It does not limit how many connections may be open; that is `Options.Pool`.

Known gap: a waiter on a core that holds connections of its own sees a raised
cap when that core releases one or its wait ends. A core that holds none polls
the cap every 2 ms, so it sees the change within that time.
