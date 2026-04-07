# AegisLink Future Realism Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move AegisLink from a polished local interoperability system into a more real Cosmos and IBC environment with a stronger trust model and production-style operator surfaces.

**Architecture:** Treat the remaining work as a realism ladder, not a rewrite. First replace the current Cosmos-inspired runtime shell with a real single-node Cosmos SDK and CometBFT application, then replace the Osmosis-lite route harness with a real local IBC path, then harden verification and operations on top of that more honest substrate.

**Tech Stack:** Go, Cosmos SDK, CometBFT, IBC-Go, Hermes or a local IBC relayer, Solidity, Foundry, Docker Compose, Prometheus, Grafana, Buf, existing AegisLink relayers and e2e harness.

---

## Scope and sequencing

This roadmap spans multiple subsystems, but they are not independent. The order matters:

1. make AegisLink a real single-node chain
2. put a real IBC-connected destination next to it
3. upgrade the trust model on top of the real runtime
4. add production-style metrics, dashboards, and operator flows
5. only then spend time on protocol expansion

If a team wants parallel execution later, split this document into one execution plan per phase. For now, keep it as one phased roadmap because each phase depends on the previous one.

## File structure and responsibility map

These are the main future files and directories this plan assumes.

- Modify: `chain/aegislink/app/app.go`
  Real Cosmos SDK app wiring, module manager, store keys, begin or end blockers only if needed.
- Modify: `chain/aegislink/app/config.go`
  Runtime config that evolves from the current shell config into a real node config bridge.
- Create: `chain/aegislink/app/encoding.go`
  Codec, interface registry, tx config, amino legacy support only if still needed.
- Create: `chain/aegislink/app/genesis.go`
  Real genesis defaults, validation, export, and import path.
- Create: `chain/aegislink/cmd/aegislinkd/cmd/root.go`
  Cobra root command for a real daemon layout.
- Create: `chain/aegislink/cmd/aegislinkd/cmd/start.go`
  Node start command wired to CometBFT and app startup.
- Create: `chain/aegislink/cmd/aegislinkd/cmd/init.go`
  Genesis and config initialization command.
- Create: `chain/aegislink/cmd/aegislinkd/cmd/query.go`
  Root query command plus module subcommands.
- Create: `chain/aegislink/cmd/aegislinkd/cmd/tx.go`
  Root tx command plus module subcommands.
- Create: `chain/aegislink/x/bridge/keeper/sdk_keeper.go`
  Store-backed bridge keeper state instead of JSON snapshot state.
- Create: `chain/aegislink/x/bridge/genesis.go`
  Genesis import/export for bridge state.
- Create: `chain/aegislink/x/bridge/client/cli/query.go`
  Bridge query CLI.
- Create: `chain/aegislink/x/bridge/client/cli/tx.go`
  Bridge tx CLI.
- Create: `chain/aegislink/x/registry/keeper/sdk_keeper.go`
  Store-backed asset registry.
- Create: `chain/aegislink/x/limits/keeper/sdk_keeper.go`
  Store-backed rate-limit state.
- Create: `chain/aegislink/x/pauser/keeper/sdk_keeper.go`
  Store-backed pause state.
- Create: `chain/aegislink/x/ibcrouter/keeper/sdk_keeper.go`
  Store-backed outbound routing and transfer state.
- Create: `chain/aegislink/x/*/module.go` updates where needed
  Real AppModule wiring, services, and genesis registration.
- Create: `proto/aegislink/bridge/v1/tx.proto`
  Real gRPC tx service definitions for bridge claims and withdrawals.
- Create: `proto/aegislink/bridge/v1/query.proto`
  Real bridge queries for claims, supply, and withdrawals.
- Create: `proto/aegislink/ibcrouter/v1/query.proto`
  Route and transfer query surfaces.
- Create: `tests/e2e/real_chain_test.go`
  Single-node chain e2e against a real app daemon.
- Create: `tests/e2e/real_ibc_route_test.go`
  Real IBC route lifecycle tests.
- Create: `localnet/compose/real-chains.yml`
  Compose file for a real AegisLink plus destination chain environment.
- Create: `localnet/config/aegislink/`
  Chain config, app config, genesis templates, relayer config.
- Create: `localnet/config/osmosis-lite/` or `localnet/config/osmo-local/`
  Destination chain config.
- Create: `scripts/localnet/bootstrap_real_chain.sh`
  Deterministic bootstrap for validator keys, genesis accounts, and gentx.
- Create: `scripts/localnet/bootstrap_ibc.sh`
  Channel and client bootstrap between AegisLink and the destination chain.
- Create: `scripts/localnet/demo_real_ibc.sh`
  One-command real chain and real route demo entrypoint.
- Create: `deploy/monitoring/prometheus.yml`
  Metrics scrape config.
- Create: `deploy/monitoring/grafana/`
  Dashboard JSON and provisioning files.
- Modify: `docs/observability.md`
  Move from demo-only inspection to real metrics and operator flow.
- Modify: `docs/project-positioning.md`
  Keep the “what is real vs simulated” section honest as phases land.
- Modify: `README.md`
  Update the top-level story as each realism milestone lands.

## Finish lines

There are five future finish lines after Phase 4:

1. **Phase 5:** AegisLink runs as a real single-node Cosmos SDK and CometBFT chain.
2. **Phase 6:** AegisLink routes to a real local IBC-connected destination chain.
3. **Phase 7:** Ethereum-side verification and signer management are strong enough to demonstrate a believable security roadmap.
4. **Phase 8:** Operators can monitor, inspect, and recover the system with real metrics and dashboards.
5. **Phase 9:** The protocol is extensible enough to support more routes, assets, and policy surfaces without redesign.

---

## Phase 5: Real AegisLink Cosmos Runtime

**Goal:** Replace the JSON-backed runtime shell with a realer single-node Cosmos SDK runtime path while preserving the current bridge behavior. In the current repo scope, this means store-backed keepers, generated proto surfaces, service-backed CLI flows, and a single-node bootstrap or e2e proof. A fully networked CometBFT or ABCI node still remains future work beyond this checkpoint.

**Phase 5 status:** Complete on April 6, 2026 for the single-node SDK-store milestone. The app now has explicit store-key, encoding-config, and genesis-validation seams, bridge plus policy state persists through real Cosmos KV stores, generated bridge or route proto surfaces exist inside the `chain/aegislink` module, `aegislinkd` tx or query flows run against the SDK-store runtime through app services, and `tests/e2e/real_chain_test.go` proves the bootstrap plus claim lifecycle end to end. This is still not yet a full networked CometBFT or ABCI chain, and that distinction should remain explicit in docs and demos.

### Task 5.1: Create the real app skeleton

**Files:**
- Modify: `chain/aegislink/app/app.go`
- Modify: `chain/aegislink/app/config.go`
- Create: `chain/aegislink/app/encoding.go`
- Create: `chain/aegislink/app/genesis.go`
- Test: `chain/aegislink/app/app_test.go`

- [x] **Step 1: Write failing bootstrap tests**

Add tests that assert:
- the app builds with store keys for `bridge`, `registry`, `limits`, `pauser`, and `ibcrouter`
- the app exposes a real codec and tx config
- default genesis validates

- [x] **Step 2: Run the focused app tests**

Run: `go test ./chain/aegislink/app -run 'TestNewApp|TestDefaultGenesis|TestEncodingConfig'`

Expected: FAIL because real app wiring does not exist yet.

- [x] **Step 3: Implement the minimal real app skeleton**

Add:
- a real encoding config
- store keys
- module manager registration
- default genesis export

- [x] **Step 4: Re-run the focused app tests**

Run: `go test ./chain/aegislink/app -run 'TestNewApp|TestDefaultGenesis|TestEncodingConfig'`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add chain/aegislink/app
git commit -m "feat: create real aegislink app skeleton"
```

### Task 5.2: Move bridge and policy state into real stores

**Files:**
- Create: `chain/aegislink/x/bridge/keeper/sdk_keeper.go`
- Create: `chain/aegislink/x/bridge/genesis.go`
- Create: `chain/aegislink/x/registry/keeper/sdk_keeper.go`
- Create: `chain/aegislink/x/limits/keeper/sdk_keeper.go`
- Create: `chain/aegislink/x/pauser/keeper/sdk_keeper.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/sdk_keeper.go`
- Test: `chain/aegislink/x/bridge/keeper/sdk_keeper_test.go`
- Test: `chain/aegislink/x/registry/keeper/sdk_keeper_test.go`
- Test: `chain/aegislink/x/limits/keeper/sdk_keeper_test.go`
- Test: `chain/aegislink/x/pauser/keeper/sdk_keeper_test.go`
- Test: `chain/aegislink/x/ibcrouter/keeper/sdk_keeper_test.go`

- [x] **Step 1: Write failing store-backed keeper tests**

Cover:
- processed claim persistence
- registry persistence
- rate-limit persistence
- pause persistence
- transfer persistence

- [x] **Step 2: Run the focused keeper tests**

Run: `go test ./chain/aegislink/x/... -run 'Test.*SDKKeeper'`

Expected: FAIL because store-backed keepers do not exist yet.

- [x] **Step 3: Implement the minimal store-backed keepers**

Keep behavior compatible with the current runtime:
- same rejection rules
- same counters
- same transfer lifecycle states

- [x] **Step 4: Re-run the focused keeper tests**

Run: `go test ./chain/aegislink/x/... -run 'Test.*SDKKeeper'`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add chain/aegislink/x
git commit -m "feat: add store-backed aegislink keepers"
```

### Task 5.3: Add real tx and query services

**Files:**
- Create: `proto/aegislink/bridge/v1/tx.proto`
- Create: `proto/aegislink/bridge/v1/query.proto`
- Create: `proto/aegislink/ibcrouter/v1/query.proto`
- Create: `chain/aegislink/x/bridge/client/cli/query.go`
- Create: `chain/aegislink/x/bridge/client/cli/tx.go`
- Create: `chain/aegislink/x/ibcrouter/client/cli/query.go`
- Modify: `buf.yaml`
- Modify: `buf.gen.yaml`
- Test: `chain/aegislink/cmd/aegislinkd/main_test.go`

- [x] **Step 1: Write failing CLI and query tests**

Cover:
- query claim by message ID
- query transfers
- tx submit deposit claim
- tx execute withdrawal

- [x] **Step 2: Run the focused runtime CLI tests**

Run: `go test ./chain/aegislink/cmd/aegislinkd -run 'TestRunQuery|TestRunTx'`

Expected: FAIL because real service-backed CLI does not exist yet.

- [x] **Step 3: Add proto services and generated types**

Run:
- `buf lint`
- `buf generate`

Expected: PASS once the proto surfaces are valid.

- [x] **Step 4: Implement service-backed CLI wiring**

Use gRPC query clients and tx broadcasting instead of direct state-file mutation.

- [x] **Step 5: Re-run focused CLI tests**

Run: `go test ./chain/aegislink/cmd/aegislinkd -run 'TestRunQuery|TestRunTx'`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add proto chain/aegislink/cmd chain/aegislink/x buf.yaml buf.gen.yaml
git commit -m "feat: add aegislink tx and query services"
```

### Task 5.4: Turn `aegislinkd` into a real single-node devnet

**Files:**
- Create: `chain/aegislink/cmd/aegislinkd/cmd/root.go`
- Create: `chain/aegislink/cmd/aegislinkd/cmd/start.go`
- Create: `chain/aegislink/cmd/aegislinkd/cmd/init.go`
- Create: `chain/aegislink/cmd/aegislinkd/cmd/query.go`
- Create: `chain/aegislink/cmd/aegislinkd/cmd/tx.go`
- Create: `localnet/config/aegislink/`
- Create: `scripts/localnet/bootstrap_real_chain.sh`
- Test: `tests/e2e/real_chain_test.go`

- [x] **Step 1: Write failing single-node e2e tests**

Cover:
- `aegislinkd init`
- `aegislinkd start`
- submit a real deposit claim tx
- query resulting state through the running node

- [x] **Step 2: Run the real-chain e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRealAegisLinkChain'`

Expected: FAIL because no real node exists yet.

- [x] **Step 3: Implement the single-node bootstrap**

Add:
- real home directory layout
- genesis generation
- bootstrap flow for the SDK-store single-node runtime
- `aegislinkd` commands that run against that runtime through `init`, `start`, `tx`, and `query`

- [x] **Step 4: Re-run the real-chain e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRealAegisLinkChain'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/cmd localnet/config/aegislink scripts/localnet tests/e2e
git commit -m "feat: run aegislink as a real single-node chain"
```

**Phase 5 exit criteria:**
- `aegislinkd` starts a real single-node SDK-store runtime
- state lives in Cosmos stores, not JSON snapshots
- tx and query flows go through generated service surfaces and app services
- current bridge behavior is preserved
- docs stay honest that this is still not yet a full CometBFT or ABCI networked chain

---

## Phase 6: Real Local IBC and Osmosis Environment

**Goal:** Replace the osmosis-lite harness with a real local IBC-connected destination chain.

**Phase 6 status:** Complete on April 7, 2026 for the current repo scope as a dual-runtime local route milestone. AegisLink now has explicit packet-send, ack, and timeout hooks in `ibcrouter`; a dedicated destination runtime can be bootstrapped through `osmo-locald`; `route-relayer` can drive transfers and acks across the AegisLink and destination homes without the older HTTP mock-target entrypoint; and `tests/e2e/real_ibc_route_test.go` proves destination bootstrap plus routed completion end to end. This is still not yet a full networked IBC-Go or Hermes-connected environment, and that distinction should remain explicit in docs and demos.

### Task 6.1: Stand up a real destination chain

**Files:**
- Create: `localnet/config/osmo-local/`
- Create: `scripts/localnet/bootstrap_destination_chain.sh`
- Create: `localnet/compose/real-chains.yml`
- Test: `tests/e2e/real_ibc_route_test.go`

- [x] **Step 1: Write a failing destination-chain bootstrap test**

Cover:
- destination chain boots
- accounts and faucet balances exist
- the chain is queryable locally

- [x] **Step 2: Run the destination-chain e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRealDestinationChainBootstrap'`

Expected: FAIL because the real destination chain localnet is not present.

- [x] **Step 3: Implement the destination bootstrap**

Choose one:
- LocalOsmosis if stable and maintainable
- a minimal Cosmos SDK destination chain if lighter weight is required

- [x] **Step 4: Re-run the destination-chain e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRealDestinationChainBootstrap'`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add localnet/config/osmo-local localnet/compose scripts/localnet tests/e2e
git commit -m "feat: bootstrap real local destination chain"
```

### Task 6.2: Add real IBC transfer wiring

**Files:**
- Modify: `chain/aegislink/x/ibcrouter/module.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/ibc_transfer.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/ibc_ack.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/ibc_timeout.go`
- Test: `chain/aegislink/x/ibcrouter/keeper/ibc_transfer_test.go`

- [x] **Step 1: Write failing IBC router tests**

Cover:
- send packet
- receive acknowledgement
- timeout handling
- refund-safe recovery

- [x] **Step 2: Run the focused IBC router tests**

Run: `go test ./chain/aegislink/x/ibcrouter/keeper -run 'TestIBC'`

Expected: FAIL because real IBC transfer wiring is missing.

- [x] **Step 3: Implement the IBC transfer hooks**

Use IBC-Go transfer module integration rather than HTTP callbacks.

- [x] **Step 4: Re-run the focused IBC router tests**

Run: `go test ./chain/aegislink/x/ibcrouter/keeper -run 'TestIBC'`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add chain/aegislink/x/ibcrouter
git commit -m "feat: integrate ibc transfer lifecycle"
```

### Task 6.3: Replace the route harness with a real local IBC relay path

**Files:**
- Modify: `relayer/internal/route/relay.go`
- Create: `relayer/internal/route/ibc_runtime.go`
- Modify: `relayer/internal/config/route_config.go`
- Create: `localnet/config/hermes/`
- Create: `scripts/localnet/bootstrap_ibc.sh`
- Test: `tests/e2e/real_ibc_route_test.go`

- [x] **Step 1: Write failing real-route e2e tests**

Cover:
- Ethereum deposit
- AegisLink mint
- real IBC send
- destination receive
- ack or timeout
- source-side completion or refund

- [x] **Step 2: Run the real-route e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRealIBCRoute'`

Expected: FAIL because routing still depends on the local mock target.

- [x] **Step 3: Implement real route runtime integration**

Choose and wire:
- Hermes or another local IBC relayer
- channel and client bootstrap
- destination queries for balances and receipts

- [x] **Step 4: Re-run the real-route e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRealIBCRoute'`

Expected: PASS

- [x] **Step 5: Commit**

```bash
git add relayer/internal/route relayer/internal/config localnet/config/hermes scripts/localnet tests/e2e
git commit -m "feat: replace mock routing with real local ibc flow"
```

### Task 6.4: Update the demo to use the real route path

**Files:**
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `docs/demo-walkthrough.md`
- Create: `scripts/localnet/demo_real_ibc.sh`

- [x] **Step 1: Write down the new real-demo expectations in docs**

Document:
- chain startup order
- IBC bootstrap order
- what is still local-only

- [x] **Step 2: Implement the scripted real IBC demo**

Add:
- `make real-demo`
- `make inspect-real-demo`

- [x] **Step 3: Verify the real demo path**

Run:
- `make real-demo`
- `make inspect-real-demo`

Expected: PASS

- [x] **Step 4: Commit**

```bash
git add Makefile README.md docs/demo-walkthrough.md scripts/localnet
git commit -m "feat: add real ibc demo path"
```

**Phase 6 exit criteria:**
- route flow no longer depends on the osmosis-lite HTTP target
- AegisLink sends explicit packet, ack, and timeout lifecycle state across a real destination runtime boundary
- destination balances and acknowledgements come from the destination runtime home, not the old HTTP-only harness
- docs remain explicit that this is still not yet full networked IBC-Go or Hermes

---

## Phase 7: Trust Model and Verifier Upgrades

**Goal:** Make the security roadmap concrete enough that the system can evolve beyond a single-attester v1 verifier.

**Phase 7 status:** Complete on April 7, 2026 for the current repo scope. The repository now has a real threshold-verifier path on Ethereum through `ThresholdBridgeVerifier.sol`, the gateway still depends on the narrow verifier interface, AegisLink bridge attestations bind to versioned signer sets with activation and expiry rules, and the verifier-evolution documentation now explains what is built versus what remains future work.

### Task 7.1: Add threshold verifier support on Ethereum

**Files:**
- Modify: `contracts/ethereum/IBridgeVerifier.sol`
- Create: `contracts/ethereum/ThresholdBridgeVerifier.sol`
- Modify: `contracts/ethereum/BridgeGateway.sol`
- Test: `contracts/ethereum/test/ThresholdBridgeVerifier.t.sol`

- [x] **Step 1: Write failing verifier tests**

Cover:
- threshold reached
- threshold not reached
- duplicate signer rejection
- signer rotation compatibility

- [x] **Step 2: Run the focused Foundry verifier tests**

Run: `cd contracts/ethereum && forge test --offline --match-test 'testThreshold'`

Expected: FAIL because threshold verification does not exist yet.

- [x] **Step 3: Implement the minimal threshold verifier**

Keep:
- `BridgeGateway` interface stable where possible
- replay protection intact

- [x] **Step 4: Re-run the focused Foundry verifier tests**

Run: `cd contracts/ethereum && forge test --offline --match-test 'testThreshold'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add contracts/ethereum
git commit -m "feat: add threshold bridge verifier"
```

### Task 7.2: Add signer-set lifecycle management

**Files:**
- Modify: `chain/aegislink/x/bridge/types/attestation.go`
- Modify: `chain/aegislink/x/bridge/keeper/verify_attestation.go`
- Create: `chain/aegislink/x/bridge/keeper/signer_set.go`
- Test: `chain/aegislink/x/bridge/keeper/signer_set_test.go`

- [x] **Step 1: Write failing signer-set tests**

Cover:
- signer-set versioning
- signer-set activation
- expiry
- mismatch rejection

- [x] **Step 2: Run the focused bridge signer tests**

Run: `go test ./chain/aegislink/x/bridge/keeper -run 'TestSignerSet'`

Expected: FAIL because signer-set lifecycle state is missing.

- [x] **Step 3: Implement the minimal signer-set lifecycle**

Add:
- versioned signer sets
- active set lookup
- attestation binding to a set version

- [x] **Step 4: Re-run the focused bridge signer tests**

Run: `go test ./chain/aegislink/x/bridge/keeper -run 'TestSignerSet'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/bridge
git commit -m "feat: add bridge signer set lifecycle"
```

### Task 7.3: Add a light-client spike and evaluation doc

**Files:**
- Create: `docs/architecture/04-verifier-evolution.md`
- Modify: `docs/security-model.md`
- Modify: `docs/project-positioning.md`

- [x] **Step 1: Document the current verifier boundary**

Describe:
- what is replaceable today
- what is still coupled

- [x] **Step 2: Add an evaluated roadmap**

Compare:
- threshold attestation
- optimistic bridge
- Ethereum light-client path

- [x] **Step 3: Review for honesty**

Check that the docs clearly state what is built versus researched.

- [ ] **Step 4: Commit**

```bash
git add docs/architecture/04-verifier-evolution.md docs/security-model.md docs/project-positioning.md
git commit -m "docs: add verifier evolution roadmap"
```

**Phase 7 exit criteria:**
- Ethereum verifier supports a believable threshold path
- signer sets can evolve safely
- future verification work is documented honestly

---

## Phase 8: Production-Style Operations

**Goal:** Give operators real metrics, dashboards, and recovery workflows instead of only demo inspection surfaces.

### Task 8.1: Add Prometheus-style metrics to the binaries

**Files:**
- Create: `chain/aegislink/internal/metrics/metrics.go`
- Create: `relayer/internal/metrics/metrics.go`
- Modify: `chain/aegislink/cmd/aegislinkd/main.go`
- Modify: `relayer/cmd/bridge-relayer/main.go`
- Modify: `relayer/cmd/route-relayer/main.go`
- Modify: `relayer/cmd/mock-osmosis-target/main.go`
- Test: `relayer/internal/metrics/metrics_test.go`

- [ ] **Step 1: Write failing metrics tests**

Cover counters and gauges for:
- processed claims
- failed claims
- pending transfers
- timed-out transfers
- destination swap failures

- [ ] **Step 2: Run the focused metrics tests**

Run: `go test ./relayer/... ./chain/aegislink/... -run 'TestMetrics'`

Expected: FAIL because metrics exporters do not exist yet.

- [ ] **Step 3: Implement minimal Prometheus surfaces**

Expose `/metrics` where appropriate.

- [ ] **Step 4: Re-run the focused metrics tests**

Run: `go test ./relayer/... ./chain/aegislink/... -run 'TestMetrics'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/internal/metrics relayer/internal/metrics chain/aegislink/cmd relayer/cmd
git commit -m "feat: add prometheus metrics surfaces"
```

### Task 8.2: Add dashboards and scrape config

**Files:**
- Create: `deploy/monitoring/prometheus.yml`
- Create: `deploy/monitoring/grafana/dashboards/`
- Create: `deploy/monitoring/grafana/provisioning/`
- Modify: `docker-compose.yml`
- Modify: `docs/observability.md`

- [ ] **Step 1: Define dashboard panels**

Include:
- claim acceptance and rejection
- route lifecycle counts
- destination execution failures
- relayer loop health

- [ ] **Step 2: Add monitoring stack config**

Wire Prometheus and Grafana into local compose.

- [ ] **Step 3: Verify monitoring stack boots**

Run: `docker compose -f docker-compose.yml up prometheus grafana`

Expected: PASS with scrape targets healthy.

- [ ] **Step 4: Commit**

```bash
git add deploy/monitoring docker-compose.yml docs/observability.md
git commit -m "feat: add local monitoring stack"
```

### Task 8.3: Strengthen runbooks and recovery drills

**Files:**
- Modify: `docs/runbooks/pause-and-recovery.md`
- Modify: `docs/runbooks/upgrade-and-rollback.md`
- Create: `docs/runbooks/incident-drills.md`
- Test: `tests/e2e/recovery_drill_test.go`

- [ ] **Step 1: Write failing recovery-drill tests**

Cover:
- relayer restart with replay persistence
- timed-out route refund
- paused asset recovery
- signer-set mismatch rejection

- [ ] **Step 2: Run the recovery-drill e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRecoveryDrill'`

Expected: FAIL because the drill path is not codified yet.

- [ ] **Step 3: Implement the drill scripts and docs**

Add:
- explicit drill steps
- expected logs and counters

- [ ] **Step 4: Re-run the recovery-drill e2e slice**

Run: `cd tests/e2e && go test ./... -run 'TestRecoveryDrill'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add docs/runbooks tests/e2e
git commit -m "docs: add recovery drill coverage"
```

**Phase 8 exit criteria:**
- binaries expose real metrics
- dashboards exist and scrape locally
- operators can follow recovery drills using docs and tests

---

## Phase 9: Protocol Expansion

**Goal:** Add extensibility only after runtime, IBC, security, and ops are credible.

### Task 9.1: Generalize destination route registry

**Files:**
- Modify: `chain/aegislink/x/ibcrouter/keeper/sdk_keeper.go`
- Create: `chain/aegislink/x/ibcrouter/types/route_profile.go`
- Test: `chain/aegislink/x/ibcrouter/keeper/route_profile_test.go`

- [ ] **Step 1: Write failing route-profile tests**

Cover:
- multiple destinations
- per-route policy
- asset allowlists by route

- [ ] **Step 2: Run focused route-profile tests**

Run: `go test ./chain/aegislink/x/ibcrouter/keeper -run 'TestRouteProfile'`

Expected: FAIL because route profiles do not exist yet.

- [ ] **Step 3: Implement minimal route profiles**

- [ ] **Step 4: Re-run focused route-profile tests**

Run: `go test ./chain/aegislink/x/ibcrouter/keeper -run 'TestRouteProfile'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/ibcrouter
git commit -m "feat: add destination route profiles"
```

### Task 9.2: Add governance-style policy changes

**Files:**
- Create: `chain/aegislink/x/governance/`
- Modify: `chain/aegislink/app/app.go`
- Modify: `README.md`
- Test: `chain/aegislink/x/governance/keeper/keeper_test.go`

- [ ] **Step 1: Write failing governance tests**

Cover:
- asset enable or disable proposal
- limit update proposal
- route policy update proposal

- [ ] **Step 2: Run the governance tests**

Run: `go test ./chain/aegislink/x/governance/...`

Expected: FAIL because governance module does not exist yet.

- [ ] **Step 3: Implement minimal policy-governance surfaces**

- [ ] **Step 4: Re-run the governance tests**

Run: `go test ./chain/aegislink/x/governance/...`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/governance chain/aegislink/app README.md
git commit -m "feat: add policy governance module"
```

### Task 9.3: Add more asset and route actions

**Files:**
- Modify: `relayer/internal/route/relay.go`
- Modify: `chain/aegislink/x/ibcrouter/keeper/sdk_keeper.go`
- Modify: `tests/e2e/real_ibc_route_test.go`

- [ ] **Step 1: Write failing extension tests**

Cover:
- new route actions beyond swap
- multi-hop path support if justified
- asset-specific route constraints

- [ ] **Step 2: Run focused route-extension tests**

Run: `cd tests/e2e && go test ./... -run 'TestRouteExtensions'`

Expected: FAIL because the new route actions are not implemented yet.

- [ ] **Step 3: Implement the smallest useful extension set**

Keep this phase intentionally narrow.

- [ ] **Step 4: Re-run focused route-extension tests**

Run: `cd tests/e2e && go test ./... -run 'TestRouteExtensions'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/route chain/aegislink/x/ibcrouter tests/e2e
git commit -m "feat: extend route actions and asset policies"
```

**Phase 9 exit criteria:**
- multiple destinations are configurable
- route policy changes are governed rather than hand-edited
- the protocol can grow without redesigning core bridge boundaries

---

## Recommended execution order

Use this exact order:

1. Phase 5: real single-node chain
2. Phase 6: real local IBC destination
3. Phase 7: trust-model upgrades
4. Phase 8: production-style operations
5. Phase 9: protocol expansion

Do not start Phase 6 before Phase 5 is stable. Do not start Phase 9 before Phases 7 and 8 are credible.

## Exit criteria for the whole roadmap

Call the next-generation AegisLink stack “substantially more real” when all of these are true:

- `aegislinkd` runs as a real single-node Cosmos SDK and CometBFT chain
- bridge and route state live in module stores
- AegisLink routes to a real local IBC-connected destination chain
- Ethereum verification supports a stronger threshold path
- metrics and dashboards exist for operators
- README and positioning docs still clearly separate what is real from what is local-only

## Suggested first execution slice

Start here:

1. Task 5.1: create the real app skeleton
2. Task 5.2: move bridge and policy state into real stores
3. Task 5.3: add real tx and query services

That is the smallest slice that meaningfully changes AegisLink from “persistent runtime shell” into “real chain application.”
