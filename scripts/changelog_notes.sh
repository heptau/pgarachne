#!/usr/bin/env bash
# Extracts the body of one version's section from CHANGELOG.md (Keep a
# Changelog format: "## [x.y.z] - date" headings), for use as GitHub release
# notes. Portable POSIX awk only — no GNU-specific tools — so it runs
# identically on macOS and CI.
#
# Usage: scripts/changelog_notes.sh <version>
#   <version> may be "1.3.0", "v1.3.0", or "Unreleased" (case-insensitive).
#
# Prints the section body to stdout (heading line excluded, leading/trailing
# blank lines trimmed). Exits non-zero if the version has no section.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHANGELOG="${SCRIPT_DIR}/CHANGELOG.md"

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>" >&2
    exit 1
fi
if [ ! -f "$CHANGELOG" ]; then
    echo "CHANGELOG.md not found at $CHANGELOG" >&2
    exit 1
fi

version="${1#v}"

# The `if` condition around the assignment lets us inspect awk's exit status
# without `set -e` aborting the script first (a bare `var=$(cmd)` assignment
# would trip errexit immediately on a non-zero exit).
if ! result="$(awk -v ver="$version" '
    /^## \[/ {
        if (printing) exit
        label = $0
        sub(/^## \[/, "", label)
        sub(/\].*/, "", label)
        printing = (tolower(label) == tolower(ver))
        if (printing) found = 1
        n = 0
        next
    }
    printing {
        n++
        buf[n] = $0
    }
    END {
        start = 1
        while (start <= n && buf[start] ~ /^[ \t]*$/) start++
        end = n
        while (end >= start && buf[end] ~ /^[ \t]*$/) end--
        for (i = start; i <= end; i++) print buf[i]
        exit (found ? 0 : 1)
    }
' "$CHANGELOG")"; then
    echo "No changelog section found for version '$version' in $CHANGELOG" >&2
    exit 1
fi

printf '%s\n' "$result"
