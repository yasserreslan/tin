# tit

Version control written in Tin. tit keeps history the way git does (content-addressed objects, trees, commits,
branches), with SHA-256 ids, and adds what git leaves to discipline: every operation is recorded and can be undone,
every change has an id that survives amending, and a git repository comes in (and goes back out) without losing a
byte. The design, formats and seams are in [design/tit.md](../../design/tit.md); every command and flag is in
[toolchain/docs/TIT.md](../../toolchain/docs/TIT.md); the milestone is [#769](https://github.com/yasserreslan/tin/issues/769).

```sh
bin/tinc -o tit products/tit/main.tin
```

Linux is where tit runs as a server (`tit serve`); the command works on Linux and macOS alike.

## A first repository

```sh
tit init
tit config set --user user.name "Ada Lovelace"
tit config set --user user.email ada@example.com
tit add .
tit commit -m "first"
tit log --oneline
```

`tit status -s`, `tit diff`, `tit diff --staged` and `tit diff --stat` print what git prints for the same changes,
apart from the ids (the scripts in `tests/sessions` run one session with both and compare). `tit commit --amend` makes a new version
of the same change: the change id stays, the commit id changes.

## Coming from git

```sh
cd my-git-project
tit adopt
tit log --format=git-id      # the same commits, line for line, as git log --format=%H
```

`tit adopt` brings in every branch, tag and remote-tracking branch, checks every object it converts by writing it
back in git's format and comparing ids, and is one operation (undoable). Run it again after new git commits and it
takes only those. A shallow clone is refused (`git fetch --unshallow` first); a git worktree adopts.

Going the other way, `tit mirror <git dir>` writes every branch and tag into a git directory: adopted commits keep
their original git ids, and a commit made in tit gets one fixed encoding (its message, then a `Change-Id:` line, no
signature), so every machine mirroring it writes the same git id. `git push` from there carries it on, or tit pushes
itself: `TIT_MIRROR_TOKEN=... tit mirror https://github.com/owner/repo.git` speaks git's smart HTTP protocol and sends
only what the server lacks.

## Never losing work

```sh
tit undo          # the last operation of this workspace
tit redo
tit oplog         # every operation, newest first
tit oplog restore 12
```

Every command that moves a branch, a tag or HEAD is one operation in `.tit/oplog`. Undoing a commit keeps its
changes in the working directory, staged; undoing a switch moves the files back (and refuses, changing nothing, if
that would overwrite an unsaved edit). An operation interrupted by a crash is rolled back when the next one begins.

## Branches

```sh
tit switch -c feature
tit commit -am "work"
tit switch main
tit merge feature        # fast-forward, or a merge commit; conflicts get git's markers
tit tag -a v1 -m "one"   # signed when you have a key
```

## Sharing

```sh
tit key                                 # your ed25519 key; prints the public half
tit serve --addr 0.0.0.0:8740           # keys allowed to sign: .tit/allowed-keys
tit clone http://host:8740/ project
tit push
tit pull
```

Requests are signed with your key over a nonce the server gives, and a push is taken whole or not at all: it names
the server's old value of each ref (and the version each change replaces), so a race or a replay is refused instead
of overwriting someone's work. The protocol is section 15 of the design.

## Stacks and sync

```sh
tit switch -c feature
tit commit -am "one"; tit commit -am "two"; tit commit -am "three"
tit stack                     # the changes above main, oldest first
tit absorb                    # each uncommitted fix goes into the change that wrote those lines
tit edit <change>             # amend a change in the middle: tit commit --amend rebases what is above it
tit move <change> --before <change>
tit split <change> <paths>
tit sync                      # fetch, rebase the stack onto origin's main, push it
```

A stack never stops halfway: a rebase that meets a conflict records it in the change (`tit stack` shows it), and
`tit sync` will not push it until `tit edit` resolves it. Each of these is one operation, so one `tit undo` reverses
it. `tit rewrite --all '<command>'` runs a command (a formatter, say) on every change of every branch, again as one
operation.

## Workspaces, snapshots and park

```sh
tit workspace new issue-42    # ../<repo>-issue-42, on its own branch, sharing this store, with its own undo
tit who products/tit          # who changed this area lately
tit timeline notes.txt        # versions of a file kept before each command that changed files
tit timeline notes.txt 3      # put version 3 back
tit watch                     # keep every save, until Ctrl-C
tit park wip                  # put the uncommitted changes aside (untracked files too), branch remembered
tit unpark wip
```

## Tin-aware history

```sh
tit diff --semantic           # added, removed, changed, moved and renamed declarations
tit history quarry.ReadFile   # the commits that changed one declaration, through renames and moves
tit overlap                   # other branches changing the declarations this one changes
```

A merge of `.tin` files whose lines conflict is tried again declaration by declaration: two branches adding
different functions at the same place merge cleanly, and a conflict names the function both changed.

## Large repositories

Files over 8 MiB are stored in content-defined chunks, so an edit stores what it touched. `tit clone --lazy` takes
the history now and file contents when something reads them; `tit focus <dir>...` checks out only those
directories (with a lazy clone, the rest is never fetched).

## Releases and benchmarks

```sh
tit ship v1.2.0                         # tag with the changelog since the last tag, signed, pushed
tit bench record parse 812 --unit ms    # kept with the change and the machine
tit bench compare parse HEAD~3 HEAD     # never macOS against Linux, never two machines
```

## Safety

`tit commit` refuses a private key (RSA, EC, Ed25519 in PEM or DER, OpenSSH, tit) or an access token (GitHub, AWS,
Slack, GitLab, Stripe, Google, Anthropic, npm, PyPI) in any file it adds or changes; `tit guard` searches the tracked
files for them, and `.tit-guard-allow` lists paths it leaves alone. `tit verify` checks commit signatures against
`.tit-signers` as of each commit's parent. `tit bisect --run <cmd> --good <rev>` finds the change that broke `cmd`.

## Tests

`sh products/tit/tests/run.sh bin/tinc` runs everything: each package's tests, the programs and sessions compared with
git's output, and the scripts in `tests/scripts` (which need git, openssl, perl and ssh-keygen): crash points at every
write, damaged inputs of every format, sharing over `tit serve`, and the rest. `tests/tinrepo.sh` checks tit against
git on a full clone of the Tin repository (adopt, revisions, status, switches, merges, mirror, clone, lazy clone); it
takes minutes and is run by hand on Linux.
