#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOME_DIR="${1:-${AEGISLINK_PUBLIC_IBC_AEGISLINK_HOME:-/tmp/aegislink-public-home}}"
RPC_ADDRESS="${AEGISLINK_DEMO_NODE_RPC_ADDRESS:-127.0.0.1:26657}"
COMET_RPC_ADDRESS="${AEGISLINK_DEMO_NODE_COMET_RPC_ADDRESS:-127.0.0.1:27657}"
GRPC_ADDRESS="${AEGISLINK_DEMO_NODE_GRPC_ADDRESS:-127.0.0.1:9090}"
ABCI_ADDRESS="${AEGISLINK_DEMO_NODE_ABCI_ADDRESS:-127.0.0.1:26658}"
READY_FILE="${AEGISLINK_DEMO_NODE_READY_FILE:-$HOME_DIR/data/demo-node-ready.json}"
TICK_INTERVAL_MS="${AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS:-50}"
DESTINATION_LCD_BASE_URL="${AEGISLINK_DEMO_NODE_DESTINATION_LCD_BASE_URL:-${AEGISLINK_PUBLIC_IBC_DESTINATION_LCD_BASE_URL:-}}"

if [[ ! -f "$HOME_DIR/config/runtime.json" ]]; then
  "$REPO_ROOT/scripts/testnet/bootstrap_aegislink_testnet.sh" "$HOME_DIR"
fi

cd "$REPO_ROOT"

DESTINATION_LCD_ARGS=()
if [[ -n "$DESTINATION_LCD_BASE_URL" ]]; then
  DESTINATION_LCD_ARGS+=(--destination-lcd-base-url "$DESTINATION_LCD_BASE_URL")
fi

exec go run ./chain/aegislink/cmd/aegislinkd demo-node start \
  --home "$HOME_DIR" \
  --rpc-address "$RPC_ADDRESS" \
  --comet-rpc-address "$COMET_RPC_ADDRESS" \
  --grpc-address "$GRPC_ADDRESS" \
  --abci-address "$ABCI_ADDRESS" \
  --ready-file "$READY_FILE" \
  "${DESTINATION_LCD_ARGS[@]}" \
  --tick-interval-ms "$TICK_INTERVAL_MS"
