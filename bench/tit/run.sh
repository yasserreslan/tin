#!/bin/sh
# tit against git on the Tin repository (#807): warm status, log and diff, the same working directory for both
# (tit adopts the clone in place), 7 alternating runs each, medians and the ratio tit/git (lower is faster).
# Linux only (date +%s%N). Usage: bench/tit/run.sh <tit> <git clone with full history> <scratch dir> [branch]
# (the branch of that clone to measure, main by default)
set -eu
tit=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
src=$2
mkdir -p "$3"
d=$(cd "$3" && pwd)
branch=${4:-main}
runs=7
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 GIT_PAGER=cat
mkdir -p "$HOME"
git clone -q --no-local "$src" "$d/repo"
cd "$d/repo"
git checkout -q -B main "origin/$branch"
start=$(date +%s%N)
"$tit" adopt . > /dev/null
adopt_ms=$((($(date +%s%N) - start) / 1000000))
commits=$(git rev-list --count main)
files=$(git ls-files | wc -l)
ms() {
	t0=$(date +%s%N)
	"$@" > /dev/null 2>&1
	echo $((($(date +%s%N) - t0) / 1000))
}
median() {
	sort -n | awk '{ a[NR] = $1 } END { print a[int((NR + 1) / 2)] }'
}
measure() {
	name=$1
	gitcmd=$2
	titcmd=$3
	sh -c "$gitcmd" > /dev/null 2>&1
	sh -c "$titcmd" > /dev/null 2>&1
	: > "$d/g" ; : > "$d/t"
	i=0
	while [ $i -lt $runs ]; do
		ms sh -c "$gitcmd" >> "$d/g"
		ms sh -c "$titcmd" >> "$d/t"
		i=$((i + 1))
	done
	g=$(median < "$d/g")
	t=$(median < "$d/t")
	ratio=$(awk -v t="$t" -v g="$g" 'BEGIN { if (g > 0) printf "%.2f", t / g; else print "-" }')
	printf '| %s | %s | %s | %s |\n' "$name" "$(awk -v x="$g" 'BEGIN { printf "%.1f", x / 1000 }')" "$(awk -v x="$t" 'BEGIN { printf "%.1f", x / 1000 }')" "$ratio"
}
echo "Tin repository: $commits commits on main, $files files. tit adopt: $adopt_ms ms. $runs alternating runs, medians in ms (each includes starting the process)."
echo
echo '| operation | git | tit | tit/git |'
echo '|---|---|---|---|'
measure "status (clean)" "git status" "'$tit' status"
measure "log (whole history)" "git log" "'$tit' log"
measure "log -n 1" "git log -n 1" "'$tit' log -n 1"
# packing on every core against one (#778): tit repack of the adopted history, from the same packs each time
cores=$(nproc 2> /dev/null || getconf _NPROCESSORS_ONLN)
cp -R .tit "$d/tit-packs"
repack() {
	rm -rf .tit
	cp -R "$d/tit-packs" .tit
	t0=$(date +%s%N)
	TIN_CORES=$1 "$tit" repack > /dev/null
	echo $((($(date +%s%N) - t0) / 1000))
}
: > "$d/r1"
: > "$d/rn"
for i in 1 2 3; do
	repack 1 >> "$d/r1"
	repack "$cores" >> "$d/rn"
done
r1=$(median < "$d/r1")
rn=$(median < "$d/rn")
# twenty files changed
for f in $(git ls-files '*.tin' | head -20); do
	printf '\n// changed\n' >> "$f"
done
measure "status (20 files changed)" "git status" "'$tit' status"
measure "diff (20 files changed)" "git diff" "'$tit' diff"
git checkout -q -- .
echo
echo "tit repack of the whole history (3 runs each, medians): 1 core $(awk -v x="$r1" 'BEGIN { printf "%.1f", x / 1000000 }') s, $cores cores $(awk -v x="$rn" 'BEGIN { printf "%.1f", x / 1000000 }') s: $(awk -v a="$r1" -v b="$rn" 'BEGIN { printf "%.2f", a / b }')x faster on every core"
