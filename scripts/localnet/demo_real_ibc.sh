#!/usr/bin/env bash

set -euo pipefail

MODE="${1:-demo}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CACHE_ROOT="${AEGISLINK_GO_CACHE_ROOT:-/tmp/aegislink-e2e-go-cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/gocache}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/gomodcache}"

cd "$REPO_ROOT/tests/e2e"

case "$MODE" in
  demo)
    echo "Running the real dual-runtime route demo..."
    echo "Proof path: AegisLink SDK-store home -> route-relayer -> osmo-locald home"
    go test ./... -run 'TestRealDestinationChainBootstrap|TestRealIBCRoute'
    ;;
  inspect)
    echo "Inspecting the real dual-runtime route path..."
    echo "Inspection points: destination status, pools, balances, packets, executions"
    go test ./... -run 'TestRealIBCRoute'
    ;;
  *)
    echo "usage: $0 [demo|inspect]" >&2
    exit 1
    ;;
esac
