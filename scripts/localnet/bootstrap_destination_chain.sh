#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <home-dir>" >&2
  exit 1
fi

HOME_DIR="$1"
CHAIN_ID="${OSMO_LOCAL_CHAIN_ID:-osmo-local-1}"
RUNTIME_MODE="${OSMO_LOCAL_RUNTIME_MODE:-osmo-local-runtime}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
POOLS_JSON='[
  {"input_denom":"ibc/uethusdc","output_denom":"uosmo","reserve_in":"500000000","reserve_out":"1000000000"},
  {"input_denom":"ibc/uatom-usdc","output_denom":"uosmo","reserve_in":"500000000","reserve_out":"1000000000"}
]'

mkdir -p "$(dirname "$HOME_DIR")"

cd "$REPO_ROOT"

go run ./relayer/cmd/osmo-locald \
  init \
  --home "$HOME_DIR" \
  --chain-id "$CHAIN_ID" \
  --runtime-mode "$RUNTIME_MODE" \
  --pools-json "$POOLS_JSON" \
  --force
