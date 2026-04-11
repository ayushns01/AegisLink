#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <home-dir>" >&2
  exit 1
fi

HOME_DIR="$1"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REGISTRY_PATH="${AEGISLINK_SEPOLIA_ASSET_REGISTRY:-$REPO_ROOT/deploy/testnet/sepolia/bridge-assets.json}"
WINDOW_SECONDS="${AEGISLINK_PUBLIC_LIMIT_WINDOW_SECONDS:-600}"
NATIVE_MAX_AMOUNT="${AEGISLINK_PUBLIC_NATIVE_MAX_AMOUNT:-}"
ERC20_MAX_AMOUNT="${AEGISLINK_PUBLIC_ERC20_MAX_AMOUNT:-}"

cd "$REPO_ROOT"

go run ./scripts/testnet/seed_public_bridge_assets.go \
  --home "$HOME_DIR" \
  --registry-file "$REGISTRY_PATH" \
  --window-seconds "$WINDOW_SECONDS" \
  --native-max-amount "$NATIVE_MAX_AMOUNT" \
  --erc20-max-amount "$ERC20_MAX_AMOUNT"
