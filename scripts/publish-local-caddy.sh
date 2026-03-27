#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"

if [ -z "${VERSION}" ]; then
  echo "Usage: ./scripts/publish-local-caddy.sh <version>"
  echo "Example: ./scripts/publish-local-caddy.sh v0.4.26"
  exit 1
fi

if [[ "${VERSION}" != v* ]]; then
  echo "Error: version must include leading v, for example v0.4.26" >&2
  exit 1
fi

DIST_DIR="${MSCODE_DIST_DIR:-/home/weizheng/work/mscode/dist}"
MIRROR_ROOT="${MSCODE_MIRROR_ROOT:-/opt/downloads/mscode/releases}"
TARGET_DIR="${MIRROR_ROOT}/${VERSION}"
LATEST_LINK="${MIRROR_ROOT}/latest"

required_files=(
  "manifest.json"
  "mscode-linux-amd64"
  "mscode-linux-arm64"
  "mscode-darwin-amd64"
  "mscode-darwin-arm64"
  "mscode-windows-amd64.exe"
)

for file in "${required_files[@]}"; do
  if [ ! -f "${DIST_DIR}/${file}" ]; then
    echo "Error: missing required asset: ${DIST_DIR}/${file}" >&2
    exit 1
  fi
done

echo "Publishing ${VERSION} from ${DIST_DIR} to ${TARGET_DIR}"
sudo mkdir -p "${TARGET_DIR}"
sudo cp "${DIST_DIR}"/* "${TARGET_DIR}/"
sudo chmod -R a+rX "${TARGET_DIR}"
sudo ln -sfn "${TARGET_DIR}" "${LATEST_LINK}"

echo ""
echo "Published ${VERSION} to local Caddy mirror:"
echo "  ${TARGET_DIR}"
echo "Latest link:"
echo "  ${LATEST_LINK} -> ${TARGET_DIR}"
