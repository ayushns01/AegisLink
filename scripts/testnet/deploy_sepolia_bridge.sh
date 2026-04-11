#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_PATH="${AEGISLINK_SEPOLIA_DEPLOY_OUTPUT:-$REPO_ROOT/deploy/testnet/sepolia/bridge-addresses.json}"
CHAIN_ID="${AEGISLINK_SEPOLIA_CHAIN_ID:-11155111}"
RPC_URL="${AEGISLINK_SEPOLIA_RPC_URL:-}"
PRIVATE_KEY="${AEGISLINK_SEPOLIA_PRIVATE_KEY:-}"
DEPLOYER_ADDRESS="${AEGISLINK_SEPOLIA_DEPLOYER_ADDRESS:-}"
ATTESTER_ADDRESS="${AEGISLINK_SEPOLIA_ATTESTER_ADDRESS:-}"

extract_field() {
  local file_path="$1"
  local field_name="$2"
  sed -n "s/.*\"${field_name}\": *\"\\([^\"]*\\)\".*/\\1/p" "$file_path" | head -n1
}

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

contract_code_exists() {
  local address="$1"
  local code
  code="$(cast code --rpc-url "$RPC_URL" "$address" 2>/dev/null || true)"
  [[ -n "$code" && "$code" != "0x" ]]
}

deploy_contract() {
  local contract_id="$1"
  local constructor_signature="$2"
  shift
  shift

  local output
  output="$(
    cd "$REPO_ROOT" &&
      forge create "$contract_id" \
        --broadcast \
        --rpc-url "$RPC_URL" \
        --private-key "$PRIVATE_KEY" \
        --constructor-args "$@" 2>&1
  )"
  local contract_address
  contract_address="$(printf '%s\n' "$output" | sed -n 's/^Deployed to: *//p' | head -n1)"
  if [[ -z "$contract_address" ]]; then
    printf '%s\n' "$output" >&2
    echo "failed to capture deployment contract address" >&2
    exit 1
  fi
  printf '%s\n' "$contract_address"
}

require_tool cast
require_tool forge
if [[ -z "$RPC_URL" ]]; then
  echo "missing AEGISLINK_SEPOLIA_RPC_URL" >&2
  exit 1
fi
if [[ -z "$PRIVATE_KEY" ]]; then
  echo "missing AEGISLINK_SEPOLIA_PRIVATE_KEY" >&2
  exit 1
fi

if [[ -z "$DEPLOYER_ADDRESS" ]]; then
  DEPLOYER_ADDRESS="$(cast wallet address --private-key "$PRIVATE_KEY")"
fi
if [[ -z "$ATTESTER_ADDRESS" ]]; then
  ATTESTER_ADDRESS="$DEPLOYER_ADDRESS"
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

if [[ -f "$OUTPUT_PATH" ]]; then
  existing_verifier="$(extract_field "$OUTPUT_PATH" "verifier_address")"
  existing_gateway="$(extract_field "$OUTPUT_PATH" "gateway_address")"
  existing_deployer="$(extract_field "$OUTPUT_PATH" "deployer_address")"
  if [[ "$existing_deployer" == "$DEPLOYER_ADDRESS" ]] && contract_code_exists "$existing_verifier" && contract_code_exists "$existing_gateway"; then
    cat "$OUTPUT_PATH"
    exit 0
  fi
fi

VERIFIER_ADDRESS="$(deploy_contract "contracts/ethereum/BridgeVerifier.sol:BridgeVerifier" "constructor(address)" "$ATTESTER_ADDRESS")"
if [[ -z "$VERIFIER_ADDRESS" ]]; then
  echo "failed to deploy BridgeVerifier" >&2
  exit 1
fi

GATEWAY_ADDRESS="$(deploy_contract "contracts/ethereum/BridgeGateway.sol:BridgeGateway" "constructor(address)" "$VERIFIER_ADDRESS")"
if [[ -z "$GATEWAY_ADDRESS" ]]; then
  echo "failed to deploy BridgeGateway" >&2
  exit 1
fi

cast send --rpc-url "$RPC_URL" --private-key "$PRIVATE_KEY" "$VERIFIER_ADDRESS" "setGateway(address)" "$GATEWAY_ADDRESS" >/dev/null

tmp_file="$(mktemp)"
cat >"$tmp_file" <<EOF
{
  "chain_id": "$CHAIN_ID",
  "deployer_address": "$DEPLOYER_ADDRESS",
  "verifier_address": "$VERIFIER_ADDRESS",
  "gateway_address": "$GATEWAY_ADDRESS",
  "output_path": "$OUTPUT_PATH"
}
EOF

mv "$tmp_file" "$OUTPUT_PATH"
cat "$OUTPUT_PATH"
