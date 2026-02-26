#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(cat "${ROOT_DIR}/VERSION")"
CHECKSUMS_FILE="${ROOT_DIR}/dist/checksums.txt"
OUTPUT_DIR="${1:-${ROOT_DIR}/dist/homebrew-tap/Formula}"
OUTPUT_FILE="${OUTPUT_DIR}/pgarachne.rb"

if [[ ! -f "${CHECKSUMS_FILE}" ]]; then
  echo "Missing ${CHECKSUMS_FILE}. Run make release-snapshot first." >&2
  exit 1
fi

sha_for() {
  local name="$1"
  awk -v n="$name" '$2==n {print $1}' "${CHECKSUMS_FILE}"
}

SHA_DARWIN_AMD64="$(sha_for pgarachne-darwin-amd64.zip)"
SHA_DARWIN_ARM64="$(sha_for pgarachne-darwin-arm64.zip)"
SHA_LINUX_AMD64="$(sha_for pgarachne-linux-amd64.tar.gz)"
SHA_LINUX_ARM64="$(sha_for pgarachne-linux-arm64.tar.gz)"

for v in SHA_DARWIN_AMD64 SHA_DARWIN_ARM64 SHA_LINUX_AMD64 SHA_LINUX_ARM64; do
  if [[ -z "${!v}" ]]; then
    echo "Missing checksum for ${v}" >&2
    exit 1
  fi
done

mkdir -p "${OUTPUT_DIR}"

cat > "${OUTPUT_FILE}" <<EOF
class Pgarachne < Formula
  desc "High-performance PostgreSQL JSON-RPC gateway with SSE support"
  homepage "https://www.pgarachne.com/"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/heptau/pgarachne/releases/download/v#{version}/pgarachne-darwin-arm64.zip"
      sha256 "${SHA_DARWIN_ARM64}"
    else
      url "https://github.com/heptau/pgarachne/releases/download/v#{version}/pgarachne-darwin-amd64.zip"
      sha256 "${SHA_DARWIN_AMD64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/heptau/pgarachne/releases/download/v#{version}/pgarachne-linux-arm64.tar.gz"
      sha256 "${SHA_LINUX_ARM64}"
    else
      url "https://github.com/heptau/pgarachne/releases/download/v#{version}/pgarachne-linux-amd64.tar.gz"
      sha256 "${SHA_LINUX_AMD64}"
    end
  end

  def install
    bin.install "pgarachne"
  end

  test do
    assert_match "PgArachne", shell_output("#{bin}/pgarachne --version")
  end
end
EOF

echo "Generated ${OUTPUT_FILE}"

