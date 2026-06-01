#!/usr/bin/env bash
# =============================================================================
# publish_release.sh — Tag, push, and publish a PgArachne release to GitHub
# and the Homebrew tap.
#
# Not meant to be run directly: `make release` runs `release-local` first
# (tests, cross-platform build, checksums, macOS .app bundles, Homebrew
# formula/cask files, and dist/RELEASE_NOTES.md), then this script.
#
# Environment variables:
#   HOMEBREW_TAP_REPO   GitHub repo of the Homebrew tap (default: heptau/tap)
# =============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DIST_DIR="dist"
HOMEBREW_TAP_REPO="${HOMEBREW_TAP_REPO:-heptau/tap}"
HOMEBREW_FORMULA_PATH="Formula/pgarachne.rb"
HOMEBREW_CASK_PATH="Casks/pgarachne-app.rb"
LOCAL_FORMULA="${DIST_DIR}/homebrew-tap/Formula/pgarachne.rb"
LOCAL_CASK="${DIST_DIR}/homebrew-tap/Casks/pgarachne-app.rb"
NOTES_FILE="${DIST_DIR}/RELEASE_NOTES.md"

VERSION="$(cat VERSION)"
TAG="v${VERSION}"

echo "PgArachne release — ${TAG}"
echo ""

command -v gh >/dev/null 2>&1 || { echo "Error: 'gh' (GitHub CLI) is required. Install from https://cli.github.com/"; exit 1; }

for f in "$NOTES_FILE" "$LOCAL_FORMULA" "$LOCAL_CASK"; do
	[[ -f "$f" ]] || { echo "Error: missing $f — run 'make release-local' first."; exit 1; }
done

# ── Guard: working tree must be clean ────────────────────────────────────────
if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "Error: uncommitted changes present. Commit or stash before releasing."
	exit 1
fi

# ── Tag ───────────────────────────────────────────────────────────────────────
echo "==> Tagging ${TAG}..."
if git tag -l "$TAG" | grep -q .; then
	echo "    Tag ${TAG} already exists locally — skipping tag creation."
else
	git tag -a "$TAG" -m "PgArachne ${TAG}"
fi
git push origin "$TAG"
echo ""

# ── GitHub release ────────────────────────────────────────────────────────────
echo "==> Creating GitHub release ${TAG}..."
ASSETS=("${DIST_DIR}"/checksums.txt "${DIST_DIR}"/*.zip "${DIST_DIR}"/*.tar.gz)
gh release create "$TAG" \
	--title "PgArachne ${TAG}" \
	--notes-file "$NOTES_FILE" \
	"${ASSETS[@]}"
echo ""

# ── Homebrew tap update via GitHub API (no local clone needed) ──────────────
update_tap_file() {
	local local_path="$1" tap_path="$2"
	echo "==> Updating ${HOMEBREW_TAP_REPO}/${tap_path}..."
	local current_sha content
	current_sha=$(gh api "repos/${HOMEBREW_TAP_REPO}/contents/${tap_path}" --jq '.sha' 2>/dev/null || true)
	content=$(base64 <"$local_path" | tr -d '\n')

	if [[ -n "$current_sha" ]]; then
		gh api "repos/${HOMEBREW_TAP_REPO}/contents/${tap_path}" \
			--method PUT \
			-f message="PgArachne ${TAG}" \
			-f content="${content}" \
			-f sha="${current_sha}" >/dev/null
	else
		gh api "repos/${HOMEBREW_TAP_REPO}/contents/${tap_path}" \
			--method PUT \
			-f message="PgArachne ${TAG}" \
			-f content="${content}" >/dev/null
	fi
}

update_tap_file "$LOCAL_FORMULA" "$HOMEBREW_FORMULA_PATH"
update_tap_file "$LOCAL_CASK" "$HOMEBREW_CASK_PATH"
echo ""

echo "======================================================================"
echo "  Released : PgArachne ${TAG}"
echo "  GitHub   : https://github.com/heptau/pgarachne/releases/tag/${TAG}"
echo "  Homebrew : brew upgrade heptau/tap/pgarachne"
echo "======================================================================"
