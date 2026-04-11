#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <home-dir>" >&2
  exit 1
fi

HOME_DIR="$1"
CHAIN_ID="${AEGISLINK_PUBLIC_CHAIN_ID:-aegislink-public-testnet-1}"
RUNTIME_MODE="${AEGISLINK_RUNTIME_MODE:-sdk-store-runtime}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPERATOR_TEMPLATE="$REPO_ROOT/deploy/testnet/aegislink/operator.json"
NETWORK_TEMPLATE="$REPO_ROOT/deploy/testnet/aegislink/network.json"

mkdir -p "$(dirname "$HOME_DIR")"

cd "$REPO_ROOT/chain/aegislink"

go run ./cmd/aegislinkd \
  init \
  --home "$HOME_DIR" \
  --chain-id "$CHAIN_ID" \
  --runtime-mode "$RUNTIME_MODE" \
  --force

mkdir -p "$HOME_DIR/config"
cp "$OPERATOR_TEMPLATE" "$HOME_DIR/config/operator.json"
cp "$NETWORK_TEMPLATE" "$HOME_DIR/config/network.json"

cat <<EOF
{
  "status": "bootstrapped",
  "chain_id": "$CHAIN_ID",
  "runtime_mode": "$RUNTIME_MODE",
  "home_dir": "$HOME_DIR",
  "operator_config": "$HOME_DIR/config/operator.json",
  "network_config": "$HOME_DIR/config/network.json"
}
EOF
