# AegisLink 0-to-100 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build AegisLink from an empty repo into a working Ethereum-to-Cosmos bridge with a Cosmos-SDK settlement chain, threshold-attested relayer flow, replay protection, rate limits, pause controls, local end-to-end tests, and an Osmosis IBC route.

**Architecture:** AegisLink has three executable surfaces: Ethereum gateway contracts, an AegisLink Cosmos-SDK chain, and a Go relayer that observes Ethereum, assembles claims, and submits them to the chain. The chain owns policy and accounting. The relayer carries evidence. Osmosis is added only after the AegisLink round-trip is stable locally.

**Tech Stack:** Go, Cosmos SDK, CometBFT, IBC-Go, Protobuf, buf, Solidity, Foundry, OpenZeppelin, Docker Compose, Anvil, GitHub Actions.

**Current checkpoint:** As of April 5, 2026, Tasks 1 through 8 and the next Task 9 routing slices have been implemented in the repository. That includes the persistent AegisLink runtime, live Ethereum RPC deposit observation, live Ethereum release execution in the local end-to-end loop, the `ibcrouter` module, route CLI surfaces, a dedicated `route-relayer`, a lightweight `mock-osmosis-target`, packet-shaped routed delivery, asynchronous acknowledgement handling, and destination-side receive state with balances or simple swap execution records. The next active roadmap target is still Task 9, but the remaining work is to deepen the route milestone into a fuller local IBC or Osmosis environment while fuller Cosmos node realism remains a worthwhile hardening track.

---

## How To Use This Plan

This is not a beginner tutorial. It is a build order.

Each task answers three questions:

- what are we building now
- why are we building it now
- what does success prove

The correct strategy is to make the system real in this order:

1. make the repo executable
2. lock the message model
3. build safety modules before bridge logic
4. build the bridge state machine
5. build Ethereum as the canonical source of events
6. build the relayer
7. prove the round-trip locally
8. only then add Osmosis routing

## Expected Repo Structure

- `chain/aegislink/cmd/aegislinkd/main.go`
- `chain/aegislink/go.mod`
- `chain/aegislink/app/app.go`
- `chain/aegislink/app/config.go`
- `chain/aegislink/x/bridge/module.go`
- `chain/aegislink/x/bridge/types/claim.go`
- `chain/aegislink/x/bridge/types/attestation.go`
- `chain/aegislink/x/bridge/types/keys.go`
- `chain/aegislink/x/bridge/types/errors.go`
- `chain/aegislink/x/bridge/keeper/keeper.go`
- `chain/aegislink/x/bridge/keeper/verify_attestation.go`
- `chain/aegislink/x/bridge/keeper/accounting.go`
- `chain/aegislink/x/bridge/keeper/msg_server.go`
- `chain/aegislink/x/bridge/keeper/keeper_test.go`
- `chain/aegislink/x/registry/module.go`
- `chain/aegislink/x/registry/types/asset.go`
- `chain/aegislink/x/registry/keeper/keeper.go`
- `chain/aegislink/x/registry/keeper/keeper_test.go`
- `chain/aegislink/x/limits/module.go`
- `chain/aegislink/x/limits/types/limits.go`
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
- `relayer/go.mod`
- `relayer/internal/config/config.go`
- `relayer/internal/evm/client.go`
- `relayer/internal/evm/watcher.go`
- `relayer/internal/evm/client_test.go`
- `relayer/internal/evm/watcher_test.go`
- `relayer/internal/attestations/collector.go`
- `relayer/internal/attestations/collector_test.go`
- `relayer/internal/cosmos/client.go`
- `relayer/internal/cosmos/watcher.go`
- `relayer/internal/cosmos/client_test.go`
- `relayer/internal/cosmos/watcher_test.go`
- `relayer/internal/replay/store.go`
- `relayer/internal/replay/store_test.go`
- `relayer/internal/pipeline/pipeline.go`
- `tests/e2e/localnet_test.go`
- `tests/e2e/bridge_roundtrip_test.go`
- `tests/e2e/osmosis_route_test.go`
- `Makefile`
- `go.work`
- `buf.yaml`
- `buf.gen.yaml`
- `foundry.toml`
- `docker-compose.yml`
- `.gitignore`

## Task 1: Bootstrap A Runnable Monorepo

**Files:**
- Create: `go.work`
- Create: `Makefile`
- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create: `foundry.toml`
- Create: `docker-compose.yml`
- Create: `.gitignore`
- Create: `chain/aegislink/go.mod`
- Create: `relayer/go.mod`
- Create: `chain/.gitkeep`
- Create: `contracts/.gitkeep`
- Create: `relayer/.gitkeep`
- Create: `proto/.gitkeep`
- Create: `tests/e2e/.gitkeep`

**What is happening:** you are creating the project shell so every later task lands in a stable place.

**What success proves:** the repo can host chain, contracts, relayer, and e2e tooling in one workspace without guessing the structure later.

- [ ] **Step 1: Create the repo skeleton**

Run: `mkdir -p chain/aegislink contracts/ethereum/test contracts/ethereum/script relayer proto tests/e2e`

Expected: the root layout exists for the three runtime surfaces and e2e harness.

- [ ] **Step 2: Add root toolchain and Go module files**

Write `go.work`, `Makefile`, `buf.yaml`, `buf.gen.yaml`, `foundry.toml`, `docker-compose.yml`, `.gitignore`, `chain/aegislink/go.mod`, and `relayer/go.mod` with placeholder but valid content.

- [ ] **Step 3: Add root commands**

Add `make format`, `make test`, `make test-e2e`, and `make devnet`.

Run: `make test`

Expected: fail because packages are not implemented yet, but the command and file discovery work.

- [ ] **Step 4: Verify the workspace shape**

Run: `find chain contracts relayer proto tests -maxdepth 2 -type d | sort`

Expected: the planned directories show up in a predictable order.

- [ ] **Step 5: Commit the bootstrap**

Run:

```bash
git add go.work Makefile buf.yaml buf.gen.yaml foundry.toml docker-compose.yml .gitignore chain contracts relayer proto tests
git commit -m "chore: bootstrap aegislink monorepo"
```

Expected: one clean baseline commit with only scaffolding.

## Task 2: Lock The Cross-Chain Message Model

**Files:**
- Create: `proto/aegislink/bridge/v1/bridge.proto`
- Create: `proto/aegislink/registry/v1/registry.proto`
- Create: `proto/aegislink/limits/v1/limits.proto`
- Create: `chain/aegislink/x/bridge/types/claim.go`
- Create: `chain/aegislink/x/bridge/types/attestation.go`
- Create: `chain/aegislink/x/bridge/types/keys.go`
- Create: `chain/aegislink/x/bridge/types/errors.go`
- Create: `chain/aegislink/x/bridge/types/claim_test.go`
- Create: `chain/aegislink/x/registry/types/asset.go`
- Create: `chain/aegislink/x/registry/types/asset_test.go`
- Create: `chain/aegislink/x/limits/types/limits.go`
- Create: `chain/aegislink/x/limits/types/limits_test.go`

**What is happening:** you are defining the claim shape before writing business logic.

**What success proves:** the whole system agrees on what a deposit claim, withdrawal claim, asset record, and replay key actually are.

- [ ] **Step 1: Write the protobuf schema**

Define claim fields for:

- source chain ID
- source contract
- tx hash
- log index
- nonce
- asset ID
- amount
- recipient
- attestation payload
- expiry or deadline

- [ ] **Step 2: Write failing identity tests**

Run: `go test ./chain/aegislink/x/...`

Expected: fail because claim identity helpers and registry types do not exist yet.

- [ ] **Step 3: Implement minimal type logic**

Create deterministic claim-key derivation in `chain/aegislink/x/bridge/types/keys.go` and validation helpers in the `types` packages.

- [ ] **Step 4: Generate protobuf bindings**

Run: `buf lint && buf generate`

Expected: lint passes and generated code lands without path ambiguity.

- [ ] **Step 5: Re-run the type tests**

Run: `go test ./chain/aegislink/x/...`

Expected: the type-layer tests now pass and the replay key format is stable.

- [ ] **Step 6: Commit the message model**

Run:

```bash
git add proto chain/aegislink/x
git commit -m "feat: define aegislink message model"
```

Expected: the protocol vocabulary is now fixed before deeper implementation starts.

## Task 3: Build Safety Modules Before Bridge Logic

**Files:**
- Create: `chain/aegislink/cmd/aegislinkd/main.go`
- Create: `chain/aegislink/app/app.go`
- Create: `chain/aegislink/app/config.go`
- Create: `chain/aegislink/x/registry/module.go`
- Create: `chain/aegislink/x/registry/keeper/keeper.go`
- Create: `chain/aegislink/x/registry/keeper/keeper_test.go`
- Create: `chain/aegislink/x/limits/module.go`
- Create: `chain/aegislink/x/limits/keeper/keeper.go`
- Create: `chain/aegislink/x/limits/keeper/keeper_test.go`
- Create: `chain/aegislink/x/pauser/module.go`
- Create: `chain/aegislink/x/pauser/keeper/keeper.go`
- Create: `chain/aegislink/x/pauser/keeper/keeper_test.go`

**What is happening:** you are building the controls that stop the bridge from becoming dangerous later.

**What success proves:** assets, limits, and pause state are enforced by the chain before claim execution is even implemented.

- [ ] **Step 1: Write failing keeper tests**

Cover:

- duplicate asset registration
- invalid asset metadata
- over-limit route attempts
- paused-flow rejection

- [ ] **Step 2: Run the keeper tests**

Run: `go test ./chain/aegislink/...`

Expected: fail because the app and keepers are not wired yet.

- [ ] **Step 3: Wire the app shell**

Register `registry`, `limits`, and `pauser` in `chain/aegislink/app/app.go` and expose a minimal binary in `chain/aegislink/cmd/aegislinkd/main.go`.

- [ ] **Step 4: Implement minimal keeper behavior**

Add just enough state and message handling to make the tests pass:

- asset create or disable
- limit set or query
- pause set or clear

- [ ] **Step 5: Re-run the chain tests**

Run: `go test ./chain/aegislink/...`

Expected: registry, limits, and pause tests pass cleanly.

- [ ] **Step 6: Commit the safety layer**

Run:

```bash
git add chain/aegislink
git commit -m "feat: add aegislink safety modules"
```

Expected: the chain can now enforce policy before bridge execution exists.

## Task 4: Implement The Bridge Verification And Accounting State Machine

**Files:**
- Create: `chain/aegislink/x/bridge/module.go`
- Create: `chain/aegislink/x/bridge/keeper/keeper.go`
- Create: `chain/aegislink/x/bridge/keeper/verify_attestation.go`
- Create: `chain/aegislink/x/bridge/keeper/accounting.go`
- Create: `chain/aegislink/x/bridge/keeper/msg_server.go`
- Create: `chain/aegislink/x/bridge/keeper/keeper_test.go`
- Create: `chain/aegislink/x/bridge/keeper/msg_server_test.go`

**What is happening:** you are building the heart of the protocol, the place where an attested claim becomes verified accounting state or a deterministic rejection.

**What success proves:** AegisLink can verify a signer threshold, enforce finality policy, map assets into local representations, and execute one claim exactly once without breaking accounting.

- [ ] **Step 1: Write failing state-machine tests**

Cover:

- valid inbound claim accepted once
- duplicate claim rejected
- insufficient attester quorum rejected
- finality-window rejection
- paused asset rejected
- over-limit claim rejected
- unknown asset rejected
- accounting state updated exactly once

- [ ] **Step 2: Run the bridge tests**

Run: `go test ./chain/aegislink/x/bridge/...`

Expected: fail because no keeper or msg server exists yet.

- [ ] **Step 3: Implement minimal claim execution**

Add:

- signer-set storage or config lookup
- threshold-attestation verification
- finality-depth or finality-window checks
- claim status store
- claim-key lookup
- policy checks against registry, limits, and pauser
- denomination mapping or representation lookup
- mint or escrow accounting for accepted inbound claims
- accepted or rejected result codes

- [ ] **Step 4: Re-run bridge tests**

Run: `go test ./chain/aegislink/x/bridge/...`

Expected: the state machine now accepts one valid claim and rejects unsafe duplicates.

- [ ] **Step 5: Run the whole chain test suite**

Run: `go test ./chain/aegislink/...`

Expected: all chain-layer tests pass together.

- [ ] **Step 6: Commit the bridge core**

Run:

```bash
git add chain/aegislink/x/bridge
git commit -m "feat: implement aegislink verification and accounting state machine"
```

Expected: the core protocol rules, verifier boundary, and inbound accounting now exist on the Cosmos side.

## Task 5: Build Ethereum As The Canonical Event Source

**Files:**
- Create: `contracts/ethereum/BridgeGateway.sol`
- Create: `contracts/ethereum/BridgeVerifier.sol`
- Create: `contracts/ethereum/test/BridgeGateway.t.sol`
- Create: `contracts/ethereum/script/Deploy.s.sol`

**What is happening:** you are defining where deposits start and where withdrawals finish.

**What success proves:** Ethereum emits the exact events the relayer and chain expect, and unsupported assets are rejected before they ever become bridge claims.

- [ ] **Step 1: Write contract tests first**

Cover:

- deposit event emission
- unsupported asset rejection
- verifier rejection on bad attestation
- pause rejection
- expiry rejection
- duplicate or reused proof rejection

- [ ] **Step 2: Run the Solidity tests**

Run: `forge test`

Expected: fail because the contracts do not exist yet.

- [ ] **Step 3: Implement the minimal contract surface**

Keep `BridgeGateway.sol` small:

- deposit entrypoint
- withdrawal release entrypoint
- supported-asset allowlist or registry gate
- canonical events
- role-gated pause control

Keep `BridgeVerifier.sol` small:

- attestation verification
- expiry check
- replay guard if needed on the contract side

- [ ] **Step 4: Re-run the Solidity tests**

Run: `forge test`

Expected: pass for the narrow happy and rejection paths.

- [ ] **Step 5: Commit the Ethereum surface**

Run:

```bash
git add contracts/ethereum foundry.toml
git commit -m "feat: add ethereum gateway contracts"
```

Expected: Ethereum is now a deterministic event source for the bridge.

## Task 6: Build The Relayer Pipeline

**Files:**
- Create: `relayer/cmd/bridge-relayer/main.go`
- Create: `relayer/internal/config/config.go`
- Create: `relayer/internal/evm/client.go`
- Create: `relayer/internal/evm/watcher.go`
- Create: `relayer/internal/evm/client_test.go`
- Create: `relayer/internal/evm/watcher_test.go`
- Create: `relayer/internal/attestations/collector.go`
- Create: `relayer/internal/attestations/collector_test.go`
- Create: `relayer/internal/cosmos/client.go`
- Create: `relayer/internal/cosmos/watcher.go`
- Create: `relayer/internal/cosmos/client_test.go`
- Create: `relayer/internal/cosmos/watcher_test.go`
- Create: `relayer/internal/replay/store.go`
- Create: `relayer/internal/replay/store_test.go`
- Create: `relayer/internal/pipeline/pipeline.go`
- Create: `relayer/internal/pipeline/pipeline_test.go`

**What is happening:** you are connecting Ethereum events to AegisLink claim submissions.

**What success proves:** the off-chain system can observe Ethereum deposits and AegisLink withdrawals, wait for finality, collect threshold attestations, checkpoint safely, and submit forward and reverse-path work without duplicating it on restart.

- [ ] **Step 1: Write failing watcher and pipeline tests**

Cover:

- event observation
- finality wait before submission
- threshold-attestation collection before submission
- checkpoint persistence
- duplicate event suppression
- submission retry after transient failure
- Cosmos withdrawal observation
- Ethereum release submission

- [ ] **Step 2: Run relayer tests**

Run: `go test ./relayer/...`

Expected: fail because watcher, replay store, and pipeline do not exist yet.

- [ ] **Step 3: Implement the minimal relayer**

Build it in this order:

- config loader
- Ethereum log watcher
- Ethereum finality waiter
- attestation collector
- replay or checkpoint store
- Cosmos withdrawal watcher
- claim assembler
- Ethereum release submitter
- Cosmos submitter
- pipeline coordinator

- [ ] **Step 4: Re-run relayer tests**

Run: `go test ./relayer/...`

Expected: pass with idempotent processing for one claim path.

- [ ] **Step 5: Commit the relayer**

Run:

```bash
git add relayer
git commit -m "feat: add aegislink relayer pipeline"
```

Expected: the off-chain evidence path now exists end to end.

## Task 7: Prove The Local Round-Trip Before Osmosis

**Files:**
- Create: `tests/e2e/localnet_test.go`
- Create: `tests/e2e/bridge_roundtrip_test.go`
- Modify: `docker-compose.yml`
- Modify: `Makefile`

**What is happening:** you are proving the system actually works as a system, not just as isolated parts.

**What success proves:** Ethereum, AegisLink, and the relayer can move one asset into the chain locally with replay and pause protections intact before reverse routing exists.

- [ ] **Step 1: Write the happy-path e2e test**

Cover:

- deposit on Ethereum
- observation by relayer
- claim acceptance on AegisLink

- [ ] **Step 2: Add failure-path e2e tests**

Cover:

- replayed submission rejected
- paused flow rejected
- disabled asset rejected

- [ ] **Step 3: Run the e2e target**

Run: `make test-e2e`

Expected: fail because the local stack is not wired yet.

- [ ] **Step 4: Wire the local stack**

Update `docker-compose.yml` and `Makefile` so Anvil, AegisLink, and the relayer can start together deterministically.

- [ ] **Step 5: Re-run the e2e target**

Run: `make test-e2e`

Expected: one narrow inbound bridge path passes end to end.

- [ ] **Step 6: Commit the local proof**

Run:

```bash
git add tests/e2e docker-compose.yml Makefile
git commit -m "test: prove local aegislink inbound flow"
```

Expected: you now have hard proof that the inbound bridge works before adding the reverse path or Osmosis route.

## Task 8: Add The Reverse Flow And Supply Invariants

**Files:**
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
- Modify: `chain/aegislink/x/bridge/keeper/keeper_test.go`
- Modify: `contracts/ethereum/BridgeGateway.sol`
- Modify: `contracts/ethereum/test/BridgeGateway.t.sol`
- Modify: `relayer/internal/cosmos/watcher.go`
- Modify: `relayer/internal/cosmos/watcher_test.go`
- Modify: `relayer/internal/evm/client.go`
- Modify: `relayer/internal/evm/client_test.go`
- Modify: `tests/e2e/bridge_roundtrip_test.go`

**What is happening:** you are extending a proven inbound bridge into a full bi-directional bridge while preserving supply accounting.

**What success proves:** the project is no longer a one-way deposit demo.

- [ ] **Step 1: Write failing reverse-flow tests**

Cover:

- burn or escrow on AegisLink
- relayed withdrawal proof back to Ethereum
- canonical release on Ethereum
- over-limit withdrawal rejected

- [ ] **Step 2: Add invariant tests**

Test:

- no double release
- no mint without accepted claim
- supply is conserved across lock, mint, burn, and release
- withdrawal rate limits are enforced deterministically

- [ ] **Step 3: Implement the minimal reverse-path logic**

Only add the exact state transitions needed for the tests, including:

- AegisLink withdrawal record emission
- relayer observation of withdrawal records
- proof or attestation assembly for the reverse path
- Ethereum release submission
- withdrawal-side limit checks before release

- [ ] **Step 4: Run focused suites**

Run:

```bash
go test ./chain/aegislink/... ./relayer/...
forge test
make test-e2e
```

Expected: the system now supports a valid reverse flow with accounting discipline intact.

- [ ] **Step 5: Commit the full bridge loop**

Run:

```bash
git add chain/aegislink contracts/ethereum relayer tests/e2e
git commit -m "feat: add reverse flow and supply invariants"
```

Expected: AegisLink is now a real bridge loop, not a one-way path.

## Task 9: Add Osmosis Routing Only After The Core Bridge Is Stable

**Files:**
- Create: `chain/aegislink/x/ibcrouter/module.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/keeper.go`
- Create: `chain/aegislink/x/ibcrouter/keeper/keeper_test.go`
- Modify: `chain/aegislink/app/app.go`
- Modify: `docker-compose.yml`
- Modify: `Makefile`
- Modify: `tests/e2e/bridge_roundtrip_test.go`
- Create: `tests/e2e/osmosis_route_test.go`

**What is happening:** you are extending a working bridge into a real Cosmos utility path.

**What success proves:** assets that are safe on AegisLink can move onward to Osmosis through a controlled and recoverable IBC route, and ack or timeout failures remain observable and recoverable.

**Current status note:** the repository already has the first deeper Task 9 slices implemented. `ibcrouter` exists, route queries and tx surfaces are wired into `aegislinkd`, failure or timeout recovery is queryable, a dedicated route-relayer can hand pending transfers to a local target, and the e2e suite now proves a live Ethereum deposit can become a completed routed transfer record on AegisLink through that local target. The main remaining gap is a fuller local IBC or Osmosis environment instead of today’s lightweight route-target harness.

- [ ] **Step 1: Write failing routing tests**

Cover:

- route enabled and channel live
- route disabled
- acknowledgement failure
- timeout failure
- recovery or refund state visible after failure

- [ ] **Step 2: Run the route tests**

Run: `go test ./chain/aegislink/... && make test-e2e`

Expected: fail because the router module, Osmosis environment, and route wiring do not exist yet.

- [ ] **Step 3: Implement minimal IBC routing**

Add only:

- route allowlist
- channel configuration
- transfer request path
- acknowledgement handling hooks
- timeout recovery or refund state handling
- queryable route status for failed transfers

- [ ] **Step 4: Add the local Osmosis and IBC-relayer path**

Extend `docker-compose.yml` and `Makefile` so the route environment includes:

- AegisLink node
- local Osmosis or equivalent route target
- IBC relayer process

The route milestone is not complete if it only passes against an in-memory simulation.

- [ ] **Step 5: Re-run focused route tests**

Run: `go test ./chain/aegislink/... && make test-e2e`

Expected: a supported asset can move from AegisLink to Osmosis in the local route path, and ack or timeout failures leave an observable recovery state.

- [ ] **Step 6: Commit the Osmosis extension**

Run:

```bash
git add chain/aegislink docker-compose.yml Makefile tests/e2e
git commit -m "feat: add osmosis route support"
```

Expected: the bridge now has a real downstream destination instead of ending at the settlement chain.

## Task 10: Harden The Project For Review, Demo, And Hiring Signal

**Files:**
- Modify: `README.md`
- Modify: `docs/architecture/01-system-architecture.md`
- Modify: `docs/architecture/02-security-and-trust-model.md`
- Modify: `docs/security-model.md`
- Modify: `docs/observability.md`
- Modify: `docs/runbooks/pause-and-recovery.md`
- Modify: `docs/runbooks/upgrade-and-rollback.md`

**What is happening:** you are turning the codebase from "it works" into "it looks like infrastructure."

**What success proves:** you understand not just implementation, but operations, failure handling, and how to present the system honestly.

- [ ] **Step 1: Add final metrics and health visibility**

Expose relayer lag, claim acceptance or rejection counters, pause events, and route failures.

- [ ] **Step 2: Update the docs to match reality**

Make sure the README, architecture docs, and runbooks describe the implemented system, not the planned one.

- [ ] **Step 3: Run the full project verification**

Run:

```bash
go test ./chain/aegislink/... ./relayer/...
forge test
make test-e2e
```

Expected: all implemented layers pass their own checks and the local end-to-end path still works.

- [ ] **Step 4: Record the demo path**

Write the exact happy-path walkthrough for:

- Ethereum deposit
- relayer observation
- AegisLink claim acceptance
- optional Osmosis route
- reverse flow if demoed

- [ ] **Step 5: Commit the hardening pass**

Run:

```bash
git add README.md docs
git commit -m "docs: harden aegislink for review and demo"
```

Expected: the repo now reads like a serious protocol project and not a class assignment.

## Recommended Start Point

If you ask "where do I start tomorrow," the answer is:

1. Task 1
2. Task 2
3. Task 3
4. stop and re-evaluate before Task 4

Reason:

- Task 1 proves the repo is runnable.
- Task 2 proves the message model is not ambiguous.
- Task 3 proves the chain can enforce safety.
- Only then are you allowed to make the bridge state machine real.

That is the shortest path to avoiding a fake bridge prototype.
