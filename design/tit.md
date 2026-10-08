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

A fifth kind for large files split into chunks is reserved for #803, which adds it with its own approval.

**Worked example.** The blob `hello, tin!\n` (12 bytes) encodes as `blob 12\0hello, tin!\n` and has id
`ebc9953649c58cea3a38091425f91d38386dfcc68628332dd79cc6c84b3556e6`.

## 4. The repository on disk

```text
.tit/
  objects/ab/cdef…        loose objects (section 5)
  packs/<hex>.pack        packs and their indexes (section 6), named by the pack's trailing hash
  packs/<hex>.idx
  refs/heads/<name>       branches; refs/tags/<name>; refs/remotes/<remote>/<name>
  packed-refs             refs packed into one file
  changes/kz/vqt…         the change index (section 8)
  git-ids                 git id ⇄ tit id for every adopted object (section 7)
  oplog/                  the operation log (section 9)
  workspaces/<name>/      per-workspace state: HEAD, index (section 10)
  config                  repository config (section 11)
  lock                    the repository write lock (section 12)
```

A workspace other than the first has a **file** `.tit` in its working directory: `store <absolute path of
the shared .tit>\nworkspace <name>\n`. The first workspace is named `default` and its `.tit` is the directory
itself.

## 5. Loose objects

`.tit/objects/<first 2 hex digits>/<other 62>`, holding zlib (`squash.Zlib`, level 6) of the encoding. Written
to `.tit/objects/tmp-<16 random hex>`, synced, then renamed into place; when the target already exists the
temporary file is removed (objects are immutable, so the existing one is the same). A reader inflates with a
bound of the declared size plus the header, so a lying header is `fault.LimitExceeded`, not a huge allocation.

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
by #794 in its own section of this file.

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
| `ErrNotFastForward` | transport | the server's branch moved |

A panic on bad input is a bug: every decoder is tested with damaged input inside `guard` (#806).
