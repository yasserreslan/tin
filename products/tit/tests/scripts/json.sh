#!/bin/sh
# --json (#788): every command writes JSON lines under --json; the commands that print data write one object a
# record, with the keys checked here (decoded by Perl's JSON::PP); a command that only reports writes
# {"command", "lines"}. Usage: json.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL json: $*"
	exit 1
}
t() { "$tit" "$@"; }
# keys "name" "key1 key2 ..." command...: every line is an object with exactly these keys, and there is a line
keys() {
	name=$1
	want=$2
	shift 2
	"$@" > "$d/out.json" || fail "$name failed"
	perl -MJSON::PP -e '
		my ($name, $want) = @ARGV[0, 1];
		open my $f, "<", $ARGV[2] or die;
		my $n = 0;
		while (my $l = <$f>) {
			my $o = eval { decode_json($l) } or do { print "FAIL json: $name: not JSON: $l"; exit 1 };
			ref $o eq "HASH" or do { print "FAIL json: $name: not an object: $l"; exit 1 };
			my $got = join " ", sort keys %$o;
			my $w = join " ", sort split / /, $want;
			$got eq $w or do { print "FAIL json: $name: keys $got, not $w\n"; exit 1 };
			$n++;
		}
		$n > 0 or do { print "FAIL json: $name wrote nothing\n"; exit 1 };
	' "$name" "$want" "$d/out.json" || exit 1
	echo "ok $name"
}
mkdir -p "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'one\n' > a.txt
t add a.txt
keys "commit" "command lines" t --json commit -m first
printf 'two\n' >> a.txt
t commit -am second > /dev/null
t tag v1
t branch side
t remote add origin http://127.0.0.1:1/r
printf 'draft\n' >> a.txt
t switch side > /dev/null
t park keep > /dev/null
printf 'package p\n\nfn F() i64 {\n\treturn 1\n}\n' > p.tin
t add p.tin
t commit -m "p.tin" > /dev/null
t bench record speed 10 --unit ms > /dev/null
keys "status" "staged unstaged untracked" t --json status
keys "log" "commit change parents author email time message" t --json log
keys "show" "commit change parents author email time message conflicts files" t --json show
keys "cat of a commit" "id kind size content" t --json cat HEAD
keys "cat of a tree" "id kind entries" t --json cat "$(t cat HEAD | sed -n 's/^tree //p')"
keys "config get" "key value" t --json config get user.name
keys "config list" "key value" t --json config list
keys "branch" "name commit current" t --json branch
keys "tag" "name commit" t --json tag
keys "remote" "name url" t --json remote
keys "oplog" "number time workspace command undone" t --json oplog
keys "stack" "change commit subject conflicts" t --json stack
keys "who" "name email changes last" t --json who a.txt
keys "workspaces" "name on root this" t --json workspaces
keys "timeline" "n time blob size before" t --json timeline a.txt
keys "park" "name branch commit" t --json park
keys "history" "commit time kind key old_key path old_path subject" t --json history p.F
keys "bench log" "Change Commit Value Unit Time Machine" t --json bench log speed
keys "switch" "command lines" t --json switch main
keys "help" "command lines" t --json help log
