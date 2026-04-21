#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPER="$SCRIPT_DIR/lib_public_bridge_env.sh"

if [[ ! -f "$HELPER" ]]; then
  echo "missing helper: $HELPER" >&2
  exit 1
fi

# shellcheck source=/dev/null
source "$HELPER"

assert_eq() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  if [[ "$expected" != "$actual" ]]; then
    echo "assertion failed: $message" >&2
    echo "expected: $expected" >&2
    echo "actual:   $actual" >&2
    exit 1
  fi
}

test_prefers_explicit_public_bridge_rpc() {
  local resolved=""
  resolved="$(resolve_public_bridge_evm_rpc_url \
    "https://eth-sepolia.g.alchemy.com/v2/real-key" \
    "https://ethereum-sepolia-rpc.publicnode.com")"
  assert_eq "https://eth-sepolia.g.alchemy.com/v2/real-key" "$resolved" "should prefer explicit relayer rpc"
}

test_falls_back_to_sepolia_deploy_rpc_when_relayer_rpc_is_placeholder() {
  local resolved=""
  resolved="$(resolve_public_bridge_evm_rpc_url \
    "https://eth-sepolia.g.alchemy.com/v2/your-key" \
    "https://rpc.sepolia.example.org")"
  assert_eq "https://rpc.sepolia.example.org" "$resolved" "should fall back to deploy rpc"
}

test_falls_back_to_publicnode_when_both_values_are_placeholders() {
  local resolved=""
  resolved="$(resolve_public_bridge_evm_rpc_url \
    "https://eth-sepolia.g.alchemy.com/v2/your-key" \
    "https://eth-sepolia.g.alchemy.com/v2/your-key")"
  assert_eq "https://ethereum-sepolia-rpc.publicnode.com" "$resolved" "should fall back to public endpoint"
}

test_prefers_publicnode_when_values_are_empty() {
  local resolved=""
  resolved="$(resolve_public_bridge_evm_rpc_url "" "")"
  assert_eq "https://ethereum-sepolia-rpc.publicnode.com" "$resolved" "should use public endpoint when no rpc is configured"
}

test_uses_osmosis_timeout_fallback_when_value_is_missing() {
  local resolved=""
  resolved="$(resolve_public_bridge_ibc_timeout_height "")"
  assert_eq "55000000" "$resolved" "should use live osmosis timeout fallback when unset"
}

test_uses_osmosis_timeout_fallback_when_value_is_too_small() {
  local resolved=""
  resolved="$(resolve_public_bridge_ibc_timeout_height "120")"
  assert_eq "55000000" "$resolved" "should override stale tiny timeout height"
}

test_preserves_large_timeout_values() {
  local resolved=""
  resolved="$(resolve_public_bridge_ibc_timeout_height "56000000")"
  assert_eq "56000000" "$resolved" "should preserve valid large timeout heights"
}

test_retries_transient_link_errors() {
  local exit_code="0"

  public_bridge_link_error_is_retryable "post failed: Post \"https://rpc.osmotest5.osmosis.zone:443\": context deadline exceeded" || exit_code="$?"
  assert_eq "0" "$exit_code" "should retry context deadline failures during path linking"

  exit_code="0"
  public_bridge_link_error_is_retryable "Failed to wait for block inclusion: context deadline exceeded" || exit_code="$?"
  assert_eq "0" "$exit_code" "should retry block inclusion timeouts during path linking"
}

test_does_not_retry_non_transient_link_errors() {
  local exit_code="0"

  public_bridge_link_error_is_retryable "identifier cannot be blank: invalid identifier" || exit_code="$?"
  assert_eq "1" "$exit_code" "should not retry irrecoverable identifier errors without a transport timeout"
}

test_cleanup_stops_processes_and_removes_status_files() {
  local temp_dir=""
  local status_file=""
  local current_status_file=""
  local sleeper=""

  temp_dir="$(mktemp -d)"
  status_file="$temp_dir/status.json"
  current_status_file="$temp_dir/current.json"
  printf '{}' >"$status_file"
  printf '{}' >"$current_status_file"

  sleep 30 &
  sleeper="$!"

  cleanup_public_bridge_startup_failure "$sleeper" "" "$status_file" "$current_status_file"
  wait "$sleeper" 2>/dev/null || true

  if kill -0 "$sleeper" >/dev/null 2>&1; then
    echo "assertion failed: expected cleanup to stop background process" >&2
    exit 1
  fi

  if [[ -e "$status_file" || -e "$current_status_file" ]]; then
    echo "assertion failed: expected cleanup to remove status files" >&2
    exit 1
  fi
}

test_prefers_explicit_public_bridge_rpc
test_falls_back_to_sepolia_deploy_rpc_when_relayer_rpc_is_placeholder
test_falls_back_to_publicnode_when_both_values_are_placeholders
test_prefers_publicnode_when_values_are_empty
test_uses_osmosis_timeout_fallback_when_value_is_missing
test_uses_osmosis_timeout_fallback_when_value_is_too_small
test_preserves_large_timeout_values
test_retries_transient_link_errors
test_does_not_retry_non_transient_link_errors
test_cleanup_stops_processes_and_removes_status_files

echo "ok"
