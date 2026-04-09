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
    echo "Running the real Hermes-shaped local route demo..."
    echo "Proof path: AegisLink SDK-store home -> packet relay -> osmo-locald home -> ack relay"
    go test ./... -run 'TestRealDestinationChainBootstrap|TestRealIBCRoute|TestRealHermesIBC'
    ;;
  inspect)
    echo "Inspecting the real Hermes-shaped local route path..."
    echo "Inspection points: link metadata, destination packets, packet-acks, balances, executions"
    go test ./... -run 'TestRealHermesIBC|TestRealIBCRoute'
    ;;
  *)
    echo "usage: $0 [demo|inspect]" >&2
    exit 1
    ;;
esac
