#!/bin/sh
# tit mirror to a git server over smart HTTP (#805), against git's own http-backend (tests/githttp.pl): adopted commits
# keep their git ids, a tit commit gets its fixed encoding, a second mirror sends only what is new and a third
# nothing, git fsck accepts the server's objects and a git clone of it has tit's files, and two machines mirroring the
# same commits to two servers give the same git ids. Usage: mirrorhttp.sh <tit> <dir>
set -eu
tit=$1
d=$2
root=$PWD
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL mirrorhttp: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$HOME" "$d/srv"
git init -q --bare -b main "$d/srv/a.git"
git init -q --bare -b main "$d/srv/b.git"
git -C "$d/srv/a.git" config http.receivepack true
git -C "$d/srv/b.git" config http.receivepack true
# the server takes a free port and says which: a port picked here can be one the system has in use already
perl "$root/products/tit/tests/githttp.pl" "$d/srv" 0 > "$d/srv.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
n=0
until grep -q listening "$d/srv.log" 2> /dev/null; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the git server did not start: $(cat "$d/srv.log")"
	sleep 0.1
done
port=$(sed -n 's/^listening on //p' "$d/srv.log")
url="http://127.0.0.1:$port"

mkdir "$d/r"
cd "$d/r"
git init -q -b main .
printf 'one\n' > a.txt
git add a.txt
git -c user.name=Ada -c user.email=ada@example.com commit -qm "from git"
adopted=$(git rev-parse main)
t adopt . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'two\n' >> a.txt
t commit -am "from tit" > /dev/null
t mirror "$url/a.git" > "$d/out.txt"
grep -q "^Mirrored to $url/a.git: 1 ref moved, " "$d/out.txt" || fail "the first mirror: $(cat "$d/out.txt")"
[ "$(git -C "$d/srv/a.git" rev-parse main~1)" = "$adopted" ] || fail "the adopted commit lost its git id"
git -C "$d/srv/a.git" log -1 --format=%B main | grep -q '^Change-Id: [k-z]' || fail "the tit commit has no Change-Id"
git -C "$d/srv/a.git" fsck --strict 2> "$d/fsck.txt" || fail "git fsck: $(cat "$d/fsck.txt")"
echo "ok the first mirror: adopted commits keep their ids, git fsck accepts the rest"

printf 'three\n' >> a.txt
t commit -am "again from tit" > /dev/null
t mirror > "$d/out.txt"
grep -q "1 ref moved, 3 objects sent" "$d/out.txt" || fail "the second mirror: $(cat "$d/out.txt")"
t mirror > "$d/out.txt"
grep -q "^Everything is mirrored" "$d/out.txt" || fail "the third mirror: $(cat "$d/out.txt")"
git clone -q "$d/srv/a.git" "$d/clone"
cmp -s "$d/clone/a.txt" a.txt || fail "a git clone of the server has other files"
echo "ok a second mirror sends what is new, a third nothing, and a git clone has tit's files"

# another machine: the same repository copied, mirrored to another server
cp -R "$d/r" "$d/other"
rm -rf "$d/other/.tit/mirror.git"
t -C "$d/other" mirror "$url/b.git" > /dev/null
[ "$(git -C "$d/srv/b.git" rev-parse main)" = "$(git -C "$d/srv/a.git" rev-parse main)" ] || fail "two machines gave two git ids"
echo "ok two machines mirroring the same commits give the same git ids"
