#!/bin/sh
# Regenerate golden/diff_git.out from git itself: write the pairs with programs/diff_git.tin, then ask git for each
# unified diff. Needs git. Usage: diff_git.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
"$compiler" -o "$tmp/diff_git" products/tit/tests/programs/diff_git.tin
"$tmp/diff_git" > /dev/null
out=products/tit/tests/golden/diff_git.out
: > "$out"
for name in same empty-to-text text-to-empty no-newline-old no-newline-new no-newline-both one-change far-apart close-together insert-start delete-end rewrite blocks; do
	(cd /tmp/tin-test-tit-diff && git -c core.quotepath=off diff --no-index --no-indent-heuristic --no-color -U3 "$name.old" "$name.new" || true) |
		grep -v '^diff --git \|^index \|^new file mode\|^deleted file mode' | sed 's/^\(@@ [^@]* @@\).*/\1/' | sed "s|^--- a/|--- a/|" >> "$out"
done
echo "wrote $out"
