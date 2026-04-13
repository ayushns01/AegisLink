#!/usr/bin/env bash

set -euo pipefail

PUBLIC_SEPOLIA_RPC_FALLBACK="https://ethereum-sepolia-rpc.publicnode.com"
PUBLIC_OSMOSIS_TIMEOUT_HEIGHT_FALLBACK="55000000"

is_placeholder_rpc_url() {
  local value="${1:-}"
  [[ -z "$value" ]] && return 0
  [[ "$value" == *"/your-key"* ]] && return 0
  return 1
}

resolve_public_bridge_evm_rpc_url() {
  local relayer_rpc_url="${1:-}"
  local sepolia_rpc_url="${2:-}"

  if ! is_placeholder_rpc_url "$relayer_rpc_url"; then
    printf '%s\n' "$relayer_rpc_url"
    return 0
  fi

  if ! is_placeholder_rpc_url "$sepolia_rpc_url"; then
    printf '%s\n' "$sepolia_rpc_url"
    return 0
  fi

  printf '%s\n' "$PUBLIC_SEPOLIA_RPC_FALLBACK"
}

resolve_public_bridge_ibc_timeout_height() {
  local configured_timeout_height="${1:-}"

  if [[ -z "$configured_timeout_height" ]]; then
    printf '%s\n' "$PUBLIC_OSMOSIS_TIMEOUT_HEIGHT_FALLBACK"
    return 0
  fi

  if [[ ! "$configured_timeout_height" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "$PUBLIC_OSMOSIS_TIMEOUT_HEIGHT_FALLBACK"
    return 0
  fi

  if (( configured_timeout_height < 1000000 )); then
    printf '%s\n' "$PUBLIC_OSMOSIS_TIMEOUT_HEIGHT_FALLBACK"
    return 0
  fi

  printf '%s\n' "$configured_timeout_height"
}
