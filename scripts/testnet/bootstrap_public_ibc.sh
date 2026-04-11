#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BRIDGE_REGISTRY_PATH="${AEGISLINK_SEPOLIA_ASSET_REGISTRY:-$REPO_ROOT/deploy/testnet/sepolia/bridge-assets.json}"
MANIFEST_PATH="${AEGISLINK_PUBLIC_IBC_MANIFEST_PATH:-$REPO_ROOT/deploy/testnet/ibc/osmosis-wallet-delivery.json}"
SOURCE_CHAIN_ID="${AEGISLINK_PUBLIC_IBC_SOURCE_CHAIN_ID:-aegislink-public-testnet-1}"
DESTINATION_CHAIN_ID="${AEGISLINK_PUBLIC_IBC_DESTINATION_CHAIN_ID:-osmosis-testnet}"
CHANNEL_ID="${AEGISLINK_PUBLIC_IBC_CHANNEL_ID:-channel-public-osmosis}"
ROUTE_ID="${AEGISLINK_PUBLIC_IBC_ROUTE_ID:-osmosis-public-wallet}"
PROVIDER="${AEGISLINK_PUBLIC_IBC_PROVIDER:-hermes}"
WALLET_PREFIX="${AEGISLINK_PUBLIC_IBC_WALLET_PREFIX:-osmo}"
PORT_ID="${AEGISLINK_PUBLIC_IBC_PORT_ID:-transfer}"
MEMO_PREFIXES="${AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES:-swap:,stake:}"
ACTION_TYPES="${AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES:-swap,stake}"
ENABLED_FLAG=false

if [[ "${AEGISLINK_ENABLE_REAL_IBC:-0}" == "1" || "${AEGISLINK_ENABLE_REAL_IBC:-0}" == "true" ]]; then
  ENABLED_FLAG=true
fi

cd "$REPO_ROOT"

go run ./scripts/testnet/bootstrap_public_ibc.go \
  --bridge-registry-file "$BRIDGE_REGISTRY_PATH" \
  --output "$MANIFEST_PATH" \
  --source-chain-id "$SOURCE_CHAIN_ID" \
  --destination-chain-id "$DESTINATION_CHAIN_ID" \
  --channel-id "$CHANNEL_ID" \
  --route-id "$ROUTE_ID" \
  --provider "$PROVIDER" \
  --wallet-prefix "$WALLET_PREFIX" \
  --port-id "$PORT_ID" \
  --memo-prefixes "$MEMO_PREFIXES" \
  --action-types "$ACTION_TYPES" \
  --enabled="$ENABLED_FLAG"
