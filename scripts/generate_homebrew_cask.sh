#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(cat "${ROOT_DIR}/VERSION")"
OUTPUT_DIR="${1:-${ROOT_DIR}/dist/homebrew-tap/Casks}"
OUTPUT_FILE="${OUTPUT_DIR}/pgarachne-app.rb"
ARM_ZIP="${ROOT_DIR}/dist/pgarachne-macos-arm64-app.zip"
AMD_ZIP="${ROOT_DIR}/dist/pgarachne-macos-amd64-app.zip"

mkdir -p "${OUTPUT_DIR}"

if [[ ! -f "${ARM_ZIP}" || ! -f "${AMD_ZIP}" ]]; then
  echo "Missing macOS app archives in dist/. Run make release-local first." >&2
  exit 1
fi

SHA_ARM="$(shasum -a 256 "${ARM_ZIP}" | awk '{print $1}')"
SHA_AMD="$(shasum -a 256 "${AMD_ZIP}" | awk '{print $1}')"

cat > "${OUTPUT_FILE}" <<EOF
cask "pgarachne-app" do
  version "${VERSION}"
  name "PgArachne"
  desc "GUI wrapper for PgArachne"
  homepage "https://www.pgarachne.com/"

  on_arm do
    url "https://github.com/heptau/pgarachne/releases/download/v#{version}/pgarachne-macos-arm64-app.zip"
    sha256 "${SHA_ARM}"
  end

  on_intel do
    url "https://github.com/heptau/pgarachne/releases/download/v#{version}/pgarachne-macos-amd64-app.zip"
    sha256 "${SHA_AMD}"
  end

  app "PgArachne.app"
end
EOF

echo "Generated ${OUTPUT_FILE}"
