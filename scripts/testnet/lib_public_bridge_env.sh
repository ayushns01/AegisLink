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

public_bridge_link_error_is_retryable() {
  local error_output="${1:-}"

  [[ -n "$error_output" ]] || return 1

  case "$error_output" in
    *"context deadline exceeded"*|*"i/o timeout"*|*"Client.Timeout exceeded"*|*"connection reset by peer"*|*"EOF"*|*"post failed: Post "*)
      return 0
      ;;
  esac

  return 1
}

terminate_public_bridge_pid() {
  local pid="${1:-}"
  local attempt=""

  if [[ -z "$pid" ]]; then
    return 0
  fi

  if ! kill -0 "$pid" >/dev/null 2>&1; then
    return 0
  fi

  kill "$pid" >/dev/null 2>&1 || true

  for attempt in 1 2 3 4 5; do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  kill -9 "$pid" >/dev/null 2>&1 || true
}

cleanup_public_bridge_startup_failure() {
  local node_pid="${1:-}"
  local relayer_pid="${2:-}"
  local status_file="${3:-}"
  local current_status_file="${4:-}"

  terminate_public_bridge_pid "$relayer_pid"
  terminate_public_bridge_pid "$node_pid"

  [[ -n "$status_file" ]] && rm -f "$status_file"
  [[ -n "$current_status_file" ]] && rm -f "$current_status_file"
}
