#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MANIFEST_PATH="${AEGISLINK_PUBLIC_IBC_MANIFEST_PATH:-$REPO_ROOT/deploy/testnet/ibc/osmosis-wallet-delivery.json}"
DESTINATION_METADATA_PATH="${AEGISLINK_RLY_DESTINATION_METADATA_PATH:-$REPO_ROOT/deploy/testnet/ibc/rly/osmosis-testnet.chain.example.json}"
OUTPUT_DIR="${AEGISLINK_RLY_OUTPUT_DIR:-$REPO_ROOT/deploy/testnet/ibc/rly/generated}"
SOURCE_READY_FILE="${AEGISLINK_RLY_SOURCE_READY_FILE:-}"
SOURCE_RPC_ADDR="${AEGISLINK_RLY_SOURCE_RPC_ADDR:-}"
SOURCE_RPC_WS_ADDR="${AEGISLINK_RLY_SOURCE_RPC_WS_ADDR:-}"
SOURCE_GRPC_ADDR="${AEGISLINK_RLY_SOURCE_GRPC_ADDR:-}"
SOURCE_KEY_NAME="${AEGISLINK_RLY_SOURCE_KEY_NAME:-aegislink-demo}"
DESTINATION_KEY_NAME="${AEGISLINK_RLY_DESTINATION_KEY_NAME:-osmosis-demo}"
DESTINATION_ACCOUNT_PREFIX="${AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX:-}"
DESTINATION_GAS_PRICE_DENOM="${AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM:-}"
DESTINATION_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT:-}"
PATH_NAME="${AEGISLINK_RLY_PATH_NAME:-}"
SOURCE_ACCOUNT_PREFIX="${AEGISLINK_RLY_SOURCE_ACCOUNT_PREFIX:-cosmos}"
SOURCE_GAS_PRICE_DENOM="${AEGISLINK_RLY_SOURCE_GAS_PRICE_DENOM:-ueth}"
SOURCE_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_SOURCE_GAS_PRICE_AMOUNT:-0.0}"

if [[ -z "$SOURCE_READY_FILE" ]]; then
  SOURCE_RPC_ADDR="${SOURCE_RPC_ADDR:-http://127.0.0.1:27657}"
  SOURCE_RPC_WS_ADDR="${SOURCE_RPC_WS_ADDR:-ws://127.0.0.1:27657/websocket}"
  SOURCE_GRPC_ADDR="${SOURCE_GRPC_ADDR:-http://127.0.0.1:9090}"
fi

cd "$REPO_ROOT"

go run ./scripts/testnet/bootstrap_rly_path.go \
  --manifest-file "$MANIFEST_PATH" \
  --destination-metadata-file "$DESTINATION_METADATA_PATH" \
  --output-dir "$OUTPUT_DIR" \
  --source-ready-file "$SOURCE_READY_FILE" \
  --source-rpc-addr "$SOURCE_RPC_ADDR" \
  --source-rpc-ws-addr "$SOURCE_RPC_WS_ADDR" \
  --source-grpc-addr "$SOURCE_GRPC_ADDR" \
  --source-key-name "$SOURCE_KEY_NAME" \
  --destination-key-name "$DESTINATION_KEY_NAME" \
  --destination-account-prefix "$DESTINATION_ACCOUNT_PREFIX" \
  --destination-gas-price-denom "$DESTINATION_GAS_PRICE_DENOM" \
  --destination-gas-price-amount "$DESTINATION_GAS_PRICE_AMOUNT" \
  --path-name "$PATH_NAME" \
  --source-account-prefix "$SOURCE_ACCOUNT_PREFIX" \
  --source-gas-price-denom "$SOURCE_GAS_PRICE_DENOM" \
  --source-gas-price-amount "$SOURCE_GAS_PRICE_AMOUNT"
