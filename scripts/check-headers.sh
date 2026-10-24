#!/usr/bin/env bash
# Checks that every Go file starts with the Machinist license header.

set -euo pipefail

failed=0
while IFS= read -r -d '' file; do
	if ! awk 'NR <= 30 && index($0, "This file is part of Machinist.") { found = 1 } END { exit !found }' "$file"; then
		printf '%s\n' "$file"
		failed=1
	fi
done < <(git ls-files --cached --others --exclude-standard -z -- '*.go')

exit "$failed"
