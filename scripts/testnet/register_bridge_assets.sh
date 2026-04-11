#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_PATH="${AEGISLINK_SEPOLIA_ASSET_REGISTRY:-$REPO_ROOT/deploy/testnet/sepolia/bridge-assets.json}"
DEPLOY_OUTPUT_PATH="${AEGISLINK_SEPOLIA_DEPLOY_OUTPUT:-$REPO_ROOT/deploy/testnet/sepolia/bridge-addresses.json}"
CHAIN_ID="${AEGISLINK_SEPOLIA_CHAIN_ID:-11155111}"
VERIFIER_ADDRESS="${AEGISLINK_SEPOLIA_VERIFIER_ADDRESS:-}"
GATEWAY_ADDRESS="${AEGISLINK_SEPOLIA_GATEWAY_ADDRESS:-}"
ERC20_ADDRESS="${AEGISLINK_SEPOLIA_ERC20_ADDRESS:-}"

extract_field() {
  local file_path="$1"
  local field_name="$2"
  sed -n "s/.*\"${field_name}\": *\"\\([^\"]*\\)\".*/\\1/p" "$file_path" | head -n1
}

if [[ -z "$VERIFIER_ADDRESS" && -f "$DEPLOY_OUTPUT_PATH" ]]; then
  VERIFIER_ADDRESS="$(extract_field "$DEPLOY_OUTPUT_PATH" "verifier_address")"
fi

if [[ -z "$GATEWAY_ADDRESS" && -f "$DEPLOY_OUTPUT_PATH" ]]; then
  GATEWAY_ADDRESS="$(extract_field "$DEPLOY_OUTPUT_PATH" "gateway_address")"
fi

if [[ -z "$VERIFIER_ADDRESS" ]]; then
  echo "missing AEGISLINK_SEPOLIA_VERIFIER_ADDRESS" >&2
  exit 1
fi

if [[ -z "$GATEWAY_ADDRESS" ]]; then
  echo "missing AEGISLINK_SEPOLIA_GATEWAY_ADDRESS" >&2
  exit 1
fi

if [[ -n "$ERC20_ADDRESS" ]] && [[ ! "$ERC20_ADDRESS" =~ ^0x[0-9a-fA-F]{40}$ ]]; then
  echo "invalid AEGISLINK_SEPOLIA_ERC20_ADDRESS: set a real 0x... token address or leave it empty for ETH-only" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

tmp_file="$(mktemp)"
assets_json='
    {
      "asset_id": "eth",
      "source_chain_id": "'"$CHAIN_ID"'",
      "source_asset_kind": "native_eth",
      "denom": "ueth",
      "decimals": 18,
      "display_name": "Ether",
      "display_symbol": "ETH",
      "enabled": true
    }'

if [[ -n "$ERC20_ADDRESS" ]]; then
  assets_json="${assets_json},
    {
      \"asset_id\": \"eth.usdc\",
      \"source_chain_id\": \"$CHAIN_ID\",
      \"source_asset_kind\": \"erc20\",
      \"source_asset_address\": \"$ERC20_ADDRESS\",
      \"denom\": \"uethusdc\",
      \"decimals\": 6,
      \"display_name\": \"USD Coin\",
      \"display_symbol\": \"USDC\",
      \"enabled\": true
    }"
fi

cat >"$tmp_file" <<EOF
{
  "chain_id": "$CHAIN_ID",
  "verifier_address": "$VERIFIER_ADDRESS",
  "gateway_address": "$GATEWAY_ADDRESS",
  "assets": [
${assets_json}
  ]
}
EOF

if [[ ! -f "$OUTPUT_PATH" ]] || ! cmp -s "$tmp_file" "$OUTPUT_PATH"; then
  mv "$tmp_file" "$OUTPUT_PATH"
else
  rm -f "$tmp_file"
fi

cat "$OUTPUT_PATH"
