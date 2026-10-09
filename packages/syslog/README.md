# syslog

Writes to the system log and to collectors, as Go's `log/syslog` does (#929): the local Unix socket,
Unix datagram and stream sockets, TCP and IPv4 UDP, the priorities and facilities, and a `herald`
sink that writes a log record in the same wire format.

## API

Connections:

- `Dial(network, raddr, priority, tag) !Writer`: `network` is `unixgram`, `unix`, `tcp` or `udp`, or
  `""` for the local system log (`raddr` is then ignored). `priority` is a facility | severity; `tag`
  `""` is the program's name. An invalid priority fails with Go's `log/syslog: invalid priority`.
- `New(priority, tag) !Writer`: `Dial("", "", priority, tag)`.
- `Writer.Write(b []u8) !i64`: one message at the writer's priority; returns `len(b)`.
- `Writer.Emerg`, `Alert`, `Crit`, `Err`, `Warning`, `Notice`, `Info`, `Debug` (`m str) !`.
- `Writer.Herald(level, msg, kv []str) !`: a herald record at its level (`herald.LDebug` → debug,
  `LInfo` → info, `LWarn` → warning, `LError` → err); see `HeraldMessage`.
- `Writer.Close()`: closes the connection; the next message connects again, as in Go.

Priorities: severities `LogEmerg` (0) to `LogDebug` (7); facilities `LogKern` (0) to `LogFtp`
(88), and `LogLocal0` (128) to `LogLocal7` (184). A priority is facility | severity.

Formatting (pure, for tests and sinks):

- `Frame(local, pri, ts, host, tag, pid, msg) str`: one message in Go's wire format. `local` frames
  (the Unix sockets) are `<PRI>TIME TAG[PID]: MSG`; network frames add the host after the time.
  A newline ends the message unless it has one.
- `HeraldMessage(msg, kv) str`: a herald record's text: the message, then ` key=value` pairs.
  A value that is empty or holds a space, a quote, `=`, a control byte or a non-ASCII byte is quoted
  with `%q`, as herald quotes it.

Collectors (the receiving side, for tests and small collectors):

- `Listen(network, addr) !Collector`: `unixgram` (one message per `Read`), `unix` (a stream: `Read`
  returns the next bytes and accepts the next connection once one ends) or `udp` (`addr` is
  `a.b.c.d:port`). A Unix path must not exist yet.
- `Collector.Read(max) !str`, `Collector.Close()`.

## Design notes

- The wire format is Go's, byte for byte: `<PRI>` then the time (the Stamp layout, `Jan _2 15:04:05`,
  on local sockets; RFC 3339 on network connections), the host on network connections only, then
  `TAG[PID]: MSG` and a newline. The time is the local zone's (`tide.Local`).
- Go's `log/syslog` does not send RFC 5424: no structured data, no octet-count framing. This package
  does not either; RFC 5424 would be a Tin-only addition with nothing to compare against.
- `Dial` with `""` tries `unixgram` then `unix`, each at `/dev/log`, `/var/run/syslog` and
  `/var/run/log`, the same list Go tries on every Unix.
- A write that fails on an open connection reconnects once and writes again (Go's `writeAndRetry`).
- Sockets are blocking. On Linux the calls are the runtime's `rt_sys_*` helpers; on macOS the same
  calls are declared per OS in `syslog_darwin.tin` (libSystem). Socket layouts (`sockaddr_un`) live
  in `syslog_linux.tin` and `syslog_darwin.tin`.
- A stream connection (`unix`, `tcp`) sets `SO_NOSIGPIPE` on macOS; on Linux the runtime ignores
  SIGPIPE process-wide (the same as `wire`), so a dropped collector is an error from the write.
- A `Writer` belongs to one core: give each core its own (the same rule as herald's state).

## Known gaps

- `udp` takes IPv4 numeric addresses only (`127.0.0.1:514`); host names are not resolved. `tcp`
  goes through `wire`, which resolves names.
- No `NewLogger` (Go's `*log.Logger` over a Writer): Tin has no `log` package; use `Herald`.
- `Close` returns nothing (Go returns the close error, which is always nil in practice).
- `Writer` has no mutex: it is per core (see above), where Go's is shared.
- The default tag (`""`) is the program's first argument, as Go's `os.Args[0]`.
- macOS datagram queues are small: a sender that outruns its collector blocks or fails. The check
  reads while the client runs for that reason.
