#!/usr/bin/env bash
# =============================================================================
# prepare_release.sh — Bump VERSION, roll CHANGELOG.md's [Unreleased] section
#                       into a dated release entry, commit and push.
#
# Called by: make prepare-release VERSION=X.Y.Z
#            make release VERSION=X.Y.Z (as the first step)
# =============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION="${VERSION:?VERSION is required, e.g. make release VERSION=2.0.3}"

# ── Guard: working tree must be clean ────────────────────────────────────────
if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "Error: uncommitted changes present. Commit or stash before releasing." >&2
	exit 1
fi

if grep -q "^## \[${VERSION}\]" CHANGELOG.md; then
	echo "Error: CHANGELOG.md already has an entry for [${VERSION}]. Did you already" >&2
	echo "prepare this release, or forget to bump VERSION?" >&2
	exit 1
fi

if ! awk '/^## \[Unreleased\]/{f=1; next} /^## \[/{if(f) exit} f{if ($0 !~ /^[[:space:]]*$/) found=1} END{exit !found}' CHANGELOG.md; then
	echo "Error: CHANGELOG.md's [Unreleased] section is empty (or missing). Add" >&2
	echo "changelog entries before releasing." >&2
	exit 1
fi

echo "==> Bumping VERSION to ${VERSION}..."
echo "${VERSION}" >VERSION

# CHANGELOG.md's header states dates are "the day the corresponding Git tag
# was created (UTC)" — match that convention here.
echo "==> Rolling CHANGELOG.md's [Unreleased] section into [${VERSION}]..."
DATE="$(date -u +%Y-%m-%d)"
awk -v ver="$VERSION" -v date="$DATE" '
  /^## \[Unreleased\]/ {
    print "## [Unreleased]"
    print ""
    print "## [" ver "] - " date
    next
  }
  { print }
' CHANGELOG.md >CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md

git add VERSION CHANGELOG.md
git commit -m "chore(release): v${VERSION}"
git push origin HEAD

echo "==> Prepared release v${VERSION} (commit pushed)."
