# tit: the command reference

tit is version control written in Tin (`products/tit`; [the README](../../products/tit/README.md) is the tour,
[design/tit.md](../../design/tit.md) the formats). This page lists every command and flag.

<!-- The switch rows are plain text, not code spans: the docs check reads a code span that says switch as edition 0
Tin. -->

```sh
tit [-C dir] [--json] [--trace] [--no-color] <command> [arguments]
```

| option | meaning |
|---|---|
| `-C dir` | run as if started in `dir` |
| `--json` | print data as JSON: `status`, `log` (one object per commit), `diff` (one per file), `adopt`, `oplog` |
| `--trace` | report the command's duration (through `policy.Trace`) |
| `--no-color` | no colour even on a terminal |

Output longer than the terminal goes through `$PAGER` (`less -FRX`); `TIT_NO_PAGER=1` turns that off. The exit
status is 0, 1 for a failure, 2 for a usage error.

## Revisions

`HEAD`; a branch, tag or remote-tracking branch (`main`, `v1`, `origin/main`); a full or short commit id; a change id
(letters `k` to `z`, as `tit log --format=change` prints them); `git:<hex>` for a commit adopted from git (a short git
id also works where it is not ambiguous). Each may be followed by `~N` (the Nth first-parent ancestor) or `^N` (the
Nth parent). `A..B` is what `B` has that `A` lacks; `A...B` what either has that the other lacks. An annotated tag
stands for the commit it tags.

## Repositories and settings

| command | |
|---|---|
| `tit init [dir]` | start a repository (`.tit/`) in `dir` |
| `tit config get <key>` | a setting |
| `tit config set [--user] <key> <value>` | set one in the repository (or, with `--user`, in `~/.config/tit/config`) |
| `tit config list` | every setting |
| `tit key [path]` | make your signing key if there is none (`~/.config/tit/key`, mode 600) and print its public half |
| `tit version [-v]` | the version (`-v`: and the faults the packages declare) |

Settings: `user.name`, `user.email`, `user.key`, `remote.<name>.url`, `mirror.url`.

## Saving work

| command | |
|---|---|
| `tit status [-s]` | staged, unstaged and untracked files; `-s` is git's short form |
| `tit add <paths>` | stage files; a directory stages what is below it (not what `.gitignore` ignores); `.` stages everything, deletions too |
| `tit rm [-r] [-f] [--cached] <paths>` | stop tracking files and remove them; `--cached` keeps them on the disk; `-f` removes one with unstaged changes |
| `tit mv <from> <to>` | move or rename a tracked file or directory |
| `tit commit [-m msg]... [-a] [--amend]` | record the staged files; each `-m` is a paragraph; `-a` stages every tracked change first; without `-m`, `$TIT_EDITOR` or `$EDITOR` asks; `--amend` makes a new version of the last change (same change id) |
| `tit diff [--staged] [--stat] [A B] [paths]` | unstaged changes, staged ones, or between two revisions, in git's patch format |
| `tit log [rev] [-n N] [--oneline] [--first-parent] [--topo] [--format=id\|change\|subject\|git-id]` | the history |
| `tit show [rev]` | a commit, its `--stat` and its diff |
| `tit cat <rev or id>` | an object's content |

## Branches and tags

| command | |
|---|---|
| `tit branch` | the branches, `*` on the current one |
| `tit branch <name> [start]` | make a branch |
| `tit branch -d <name>` / `-D` | delete a branch merged into HEAD / any branch |
| `tit branch -m <old> <new>` | rename a branch |
| tit switch *branch* | move the working directory to a branch; refuses, changing nothing, over unsaved or staged changes it would lose |
| tit switch -c *new* [*start*] | make a branch and switch to it |
| tit switch --detach *rev* | go to a commit with no branch |
| `tit merge <rev> [-m msg]` | fast-forward, or a merge commit (`Merge branch 'x'`); on conflicts the files get git's markers and `status` shows `UU`; `tit add` each file, then `tit commit` |
| `tit merge --abort` | put the index and the files back at HEAD |
| `tit tag` | the tags |
| `tit tag <name> [rev]` | a lightweight tag |
| `tit tag -a <name> -m <msg> [rev]` | an annotated tag, signed when you have a key |
| `tit tag -d <name>` | delete a tag |

## Workspaces

| command | |
|---|---|
| `tit workspace new <name> [dir] [--from rev]` | another working directory on this store (`../<repo>-<name>` by default), on its own new branch from `main` (or `rev`), with its own HEAD, index and undo |
| `tit workspaces` | the workspaces, `*` on this one, with their branch and directory |
| `tit workspace rm <name>` | forget a workspace and remove its directory, when nothing in it is uncommitted; its branch stays |
| `tit who <path> [-n N]` | who changed a file or directory in the last N commits (500), most changes first |

## Undo

| command | |
|---|---|
| `tit undo [--op N]` | reverse the last operation of this workspace (or operation N); refuses when a later operation moved the same refs, naming it |
| `tit redo` | apply again what the last undo reversed |
| `tit oplog` | the operations, newest first (undone ones marked) |
| `tit oplog show N` | one operation and its ref changes |
| `tit oplog restore N [--yes]` | every ref as it was after operation N; `--yes` when that also reverses other workspaces' operations |

## Sharing

| command | |
|---|---|
| `tit remote` / `tit remote add <name> <url>` | the remotes, or add one |
| `tit clone <url> [dir]` | copy a repository and check out `main` (or the first branch) |
| `tit fetch [remote]` | bring a remote's new commits in, as `refs/remotes/<remote>/*` |
| `tit push [remote] [branch]` | send a branch (origin and the current branch by default); refuses when the server's branch has moved on (`tit pull` first) |
| `tit pull [remote]` | fetch, then merge `<remote>/<current branch>` |
| `tit serve [--addr host:port] [--public] [--allow file]` | serve this repository; `.tit/allowed-keys` (or `--allow`) lists the keys that may sign, one `<email> ed25519 <base64>` a line; `--public` lets clone and fetch go unsigned |

## git

| command | |
|---|---|
| `tit adopt [git dir]` | bring a git repository in (the first time: start a tit repository in its working directory); again later: only what is new |
| `tit mirror [git dir]` | write every branch and tag into a git directory as git objects (remembered as `mirror.url`) |

## Safety

| command | |
|---|---|
| `tit guard [paths]` | search the tracked files (or these) for private keys and access tokens; `tit commit` runs it on what it adds or changes; `.tit-guard-allow` lists paths it leaves alone |
| `tit verify [rev or range]` | check each commit's signature against `.tit-signers` as of its parent |
| `tit bisect --run <cmd> --good <rev> [--bad <rev>]` | binary-search the first-parent line for the first change where `cmd` fails (status 125 skips a commit) |

## Files

| path | |
|---|---|
| `.tit/` | the repository: `objects/` and `packs/`, `refs/`, `changes/`, `oplog/`, `workspaces/<name>/` (HEAD, index), `config`, `git-ids` |
| `.gitignore` | ignore rules, as git reads them (tit reads `.tit/info/exclude` and `~/.config/tit/ignore` too) |
| `.tit-signers` | trusted keys, `<email> ed25519 <base64>` a line; a commit is checked against the file as of its parent |
| `.tit-guard-allow` | paths (ignore patterns) `tit guard` does not search |
