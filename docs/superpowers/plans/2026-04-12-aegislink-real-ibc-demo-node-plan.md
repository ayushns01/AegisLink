# AegisLink Real IBC Demo Node Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the smallest honest single-validator AegisLink demo node that exposes real network surfaces for IBC relaying and can eventually deliver a bridged asset from AegisLink to a real Osmosis wallet.

**Architecture:** Keep the current harness alive, but add a real networked demo-node path inside `chain/aegislink` that uses Cosmos SDK plus `ibc-go` transfer support. Bootstrap that node with one command, seed the same public bridge assets into it, then add relayer bootstrap assets for `rly` first and Hermes only as a compatibility fallback.

**Tech Stack:** Go, Cosmos SDK, CometBFT, ibc-go transfer, existing AegisLink keepers, shell bootstrap scripts, `rly`, chain-registry metadata, local e2e harnesses.

---

## File Structure

- Modify: `chain/aegislink/cmd/aegislinkd/main.go`
  - Add a real demo-node entrypoint without breaking current harness commands.
- Create: `chain/aegislink/networked/`
  - Hold the minimal real app wiring for the demo node.
- Create: `chain/aegislink/networked/app.go`
  - Construct the networked Cosmos SDK app.
- Create: `chain/aegislink/networked/config.go`
  - Keep networked-node config parsing isolated from current harness config.
- Create: `chain/aegislink/networked/start.go`
  - Contain the real node startup orchestration and readiness checks.
- Create: `chain/aegislink/networked/testutil/`
  - Shared helpers for networked-node tests.
- Create: `scripts/testnet/start_aegislink_ibc_demo.sh`
  - One-command local bootstrap for the real AegisLink node.
- Create: `scripts/testnet/bootstrap_rly_path.sh`
  - Generate or apply `rly` path config against AegisLink and Osmosis metadata.
- Modify: `deploy/testnet/aegislink/network.json`
  - Upgrade it from “intended endpoints” toward the real-node endpoint contract.
- Create: `deploy/testnet/ibc/rly/`
  - Store relayer config templates and generated path artifacts.
- Create: `tests/e2e/real_ibc_demo_node_test.go`
  - Cover local demo-node startup and endpoint readiness.
- Create: `tests/e2e/rly_bootstrap_test.go`
  - Cover local `rly` config/bootstrap generation without claiming live Osmosis yet.
- Modify: `docs/runbooks/public-bridge-ops.md`
  - Add the real-node bootstrap and `rly` workflow.
- Modify: `README.md`
  - Keep public claims honest as the real-node path appears.

## Entrypoint Contract

The real-node path should use a new explicit CLI contract instead of overloading the current harness `start` command:

- `aegislinkd demo-node start --home <dir>`
- `aegislinkd demo-node status --home <dir>`

The existing `aegislinkd start` command should keep its current harness semantics. This avoids breaking the current repo behavior while giving relayer and bootstrap scripts a fixed real-node target.

## Task 1: Add a Networked Demo-Node Mode to `aegislinkd`

**Files:**
- Create: `chain/aegislink/networked/app.go`
- Create: `chain/aegislink/networked/config.go`
- Create: `chain/aegislink/networked/start.go`
- Modify: `chain/aegislink/cmd/aegislinkd/main.go`
- Test: `tests/e2e/real_ibc_demo_node_test.go`

- [ ] **Step 1: Write the failing e2e test for demo-node startup**

Create a test that expects:
- the exact entrypoint `aegislinkd demo-node start --home <dir>`
- a health-checkable RPC endpoint
- a health-checkable gRPC endpoint declaration or readiness result

- [ ] **Step 2: Run the focused test to confirm it fails**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNodeStartsAndExposesEndpoints' -count=1`
Expected: FAIL because the real networked startup mode does not exist yet.

- [ ] **Step 3: Add minimal networked config and startup seams**

Implement a dedicated `networked` package that keeps the new node startup logic isolated from the current harness.

- [ ] **Step 4: Add a `start` mode or subcommand for the networked demo node**

The CLI contract should be exactly:

```bash
go run ./chain/aegislink/cmd/aegislinkd demo-node start --home /tmp/aegislink-ibc-demo-home
```

Do not hide this behind the existing harness `start` command.

- [ ] **Step 5: Re-run the focused startup test**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNodeStartsAndExposesEndpoints' -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add chain/aegislink/cmd/aegislinkd/main.go chain/aegislink/networked tests/e2e/real_ibc_demo_node_test.go
git commit -m "feat: add aegislink demo node startup mode"
```

## Task 2: Wire Real SDK and IBC Transfer App Surfaces

**Files:**
- Modify: `chain/aegislink/networked/app.go`
- Modify: `chain/aegislink/networked/start.go`
- Test: `tests/e2e/real_ibc_demo_node_test.go`

- [ ] **Step 1: Write failing tests for transfer-ready app wiring**

Add tests that expect:
- the app to expose transfer-capable module wiring
- bank state to be reachable through the real node path
- seeded bridge assets to become bank or module state on the real node path
- the transfer module account and keeper composition to exist for ICS20

- [ ] **Step 2: Run the focused tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNodeWiresTransferModule|TestRealIBCDemoNodeSeedsBridgeAssetsIntoBankState' -count=1`
Expected: FAIL because transfer wiring is incomplete.

- [ ] **Step 3: Implement minimal app wiring**

Keep this narrow:
- auth essentials
- bank keeper and module account setup
- bridge-state bootstrap into bank and registry state
- `ibc-go` core plus transfer-module composition needed for ICS20
- query and tx surfaces required for:
  - bank balance inspection
  - transfer initiation
  - relayer state queries against the node

Do not add governance, staking, or unrelated modules beyond what is necessary to start a valid single-validator demo chain and send ICS20 transfers.

- [ ] **Step 4: Re-run the focused tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNodeWiresTransferModule|TestRealIBCDemoNodeSeedsBridgeAssetsIntoBankState' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/networked tests/e2e/real_ibc_demo_node_test.go
git commit -m "feat: wire transfer-ready aegislink demo app"
```

## Task 3: Add the One-Command Demo Bootstrap

**Files:**
- Create: `scripts/testnet/start_aegislink_ibc_demo.sh`
- Modify: `deploy/testnet/aegislink/network.json`
- Modify: `docs/runbooks/public-bridge-ops.md`
- Test: `tests/e2e/real_ibc_demo_node_test.go`

- [ ] **Step 1: Write the failing bootstrap test**

Add a test that expects the shell bootstrap to:
- initialize the demo node home
- start or print the real networked startup command
- expose the configured endpoints in a machine-readable way

- [ ] **Step 2: Run the focused bootstrap test**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestStartAegisLinkIBCDemoBootstrap' -count=1`
Expected: FAIL because the bootstrap script does not exist.

- [ ] **Step 3: Implement the bootstrap script**

Keep it local and demo-oriented:
- one validator
- one home dir
- one endpoint set
- no production orchestration

- [ ] **Step 4: Re-run the bootstrap test**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestStartAegisLinkIBCDemoBootstrap' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/testnet/start_aegislink_ibc_demo.sh deploy/testnet/aegislink/network.json docs/runbooks/public-bridge-ops.md tests/e2e/real_ibc_demo_node_test.go
git commit -m "feat: add one-command aegislink ibc demo bootstrap"
```

## Task 4: Add `rly` Bootstrap Assets for AegisLink and Osmosis

**Files:**
- Create: `deploy/testnet/ibc/rly/`
- Create: `scripts/testnet/bootstrap_rly_path.sh`
- Test: `tests/e2e/rly_bootstrap_test.go`
- Modify: `docs/runbooks/public-bridge-ops.md`

- [ ] **Step 1: Write failing tests for `rly` config generation**

Cover:
- AegisLink chain config rendering
- Osmosis metadata ingestion
- path config generation

- [ ] **Step 2: Run the focused test**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRlyBootstrapGeneratesPathConfig' -count=1`
Expected: FAIL because `rly` bootstrap artifacts do not exist.

- [ ] **Step 3: Implement the bootstrap assets**

Pull current Osmosis metadata from chain-registry-compatible inputs rather than burying stale constants everywhere in the repo.

- [ ] **Step 4: Re-run the focused test**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRlyBootstrapGeneratesPathConfig' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add deploy/testnet/ibc/rly scripts/testnet/bootstrap_rly_path.sh tests/e2e/rly_bootstrap_test.go docs/runbooks/public-bridge-ops.md
git commit -m "feat: add rly bootstrap for aegislink demo node"
```

## Task 5: Prove Local Packet Lifecycle Against the Real Demo Node

**Files:**
- Modify: `tests/e2e/real_ibc_demo_node_test.go`
- Modify: `tests/e2e/rly_bootstrap_test.go`
- Modify: `docs/runbooks/public-bridge-ops.md`

- [ ] **Step 1: Write the failing packet-lifecycle tests**

Cover:
- transfer initiation from the real AegisLink node
- relayer-observable packet intent
- acknowledgement or timeout persistence
- a local counterparty chain only for this milestone, not live Osmosis yet

- [ ] **Step 2: Run the focused tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNodePacketLifecycle|TestRlyBootstrapPathCanRelayLocalPackets' -count=1`
Expected: FAIL because the end-to-end packet lifecycle is not yet wired.

- [ ] **Step 3: Implement the minimal lifecycle support**

Use a local counterparty demo chain for this milestone. Do not depend on live Osmosis testnet yet. Do not add swaps or stake routing here. Plain asset-preserving ICS20 only.

- [ ] **Step 4: Re-run the focused tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNodePacketLifecycle|TestRlyBootstrapPathCanRelayLocalPackets' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/e2e/real_ibc_demo_node_test.go tests/e2e/rly_bootstrap_test.go docs/runbooks/public-bridge-ops.md
git commit -m "test: prove local packet lifecycle on aegislink demo node"
```

## Task 6: Add a Gated Live Osmosis Wallet Smoke Test

Current status in this worktree: the live `AegisLink -> Osmosis` wallet-delivery proof now exists. The remaining value of this task is turning that manually proven path into a gated repeatable smoke test and documenting the operator procedure cleanly.

**Files:**
- Modify: `tests/e2e/osmosis_wallet_delivery_test.go`
- Modify: `docs/runbooks/public-bridge-ops.md`
- Modify: `README.md`

- [ ] **Step 1: Write the gated live test**

The test should only run when real local env is present for:
- `AEGISLINK_LIVE_OSMOSIS_RPC_ADDR`
- `AEGISLINK_LIVE_OSMOSIS_GRPC_ADDR`
- `AEGISLINK_LIVE_OSMOSIS_CHAIN_ID`
- `AEGISLINK_LIVE_OSMOSIS_RELAYER_KEY_NAME`
- `AEGISLINK_LIVE_OSMOSIS_WALLET_ADDRESS`
- `AEGISLINK_LIVE_RLY_BIN`
- `AEGISLINK_LIVE_RLY_HOME`
- any local AegisLink demo-node home and key material required by the same run

- [ ] **Step 2: Run the test in default mode**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestLiveOsmosisWalletReceipt' -count=1`
Expected: SKIP without the required live env.

- [ ] **Step 3: Document the live-smoke procedure**

Explain:
- what env is required
- how to start the demo node
- how to bootstrap `rly`
- how to verify receipt in the target Osmosis wallet
- that this test is the first point where the repo may claim live Osmosis wallet delivery

That claim boundary is now crossed for the `AegisLink -> Osmosis` leg in the current repo scope. What remains is the automated gated test and the stricter Sepolia-backed source coupling into the same live path.

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/osmosis_wallet_delivery_test.go docs/runbooks/public-bridge-ops.md README.md
git commit -m "test: add gated live osmosis wallet smoke"
```

## Final Verification

- [ ] Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/... -count=1`
- [ ] Run: `GOCACHE=/tmp/aegislink-gocache go test ./tests/e2e -run 'TestRealIBCDemoNode|TestRlyBootstrap|TestOsmosisWalletDelivery' -count=1`
- [ ] Run the one-command local bootstrap: `scripts/testnet/start_aegislink_ibc_demo.sh`
- [ ] Verify that the generated `rly` config is present under `deploy/testnet/ibc/rly/`
- [ ] Run: `git diff --check`

## Notes for Workers

- Keep current harness paths working while the real demo-node path is being added.
- The repo now has a manually proven live Osmosis-delivery run for the `AegisLink -> Osmosis` leg, but keep the stricter Sepolia-backed boundary explicit until the gated live test and source-side coupling are both complete.
- Pull unstable chain metadata from current sources during bootstrap where possible instead of freezing assumptions into checked-in constants.
- Prefer `rly` first. Only pivot to Hermes if `rly` proves clearly incompatible with the final node shape.
- Treat Task 5 as the local IBC milestone and Task 6 as the first live Osmosis milestone. Do not collapse those into one claim.
