#!/bin/sh
# Rebuild tests/corpus/merge from the Tin repository's own history: for branches of origin, the files both the
# branch and main changed since their merge base give (base, ours = main, theirs = the branch) triples, as long as
# each version is under 24 KiB; then golden/merge_corpus.out records git merge-file's verdict for each (clean, and
# the merged text's SHA-256, or conflict). Needs git and a clone with origin's branches. Usage: merge_corpus.sh [max]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
max=${1:-40}
dir=products/tit/tests/corpus/merge
rm -rf "$dir"
mkdir -p "$dir"
n=0
for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin | grep -v HEAD | sort); do
	[ $n -lt $max ] || break
	base=$(git merge-base origin/main "$b" 2>/dev/null) || continue
	both=$( (git diff --name-only "$base" origin/main; git diff --name-only "$base" "$b") | sort | uniq -d | grep -E '\.(tin|md|go|sh)$' || true)
	for f in $both; do
		[ $n -lt $max ] || break
		git cat-file -e "$base:$f" 2>/dev/null && git cat-file -e "origin/main:$f" 2>/dev/null && git cat-file -e "$b:$f" 2>/dev/null || continue
		for side in base:$base ours:origin/main theirs:$b; do
			git show "${side#*:}:$f" > "$dir/$n.${side%%:*}"
		done
		big=$(wc -c < "$dir/$n.base")
		if [ "$big" -gt 24576 ] || [ "$(wc -c < "$dir/$n.ours")" -gt 24576 ] || [ "$(wc -c < "$dir/$n.theirs")" -gt 24576 ]; then
			rm -f "$dir/$n.base" "$dir/$n.ours" "$dir/$n.theirs"
			continue
		fi
		n=$((n + 1))
	done
done
out=products/tit/tests/golden/merge_corpus.out
: > "$out"
k=0
while [ $k -lt $n ]; do
	if git merge-file -p "$dir/$k.ours" "$dir/$k.base" "$dir/$k.theirs" > /tmp/merge_corpus.m 2>/dev/null; then
		echo "$k clean $(shasum -a 256 /tmp/merge_corpus.m | cut -d' ' -f1)" >> "$out"
	else
		echo "$k conflict" >> "$out"
	fi
	k=$((k + 1))
done
echo "wrote $n triples and $out"
