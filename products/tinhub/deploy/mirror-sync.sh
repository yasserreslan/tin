#!/bin/sh
# mirror-sync.sh DIR [BRANCH]: keeps a tinhub repository a read-only mirror of a git repository (phase 1: the Tin repo
# from GitHub, which stays the source of truth until the cutover; deploy/RUNBOOK.md). DIR is a git clone of the source
# whose tit remote "tinhub" is the mirror; the first run there needs `tit adopt` and `tit remote add tinhub URL` done
# (the runbook's steps). Each run fetches BRANCH (main) from the git remote origin, converts what is new with tit adopt,
# and pushes it to tinhub with the mirror account's key, the only key with write access there.
set -eu
dir=${1:?usage: mirror-sync.sh DIR [BRANCH]}
branch=${2:-main}
tit=${TIT:-tit}
cd "$dir"
git fetch --quiet --prune origin "$branch"
git checkout --quiet "$branch"
git reset --quiet --hard "origin/$branch"
"$tit" adopt > /dev/null
"$tit" push tinhub "$branch"
echo "mirror-sync: $branch at $(git rev-parse --short HEAD) is on tinhub"
