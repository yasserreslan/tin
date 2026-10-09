#!/bin/sh
# The manual (#808): every command and every flag in tit's usage text is in toolchain/docs/TIT.md, and the manual's
# example session runs, line by line, in an empty directory. Usage: docs.sh <tit> <dir>
set -eu
tit=$1
d=$2
root=$PWD
manual="$root/toolchain/docs/TIT.md"
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL docs: $*"
	exit 1
}
mkdir -p "$HOME"
"$tit" help > "$d/usage.txt"
# the commands: the first word of each command line, and the first word after each " | " (a command of its own, or
# a subcommand of the line's command: "oplog [show N | restore N]")
sed -n 's/^  \([a-z]\)/\1/p' "$d/usage.txt" | sed 's/  .*//' | awk '{ n = split($0, a, / \| /); split(a[1], f, " "); print f[1] "\t"; for (i = 2; i <= n; i++) { split(a[i], w, " "); if (w[1] ~ /^[a-z]/) print f[1] "\t" w[1] } }' | sort -u > "$d/commands.txt"
n=0
while IFS="$(printf '\t')" read -r first alt; do
	if [ -z "$alt" ]; then
		grep -q "tit $first\b" "$manual" || fail "tit $first is not in toolchain/docs/TIT.md"
	else
		grep -q "tit $alt\b\|tit $first $alt\b" "$manual" || fail "tit $alt (or tit $first $alt) is not in toolchain/docs/TIT.md"
	fi
	n=$((n + 1))
done < "$d/commands.txt"
[ $n -gt 40 ] || fail "only $n commands found in the usage text"
# the flags
grep -o -- '--[a-z][a-z-]*\| -[a-zA-Z]\b' "$d/usage.txt" | sed 's/^ //' | sort -u > "$d/flags.txt"
f=0
while read -r flag; do
	grep -q -- "$flag" "$manual" || fail "the flag $flag is not in toolchain/docs/TIT.md"
	f=$((f + 1))
done < "$d/flags.txt"
echo "ok all $n commands and $f flags of the usage text are in the manual"
# the session: the sh block after "## A session"
awk '/^## A session/ { s = 1 } s && /^```sh/ { b = 1; next } b && /^```/ { exit } b { print }' "$manual" > "$d/session.sh"
[ "$(wc -l < "$d/session.sh")" -gt 10 ] || fail "the manual's session is missing"
mkdir "$d/session"
cd "$d/session"
k=0
while IFS= read -r line; do
	k=$((k + 1))
	cmd=$(printf '%s\n' "$line" | sed "s|^tit |\"$tit\" |")
	sh -c "$cmd" > "$d/line.out" 2>&1 || fail "line $k of the manual's session failed: $line
$(cat "$d/line.out")"
done < "$d/session.sh"
echo "ok the manual's session runs, all $k lines"
