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
mkdir -p "$HOME_DIR/config"
cp "$OPERATOR_TEMPLATE" "$HOME_DIR/config/operator.json"
cp "$NETWORK_TEMPLATE" "$HOME_DIR/config/network.json"

OPERATOR_ALLOWED_SIGNERS="$(
  ruby -rjson -e 'config = JSON.parse(File.read(ARGV[0])); puts Array(config["allowed_signers"]).join(",")' "$OPERATOR_TEMPLATE"
)"
OPERATOR_GOVERNANCE_AUTHORITIES="$(
  ruby -rjson -e 'config = JSON.parse(File.read(ARGV[0])); puts Array(config["governance_authorities"]).join(",")' "$OPERATOR_TEMPLATE"
)"
OPERATOR_REQUIRED_THRESHOLD="$(
  ruby -rjson -e 'config = JSON.parse(File.read(ARGV[0])); puts Integer(config["required_threshold"] || 0)' "$OPERATOR_TEMPLATE"
)"

cd "$REPO_ROOT/chain/aegislink"

go run ./cmd/aegislinkd \
  init \
  --home "$HOME_DIR" \
  --chain-id "$CHAIN_ID" \
  --runtime-mode "$RUNTIME_MODE" \
  --allowed-signers "$OPERATOR_ALLOWED_SIGNERS" \
  --governance-authorities "$OPERATOR_GOVERNANCE_AUTHORITIES" \
  --required-threshold "$OPERATOR_REQUIRED_THRESHOLD" \
  --force

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
