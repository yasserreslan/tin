#!/bin/sh
# tit against git on the Tin repository itself (#781, #783, #784, #789, #791, #805). Not part of run.sh: it needs a
# clone with the whole history (CI's checkout is shallow) and takes minutes. Run it on Linux:
#   sh products/tit/tests/tinrepo.sh <tit> <git clone of the Tin repo, full history> <empty dir>
# Every check prints "ok ..." or stops at the first "FAIL ...".
set -eu
tit=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
src=$2
d=$3
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 GIT_PAGER=cat TZ=UTC
mkdir -p "$HOME"
fail() {
	echo "FAIL tinrepo: $*"
	exit 1
}
t() { "$tit" "$@"; }
# the files of the working directory as git sees them: "<mode> <blob> <path>", sorted (no .git or .tit)
files() {
	(cd "$1" && find . \( -name .git -o -name .tit \) -prune -o \( -type f -o -type l \) -print | sed 's|^\./||' | LC_ALL=C sort |
		while IFS= read -r f; do
			if [ -L "$f" ]; then
				echo "120000 $(readlink "$f" | tr -d '\n' | git hash-object --stdin) $f"
			elif [ -x "$f" ]; then
				echo "100755 $(git hash-object "$f") $f"
			else
				echo "100644 $(git hash-object "$f") $f"
			fi
		done) | LC_ALL=C sort
}
# the files of a git tree, in the same form
gitfiles() {
	git -C "$1" ls-tree -r "$2" | sed 's/^\([0-9]*\) [a-z]* \([0-9a-f]*\)	/\1 \2 /' | LC_ALL=C sort
}
started=$(date +%s)

git clone -q --no-local "$src" "$d/repo"
cd "$d/repo"
git checkout -q -B main origin/main
for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin | grep -v HEAD | head -40); do
	git branch -q -f "${b#origin/}" "$b" 2> /dev/null || true
done
t config set --user user.name Ada
t config set --user user.email ada@example.com

# #789: adopt, every commit, git ids
at=$(date +%s)
"$tit" adopt . > "$d/adopt.txt" 2>&1 || fail "adopt: $(cat "$d/adopt.txt")"
echo "ok #789 adopt took $(($(date +%s) - at)) s"
commits=$(git rev-list --count main)
t log --format=git-id > "$d/tit.ids"
git log --format=%H > "$d/git.ids"
cmp -s "$d/tit.ids" "$d/git.ids" || fail "tit log --format=git-id is not git log --format=%H"
echo "ok #789 adopted $commits commits; tit log --format=git-id equals git log --format=%H line for line"
first=$(git rev-list --max-parents=0 main | tail -1)
t show "git:$(git rev-parse --short=8 "$first")" > /dev/null || fail "tit show of the first commit by its short git id"
echo "ok #789 tit show git:<short id> shows an adopted commit"

# #781: revisions resolve to what git rev-parse gives, through the git-ids table
merge=$(git rev-list --merges -n 1 main || true)
tag=$(git tag | head -1 || true)
revs="main HEAD HEAD~1 HEAD~10 HEAD^ HEAD^^ main~100 HEAD~$((commits - 1))"
[ -n "$merge" ] && revs="$revs $merge^1 $merge^2 $merge~3"
[ -n "$tag" ] && revs="$revs $tag $tag~2"
for b in $(git branch --format='%(refname:short)' | head -10); do
	revs="$revs $b $b~1"
done
n=0
for rev in $revs; do
	want=$(git rev-parse --verify -q "$rev^{commit}" || true)
	[ -n "$want" ] || continue
	trev=$rev
	case $rev in
	[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) trev="git:$rev" ;;
	esac
	got=$(t log -n 1 --format=git-id "$trev" 2> "$d/err.txt" || true)
	[ "$got" = "$want" ] || fail "revision $rev: tit gives $got ($(cat "$d/err.txt")), git $want"
	n=$((n + 1))
done
# short git ids, from 7 hex digits
for id in $(git rev-list main | awk 'NR % 97 == 1'); do
	got=$(t log -n 1 --format=git-id "git:$(echo "$id" | cut -c1-10)")
	[ "$got" = "$id" ] || fail "git:$(echo "$id" | cut -c1-10) gives $got"
	n=$((n + 1))
done
echo "ok #781 $n revisions resolve to what git rev-parse gives"
# the runtime says when the memory pool passes TIN_POOL_WARN_MB
TIN_POOL_WARN_MB=64 "$tit" log > /dev/null 2> "$d/log.err"
grep -q "memory pool holds" "$d/log.err" && fail "a log of the whole history: $(cat "$d/log.err")"
echo "ok #781 a log of all $commits commits stays under 64 MiB of pool memory (TIN_POOL_WARN_MB=64 says nothing)"

# #784 and #789: switching between commits sampled across history leaves exactly each commit's files
step=$((commits / 12 + 1))
k=0
for id in $(git rev-list --reverse main | awk -v s=$step 'NR % s == 0'); do
	t switch --detach "git:$id" > /dev/null || fail "switch to $id"
	files . > "$d/tit.files"
	gitfiles . "$id" > "$d/git.files"
	cmp -s "$d/tit.files" "$d/git.files" || fail "after switching to $id the files differ: $(diff "$d/git.files" "$d/tit.files" | head -5)"
	k=$((k + 1))
done
t switch main > /dev/null
files . > "$d/tit.files"
gitfiles . main > "$d/git.files"
cmp -s "$d/tit.files" "$d/git.files" || fail "back on main the files differ"
echo "ok #784 #789 switching through $k commits across history leaves exactly each commit's files"
f=""
for c in $(git log --name-only --format= main -- '*.tin' | grep . | sort | uniq -c | sort -rn | awk '{ print $2 }'); do
	if git cat-file -e "main:$c" 2> /dev/null; then
		f=$c
		break
	fi
done
old=$(git rev-list main -n 1 --skip=1 -- "$f" || true)
if [ -n "$old" ] && [ "$(git rev-parse "$old:$f")" != "$(git rev-parse "main:$f")" ]; then
	printf '\n// unsaved\n' >> "$f"
	if t switch --detach "git:$old" > "$d/out.txt" 2>&1; then
		fail "a switch over an unsaved $f"
	fi
	grep -q "$f" "$d/out.txt" || fail "the refusal does not name $f: $(cat "$d/out.txt")"
	tail -1 "$f" | grep -q unsaved || fail "the refused switch touched $f"
	git checkout -q -- "$f" 2> /dev/null || t timeline "$f" > /dev/null
	git show "main:$f" > "$f"
	echo "ok #784 a dirty file blocks the switch, naming it"
else
	fail "no file to check the dirty switch with"
fi

# #783: status equals git's after scripted changes
cp -R "$d/repo" "$d/both"
cd "$d/both"
git reset -q --hard main
t switch main > /dev/null 2>&1 || true
a=$(git ls-files | grep '\.md$' | head -1)
b=$(git ls-files | grep '\.tin$' | sed -n 5p)
c=$(git ls-files | grep '\.tin$' | sed -n 9p)
e=$(git ls-files | grep '\.sh$' | head -1)
printf 'edited\n' >> "$a"
rm "$b"
printf 'new\n' > brand-new.txt
chmod +x "$c"
ln -s "$a" a-link
mkdir -p ignored-dir
grep -qx 'ignored-dir/' .gitignore 2> /dev/null || printf 'ignored-dir/\n' >> .gitignore
printf 'x\n' > ignored-dir/x
mv "$e" "$e.moved"
git status -s --untracked-files=all > "$d/git.status"
t status -s > "$d/tit.status"
# git lists untracked files one by one with --untracked-files=all; tit collapses directories like git's default
git status -s > "$d/git.status.default"
cmp -s "$d/git.status.default" "$d/tit.status" || fail "status differs from git's:
$(diff "$d/git.status.default" "$d/tit.status")"
echo "ok #783 status equals git status -s after an edit, a delete, an add, a chmod, a symlink, an ignored file and a move"
t add . > /dev/null
git add -A .
git status -s > "$d/git.status"
t status -s > "$d/tit.status"
cmp -s "$d/git.status" "$d/tit.status" || fail "staged status differs from git's:
$(diff "$d/git.status" "$d/tit.status")"
echo "ok #783 staged, the rename shows as git shows it"

# #791: merging real branches into main gives the trees git gives
cd "$d/repo"
m=0
for br in $(git branch --format='%(refname:short)' | grep -v '^main$' | head -40); do
	gtree=$(git merge-tree --write-tree main "$br" 2> /dev/null | head -1) || continue
	[ "$(git merge-base main "$br")" = "$(git rev-parse "$br")" ] && continue
	[ "$(git merge-base main "$br")" = "$(git rev-parse main)" ] && continue
	t switch main > /dev/null
	t branch -D tinrepo-merge > /dev/null 2>&1 || true
	t switch -c tinrepo-merge > /dev/null
	t merge "$br" -m "merge $br" > "$d/merge.txt" 2>&1 || fail "git merges $br cleanly, tit does not: $(cat "$d/merge.txt")"
	files . > "$d/tit.files"
	gitfiles . "$gtree" > "$d/git.files"
	cmp -s "$d/tit.files" "$d/git.files" || fail "the merge of $br differs from git's: $(diff "$d/git.files" "$d/tit.files" | head -5)"
	m=$((m + 1))
	[ $m -lt 10 ] || break
done
t switch main > /dev/null
t branch -D tinrepo-merge > /dev/null 2>&1 || true
echo "ok #791 $m real branches merge into main to the trees git merge-tree gives"

# #805: the mirror reproduces git's ids, and a git clone of it has the same trees
git init -q --bare "$d/mirror.git"
t mirror "$d/mirror.git" > /dev/null
[ "$(git -C "$d/mirror.git" rev-parse main)" = "$(git rev-parse main)" ] || fail "the mirror's main is $(git -C "$d/mirror.git" rev-parse main), not git's"
git clone -q "$d/mirror.git" "$d/cloned"
files "$d/cloned" > "$d/cloned.files"
gitfiles . main > "$d/git.files"
cmp -s "$d/cloned.files" "$d/git.files" || fail "a git clone of the mirror has other files"
echo "ok #805 the mirror's main is git's main ($(git rev-parse --short main)), and a git clone of it checks out the same files"

# #778: the whole adopted history packed by tit serve and unpacked by tit clone gives the same commits and files
port=$((20000 + $$ % 20000))
(cd "$d/repo" && exec "$tit" serve --public --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
n=0
until t clone "http://127.0.0.1:$port/" "$d/clone" > "$d/clone.txt" 2>&1; do
	n=$((n + 1))
	[ $n -lt 100 ] || fail "clone of the adopted repository: $(cat "$d/clone.txt") $(cat "$d/serve.log")"
	rm -rf "$d/clone"
	sleep 0.2
done
# #803: a lazy clone of the Tin repo, then tit log, fetches no file contents
t clone --lazy "http://127.0.0.1:$port/" "$d/lazy" > /dev/null || fail "lazy clone"
objs() {
	find "$1/.tit/objects" "$1/.tit/packs" -type f | grep -v '/tmp-' | wc -l | tr -d ' '
}
before=$(objs "$d/lazy")
t -C "$d/lazy" log > /dev/null
[ "$(objs "$d/lazy")" = "$before" ] || fail "tit log in the lazy clone fetched objects"
[ "$(t -C "$d/lazy" log --format=id | wc -l)" = "$(t log --format=id main | wc -l)" ] || fail "the lazy clone has another history"
echo "ok #803 a lazy clone of the Tin repo, then tit log of all $commits commits, fetches no file contents"
kill $server 2> /dev/null || true
[ "$(t -C "$d/clone" log --format=id | wc -l)" = "$(t log --format=id main | wc -l)" ] || fail "the clone has another history"
[ "$(t -C "$d/clone" log --format=id -n 1)" = "$(t log --format=id -n 1 main)" ] || fail "the clone's main is another commit"
files "$d/clone" > "$d/clone.files"
gitfiles . main > "$d/git.files"
cmp -s "$d/clone.files" "$d/git.files" || fail "the clone checks out other files"
echo "ok #778 tit clone of the adopted repository over tit serve: $(sed -n 's/.*(\([0-9]*\) objects).*/\1/p' "$d/clone.txt") objects packed and unpacked, the same $commits commits and files"

echo "all checks passed in $(($(date +%s) - started)) s on $(uname -sm), $(git --version)"
