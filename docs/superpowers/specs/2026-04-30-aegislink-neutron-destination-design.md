# AegisLink — Neutron Testnet Destination Design

**Date:** 2026-04-30
**Status:** Approved for implementation

## Goal

Add Neutron testnet (`pion-1`) as a second live destination alongside the existing Osmosis testnet (`osmo-test-5`). A user on the frontend selects Neutron, enters a `neutron1...` recipient, deposits Sepolia testnet ETH, and receives an IBC-wrapped `ibc/ueth` denom on their Neutron testnet address, visible on the `pion-1` explorer.

This is a testnet-to-testnet bridge demo. Sepolia testnet ETH has no monetary value.

## Scope

- Both Osmosis testnet and Neutron testnet are live simultaneously. Osmosis is unchanged.
- Asset: ETH only (`eth` asset ID, `ueth` source denom, `ibc/ueth` destination denom on Neutron).
- Action types: `swap` and `stake` (same policy as Osmosis route).
- No changes to the chain runtime (`chain/aegislink/x/ibcrouter/`). The `RouteProfile` keeper already supports multiple profiles.
- No changes to `route-relayer` or `RouteConfig`. That code path is for the local dual-runtime demo and is not in the live Sepolia→Cosmos flow.

## Architecture Overview

```
[Frontend]
TransferPage.tsx
  destinations[]  ── add Neutron testnet entry (enabled: true, prefix: "neutron1",
                      routeId: "neutron-public-wallet")
  handleSubmit()  ── routeId now comes from selected destination (was hardcoded)

[public-bridge-relayer]
autodelivery/runtime.go
  RlyFlusher.PathName (string)         →  RlyFlusher.PathByRoute (map[routeID]pathName)
  Flusher.Flush(ctx, channelID)        →  Flusher.Flush(ctx, routeID, channelID)

[start_public_bridge_backend.sh]
  write_rly_config()  ── 2 chains  →  3 chains (+ pion-1)
  transact link       ── 1 path    →  2 paths (osmosis + neutron)
  set-route-profile   ── 1 call    →  2 calls (osmosis + neutron)
  AEGISLINK_RELAYER_RLY_PATH_NAME  →  AEGISLINK_RELAYER_RLY_PATH_MAP (CSV map)

[New config files]
  deploy/testnet/ibc/neutron-wallet-delivery.example.json
  deploy/testnet/ibc/rly/neutron-testnet.chain.example.json
  .env.public-ibc.neutron.local.example
```

## Neutron Testnet Specifics

| Field | Value |
|---|---|
| Chain ID | `pion-1` |
| Bech32 prefix | `neutron` |
| Native gas denom | `untrn` |
| RPC (default) | `https://rpc-falcron.pion-1.ntrn.tech:443` |
| gRPC (default) | `https://grpc-falcron.pion-1.ntrn.tech:443` |
| LCD (default) | `https://rest-falcron.pion-1.ntrn.tech` |
| Route ID in AegisLink | `neutron-public-wallet` |
| Asset on AegisLink | `eth` (same as Osmosis route) |
| Destination denom | `ibc/ueth` |
| IBC channel | Opened fresh per backend run via `transact link` |

The Neutron relayer key must be funded with testnet NTRN before running the backend. The script checks and prints the address if unfunded, matching the existing Osmosis key check pattern.

## Go Changes

### `relayer/internal/autodelivery/runtime.go`

Replace `RlyFlusher.PathName string` with a route-keyed map plus a fallback:

```go
type RlyFlusher struct {
    Command     string
    PathByRoute map[string]string // routeID → rly path name
    DefaultPath string            // fallback when routeID not in map
    Home        string
}
```

`Flush` signature changes to `Flush(ctx context.Context, routeID, channelID string) error`. Implementation looks up `PathByRoute[routeID]`, falls back to `DefaultPath`, errors if both are empty.

### `relayer/internal/autodelivery/coordinator.go`

`Flusher` interface changes to:

```go
type Flusher interface {
    Flush(ctx context.Context, routeID, channelID string) error
}
```

`RunOnce` already holds `intent.RouteID` and `transfer.ChannelID` in scope. Pass both into `Flush`. No logic change.

### `relayer/cmd/public-bridge-relayer/main.go`

Parse a new env var `AEGISLINK_RELAYER_RLY_PATH_MAP` in `routeID:pathName,routeID:pathName` format. Build `map[string]string` and assign to `RlyFlusher.PathByRoute`. Old `AEGISLINK_RELAYER_RLY_PATH_NAME` becomes `DefaultPath` — existing single-destination deployments continue working with no env var changes.

## Frontend Changes

**File:** `web/src/features/bridge/TransferPage.tsx`

Add `routeId string` field to the `Destination` type.

Add Neutron testnet to `destinations[]`:

```ts
{
  id: "neutron-testnet-ntrn",
  label: "Neutron Testnet (NTRN)",
  symbol: "NTRN",
  helper: "Live route available now",
  enabled: true,
  prefix: "neutron1",
  routeId: "neutron-public-wallet",
},
```

Add `routeId: "osmosis-public-wallet"` to the existing Osmosis testnet entry.

In `handleSubmit`, replace the hardcoded `routeId: "osmosis-public-wallet"` with `routeId: destination.routeId`.

CTA button label changes from `"Bridge to Osmosis"` to `"Bridge to {destination.label}"`.

Recipient helper text removes the "Osmosis only" caveat.

## Backend Script Changes

**File:** `scripts/testnet/start_public_bridge_backend.sh`

New env vars (all overridable):

```bash
NEUTRON_CHAIN_NAME="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_CHAIN_NAME:-pion-1}"
NEUTRON_CHAIN_ID="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_CHAIN_ID:-pion-1}"
NEUTRON_RPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR:-https://rpc-falcron.pion-1.ntrn.tech:443}"
NEUTRON_GRPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR:-https://grpc-falcron.pion-1.ntrn.tech:443}"
NEUTRON_LCD_BASE_URL="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL:-https://rest-falcron.pion-1.ntrn.tech}"
NEUTRON_ACCOUNT_PREFIX="${AEGISLINK_RLY_NEUTRON_ACCOUNT_PREFIX:-neutron}"
NEUTRON_GAS_PRICE_DENOM="${AEGISLINK_RLY_NEUTRON_GAS_PRICE_DENOM:-untrn}"
NEUTRON_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_NEUTRON_GAS_PRICE_AMOUNT:-1.3}"
NEUTRON_KEY_NAME="${AEGISLINK_RLY_NEUTRON_KEY_NAME:-neutron-demo}"
NEUTRON_MNEMONIC="${AEGISLINK_RLY_NEUTRON_MNEMONIC:-}"
NEUTRON_PATH_NAME="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_PATH_NAME:-live-ntrn-ui-$RUN_ID}"
NEUTRON_ROUTE_ID="neutron-public-wallet"
```

`write_rly_config()` gains a third chain block for `pion-1`.

`run_relayer_link_with_retry` currently reads `$PATH_NAME` from outer scope. It must be updated to accept the path name as a `$1` positional argument so it can be called for both Osmosis and Neutron paths without duplicating the retry logic.

After the Osmosis bootstrap (unchanged), the script:

1. Ensures the Neutron relayer key exists (restore from mnemonic or generate)
2. Checks the Neutron key has testnet NTRN (prints address and exits if unfunded)
3. Runs `./bin/relayer paths new aegislink-public-testnet-1 "$NEUTRON_CHAIN_NAME" "$NEUTRON_PATH_NAME"`
4. Runs `run_relayer_link_with_retry "$NEUTRON_PATH_NAME"` for the Neutron path (and updates the existing Osmosis call to `run_relayer_link_with_retry "$PATH_NAME"`)
5. Calls `aegislinkd tx set-route-profile` for the Neutron route
6. Exports `AEGISLINK_RELAYER_RLY_PATH_MAP` before starting the relayer

```bash
export AEGISLINK_RELAYER_RLY_PATH_MAP="osmosis-public-wallet:$PATH_NAME,neutron-public-wallet:$NEUTRON_PATH_NAME"
```

## New Config Files

### `deploy/testnet/ibc/neutron-wallet-delivery.example.json`

Same structure as `osmosis-wallet-delivery.example.json`:

```json
{
  "enabled": false,
  "source_chain_id": "aegislink-public-testnet-1",
  "destination_chain_id": "pion-1",
  "provider": "hermes",
  "wallet_prefix": "neutron",
  "channel_id": "channel-public-neutron",
  "port_id": "transfer",
  "route_id": "neutron-public-wallet",
  "allowed_memo_prefixes": ["swap:", "stake:", "bridge:"],
  "allowed_action_types": ["swap", "stake", "bridge"],
  "assets": [
    {
      "asset_id": "eth",
      "source_denom": "ueth",
      "destination_denom": "ibc/ueth"
    }
  ]
}
```

### `deploy/testnet/ibc/rly/neutron-testnet.chain.example.json`

Same structure as `osmosis-testnet.chain.example.json` with `pion-1` values.

### `.env.public-ibc.neutron.local.example`

```bash
AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR=https://rpc-falcron.pion-1.ntrn.tech:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR=https://grpc-falcron.pion-1.ntrn.tech:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL=https://rest-falcron.pion-1.ntrn.tech
AEGISLINK_RLY_NEUTRON_KEY_NAME=neutron-demo
AEGISLINK_RLY_NEUTRON_MNEMONIC=
```

## Testing

### Go unit tests

- `relayer/internal/autodelivery/coordinator_test.go` — update `Flush(channelID)` call sites to `Flush(routeID, channelID)`. Add test: intent with `RouteID: "neutron-public-wallet"` selects correct path from `PathByRoute`.
- `relayer/internal/autodelivery/runtime_test.go` — update signature. Add test: `DefaultPath` fallback is used when `routeID` not in map (backwards compat).
- `relayer/cmd/public-bridge-relayer/main_test.go` — add tests for `AEGISLINK_RELAYER_RLY_PATH_MAP` parsing: empty, single entry, two-entry cases.

### Frontend unit tests

- `web/src/features/bridge/bridge.test.tsx` — verify Neutron testnet appears as enabled, prefix is `neutron1`, `routeId` is `neutron-public-wallet`. Verify `handleSubmit` uses the correct `routeId` per selected destination.

### Out of scope

- Live `pion-1` connectivity tests (operator validation, not CI)
- End-to-end Sepolia → Neutron test (requires funded testnet keys and live chain access)
- New e2e test file (can be added once the Neutron path is live and stable)

## Backwards Compatibility

Existing Osmosis-only deployments using only `AEGISLINK_RELAYER_RLY_PATH_NAME` continue working. The old env var becomes `RlyFlusher.DefaultPath`. No operator action required to keep the Osmosis flow running.
