#!/bin/sh
# tit mirror against git itself (#805): after adopting a git repository and committing in tit, mirroring into an
# empty bare repository gives objects git fsck accepts, a clone with tit's files, the adopted commits under their
# original ids, the tit commit with its Change-Id, the same ids from a second mirror, and nothing to do the second
# time. Usage: mirror.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export GIT_CONFIG_NOSYSTEM=1 HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
export GIT_AUTHOR_NAME="Ada Lovelace" GIT_AUTHOR_EMAIL=ada@example.com GIT_COMMITTER_NAME="Ada Lovelace" GIT_COMMITTER_EMAIL=ada@example.com
export GIT_AUTHOR_DATE="1700000000 +0200" GIT_COMMITTER_DATE="1700000000 +0200"
fail() {
	echo "FAIL mirror: $*"
	exit 1
}
t() { "$tit" "$@"; }

mkdir "$d/g"
cd "$d/g"
git init -q -b main .
printf 'one\n' > a.txt
mkdir src && printf 'package main\n' > src/main.tin
printf '#!/bin/sh\n' > run.sh && chmod +x run.sh
git add -A && git commit -q -m "first"
printf 'two\n' >> a.txt && git commit -q -am "second"
git tag -a v1 -m "version one"
original=$(git rev-parse HEAD)
t adopt > /dev/null
t config set user.name "Ada Lovelace"
t config set user.email ada@example.com
printf 'three, from tit\n' >> a.txt
TIT_TIME=1700000500 t commit -am "made in tit" > /dev/null
change=$(t log --format=change -n 1)

git init -q --bare "$d/m1.git"
t mirror "$d/m1.git" > "$d/out.txt"
grep -q "^Mirrored .* objects into .*; 2 refs written\.$" "$d/out.txt" || fail "mirror said: $(cat "$d/out.txt")"
git -C "$d/m1.git" fsck --strict > "$d/fsck.txt" 2>&1 || fail "git fsck: $(cat "$d/fsck.txt")"
echo "ok git fsck"
git clone -q "$d/m1.git" "$d/c1"
for f in a.txt src/main.tin run.sh; do
	cmp "$d/g/$f" "$d/c1/$f" || fail "$f differs in the clone"
done
[ -x "$d/c1/run.sh" ] || fail "run.sh lost its exec bit"
echo "ok the clone has tit's files"
[ "$(git -C "$d/c1" rev-parse HEAD~1)" = "$original" ] || fail "the adopted commit has a new git id"
[ "$(git -C "$d/c1" rev-parse 'v1^{commit}')" = "$original" ] || fail "the tag"
echo "ok adopted commits keep their git ids"
git -C "$d/c1" log -1 --format=%B | grep -qx "Change-Id: $change" || fail "the Change-Id: $(git -C "$d/c1" log -1 --format=%B)"
[ "$(git -C "$d/c1" log -1 --format=%s)" = "made in tit" ] || fail "the subject"
echo "ok the tit commit carries its Change-Id"
git init -q --bare "$d/m2.git"
t mirror "$d/m2.git" > /dev/null
[ "$(git -C "$d/m2.git" rev-parse main)" = "$(git -C "$d/m1.git" rev-parse main)" ] || fail "a second mirror gave another id"
echo "ok the same ids from a second mirror"
t mirror "$d/m1.git" > "$d/again.txt"
grep -q "^Mirrored 0 objects" "$d/again.txt" || fail "mirroring again: $(cat "$d/again.txt")"
echo "ok nothing to do the second time"
