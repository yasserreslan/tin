# Interface: tit, version control in Tin (formats, package seams, faults) — #770

Status: **proposed**, for approval by the assignees of the issues it serves: #776 – #787 (the engine),
#789 (adopt), #794 (the protocol, which adds its own section) and #805 (mirror). Nothing builds on it before
the PR that adds this file is merged (AGENTS.md, Tin 1 rule 5). After that, a change to a name, a byte layout
or a rule below needs the same approvals.

Overview and the reasons behind the choices: the umbrella issue #769 and the design page linked from it.
This file fixes what several issues share: byte layouts, file names, exported names and fault sentinels.

Numbers below are little-endian unless the text says otherwise. A **uvarint** is LEB128 (std `pack`:
`PutUvarint`, `Uvarint`). **hex** is lower-case.

---

## 1. Where things live

```text
products/tit/main.tin            the tit command (#788)
products/tit/<part>/             one package per part, imported as "../<part>" (as Tinland's are)
products/tit/tests/run.sh        the tit checks: programs with golden output, scripts against git (#806)
```

| package | owns | issue | capabilities (`tin caps`) |
|---|---|---|---|
| `object` | ids, the object kinds, canonical encoding | #776 | none |
| `store` | the `ObjectStore` shape, memory and loose stores, layering, the decoded cache | #777 | files |
| `packfile` | packs and their indexes, deltas | #778 | files |
| `change` | refs, `HEAD`, locks, change ids and versions | #779 | files |
| `oplog` | the operation log, undo and redo | #780 | files |
| `revwalk` | revision syntax, walks, merge-base | #781 | files |
| `ignore` | ignore rules (shared with Tinland) | #782 | files |
| `index` | the staging area and status | #783 | files |
| `worktree` | checkout, switch, workspaces, snapshots | #784 | files |
| `diff` | the generic diff, unified output, tree diff | #785 | none |
| `merge` | three-way merge of trees and lines | #786 | none |
| `identity` | config, names, keys, signing, the clock | #787 | files |
| `git` | reading `.git` (adopt) and writing git objects (mirror) | #789, #805 | files, net |
| `transport`, `serve` | the tit protocol | #794 – #796 | net (serve: net, files) |
| `semantic` | declarations from the compiler | #798 | exec |

The names `pack` and `lane` are taken by std, so the pack package is `packfile` and parallel working folders
are **workspaces**.

## 2. Ids

```tin
type Id value struct {        // exactly 32 bytes: the SHA-256 of an object's encoding
	raw str
}
type ChangeId value struct {  // exactly 16 bytes, printed in the letters k..z
	raw str
}
type OpNumber i64             // an operation's number in the oplog, from 1 (package oplog)
```

- `Id` is a value struct around the bytes, not `[32]u8` (a slice: mutable, not a map key) and not a named
  `str` (which cannot be a field or enum payload in another package yet, #824). It compares with `==`, is a
  map key and is copied inline. `object.IdOf(raw)` checks the length; `id.Bytes()` and `id.Hex()` read it.
- An `Id` prints as 64 hex digits. A **short id** is the shortest unique prefix of at least 6 digits.
- A `ChangeId` prints as 32 letters: each 4-bit half of a byte, high half first, is the letter `'k' + n`
  (`k` = 0 … `z` = 15). It is shown shortened to the shortest unique prefix of at least 5 letters (`kzvqt`).
  The letters k–z never occur in hex, so a change id is never mistaken for an object id.
- A new change id is 16 bytes of `seal.RandomBytes`. An **adopted** commit's change id is the first 16 bytes
  of `SHA-256("tit change " + <the git commit id in hex>)`, so the same git history always gives the same
  change ids.
- **Git ids** (40 hex digits, SHA-1) appear only in the git-ids table (section 7) and in revisions written
  `git:<hex>`.

## 3. Objects

An object's **encoding** is `<kind> <size>\0<body>`: the kind in ASCII, one space, the body's length in
decimal, a NUL byte, the body. Its id is the SHA-256 of the encoding. Decoding rejects every non-canonical
form (`object.ErrCorrupt`, naming the rule broken), so equal content always has one encoding and one id.

```tin
type Kind enum {
	Blob
	Tree
	Commit
	Tag
}

type Object enum {
	Blob(str)
	Tree([]Entry)
	Commit(CommitData)
	Tag(TagData)
}

type Mode enum {
	File    // 100644
	Exec    // 100755
	Link    // 120000
	Dir     // 40000
}

type Entry struct {
	Mode Mode
	Name str
	Id   Id
}
```

**blob**: the file's bytes, unchanged.

**tree**: entries, each `<mode> <name>\0<32-byte id>`, with mode `100644`, `100755`, `120000` or `40000`.
Rules: names are non-empty, contain no `/` and no NUL, are not `.`, `..` or `.tit`; no duplicates; sorted by
the bytes of the name, where a `Dir` entry sorts as if its name ended in `/` (git's order, so adoption keeps
the order).

**commit**: header lines, an empty line, the message.

```text
tree <hex id>
parent <hex id>                      zero or more, in order
change <change id in letters>
author <name> <<email>> <unix seconds> <+hhmm|-hhmm>
committer <name> <<email>> <unix seconds> <+hhmm|-hhmm>
conflict <path>\t<base>\t<ours>\t<theirs>   zero or more: conflicts a rebase recorded (#786); ids in hex, - for no
                                     file; the tree holds our version at the path; push and mirror refuse them
git-<name> <value>                   zero or more: headers kept from an adopted git commit (gpgsig, encoding,
                                     mergetag), in their original order; continuation lines start with a space
signature <base64 of 64 bytes>       optional, last: ed25519 over the encoding of this commit without this line

<message>
```

Rules: the headers appear exactly in this order; exactly one `tree`, `change`, `author`, `committer`;
names contain no `<`, `>` or newline; emails no `>` or newline. The message is kept byte for byte (no trailing
newline is added or removed). Adopted commits have no `signature` (their GPG signature, if any, stays as
`git-gpgsig` text and is never checked).

```tin
type Person struct {
	Name  str
	Email str
	When  i64    // unix seconds
	Zone  i64    // minutes east of UTC
}

type CommitData struct {
	Tree      Id
	Parents   []Id
	Change    ChangeId
	Author    Person
	Committer Person
	GitHeaders []str   // "name value" pairs kept from git, in order
	Signature str      // 64 bytes, or "" when unsigned
	Message   str
}
```

**tag**: `object <hex id>`, `type <kind>`, `tag <name>`, `tagger <person>`, optional `signature`, an empty
line, the message. `TagData` mirrors it.

Large files need no fifth kind: a blob over 8 MiB keeps its id and is stored in chunks by the loose store (section
5), so every other part of tit, and git mirrors, see an ordinary blob (#803).

**Worked example.** The blob `hello, tin!\n` (12 bytes) encodes as `blob 12\0hello, tin!\n` and has id
`ebc9953649c58cea3a38091425f91d38386dfcc68628332dd79cc6c84b3556e6`.

## 4. The repository on disk

```text
.tit/
  objects/ab/cdef…        loose objects (section 5)
  objects/chunks/ab/cdef… the chunks of large blobs (section 5)
  packs/<hex>.pack        packs and their indexes (section 6), named by the pack's trailing hash; adopt and
  packs/<hex>.idx         repack write commits and tags, trees, and blobs in packs of their own (adopt merges
                          its batches' packs of each kind into one at its end)
  refs/heads/<name>       branches; refs/tags/<name>; refs/remotes/<remote>/<name>
  packed-refs             refs packed into one file
  changes/kz/vqt…         the change index (section 8)
  git-ids                 git id ⇄ tit id for every adopted object (section 7)
  oplog/                  the operation log (section 9)
  workspaces/<name>/      per-workspace state: HEAD, index (section 10), and:
    head-tree             "<commit> <tree>": the tree of HEAD's commit, a cache (status reads no object for it)
    snapshots/<time>      files kept before a command changed them: the command, then "<blob>\t<f|x|l>\t<path>"
                          lines (tit timeline; kept 14 days, never pushed)
    parked/<name>         tit park: the branch, the commit, its tree and the tree of the parked files
    focus                 tit focus: the directories checked out, one a line
    EDIT                  tit edit: the branch, its tip and the change being edited
  bench/<name>.jsonl      tit bench results
  mirror.git/             a mirror over HTTP(S): the git objects written, then pushed from here (section 7)
  config                  repository config (section 11)
  lock                    the repository write lock (section 12)
  purged                  tit purge's record: "<unix seconds> <hex id>... <reason>" a line (#1010)
  packs/staged/, retired/ pending and retired packs (section 16)
```

A workspace other than the first has a **file** `.tit` in its working directory: `store <absolute path of
the shared .tit>\nworkspace <name>\n`. The first workspace is named `default` and its `.tit` is the directory
itself.

## 5. Loose objects

`.tit/objects/<first 2 hex digits>/<other 62>`, holding zlib (`squash.Zlib`, level 6) of the encoding. Written
to `.tit/objects/tmp-<16 random hex>`, synced, then renamed into place; when the target already exists the
temporary file is removed (objects are immutable, so the existing one is the same). A reader inflates with a
bound of the declared size plus the header, so a lying header is `fault.LimitExceeded`, not a huge allocation.

**Large blobs in chunks (#803).** A blob over 8 MiB is written by the loose store as a *manifest*: its file at the
blob's path holds zlib of `chunks <size>\0` followed, for each chunk in order, by the chunk's SHA-256 (32 bytes) and
its length (8 bytes, little-endian). Each chunk is its own file, `objects/chunks/ab/cdef…` (named by the hex of its
SHA-256), holding zlib of its bytes. Chunks are cut by content (FastCDC: a gear rolling hash over 256 values from
splitmix64 seeded with `tit-cdc1`; no cut before 128 KiB, a 21-bit mask up to 512 KiB, a 17-bit mask after, a cut at
2 MiB at the latest), so an edit changes the chunks it touches and the rest are stored once. Reading assembles the
chunks, checks each against its hash and the whole against the blob's id. Packs and the protocol carry the whole blob.

## 6. Packs (version 1)

A pack holds many objects, some stored as deltas. It differs from git's: ids are SHA-256, and every entry
records its compressed length, so a reader never inflates an entry to find the next one.

```text
pack:   "TITPACK\0"  u32 version = 1  u64 count
        entries
        32 bytes: SHA-256 of everything before

entry:  u8 type             1 blob, 2 tree, 3 commit, 4 tag, 8 delta against an earlier entry, 9 delta against an id
        uvarint size        the object's body size (for a delta: the size of the result)
        [type 8: uvarint distance back from this entry's start to the base entry's start]
        [type 9: 32-byte base id, which may be in another pack or loose]
        uvarint clen        the compressed length
        clen bytes          zlib of the body (or of the delta)

delta:  uvarint base size   uvarint result size
        ops until the end:  0x00 uvarint n, then n bytes     insert
                            0x01 uvarint offset, uvarint n   copy n bytes of the base from offset

index:  "TITIDX\0\0"  u32 version = 1  u64 count
        256 × u32: fan-out (entries whose id's first byte is ≤ i)
        count × 32 bytes: ids, sorted
        count × u64: entry offsets in the pack, in id order
        count × u32: CRC-32 (IEEE) of each entry's bytes in the pack
        32 bytes: the pack's trailing hash   32 bytes: SHA-256 of the index before this
```

Delta chains are at most 50 deep. A reader rejects a delta whose base is missing, a chain past the bound, a
result whose size differs from the declared size, or an index whose ids are not strictly increasing.

## 7. The git-ids table

`.tit/git-ids` maps every adopted object both ways.

```text
"TITGIDS\0"  u32 version = 1  u64 count
count × (20-byte git id, 32-byte tit id), sorted by git id
count × u32: positions into the first list, sorted by tit id
32 bytes: SHA-256 of everything before
```

It is rewritten whole (temporary file, sync, rename) by each `tit adopt`.

## 8. Refs, HEAD and changes

- A **ref file** holds one line: `<hex id>\n` (names a commit) or `change <change id>\n` (a stack branch:
  names a change and follows its newest version). `main` and tags name commits; published history does not
  move.
- **HEAD** (per workspace, section 10): `ref: refs/heads/<name>\n`, or `<hex id>\n` when detached.
- **packed-refs**: `# tit packed-refs 1\n`, then `<target> <name>\n` sorted by name, where target is a hex id
  or `change:<letters>`. A loose ref file wins over a packed entry.
- **Updates** are compare-and-swap: `Update(name, old, next)` takes the ref's own lock file `<ref>.lock`
  (exclusive create; an operation also holds the repository lock of section 12 around all its updates), checks the current
  value equals `old` (`nil` = must not exist), writes the new value to `<file>.tmp`, syncs, renames, syncs the
  directory. A mismatch is `change.ErrConflict`, naming the current value.
- **The change index** `.tit/changes/<first 2 letters>/<other 30>`: one line per version,
  `<op number> <hex commit id> <unix ns>\n`, oldest first. It can be rebuilt from the commits' `change`
  headers and the oplog, so it is a cache, not the source of truth.

```tin
type Target enum {
	Commit(Id)
	Change(ChangeId)
}

type Ref struct {
	Name   str
	Target Target
}

shape RefStore {
	Get(name str) !?Ref
	List(prefix str) ![]Ref
	mut Update(name str, old ?Target, next ?Target) !    // last: a bare ! swallows the next line (#825)
}
```

## 9. The operation log

Every command that changes the repository records an **operation** before its effects become visible.

```tin
type RefChange struct {
	Name   str
	Before ?Target
	After  ?Target
}

type Op enum {
	Init
	Adopt(str)                // the git directory
	Commit(ChangeId)
	Amend(ChangeId)
	Rebase([]ChangeId)
	Merge(Id)
	Switch(str)
	Branch(str)
	Tag(str)
	Fetch(str)                // the remote
	Push(str)
	Undo(OpNumber)
	Redo(OpNumber)
	Restore(OpNumber)
	Absorb
	Split(ChangeId)
	Move(ChangeId)
	Rewrite(str)              // the command run
}

type Record struct {
	Number    OpNumber
	Time      i64             // unix ns
	Workspace str
	Command   str             // the command line, for `tit oplog`
	Op        Op
	Refs      []RefChange
	Snapshot  Id              // the tree of the working directory's changed files before the operation, or ""
}
```

Records are JSON (`argo`), one file each: `.tit/oplog/<number, 16 digits>.op`. A new variant of `Op` must
say how it is undone: `match` on `Op` has no `_` arm anywhere in tit.

**Writing an operation** (the order is the crash-safety rule):

1. Take the repository lock.
2. Write every new object (each synced). Unreferenced objects are harmless.
3. Write the record to `.tit/oplog/pending.op` and sync it.
4. Apply the ref changes and the workspace files (HEAD, index), each by temporary file, sync, rename.
5. Rename `pending.op` to `<number>.op` and sync the directory. **This is the commit point.**
6. Release the lock.

**Recovery**, when a command opens the repository and finds `pending.op`: the operation did not reach its
commit point, so every ref it moved (every ref holding its `After`) is set back to `Before`, the workspace files
are restored from the snapshot, and `pending.op` is removed. A ref still at `Before` was not reached; a ref holding
neither was never the operation's (its compare-and-swap failed) and is left alone. The repository is then exactly
as before the operation.

A ref change named `workspaces/<name>/HEAD` is that workspace's HEAD, written as its file holds it (`ref: <branch>`
or a commit): a switch records it, so undo, redo, restore and recovery move HEAD the way they move refs.

**Undo** of operation *n* is a new operation whose ref changes are *n*'s reversed. It applies only if every
ref is still at *n*'s `After`; otherwise it fails with `oplog.ErrMoved`, naming the ref and the workspace whose
later operation moved it. `tit undo` picks the newest operation of the current workspace.

## 10. Workspaces, HEAD and the index

`.tit/workspaces/<name>/HEAD` and `.tit/workspaces/<name>/index`.

```text
index:  "TITINDEX"  u32 version = 1  u64 count
        entries, sorted by path bytes:
          uvarint path length, path bytes ("/"-separated, relative to the working directory)
          u8 mode (0 File, 1 Exec, 2 Link)   u8 stage (0 normal; 1 base, 2 ours, 3 theirs in a conflict)
          32 bytes id
          u64 size  i64 mtime ns  i64 ctime ns  u64 inode  u64 device
        32 bytes: SHA-256 of everything before
```

A file whose size, mtime, ctime, inode and device all equal its entry's is not read by `status`.

## 11. Config

`.tit/config` (repository) and `$XDG_CONFIG_HOME/tit/config` or `~/.config/tit/config` (user); the repository
file wins. Lines `key = value`; `[section]` and `[section "name"]` headers; `#` comments.

| key | meaning |
|---|---|
| `user.name`, `user.email` | the person in new commits |
| `user.key` | path of the ed25519 private key (default `~/.config/tit/key`, mode 0600) |
| `remote.<name>.url` | a remote (`https://…` for `tit serve` or tinhub) |
| `mirror.url` | the git remote for `tit mirror` |

The tracked file `.tit-signers` (in the working tree, so it is versioned) lists trusted keys, one per line:
`<email> ed25519 <base64 public key>`. A commit is verified against `.tit-signers` **as it is in the commit's
first parent**, so a commit cannot add the key that signs it.

## 12. Locks

`.tit/lock` is created with exclusive create, holding `<pid> <hostname> <unix seconds>\n`. A command waits for
it up to the enclosing `within` deadline (2 s by default), then fails with `change.ErrLocked`, naming the
holder. A lock whose process no longer exists on this host is removed and taken. In Tin:

```tin
use lock = change.Lock(dir)    // `use` is `let lock = try …` plus a deferred lock.Close()
```

## 13. The seams

```tin
// package store
shape ObjectStore {
	Has(id object.Id) bool
	mut Get(id object.Id) !object.Object        // mut: a store may remember what it read (a cache, an open pack)
	mut GetRaw(id object.Id) !(object.Kind, str)    // the body, without decoding
	mut Put(o object.Object) !object.Id
	mut PutRaw(kind object.Kind, body str) !object.Id
}

// package identity
shape Clock {
	Now() i64                                   // unix ns; tests freeze it
}
```

Hot paths take a store as a type parameter (`fn walk[S store.ObjectStore](s S, …)`, monomorphized); layers
hold `dyn store.ObjectStore` (a memory store over a pack store over a lazy remote store). `Transport` is fixed
by #794 in section 15.

## 14. Faults

Each package declares its sentinels as `let ErrX = fault("…")` and wraps them with context
(`fault.Wrap(ErrCorrupt, "tree 3f2a9c: entries not sorted")`), so callers test with `fault.Is`.

| sentinel | package | meaning |
|---|---|---|
| `ErrCorrupt` | object | bytes that are not a canonical encoding |
| `ErrNotFound` | store | no object with this id |
| `ErrBadPack` | packfile | a pack or index that breaks section 6 |
| `ErrLocked` | change | the repository lock is held |
| `ErrConflict` | change | a compare-and-swap found another value |
| `ErrNoRef` | change | no such ref |
| `ErrMoved` | oplog | undo found a ref moved by a later operation |
| `ErrAmbiguous` | revwalk | a short id or change id matches more than one |
| `ErrBadRev` | revwalk | a revision that does not parse or resolve |
| `ErrDirty` | worktree | a switch would overwrite uncommitted changes |
| `ErrNoIdentity` | identity | no `user.name`, `user.email` or key |
| `ErrBadSignature` | identity | a signature that does not verify against the signers file |
| `ErrRefused` | transport | the server refused a request; the fault's text has its `Code` and message |

A panic on bad input is a bug: every decoder is tested with damaged input inside `guard` (#806).

## 15. The tit protocol, version 1

Status: **proposed** by #794, for #795 (the client, `tit/transport`), #796 (`tit serve`) and tinhub later. The
stub types are in `products/tit/transport`.

### Requests

Three requests over HTTP or HTTPS (`wire`), under a repository's URL (`remote.<name>.url`):

| request | body | answer |
|---|---|---|
| `GET <url>/tit/v1/heads` | none | the refs, the newest version of each change a ref follows, a nonce |
| `POST <url>/tit/v1/fetch` | wants and haves | a pack with what the wants reach and the haves do not |
| `POST <url>/tit/v1/push` | change versions, ref updates, a pack | which refs moved, or why nothing did |

### Message encoding

A body is a frame: `"TITP"`, u8 protocol version (1), u8 kind, u32 little-endian length *n*, *n* bytes of
header (JSON, `argo`), then for the kinds that carry objects a tit pack (section 6) to the end of the body.
Kinds: 1 heads answer, 2 fetch request, 3 fetch answer (header and pack), 4 push request (header and pack),
5 push answer, 6 error; 7 replay push, 8 replay answer, 9 replay capsule (§17). A header is at most 16 MiB. A reader rejects any other magic, version or kind with
`BadRequest`; a later version is a new path (`/tit/v2/`), never a changed frame.

```text
heads answer:   {"refs": [{"name": "refs/heads/main", "target": "<hex> | change <letters>"}],
                 "changes": [{"change": "<letters>", "version": <op>, "commit": "<hex>"}],
                 "nonce": "<base64>", "server": "tit 1", "limits": {"pack": <bytes>, "refs": <count>}}
fetch request:  {"wants": ["<hex>"], "haves": ["<hex>"]}
fetch answer:   {"objects": <count>}                                  then the pack
push request:   {"changes": [{"change": "<letters>", "replaces": "<hex> | \"\"", "commit": "<hex>"}],
                 "refs": [{"name": "refs/heads/main", "old": "<target text> | \"\"", "new": "<target text> | \"\""}]}
                                                                       then the pack
push answer:    {"refs": ["refs/heads/main"]}
error:          {"code": "<Code>", "message": "<text for a person>", "ref": "<name, when one is to blame>",
                 "nonce": "<base64, on Unauthorized and NonceExpired>"}
```

Fetch is one round: the client names as haves the tips it has (every local ref), the server ignores those it
does not know and sends a self-contained pack (no delta against an object outside it) of everything reachable
from the wants and from none of the known haves. Clone is heads, then fetch with no haves.

### Push

The server takes a push whole or not at all, under its repository lock, in one operation (`Op.Push`):

1. The signature and its nonce check (below), or `Unauthorized` / `NonceExpired`.
2. Every change's newest version on the server equals the version the push says it replaces (`""` for a new
   change), or `Moved`, naming the change: someone pushed a newer version since this client fetched. A change whose
   newest version on the server is already the pushed commit is no update and is left out: the same push sent again
   after its answer was lost, or a version a host recorded before the push that carries it.
3. Every ref holds the push's `old` value, or `RefChanged` naming the ref (compare-and-swap, as section 8).
4. No pushed commit records a conflict (#786): `Conflicted`, naming the change.
5. The pack verifies (section 6) and is complete: every pushed commit's tree, parents and blobs are in the pack
   or already on the server, or `BadPack`.
6. When the repository has a `.tit-signers` file, each pushed commit verifies against it as of its first parent
   (section 11), or `BadSignature`.
7. The pack is written, the change versions are added and the refs moved; the answer lists the refs.

The version check is the main defence against a replayed or stale push: a push recorded and sent again names
versions that have moved since.

### Authentication

The server keeps each user's ed25519 public keys (the same keys `tit key` makes). Every request may carry
`Tit-Signature: <email> <nonce> <base64 of 64 bytes>`, an ed25519 signature over

```text
"tit v1 " + method + " " + path + "\n" + nonce + "\n" + hex(SHA-256(body)) + "\n"
```

where the nonce is the one the last heads answer gave. An `Unauthorized` or `NonceExpired` error carries a fresh
nonce, so a client with no nonce yet (or an old one) signs with it and sends the request once more. Fetch and heads may go unsigned where the repository is
public; push is always signed. The nonce is `base64(u64 big-endian unix seconds, 16 random bytes,
HMAC-SHA256(server secret, the first 24 bytes))`: the server checks the HMAC and that the time is less than 60 s
old, so any node of a service checks it with no shared store, and a signed request cannot be sent again after a
minute (and within the minute, the version check above refuses it).

### Errors and limits

Errors answer with an HTTP status (400, 401, 403, 404, 409, 413 or 500) and an error frame whose code is one of
`BadRequest`, `Unauthorized`, `NonceExpired`, `NotFound`, `Moved`, `RefChanged`, `Conflicted`, `BadPack`,
`BadSignature`, `TooLarge`, `Internal` (`transport.Code`). The heads answer gives the server's limits: the
largest pack it takes (default 2 GiB) and the most refs in one push (default 10 000). The client retries heads
and fetch (`policy.Retry(3)`, each inside `within`); a push is retried only by running the command again, which
fetches first.

### In Tin

```tin
// package transport: the client (#795) and an in-process server for tests both satisfy it
shape Transport {
	mut Heads() !Heads
	mut Fetch(req FetchRequest) !(FetchAnswer, str)       // the header and the pack
	mut Push(req PushRequest, pack str) !PushAnswer        // fails with ErrRefused (its Code) on any refusal
}
```

`Frame` and `ReadFrame` write and read the message encoding, `NewNonce` and `CheckNonce` make and check nonces,
`SigningPayload` is the bytes a request signature covers.

## 16. The server's repository (#1003)

Status: **proposed** by #1003, for #1006 (tit serve on it), #1007 (repack), #1009 (streaming), #1010 (prune and
purge) and tinhub (#1011 pack store, #1016 refs in Postgres, #1018 the protocol on tinhub). The stubs are in
`products/tit/transport/repo.tin`.

The server of §15 reads and writes one `.tit` directory. tinhub keeps refs and change versions in Postgres and packs
in a pack store, and runs on several nodes. Both serve the protocol through the same code (`transport.Server`) over a
`Repo`: tit's is a `.tit` directory (`DirRepo`, #1006), tinhub's is its own (#1016).

### Packs: pending, live, retired

A pack is written once, named by its trailing hash (hex), and never modified. It is in one of three states:

| state | who reads it | how it gets there | how it leaves |
|---|---|---|---|
| pending | nobody: no ref reaches it | `Stage` | `Commit` makes it live; `Drop`, or the sweep of pending packs older than the push deadline, deletes it |
| live | every reader (`Objects`, `Packs`) | `Commit` | `Retire` |
| retired | readers that listed it before it retired | `Retire` | deleted after the grace period (default 1 hour, never less than the longest fetch) |

A `.tit` directory keeps pending packs in `packs/staged/` and retired ones in `packs/retired/`; a live pack is
`packs/<hex>.pack` with its `.idx` (the index is renamed in first, so a listed pack always has its index). tinhub
keeps a pack at `repos/<repo id>/packs/<hex>.pack` and `.idx` with its state in Postgres; no pack is shared between
repos.

### A push over a Repo

1. The request's pack is streamed to a file under `TempDir()` and verified and indexed where it lies (§6,
   `packfile.KeepFile`, #1009); the signer and nonce are checked before the body is read, the signature after.
2. Checks without the lock, against what the repository holds now: each change's newest version is the one the push
   replaces (`Moved`), each ref holds the push's `old` (`RefChanged`), every pushed commit is complete, has no
   conflict and verifies against the signers file (`BadPack`, `Conflicted`, `BadSignature`).
3. `Stage`: the pack becomes pending. Data first.
4. `Commit`: under the repository lock and as one change, the checks of step 2 on versions and refs are made again
   (another push may have landed since), then the pack becomes live, the refs move and the versions are added. A
   mismatch refuses the push and changes nothing; the server then `Drop`s the pack.

The lock is held only for step 4. A crash before step 4 leaves a pending pack that nothing reaches; a crash inside it
leaves the repository as before or after the push (tit: the oplog's pending operation, §9; tinhub: the transaction).

A fetch's answer is streamed (`packfile.ReuseTo`, #1009): entries that go as they are stored are copied from the
pack's file a window at a time (1 MiB), each window freed before the next, so a clone of a file of any size costs the
server a window, not the file (#1076). Only objects that go whole (a delta whose base is not sent, a loose object) are
held in memory, one at a time.

Repack (#1007) and prune (#1010) use the same calls: write a pack, `Stage`, `Commit` an update with no refs and no
changes (the pack becomes live), then `Retire` the packs it replaces. Prune never removes an object younger than its
retention, so a push whose checks saw an object still finds it at `Commit`.

### In Tin

```tin
// package transport

// Staged is a pending pack: its trailing hash, hex.
type Staged struct {
	Name str
}

// Update is what Commit makes current in one step.
type Update struct {
	Who     str              // the email the request was signed as ("" for tit's own commands)
	Command str              // for the record: "push by ada@example.com", "repack", "prune"
	Changes []ChangeUpdate   // §15: each change's new commit and the version it replaces
	Refs    []RefUpdate      // §15: each ref's old and new target, as a ref file writes them
	Pack    str              // a staged pack's name, "" for none
	Time    i64              // unix ns: the new versions' time
}

// Repo is what a server needs of one repository.
shape Repo {
	mut Ref(name str) !?change.Ref
	mut Refs(prefix str) ![]change.Ref
	mut Newest(c object.ChangeId) !?change.Version
	mut Objects() !store.Layered           // every live object
	mut Packs() ![]packfile.Pack           // the live packs, opened (for packfile.Reuse)
	TempDir() str                          // where a pack is written before Stage, on the store's own file system
	mut Stage(pack str) !Staged            // pack: a written, verified .pack path with its .idx beside it; moved in, pending
	mut Commit(u Update) !i64              // the operation number the new versions record (§8)
	mut Drop(s Staged) !bool               // deletes a pending pack; false when it was gone
	mut Retire(names []str) !i64           // live packs out of new readers' sight; how many were live
}
```

`Commit` refuses with `ErrRefused` and the §15 code in its text (`Moved: <change> …`, `RefChanged: <ref> …`), as
the checks of step 2 do, so `Server.Handle` answers both the same way. A version's operation number is the oplog's
(tit) or the repository's push sequence (tinhub); it only orders the versions of one change.

The packages tinhub imports from tit (`object`, `store`, `packfile`, `change`, `transport`, `semantic`, `diff`,
`merge`, `revwalk`) keep no process-wide state that two repositories in one process would share.

## 17. Replay capsules over the protocol (#1027)

Status: **proposed** by #1027, for #1022 (tinhub's capsule store, which serves the same requests). `tit serve` serves
them too, from `.tit/replay/`. Only version 2 capsules (design/interface_replay.md §6.1) are taken: their plain
summary and signature are readable without a key, so a server stores and groups capsules it cannot open.

| request | body | answer |
|---|---|---|
| `POST <url>/tit/v1/replay` | kind 7 (replay push): header `{"commit": "<hex>", "name": "<file name>"}`, then the capsule | kind 8: `{"id": "<hex>", "group": "<hex>", "new": true}` |
| `GET <url>/tit/v1/replay` | none | kind 8: `{"capsules": [{"id", "group", "name", "commit", "signer", "summary", "time"}]}`, newest first |
| `GET <url>/tit/v1/replay/<id>` | none | kind 9: `{"id", "name", "commit"}`, then the capsule |

- Every replay request is signed (§15 Authentication), even on a public repository: capsules hold real requests.
- **id**: the hex SHA-256 of the capsule's bytes. A capsule pushed again answers `"new": false`, so an interrupted
  upload simply runs again. **group**: the hex SHA-256 of the signer and the summary, so the same failure from the
  same server falls in one group.
- The server refuses a version 1 capsule, a damaged one, and one whose signature does not verify against the signer
  it names (`BadRequest`), and one larger than 64 MiB (`TooLarge`).
- `tit serve` keeps `.tit/replay/<id>.tcap` and `.tit/replay/<id>.json` (the answer's fields).

```text
tit replay push [--remote R] [--commit REV] [SPOOL]   send every .tcap in SPOOL (TIN_REPLAY_DIR); each sent file is removed
tit replay ls [--remote R] [--group]                  the capsules (or the groups, with counts), newest first
tit replay ID [--remote R] [--against BUILD [tin replay's options]]
                                                      fetch a capsule (an id or a unique prefix) to .tit/replay/ID.tcap;
                                                      with --against, run `tin replay` on it (TIT_TIN, or tin on the PATH)
```

## 18. Signing in and adding keys (#1017)

Status: **proposed** by #1017, for #1012 (tinhub's accounts). `tit serve` serves both, for its allowed keys.

**Sign-in with a key, no password.** A browser (or anything that wants a session) asks for a login request; the
holder of a registered key approves it with a signed request.

| request | body | answer |
|---|---|---|
| `POST <url>/tit/v1/login` | none (unsigned) | `{"code": "ABCD-EFGH", "url": "<where a browser waits>", "expires": <unix s>}` |
| `POST <url>/tit/v1/login/<code>` | none, **signed** | `{"email": "<who approved>"}` |
| `GET <url>/tit/v1/login/<code>` | none | `{"state": "pending" \| "approved" \| "expired", "email": "…"}` |

A code is 8 letters from an alphabet without look-alikes (`ABCDEFGHJKMNPQRSTUVWXYZ23456789`), lives 10 minutes and is
approved once: a second approval, or one after it expired, is refused (`Moved`). tinhub turns an approved code into
the browser's session; `tit serve` only records it (`.tit/logins/<code>`). These answers are plain JSON
(`application/json`), not frames: a browser reads them.

**Adding a key with an invite.** `tit invite [--email ADDRESS]`, run in a served repository, makes a one-use code
(`.tit/invites/<code>`, valid 7 days). `tit key add URL CODE` sends `POST <url>/tit/v1/keys` with
`{"email", "key": "<base64 ed25519 public key>", "invite": "<code>"}`, signed by that key (it proves the sender holds
it); the server checks and spends the invite and adds the key to its allowed keys (`.tit/allowed-keys` for `tit
serve`). A spent, expired or unknown invite is refused (`Unauthorized`).

```text
tit login [URL or remote]     ask for a login request, approve it with your key, and print the code and the URL
tit invite [--email ADDRESS]  (in a served repository) a one-use invite for tit key add
tit key add URL CODE          register this machine's key at URL with an invite
```
