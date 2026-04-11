#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <home-dir>" >&2
  exit 1
fi

HOME_DIR="$1"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MANIFEST_PATH="${AEGISLINK_PUBLIC_IBC_MANIFEST_PATH:-$REPO_ROOT/deploy/testnet/ibc/osmosis-wallet-delivery.json}"

cd "$REPO_ROOT"

go run ./scripts/testnet/seed_public_ibc_route.go \
  --home "$HOME_DIR" \
  --manifest-file "$MANIFEST_PATH"
