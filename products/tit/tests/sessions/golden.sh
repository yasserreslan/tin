#!/bin/sh
# Regenerate products/tit/tests/golden/session_<name>.out for each session from git itself. Needs git.
# Usage: golden.sh [session name ...]
set -eu
cd "$(dirname "$0")/../../../.." || exit 1
names=${*:-$(cd products/tit/tests/sessions && ls *.sh | grep -v golden.sh | sed 's/\.sh$//')}
for n in $names; do
	d=$(mktemp -d)
	mkdir "$d/repo" "$d/home"
	# no user or system git config: only what the session sets
	HOME="$d/home" XDG_CONFIG_HOME="$d/home" GIT_CONFIG_NOSYSTEM=1 TZ=UTC sh "products/tit/tests/sessions/$n.sh" git "$d/repo" > "products/tit/tests/golden/session_$n.out"
	rm -rf "$d"
	echo "wrote products/tit/tests/golden/session_$n.out"
done
