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
signature), so every machine mirroring it writes the same git id. `git push` from there carries it on.

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

## Safety

`tit commit` refuses a private key (RSA, EC, Ed25519 in PEM or DER, OpenSSH, tit) or an access token (GitHub, AWS,
Slack, GitLab, Stripe, Google, Anthropic, npm, PyPI) in any file it adds or changes; `tit guard` searches the tracked
files for them, and `.tit-guard-allow` lists paths it leaves alone. `tit verify` checks commit signatures against
`.tit-signers` as of each commit's parent. `tit bisect --run <cmd> --good <rev>` finds the change that broke `cmd`.

## Tests

`sh products/tit/tests/run.sh bin/tinc` runs everything: each package's tests, the programs and sessions compared with
git's output, and the scripts (`adopt`, `undo`, `share`, `mirror`, `guard`, `bisect`), which need git, openssl and
ssh-keygen.
