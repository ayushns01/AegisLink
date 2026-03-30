# AegisLink Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build AegisLink as a verifiable Ethereum-to-Cosmos interoperability layer with a Cosmos-SDK bridge zone, threshold-attested relaying, replay protection, rate limits, pause controls, an asset registry, and a later IBC route to Osmosis.

**Architecture:** AegisLink is a monorepo with three core surfaces: a Cosmos-SDK chain that owns bridge state and safety controls, Ethereum contracts that emit and validate bridge activity, and a Go relayer that observes both sides and submits threshold-attested messages. Phase 1 delivers Ethereum <-> bridge zone transfers with an explicit trust model. Phase 2 adds IBC routing from the bridge zone to Osmosis without changing the bridge security boundaries.

**Tech Stack:** Go, Cosmos SDK, CometBFT, IBC-Go, Protobuf, buf, Solidity, Foundry, OpenZeppelin, Docker Compose, Anvil, GitHub Actions.

---

## Expected Repo Structure

Create and keep responsibilities isolated by surface area:

- `chain/aegislink/cmd/aegislinkd/main.go`
- `chain/aegislink/app/app.go`
- `chain/aegislink/app/config.go`
- `chain/aegislink/x/bridge/module.go`
- `chain/aegislink/x/bridge/keeper/keeper.go`
- `chain/aegislink/x/bridge/keeper/keeper_test.go`
- `chain/aegislink/x/registry/module.go`
- `chain/aegislink/x/registry/keeper/keeper.go`
- `chain/aegislink/x/registry/keeper/keeper_test.go`
- `chain/aegislink/x/limits/module.go`
- `chain/aegislink/x/limits/keeper/keeper.go`
- `chain/aegislink/x/limits/keeper/keeper_test.go`
- `chain/aegislink/x/pauser/module.go`
- `chain/aegislink/x/pauser/keeper/keeper.go`
- `chain/aegislink/x/pauser/keeper/keeper_test.go`
- `chain/aegislink/x/ibcrouter/module.go`
- `chain/aegislink/x/ibcrouter/keeper/keeper.go`
- `chain/aegislink/x/ibcrouter/keeper/keeper_test.go`
- `proto/aegislink/bridge/v1/bridge.proto`
- `proto/aegislink/registry/v1/registry.proto`
- `proto/aegislink/limits/v1/limits.proto`
- `contracts/ethereum/BridgeGateway.sol`
- `contracts/ethereum/BridgeVerifier.sol`
- `contracts/ethereum/test/BridgeGateway.t.sol`
- `contracts/ethereum/script/Deploy.s.sol`
- `relayer/cmd/bridge-relayer/main.go`
- `relayer/internal/evm/watcher.go`
- `relayer/internal/evm/watcher_test.go`
- `relayer/internal/attestations/collector.go`
- `relayer/internal/attestations/collector_test.go`
- `relayer/internal/cosmos/client.go`
- `relayer/internal/cosmos/client_test.go`
- `relayer/internal/replay/store.go`
- `relayer/internal/replay/store_test.go`
- `tests/e2e/bridge_roundtrip_test.go`
- `tests/e2e/localnet_test.go`
- `README.md`
- `docs/foundations/01-bridge-basics.md`
- `docs/foundations/02-eth-cosmos-primer.md`
- `docs/architecture/01-system-architecture.md`
- `docs/architecture/02-security-and-trust-model.md`
- `docs/security-model.md`
- `docs/observability.md`
- `docs/runbooks/pause-and-recovery.md`
- `docs/runbooks/upgrade-and-rollback.md`
- `Makefile`
- `go.work`
- `buf.yaml`
- `buf.gen.yaml`
- `foundry.toml`
- `docker-compose.yml`

## Task 1: Bootstrap The Monorepo

**Files:**
- Create: `go.work`
- Create: `Makefile`
- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create: `foundry.toml`
- Create: `docker-compose.yml`
- Create: `.gitignore`

- [ ] **Step 1: Write the empty workspace and toolchain files**

Add the root workspace and build metadata so future commands can target the correct packages and tools.

Create the top-level directory skeleton for `chain/aegislink`, `contracts/ethereum`, `relayer`, `proto`, and `tests/e2e` as part of this step.

- [ ] **Step 2: Run the first repo-wide checks**

Run: `make test`

Expected: fail cleanly because the packages do not exist yet, but the command should resolve the root files correctly.

- [ ] **Step 3: Add the minimal bootstrap targets**

Create `make format`, `make test`, `make devnet`, and `make test-e2e` targets with placeholder command wiring.

- [ ] **Step 4: Verify the repo shape**

Run: `find chain contracts relayer proto tests -maxdepth 2 -type d`

Expected: the top-level directories exist and can be used by later tasks.

## Task 2: Define Shared Bridge Messages

**Files:**
- Create: `proto/aegislink/bridge/v1/bridge.proto`
- Create: `proto/aegislink/registry/v1/registry.proto`
- Create: `proto/aegislink/limits/v1/limits.proto`
- Create: `chain/aegislink/x/bridge/types/keys.go`
- Create: `chain/aegislink/x/bridge/types/errors.go`
- Create: `chain/aegislink/x/registry/types/asset.go`
- Create: `chain/aegislink/x/registry/types/asset_test.go`
- Create: `chain/aegislink/x/limits/types/limits.go`
- Create: `chain/aegislink/x/limits/types/limits_test.go`

- [ ] **Step 1: Write the protobuf schema for bridge events and attestations**

Define message IDs, nonces, chain IDs, asset IDs, timestamps, and expiration fields.

- [ ] **Step 2: Add validation tests for message identity**

Create tests that prove every bridge message has a unique replay key and rejects missing required fields.

- [ ] **Step 3: Add codegen wiring**

Wire `buf generate` so generated types land in predictable package paths.

- [ ] **Step 4: Run schema and validation checks**

Run: `buf lint && buf generate`

Expected: schemas pass linting and generate stable Go types.

## Task 3: Build The Bridge Zone Core

**Files:**
- Create: `chain/aegislink/cmd/aegislinkd/main.go`
- Create: `chain/aegislink/app/app.go`
- Create: `chain/aegislink/app/config.go`
- Create: `chain/aegislink/x/bridge/module.go`
- Create: `chain/aegislink/x/bridge/keeper/keeper.go`
- Create: `chain/aegislink/x/bridge/keeper/keeper_test.go`
- Create: `chain/aegislink/x/registry/module.go`
- Create: `chain/aegislink/x/registry/keeper/keeper.go`
- Create: `chain/aegislink/x/registry/keeper/keeper_test.go`
- Create: `chain/aegislink/x/limits/module.go`
- Create: `chain/aegislink/x/limits/keeper/keeper.go`
- Create: `chain/aegislink/x/limits/keeper/keeper_test.go`
- Create: `chain/aegislink/x/pauser/module.go`
- Create: `chain/aegislink/x/pauser/keeper/keeper.go`
- Create: `chain/aegislink/x/pauser/keeper/keeper_test.go`

- [ ] **Step 1: Write failing keeper tests for registry, replay, rate limits, and pause behavior**

Focus on duplicate asset registration, duplicate message processing, over-limit routing, and paused-flow rejection.

- [ ] **Step 2: Run the chain unit tests**

Run: `go test ./chain/aegislink/...`

Expected: fail until the core modules and keepers exist.

- [ ] **Step 3: Implement the minimal module and keeper logic**

Add the smallest set of handlers, stores, and queries that make the tests pass.

- [ ] **Step 4: Re-run the chain tests**

Run: `go test ./chain/aegislink/...`

Expected: pass with registry, replay, limit, and pause checks enforced.

## Task 4: Build The Ethereum Contracts

**Files:**
- Create: `contracts/ethereum/BridgeGateway.sol`
- Create: `contracts/ethereum/BridgeVerifier.sol`
- Create: `contracts/ethereum/test/BridgeGateway.t.sol`
- Create: `contracts/ethereum/script/Deploy.s.sol`

- [ ] **Step 1: Write contract tests for the happy path and rejection paths**

Cover accepted attestations, expired attestations, replayed attestations, and paused contract flows.

- [ ] **Step 2: Run the Solidity test suite**

Run: `forge test`

Expected: fail until the contracts and test harness are in place.

- [ ] **Step 3: Implement the minimal contracts**

Add the bridge gateway and verifier surface needed for the first bridge flow.

- [ ] **Step 4: Re-run the Solidity test suite**

Run: `forge test`

Expected: pass with deterministic event emission and proof validation.

## Task 5: Build The Go Relayer

**Files:**
- Create: `relayer/cmd/bridge-relayer/main.go`
- Create: `relayer/internal/evm/watcher.go`
- Create: `relayer/internal/evm/watcher_test.go`
- Create: `relayer/internal/attestations/collector.go`
- Create: `relayer/internal/attestations/collector_test.go`
- Create: `relayer/internal/cosmos/client.go`
- Create: `relayer/internal/cosmos/client_test.go`
- Create: `relayer/internal/replay/store.go`
- Create: `relayer/internal/replay/store_test.go`

- [ ] **Step 1: Write failing tests for event watching, attestation collection, and idempotent submission**

The relayer must survive restarts without double-submitting the same bridge message.

- [ ] **Step 2: Run the relayer tests**

Run: `go test ./relayer/...`

Expected: fail until the watch, collect, submit, and replay paths are implemented.

- [ ] **Step 3: Implement the minimal relayer pipeline**

Watch Ethereum events, gather threshold attestations, persist checkpoints, and submit Cosmos transactions.

- [ ] **Step 4: Re-run the relayer tests**

Run: `go test ./relayer/...`

Expected: pass with duplicate-event protection and basic retry logic.

## Task 6: Add The Local End-To-End Harness

**Files:**
- Create: `tests/e2e/localnet_test.go`
- Create: `tests/e2e/bridge_roundtrip_test.go`
- Modify: `docker-compose.yml`
- Modify: `Makefile`

- [ ] **Step 1: Write the end-to-end happy-path test**

Prove one asset can move from Ethereum to the bridge zone and back again.

- [ ] **Step 2: Run the e2e test target**

Run: `make test-e2e`

Expected: fail at first because the localnet orchestration is not wired up yet.

- [ ] **Step 3: Wire the local Ethereum and Cosmos services together**

Make the local stack deterministic enough for repeatable bridge tests.

- [ ] **Step 4: Re-run the e2e target**

Run: `make test-e2e`

Expected: pass for the happy path and reject replayed or paused flows.

## Task 7: Route Supported Assets To Osmosis

**Files:**
- Create: `chain/aegislink/x/ibcrouter/module.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/keeper.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/keeper_test.go`
- Modify: `chain/aegislink/app/app.go`
- Modify: `proto/aegislink/bridge/v1/bridge.proto`
- Modify: `tests/e2e/bridge_roundtrip_test.go`
- Create: `tests/e2e/osmosis_route_test.go`

- [ ] **Step 1: Write the IBC routing tests**

Cover the first supported asset moving from AegisLink to Osmosis and receiving acknowledgement handling.

- [ ] **Step 2: Implement the minimal IBC wiring**

Add the IBC-facing module hooks and transfer flow needed for the first route.

- [ ] **Step 3: Re-run the e2e suite**

Run: `make test-e2e`

Expected: pass the bridge route and Osmosis handoff scenario.

## Task 8: Harden The Launch

**Files:**
- Modify: `docs/security-model.md`
- Modify: `docs/runbooks/pause-and-recovery.md`
- Modify: `docs/runbooks/upgrade-and-rollback.md`
- Modify: `docs/observability.md`
- Modify: `README.md`

- [ ] **Step 1: Document the trust assumptions and operational controls**

Explain what v1 trusts, what it does not trust, and how operators should respond to incidents.

- [ ] **Step 2: Add monitoring and runbook coverage**

Document the alerts, metrics, and manual controls the team needs before external testing.

- [ ] **Step 3: Verify the final release checklist**

Run: `make test && make test-e2e && forge test`

Expected: the full suite is green before any public testnet or audit handoff.
