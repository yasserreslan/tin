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
| `--json` | print JSON lines: one object a record for the commands that print data (`status`, `log`, `show`, `cat`, `diff`, `config`, `branch`, `tag`, `remote`, `oplog`, `stack`, `who`, `workspaces`, `timeline`, `park`, `history`, `bench log`, `adopt`); any other command's report as `{"command": ..., "lines": [...]}` |
| `--trace` | report the command's duration (through `policy.Trace`) |
| `--no-color` | no colour even on a terminal |

Output longer than the terminal goes through `$PAGER` (`less -FRX`); `TIT_NO_PAGER=1` turns that off. The exit
status is 0, 1 for a failure, 2 for a usage error, 130 after Ctrl-C. `tit help <command>` (or `tit <command> --help`)
prints one command's usage.

## A session

<!-- tests/scripts/docs.sh runs this block, line by line, in an empty directory: keep it working. -->

```sh
tit init
tit config set user.name "Ada Lovelace"
tit config set user.email ada@example.com
printf 'one\n' > notes.txt
tit add notes.txt
tit commit -m "first"
tit switch -c idea
printf 'two\n' >> notes.txt
tit commit -am "second"
tit stack
tit switch main
tit merge idea
tit log --oneline
tit undo
tit timeline notes.txt
tit park --help
tit status -s
```

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
| `tit add -p [paths]` | stage the changes of tracked files hunk by hunk, answering each question with a line: `y` stage it, `n` leave it, `a` it and the rest of the file, `d` none of the rest of the file, `q` stop |
| `tit rm [-r] [-f] [--cached] <paths>` | stop tracking files and remove them; `--cached` keeps them on the disk; `-f` removes one with unstaged changes |
| `tit mv <from> <to>` | move or rename a tracked file or directory |
| `tit commit [-m msg]... [-a] [--amend]` | record the staged files; each `-m` is a paragraph; `-a` stages every tracked change first; without `-m`, `$TIT_EDITOR` or `$EDITOR` asks; `--amend` makes a new version of the last change (same change id) |
| `tit diff [--staged] [--stat] [--word] [--semantic] [A B] [paths]` | unstaged changes, staged ones, or between two revisions, in git's patch format (each hunk header with the function line above it, as git's); `--word` as git's `--word-diff=plain`; `--semantic` by Tin declaration |
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
| `tit ship <version> [--remote name] [--no-push] [--dry-run]` | a release: an annotated tag on HEAD whose message is the changelog (the first line of every change since the last tag, oldest first), signed when you have a key, then the branch and the tag pushed (`origin` by default); refuses uncommitted changes; `--dry-run` prints the changelog |

## Workspaces

| command | |
|---|---|
| `tit workspace new <name> [dir] [--from rev]` | another working directory on this store (`../<repo>-<name>` by default), on its own new branch from `main` (or `rev`), with its own HEAD, index and undo |
| `tit workspaces` | the workspaces, `*` on this one, with their branch and directory |
| `tit workspace rm <name>` | forget a workspace and remove its directory, when nothing in it is uncommitted; its branch stays |
| `tit who <path> [-n N]` | who changed a file or directory in the last N commits (500), most changes first |

## Stacks

A stack is the current branch's changes above the trunk (`stack.trunk`, `main` by default). Each command below that
rewrites a change rebases every change above it, as one operation (one undo). A conflict does not stop the rebase:
the new version records it, `tit stack` shows it, and `tit edit` puts git's markers back on the disk.

| command | |
|---|---|
| `tit stack` | the changes above the trunk, oldest first, with the conflicts they record |
| `tit edit <change>` | go to a change to amend it (its conflicts as markers and index stages); `tit commit --amend` makes the new version, rebases the changes above it and goes back to the branch |
| `tit move <change> --before\|--after <change>` | reorder the stack |
| `tit split <change> <paths> [-m msg]` | a new change, just below, holding what the change did to these paths |
| `tit absorb` | each uncommitted hunk goes into the change that wrote those lines; a hunk no single change owns stays (listed) |
| `tit sync [remote]` | fetch, rebase the stack onto `<remote>/<trunk>` (the local trunk follows), push the branch; nothing that records a conflict is pushed |
| `tit rewrite [--stack\|--all] <cmd>` | run `cmd` (through `/bin/sh`) on each change's own files, in a temporary directory: what it leaves is the change's new version, and the changes above follow. `--stack` (the default) is this branch's stack, `--all` every branch above the trunk; a change where it fails is listed and kept; one undo reverses it all |

## Tin declarations

| command | |
|---|---|
| `tit diff --semantic [A B]` | the changes as declarations: added, removed, changed, moved (to another file) and renamed (same body, new name); a reformat is no change |
| `tit history <[package.]Name\|Type.Method> [-n N]` | the commits that changed one declaration, newest first, following its renames and moves (in the last N commits) |
| `tit overlap` | the other branches (and fetched remote ones) above the trunk that change declarations this branch changes, and which |

A merge of `.tin` files whose lines conflict is tried again declaration by declaration (design/tit.md): different
declarations added or changed on each side merge cleanly, and a conflict names the declaration both sides changed.

## Benchmarks

| command | |
|---|---|
| `tit bench record <name> <value> [--unit u]` | keep a result for HEAD's change, with this machine (OS, architecture, CPU, cores, kernel) |
| `tit bench log <name>` | every result of a benchmark |
| `tit bench compare <name> <A> <B>` | the newest result of each revision's change and their ratio; refuses macOS against Linux, and two machines |

## Undo

| command | |
|---|---|
| `tit undo [--op N] [--yes]` | reverse the last operation of this workspace (or operation N); refuses when a later operation moved the same refs, naming it. Undoing a push puts the server's branch back (by the same compare-and-swap as a push) and asks for `--yes` first |
| `tit redo` | apply again what the last undo reversed |
| `tit oplog` | the operations, newest first (undone ones marked) |
| `tit oplog show N` | one operation and its ref changes |
| `tit oplog restore N [--yes]` | every ref as it was after operation N; `--yes` when that also reverses other workspaces' operations |
| `tit timeline <file> [N]` | the versions of a file kept before each command that changes files (and by `watch`), newest first; with N, puts version N back (keeping the file it replaces as a new version) |
| `tit watch [--every ms] [--for s]` | keep each saved version of the changed files, polling every `ms` (1000), until Ctrl-C (or for `s` seconds) |
| `tit park [name]` | put every uncommitted change aside under `name` (untracked files too) and go back to the last commit; without a name, list what is parked |
| `tit unpark <name>` | bring parked changes back, unstaged, on the branch they were parked on; merged onto it when it moved since |

Kept versions stay in `.tit/workspaces/<ws>/snapshots` for 14 days. They never leave the machine (no commit names
their blobs), a file `tit guard` flags is never kept, and neither is a file over 16 MiB.

## Large files

A file over 8 MiB is stored in content-defined chunks (design/tit.md section 5): an edit stores the chunks it
touches, not the whole file again. Nothing changes in how the file is used; commits and git mirrors see one blob.

| command | |
|---|---|
| `tit focus <dir>...` | check out only these directories (and the files at the root); the others stay in every commit as they are, and status, diff, `add` and `commit -a` leave them alone; refuses, changing nothing, when a file leaving the focus has changes |
| `tit focus` | the directories in focus |
| `tit focus --all` | check out everything again |
| `tit repack` | every pack into two, commits, trees and tags in one and blobs in the other (a log reads only the first), its deltas searched again and its entries compressed on every core (`TIN_CORES` sets how many); adopt leaves two packs for each batch it converts |

With `tit clone --lazy`, a focus also limits what is fetched: files outside it are never read.

## Sharing

| command | |
|---|---|
| `tit remote` / `tit remote add <name> <url>` | the remotes, or add one |
| `tit clone [--lazy] <url> [dir]` | copy a repository and check out `main` (or the first branch); `--lazy` takes every commit and tree but only the files it checks out: any other file comes from the server the first time something reads it (`lazy.remote` in the config), so `tit log` needs nothing more |
| `tit fetch [remote]` | bring a remote's new commits in, as `refs/remotes/<remote>/*` |
| `tit push [remote] [branch]` | send a branch (origin and the current branch by default); refuses when the server's branch has moved on (`tit pull` first) |
| `tit pull [remote]` | fetch, then merge `<remote>/<current branch>` |
| `tit serve [--addr host:port] [--public] [--allow file]` | serve this repository (anvil's recording applies: a request kept with `TIN_REPLAY_DIR` replays with `tin replay`); `.tit/allowed-keys` (or `--allow`) lists the keys that may sign, one `<email> ed25519 <base64>` a line; `--public` lets clone and fetch go unsigned |

## git

| command | |
|---|---|
| `tit adopt [git dir]` | bring a git repository in (the first time: start a tit repository in its working directory); again later: only what is new |
| `tit mirror [git dir or URL]` | write every branch and tag into a git directory as git objects (remembered as `mirror.url`); given an `http(s)://` URL (`https://github.com/owner/repo.git`), write them into `.tit/mirror.git` and push them there over git's smart HTTP protocol, sending only what the server lacks; `TIT_MIRROR_TOKEN` is sent as the password (a GitHub token). One way: nothing is read from the server but its refs |

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
