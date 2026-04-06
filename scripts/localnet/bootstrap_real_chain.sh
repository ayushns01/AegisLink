#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <home-dir>" >&2
  exit 1
fi

HOME_DIR="$1"
CHAIN_ID="${AEGISLINK_CHAIN_ID:-aegislink-sdk-1}"
RUNTIME_MODE="${AEGISLINK_RUNTIME_MODE:-sdk-store-runtime}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

mkdir -p "$(dirname "$HOME_DIR")"

cd "$REPO_ROOT/chain/aegislink"

go run ./cmd/aegislinkd \
  init \
  --home "$HOME_DIR" \
  --chain-id "$CHAIN_ID" \
  --runtime-mode "$RUNTIME_MODE" \
  --force
