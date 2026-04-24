#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

# shellcheck source=/dev/null
source "$REPO_ROOT/scripts/testnet/lib_public_bridge_env.sh"

RUN_ID="${AEGISLINK_PUBLIC_BACKEND_RUN_ID:-$(date +%Y%m%d-%H%M%S)}"
HOME_DIR="${AEGISLINK_PUBLIC_BACKEND_HOME_DIR:-/tmp/aegislink-public-home-ui-auto-$RUN_ID}"
RUNTIME_DIR="${AEGISLINK_PUBLIC_BACKEND_RUNTIME_DIR:-/tmp/aegislink-public-backend-$RUN_ID}"
READY_FILE="$HOME_DIR/data/demo-node-ready.json"
NODE_LOG="$RUNTIME_DIR/node.log"
RELAYER_LOG="$RUNTIME_DIR/relayer.log"
REPLAY_STORE="$RUNTIME_DIR/replay.json"
ATTESTATION_STATE="$RUNTIME_DIR/attestations.json"
PATH_NAME="${AEGISLINK_PUBLIC_BACKEND_RLY_PATH_NAME:-live-osmo-ui-$RUN_ID}"
DESTINATION_LCD_BASE_URL="${AEGISLINK_DEMO_NODE_DESTINATION_LCD_BASE_URL:-https://lcd.osmotest5.osmosis.zone}"
DESTINATION_RPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_RPC_ADDR:-https://rpc.osmotest5.osmosis.zone:443}"
DESTINATION_GRPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_GRPC_ADDR:-https://grpc.osmotest5.osmosis.zone:443}"
DESTINATION_CHAIN_NAME="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_NAME:-osmo-test-5}"
DESTINATION_CHAIN_ID="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_ID:-osmo-test-5}"
DESTINATION_ACCOUNT_PREFIX="${AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX:-osmo}"
DESTINATION_GAS_PRICE_DENOM="${AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM:-uosmo}"
DESTINATION_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT:-1.3}"
RLY_TIMEOUT_SECONDS="${AEGISLINK_PUBLIC_BACKEND_RLY_TIMEOUT_SECONDS:-45}"
LINK_RETRIES="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_RETRIES:-3}"
LINK_RETRY_DELAY_SECONDS="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_RETRY_DELAY_SECONDS:-5}"
LINK_TIMEOUT_DURATION="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_TIMEOUT_DURATION:-5m}"
LINK_MAX_RETRIES="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_MAX_RETRIES:-6}"
LINK_BLOCK_HISTORY="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_BLOCK_HISTORY:-5}"
PERSISTENT_RLY_HOME="${AEGISLINK_RELAYER_RLY_HOME:-$HOME/.aegislink-live-rly}"
GOCACHE="${GOCACHE:-/tmp/aegislink-gocache}"
STATUS_FILE="$RUNTIME_DIR/status.json"
CURRENT_STATUS_FILE="${AEGISLINK_PUBLIC_BACKEND_CURRENT_STATUS_FILE:-/tmp/aegislink-public-backend-current.json}"
NODE_PID=""
RELAYER_PID=""
STARTUP_COMPLETE="0"

source_required_env() {
  local env_file="$1"
  if [[ ! -f "$env_file" ]]; then
    echo "missing required env file: $env_file" >&2
    exit 1
  fi
  set -a
  # shellcheck source=/dev/null
  source "$env_file"
  set +a
}

ensure_csv_contains() {
  local current="$1"
  local required="$2"
  local normalized=""
  local item=""
  local found="0"

  IFS=',' read -r -a items <<<"$current"
  for item in "${items[@]}"; do
    item="${item//[[:space:]]/}"
    if [[ -z "$item" ]]; then
      continue
    fi
    if [[ "$item" == "$required" ]]; then
      found="1"
    fi
    if [[ -n "$normalized" ]]; then
      normalized+=","
    fi
    normalized+="$item"
  done

  if [[ "$found" != "1" ]]; then
    if [[ -n "$normalized" ]]; then
      normalized+=","
    fi
    normalized+="$required"
  fi

  printf '%s\n' "$normalized"
}

write_rly_config() {
  local rly_home="$1"
  local source_key_name="$2"
  local destination_key_name="$3"

  mkdir -p "$rly_home/config"
  cat >"$rly_home/config/config.yaml" <<EOF
global:
  api-listen-addr: :5183
  timeout: ${RLY_TIMEOUT_SECONDS}s
  memo: ""
chains:
  aegislink-public-testnet-1:
    type: cosmos
    value:
      key: $source_key_name
      chain-id: aegislink-public-testnet-1
      rpc-addr: http://127.0.0.1:27657
      websocket-addr: ws://127.0.0.1:27657/websocket
      grpc-addr: http://127.0.0.1:9090
      account-prefix: ${AEGISLINK_RLY_SOURCE_ACCOUNT_PREFIX:-cosmos}
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: ${AEGISLINK_RLY_SOURCE_GAS_PRICE_AMOUNT:-0.0}${AEGISLINK_RLY_SOURCE_GAS_PRICE_DENOM:-ueth}
      debug: false
      timeout: ${RLY_TIMEOUT_SECONDS}s
      output-format: json
      sign-mode: direct
  $DESTINATION_CHAIN_NAME:
    type: cosmos
    value:
      key: $destination_key_name
      chain-id: $DESTINATION_CHAIN_ID
      rpc-addr: $DESTINATION_RPC_ADDR
      websocket-addr: ""
      grpc-addr: $DESTINATION_GRPC_ADDR
      account-prefix: $DESTINATION_ACCOUNT_PREFIX
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: ${DESTINATION_GAS_PRICE_AMOUNT}${DESTINATION_GAS_PRICE_DENOM}
      debug: false
      timeout: ${RLY_TIMEOUT_SECONDS}s
      output-format: json
      sign-mode: direct
paths: {}
EOF
}

cleanup_failed_startup() {
  local exit_code="$?"
  trap - EXIT

  if [[ "$STARTUP_COMPLETE" != "1" ]]; then
    cleanup_public_bridge_startup_failure "$NODE_PID" "$RELAYER_PID" "$STATUS_FILE" "$CURRENT_STATUS_FILE"
  fi

  exit "$exit_code"
}

run_relayer_link_with_retry() {
  local attempt=""
  local link_log=""
  local exit_code="0"
  local output=""

  for ((attempt = 1; attempt <= LINK_RETRIES; attempt += 1)); do
    link_log="$RUNTIME_DIR/rly-link-attempt-$attempt.log"
    echo "+ ./bin/relayer transact link $PATH_NAME --home $RLY_HOME --override --timeout $LINK_TIMEOUT_DURATION --max-retries $LINK_MAX_RETRIES --block-history $LINK_BLOCK_HISTORY --debug --log-level debug"

    set +e
    ./bin/relayer transact link "$PATH_NAME" \
      --home "$RLY_HOME" \
      --override \
      --timeout "$LINK_TIMEOUT_DURATION" \
      --max-retries "$LINK_MAX_RETRIES" \
      --block-history "$LINK_BLOCK_HISTORY" \
      --debug \
      --log-level debug >"$link_log" 2>&1
    exit_code="$?"
    set -e

    cat "$link_log"

    if [[ "$exit_code" == "0" ]]; then
      return 0
    fi

    output="$(cat "$link_log")"
    if (( attempt < LINK_RETRIES )) && public_bridge_link_error_is_retryable "$output"; then
      echo "transient relayer path-link failure on attempt $attempt/$LINK_RETRIES; retrying in ${LINK_RETRY_DELAY_SECONDS}s" >&2
      sleep "$LINK_RETRY_DELAY_SECONDS"
      continue
    fi

    return "$exit_code"
  done
}

ensure_relayer_key() {
  local chain_name="$1"
  local key_name="$2"
  local rly_home="$3"
  local mnemonic="${4:-}"

  if ./bin/relayer keys show "$chain_name" "$key_name" --home "$rly_home" >/dev/null 2>&1; then
    return 0
  fi

  if [[ -n "$mnemonic" ]]; then
    ./bin/relayer keys restore "$chain_name" "$key_name" "$mnemonic" --home "$rly_home" >/dev/null 2>&1
    return 0
  fi

  ./bin/relayer keys add "$chain_name" "$key_name" --home "$rly_home" >/dev/null 2>&1
}

ensure_rly_home() {
  local rly_home="$1"
  local source_key_name="$2"
  local destination_key_name="$3"
  local destination_mnemonic="${4:-}"

  write_rly_config "$rly_home" "$source_key_name" "$destination_key_name"
  ensure_relayer_key "aegislink-public-testnet-1" "$source_key_name" "$rly_home"
  ensure_relayer_key "$DESTINATION_CHAIN_NAME" "$destination_key_name" "$rly_home" "$destination_mnemonic"
}

destination_key_has_funds() {
  local rly_home="$1"
  local destination_key_name="$2"
  local balance_output=""

  balance_output="$(
    ./bin/relayer query balance "$DESTINATION_CHAIN_NAME" "$destination_key_name" --home "$rly_home" -o json 2>/dev/null || true
  )"
  [[ "$balance_output" =~ \"amount\":\"[1-9][0-9]*\" ]] || \
    [[ "$balance_output" =~ \"balance\":\"[1-9][0-9]*[[:alpha:]]+\" ]]
}

run() {
  echo "+ $*"
  "$@"
}

wait_for_http() {
  local url="$1"
  local label="$2"
  local retries="${3:-60}"
  local i=""
  for ((i = 0; i < retries; i += 1)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "$label did not become ready: $url" >&2
  exit 1
}

write_replay_state() {
  local evm_rpc_url="$1"
  local deposit_checkpoint="0"
  local latest_block=""

  if command -v cast >/dev/null 2>&1 && [[ -n "$evm_rpc_url" ]]; then
    latest_block="$(cast block-number --rpc-url "$evm_rpc_url" 2>/dev/null || true)"
    if [[ "$latest_block" =~ ^[0-9]+$ ]]; then
      if (( latest_block > 10 )); then
        deposit_checkpoint="$((latest_block - 10))"
      else
        deposit_checkpoint="$latest_block"
      fi
    fi
  fi

  cat >"$REPLAY_STORE" <<EOF
{
  "checkpoints": {
    "evm-deposits": $deposit_checkpoint,
    "cosmos-withdrawals": 0
  },
  "processed": []
}
EOF
  cat >"$ATTESTATION_STATE" <<'EOF'
{"votes":[]}
EOF
}

source_required_env "$REPO_ROOT/.env.sepolia.deploy.local"
source_required_env "$REPO_ROOT/.env.public-bridge.local"
source_required_env "$REPO_ROOT/.env.public-ibc.local"

AEGISLINK_RELAYER_EVM_RPC_URL="$(
  resolve_public_bridge_evm_rpc_url \
    "${AEGISLINK_RELAYER_EVM_RPC_URL:-}" \
    "${AEGISLINK_SEPOLIA_RPC_URL:-}"
)"
export AEGISLINK_RELAYER_EVM_RPC_URL
AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT="$(
  resolve_public_bridge_ibc_timeout_height \
    "${AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT:-}"
)"
export AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT

export GOCACHE
export AEGISLINK_ENABLE_REAL_IBC=1
export AEGISLINK_PUBLIC_IBC_AEGISLINK_HOME="$HOME_DIR"
export AEGISLINK_RLY_SOURCE_READY_FILE="$READY_FILE"
export AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES="$(
  ensure_csv_contains "${AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES:-swap:,stake:}" "bridge:"
)"
export AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES="$(
  ensure_csv_contains "${AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES:-swap,stake}" "bridge"
)"
SOURCE_KEY_NAME="${AEGISLINK_RLY_SOURCE_KEY_NAME:-aegislink-demo}"
DESTINATION_KEY_NAME="${AEGISLINK_RLY_DESTINATION_KEY_NAME:-osmosis-demo}"
DESTINATION_MNEMONIC="${AEGISLINK_RLY_DESTINATION_MNEMONIC:-}"
RLY_HOME="$PERSISTENT_RLY_HOME"

mkdir -p "$RUNTIME_DIR"
rm -f "$STATUS_FILE" "$CURRENT_STATUS_FILE"

trap cleanup_failed_startup EXIT

pkill -f start_aegislink_ibc_demo.sh >/dev/null 2>&1 || true
pkill -f public-bridge-relayer >/dev/null 2>&1 || true
pkill -f 'aegislinkd demo-node start' >/dev/null 2>&1 || true
sleep 1

ensure_rly_home "$RLY_HOME" "$SOURCE_KEY_NAME" "$DESTINATION_KEY_NAME" "$DESTINATION_MNEMONIC"

DESTINATION_RELAYER_ADDRESS="$(
  ./bin/relayer keys show "$DESTINATION_CHAIN_NAME" "$DESTINATION_KEY_NAME" --home "$RLY_HOME" | tr -d '\r\n'
)"
if ! destination_key_has_funds "$RLY_HOME" "$DESTINATION_KEY_NAME"; then
  echo "osmosis relayer key is not funded yet" >&2
  echo "Relayer home: $RLY_HOME" >&2
  echo "Fund this address with testnet OSMO once, then rerun this same command:" >&2
  echo "  $DESTINATION_RELAYER_ADDRESS" >&2
  exit 1
fi

run bash scripts/testnet/bootstrap_aegislink_testnet.sh "$HOME_DIR"
run bash scripts/testnet/seed_public_bridge_assets.sh "$HOME_DIR"
run bash scripts/testnet/bootstrap_public_ibc.sh
run bash scripts/testnet/seed_public_ibc_route.sh "$HOME_DIR"

echo "+ starting demo node"
AEGISLINK_DEMO_NODE_DESTINATION_LCD_BASE_URL="$DESTINATION_LCD_BASE_URL" \
  nohup bash scripts/testnet/start_aegislink_ibc_demo.sh "$HOME_DIR" >"$NODE_LOG" 2>&1 &
NODE_PID=$!
disown "$NODE_PID"

wait_for_http "http://127.0.0.1:26657/healthz" "demo node"

write_replay_state "${AEGISLINK_RELAYER_EVM_RPC_URL:-}"

SOURCE_RELAYER_ADDRESS="$(
  ./bin/relayer keys show aegislink-public-testnet-1 "$SOURCE_KEY_NAME" --home "$RLY_HOME" | tr -d '\r\n'
)"
run curl -sS -X POST http://127.0.0.1:26657/tx/fund-account \
  -H 'Content-Type: application/json' \
  -d "{\"address\":\"$SOURCE_RELAYER_ADDRESS\",\"denom\":\"stake\",\"amount\":\"1000000000\"}"

run ./bin/relayer paths new aegislink-public-testnet-1 "$DESTINATION_CHAIN_NAME" "$PATH_NAME" --home "$RLY_HOME"
run_relayer_link_with_retry

run go run ./chain/aegislink/cmd/aegislinkd tx set-route-profile \
  --home "$HOME_DIR" \
  --demo-node-ready-file "$READY_FILE" \
  --route-id osmosis-public-wallet \
  --destination-chain-id osmo-test-5 \
  --channel-id channel-0 \
  --asset-id eth \
  --destination-denom ibc/ueth \
  --enabled=true \
  --memo-prefixes "$AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES" \
  --action-types "$AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES"

echo "+ starting public bridge relayer"
export GOCACHE
export AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS="run ./chain/aegislink/cmd/aegislinkd --home $HOME_DIR --demo-node-ready-file $READY_FILE"
export AEGISLINK_RELAYER_REPLAY_STORE_PATH="$REPLAY_STORE"
export AEGISLINK_RELAYER_ATTESTATION_STATE_PATH="$ATTESTATION_STATE"
export AEGISLINK_RELAYER_RLY_CMD="${AEGISLINK_RELAYER_RLY_CMD:-./bin/relayer}"
export AEGISLINK_RELAYER_RLY_HOME="$RLY_HOME"
export AEGISLINK_RELAYER_RLY_PATH_NAME="$PATH_NAME"
export AEGISLINK_RELAYER_AUTODELIVERY_ENABLED=true
export AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT
export AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL="$DESTINATION_LCD_BASE_URL"
export AEGISLINK_RELAYER_POLL_INTERVAL_MS="${AEGISLINK_RELAYER_POLL_INTERVAL_MS:-4000}"
export AEGISLINK_RELAYER_FAILURE_BACKOFF_MS="${AEGISLINK_RELAYER_FAILURE_BACKOFF_MS:-9000}"
nohup go run ./relayer/cmd/public-bridge-relayer --loop >"$RELAYER_LOG" 2>&1 &
RELAYER_PID=$!
disown "$RELAYER_PID"
echo "$RELAYER_PID" >"$RUNTIME_DIR/relayer.pid"
if ! kill -0 "$NODE_PID" >/dev/null 2>&1; then
  echo "demo node exited early; see $NODE_LOG" >&2
  exit 1
fi
if ! kill -0 "$RELAYER_PID" >/dev/null 2>&1; then
  echo "public bridge relayer exited early; see $RELAYER_LOG" >&2
  exit 1
fi

STARTUP_COMPLETE="1"
trap - EXIT

cat >"$STATUS_FILE" <<EOF
{
  "home_dir": "$HOME_DIR",
  "ready_file": "$READY_FILE",
  "node_log": "$NODE_LOG",
  "relayer_log": "$RELAYER_LOG",
  "relayer_home": "$RLY_HOME",
  "relayer_path_name": "$PATH_NAME",
  "replay_store": "$REPLAY_STORE",
  "attestation_state_path": "$ATTESTATION_STATE",
  "node_pid": $NODE_PID,
  "relayer_pid": $RELAYER_PID,
  "rpc_address": "http://127.0.0.1:26657",
  "comet_rpc_address": "http://127.0.0.1:27657",
  "destination_lcd_base_url": "$DESTINATION_LCD_BASE_URL"
}
EOF
cp "$STATUS_FILE" "$CURRENT_STATUS_FILE"

echo
echo "Backend ready."
echo "Home: $HOME_DIR"
echo "Ready file: $READY_FILE"
echo "Node log: $NODE_LOG"
echo "Relayer log: $RELAYER_LOG"
echo "Relayer home: $RLY_HOME"
echo "Relayer path: $PATH_NAME"
echo "Status file: $STATUS_FILE"
echo
echo "Frontend:"
echo "  cd $REPO_ROOT/web && npm run dev"
