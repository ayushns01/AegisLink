# Real Wallet Asset Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn AegisLink from a verified local bridge system into a public testnet bridge that can move both native Sepolia ETH and Sepolia ERC-20 assets into a real Cosmos wallet, then redeem those bridged assets back to Sepolia.

**Architecture:** Keep Ethereum as the canonical source chain and AegisLink as the bridge zone. Add two explicit deposit classes on Ethereum, one for native ETH and one for ERC-20 custody, then mint canonical bridge representations on a real AegisLink testnet that users can hold in a real Cosmos wallet. After that, add real redeem flow back to Sepolia and only then extend optional delivery onward to Osmosis testnet over real IBC if the product goal still requires an Osmosis wallet instead of an AegisLink wallet.

**Tech Stack:** Solidity, Go, Cosmos SDK, CometBFT, IBC-Go or Hermes, Foundry, Anvil, Sepolia RPC, Cosmos wallet tooling, Docker Compose, relayers, current AegisLink runtime.

---

## Scope and finish line

This plan is intentionally broader than the local-harness roadmap that is already complete. The new finish line is:

1. a user sends native ETH or ERC-20 from a Sepolia EVM wallet
2. the bridge verifies and settles that deposit
3. the user receives the bridged asset in a **real Cosmos wallet**
4. the user can burn or redeem that bridged asset
5. the corresponding ETH or ERC-20 is released back on Sepolia

This plan keeps the asset identity intact:

- native ETH deposit -> bridged ETH representation on Cosmos
- ERC-20 deposit -> bridged representation of that same ERC-20 on Cosmos

This plan does **not** treat destination-side swaps as part of the core bridge. Swap support is optional and belongs after the canonical asset-bridge path works.

## Recommended delivery order

Do this in two product milestones:

1. **Milestone 1:** Sepolia ETH or ERC-20 -> real AegisLink testnet wallet -> redeem back to Sepolia
2. **Milestone 2:** AegisLink testnet wallet -> real Osmosis testnet wallet over IBC, if public Osmosis delivery is still required

Milestone 1 is the fastest honest public demo. Milestone 2 is bigger and should only start once Milestone 1 is stable.

## File structure and responsibility map

These are the main files and directories this plan expects to create or modify.

- Modify: `contracts/ethereum/BridgeGateway.sol`
  Add explicit native ETH deposit and native ETH release handling next to ERC-20 flow.
- Modify: `contracts/ethereum/IBridgeVerifier.sol`
  Keep the verifier boundary stable while the gateway adds native ETH and typed asset-class semantics.
- Modify: `contracts/ethereum/BridgeVerifier.sol`
  Ensure release verification remains compatible with both ETH and ERC-20 release payloads.
- Modify: `contracts/ethereum/ThresholdBridgeVerifier.sol`
  Keep the threshold path aligned with the widened release payloads.
- Create: `contracts/ethereum/test/BridgeGateway.native.t.sol`
  Focused native ETH deposit, custody, release, replay, and redeem coverage.
- Modify: `contracts/ethereum/test/BridgeGateway.t.sol`
  Expand generic ERC-20 behavior around mixed asset classes.
- Modify: `chain/aegislink/x/registry/types/asset.go`
  Add asset-class metadata so AegisLink can distinguish native ETH representations from ERC-20-backed representations.
- Modify: `chain/aegislink/x/registry/keeper/keeper.go`
  Persist source-chain asset metadata, decimals, Ethereum addresses, and destination denoms.
- Modify: `chain/aegislink/x/bridge/types/claim.go`
  Extend deposit and withdrawal claim payloads to describe native ETH vs ERC-20 source assets clearly.
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
  Mint and burn per-asset bridge denoms for both ETH and ERC-20 classes.
- Modify: `chain/aegislink/x/bridge/keeper/accounting.go`
  Track custody-backed supply per asset class, not only by denom string.
- Modify: `chain/aegislink/x/bridge/keeper/invariants.go`
  Add cross-checks for locked ETH, locked ERC-20, minted supply, and redeemed release totals.
- Modify: `chain/aegislink/app/app.go`
  Expose new bridge asset metadata and wallet-facing balances through runtime status and services.
- Modify: `chain/aegislink/app/service.go`
  Add wallet-oriented query surfaces for bridged assets and redeemable balances.
- Modify: `chain/aegislink/cmd/aegislinkd/main.go`
  Add operator and wallet-facing CLI for public testnet bridging and redemption.
- Create: `chain/aegislink/x/bank/` or integrate with existing Cosmos bank module seams
  Real wallet balances must exist on-chain as transferable Cosmos balances.
- Create: `deploy/testnet/aegislink/`
  Public testnet node config, genesis, peers, and validator bootstrap for a real AegisLink chain.
- Create: `scripts/testnet/deploy_sepolia_bridge.sh`
  Deploy verifier and gateway contracts to Sepolia.
- Create: `scripts/testnet/bootstrap_aegislink_testnet.sh`
  Start a real AegisLink public testnet or a reproducible public devnet.
- Create: `scripts/testnet/register_bridge_assets.sh`
  Register native ETH and ERC-20 assets with the live AegisLink chain.
- Modify: `relayer/internal/evm/rpc_source.go`
  Observe both native ETH deposit events and ERC-20 deposit events from Sepolia.
- Modify: `relayer/internal/evm/rpc_release.go`
  Release either native ETH or ERC-20 during redeem.
- Modify: `relayer/internal/attestations/collector.go`
  Keep attestation payloads aligned with widened asset metadata.
- Modify: `relayer/internal/pipeline/pipeline.go`
  Carry source asset class and address cleanly through the bridge pipeline.
- Create: `relayer/cmd/public-bridge-relayer/main.go`
  Public testnet relayer entrypoint with explicit Sepolia and AegisLink network config.
- Create: `tests/e2e/sepolia_bridge_smoke_test.go`
  Realistic public-config smoke coverage with mocked RPC where needed.
- Create: `tests/e2e/public_wallet_delivery_test.go`
  Prove a deposit results in a balance held by a real Cosmos wallet address on the live AegisLink chain.
- Create: `tests/e2e/public_redeem_test.go`
  Prove burn-and-release back to Sepolia for both ETH and ERC-20.
- Create: `docs/runbooks/public-bridge-ops.md`
  Operator steps for Sepolia bridge deployment and recovery.
- Modify: `docs/project-positioning.md`
  Keep “what is real” wording honest as the public bridge comes online.
- Modify: `docs/security-model.md`
  Add native ETH custody, public-key ops, and redeem risks.
- Modify: `README.md`
  Add the public bridge demo path once it is real.

If Milestone 2 is required:

- Create: `deploy/testnet/ibc/`
  Hermes or relayer config for real IBC between AegisLink and Osmosis testnet.
- Modify: `chain/aegislink/x/ibcrouter/keeper/`
  Convert bridged balances into outbound IBC sends.
- Create: `tests/e2e/osmosis_wallet_delivery_test.go`
  Prove real wallet receipt on Osmosis testnet.

## Phase G: Public Asset Model and Native ETH Support

Current status:
- Task G1 is complete in the worktree: registry assets now distinguish native ETH from ERC-20 custody, derive canonical destination denoms, and keep legacy ERC-20 claim hashing compatible.
- Task G2 is complete in the worktree: the Ethereum gateway now supports native ETH deposit and release locally with replay, pause, and forwarding-recipient coverage.
- Task G3 is complete in the worktree: AegisLink now mints and burns canonical bridged denoms for native ETH and ERC-20 assets, and native ETH withdrawals emit the canonical zero-address source asset for Sepolia release routing.

### Task G1: Model bridge assets as generic public-testnet assets

**Files:**
- Modify: `chain/aegislink/x/registry/types/asset.go`
- Modify: `chain/aegislink/x/registry/keeper/keeper.go`
- Modify: `chain/aegislink/x/bridge/types/claim.go`
- Test: `chain/aegislink/x/registry/keeper/keeper_test.go`
- Test: `chain/aegislink/x/bridge/types/claim_test.go`

- [x] **Step 1: Write failing registry tests**

Cover:
- native ETH asset registration
- ERC-20 asset registration
- source asset address or source asset class is persisted
- destination denom is derived deterministically

- [x] **Step 2: Run focused tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/registry/... ./chain/aegislink/x/bridge/types/...`
Expected: FAIL because assets are not modeled with a native-or-token class yet.

- [x] **Step 3: Implement minimal asset-class metadata**

Add fields like:
- `source_asset_kind` with values `native_eth` or `erc20`
- `source_asset_address`
- `destination_denom`
- `display_symbol`

- [x] **Step 4: Re-run tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/registry/... ./chain/aegislink/x/bridge/types/...`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add chain/aegislink/x/registry chain/aegislink/x/bridge/types
git commit -m "feat: model native and token bridge assets"
```

### Task G2: Add native ETH deposit and release support on Ethereum

**Files:**
- Modify: `contracts/ethereum/BridgeGateway.sol`
- Modify: `contracts/ethereum/BridgeVerifier.sol`
- Modify: `contracts/ethereum/ThresholdBridgeVerifier.sol`
- Create: `contracts/ethereum/test/BridgeGateway.native.t.sol`
- Modify: `contracts/ethereum/test/BridgeGateway.t.sol`

- [x] **Step 1: Write failing native ETH tests**

Cover:
- payable ETH deposit emits the canonical event
- ETH custody balance increases exactly by the deposit amount
- release sends ETH back to the recipient
- replayed proof is rejected
- paused gateway rejects ETH deposit and ETH release

- [x] **Step 2: Run focused Foundry tests**

Run: `cd contracts/ethereum && forge test --offline --match-path 'test/BridgeGateway.native.t.sol'`
Expected: FAIL because the gateway does not expose payable ETH deposit or ETH release yet.

- [x] **Step 3: Implement the smallest useful native-ETH gateway path**

Add:
- `depositETH(string recipient, uint64 expiry) payable`
- canonical ETH asset ID handling
- ETH release branch inside `release`
- custody accounting that distinguishes ETH from ERC-20

- [x] **Step 4: Re-run focused Foundry tests**

Run: `cd contracts/ethereum && forge test --offline --match-path 'test/BridgeGateway.native.t.sol'`
Expected: PASS

- [x] **Step 5: Re-run broader Solidity tests**

Run: `cd contracts/ethereum && forge test --offline`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add contracts/ethereum
git commit -m "feat: add native eth bridge support"
```

### Task G3: Mint canonical bridged assets on the AegisLink chain

**Files:**
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
- Modify: `chain/aegislink/x/bridge/keeper/accounting.go`
- Modify: `chain/aegislink/x/bridge/keeper/invariants.go`
- Test: `chain/aegislink/x/bridge/keeper/keeper_test.go`

- [x] **Step 1: Write failing keeper tests**

Cover:
- native ETH deposit mints a bridged ETH denom
- ERC-20 deposit mints a token-specific bridged denom
- redeem burns the correct denom and amount
- accounting remains asset-specific

- [x] **Step 2: Run focused keeper tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper -run 'TestBridge'`
Expected: FAIL because the keeper does not yet model native ETH and ERC-20 as separate canonical asset classes.

- [x] **Step 3: Implement minimal mint or burn logic**

Ensure:
- one bridge denom per registered source asset
- burn path is exact and replay-safe
- invariants report supply by source asset

- [x] **Step 4: Re-run tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/bridge
git commit -m "feat: mint canonical bridged assets"
```

## Phase H: Real Cosmos Wallet Delivery

### Task H1: Put bridged balances into a real wallet-holding chain account model

Current status in this worktree: complete. Bridged deposits now credit bank-backed wallet balances for real Bech32 recipients, `query balances` exposes them, deposit acceptance rolls back cleanly if wallet crediting fails, and JSON or SDK-store reloads preserve balances with explicit legacy-migration safeguards for older runtimes.

**Files:**
- Create or modify: `chain/aegislink/x/bank/`
- Modify: `chain/aegislink/app/app.go`
- Modify: `chain/aegislink/app/service.go`
- Modify: `chain/aegislink/cmd/aegislinkd/main.go`
- Test: `tests/e2e/public_wallet_delivery_test.go`

- [x] **Step 1: Write failing wallet-delivery tests**

Cover:
- deposit claim credited to a real Bech32 wallet address
- wallet balance query shows bridged ETH
- wallet balance query shows bridged ERC-20
- balances survive restart or reload

- [x] **Step 2: Run focused tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicWalletDelivery'`
Expected: FAIL because current runtime status is not yet a real public wallet balance surface.

- [x] **Step 3: Implement minimal on-chain bank balance integration**

Do not build full staking or governance integration here. Just ensure bridged assets live in transfer-capable wallet balances.

- [x] **Step 4: Re-run tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicWalletDelivery'`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add chain/aegislink/app chain/aegislink/x/bank tests/e2e
git commit -m "feat: credit bridged assets to real wallet balances"
```

### Task H2: Stand up a public AegisLink testnet

Current status in this worktree: complete. The repo now has a reproducible single-validator public-testnet scaffold, operator and network config artifacts, a wallet-query-capable smoke test, and an operator runbook for the new bootstrap path.

**Files:**
- Create: `deploy/testnet/aegislink/`
- Create: `scripts/testnet/bootstrap_aegislink_testnet.sh`
- Modify: `README.md`
- Modify: `docs/project-positioning.md`
- Create: `docs/runbooks/public-bridge-ops.md`

- [x] **Step 1: Write the failing smoke test**

Cover:
- a node comes up from public-testnet config
- a wallet can query balances
- operator config loads required bridge settings

- [x] **Step 2: Run the smoke test**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicAegisLinkTestnet'`
Expected: FAIL because the public testnet bootstrap does not exist yet.

- [x] **Step 3: Implement minimal public testnet bootstrap**

Keep this small:
- single validator first
- reproducible config
- documented RPC and gRPC endpoints

- [x] **Step 4: Re-run smoke test**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicAegisLinkTestnet'`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add deploy/testnet scripts/testnet README.md docs/project-positioning.md docs/runbooks/public-bridge-ops.md
git commit -m "feat: bootstrap public aegislink testnet"
```

## Phase I: Sepolia-to-Cosmos Public Flow

### Task I1: Deploy and register public bridge assets

**Files:**
- Create: `scripts/testnet/deploy_sepolia_bridge.sh`
- Create: `scripts/testnet/register_bridge_assets.sh`
- Modify: `relayer/internal/config/config.go`
- Test: `tests/e2e/sepolia_bridge_smoke_test.go`

- [ ] **Step 1: Write failing deployment-config tests**

Cover:
- Sepolia bridge deployment outputs contract addresses
- relayer config accepts deployed verifier and gateway addresses
- asset-registration script writes both ETH and ERC-20 registry entries

- [ ] **Step 2: Run focused tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestSepoliaBridgeSmoke'`
Expected: FAIL because public deployment scripts are missing.

- [ ] **Step 3: Implement minimal deployment and registration scripts**

Make them idempotent enough for repeated public-devnet deployment.

- [ ] **Step 4: Re-run tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestSepoliaBridgeSmoke'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add scripts/testnet relayer/internal/config tests/e2e
git commit -m "feat: deploy public sepolia bridge assets"
```

### Task I2: Deliver public deposits into a real Cosmos wallet

**Files:**
- Modify: `relayer/internal/evm/rpc_source.go`
- Modify: `relayer/internal/pipeline/pipeline.go`
- Create: `relayer/cmd/public-bridge-relayer/main.go`
- Test: `tests/e2e/public_wallet_delivery_test.go`

- [ ] **Step 1: Write failing public-deposit tests**

Cover:
- Sepolia ETH deposit reaches a configured Cosmos wallet
- Sepolia ERC-20 deposit reaches a configured Cosmos wallet
- relayer records asset class correctly

- [ ] **Step 2: Run focused tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicWalletDelivery'`
Expected: FAIL because the live relayer path is not yet wired to public deployment state.

- [ ] **Step 3: Implement minimal public bridge relayer**

Keep:
- one source chain
- one destination chain
- no route swap logic
- just canonical asset delivery

- [ ] **Step 4: Re-run tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicWalletDelivery'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/evm relayer/internal/pipeline relayer/cmd/public-bridge-relayer tests/e2e
git commit -m "feat: deliver sepolia assets to cosmos wallets"
```

## Phase J: Reverse Redeem Back to Sepolia

### Task J1: Add public redeem flow for ETH and ERC-20

**Files:**
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
- Modify: `relayer/internal/evm/rpc_release.go`
- Modify: `relayer/internal/pipeline/pipeline.go`
- Test: `tests/e2e/public_redeem_test.go`

- [ ] **Step 1: Write failing redeem tests**

Cover:
- bridged ETH burns on Cosmos and releases ETH on Sepolia
- bridged ERC-20 burns on Cosmos and releases ERC-20 on Sepolia
- replayed redeem proof is rejected

- [ ] **Step 2: Run focused tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicRedeem'`
Expected: FAIL because public redeem wiring is not complete yet.

- [ ] **Step 3: Implement minimal redeem pipeline**

Ensure:
- burn happens before external release
- release asset class matches original custody class
- failed release does not silently lose redeem state

- [ ] **Step 4: Re-run tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestPublicRedeem'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/bridge relayer/internal/evm relayer/internal/pipeline tests/e2e
git commit -m "feat: redeem bridged assets back to sepolia"
```

### Task J2: Publish the real public demo path

**Files:**
- Modify: `README.md`
- Modify: `docs/demo-walkthrough.md`
- Modify: `docs/project-positioning.md`
- Modify: `docs/security-model.md`
- Create: `docs/runbooks/public-bridge-ops.md`

- [ ] **Step 1: Write the public demo and risk notes**

Document:
- how to make a Sepolia ETH deposit
- how to make a Sepolia ERC-20 deposit
- how to see the balance in a real Cosmos wallet
- how to redeem back to Sepolia
- what is still local or testnet-only

- [ ] **Step 2: Verify docs against the live commands**

Run:
- `make test-e2e`
- any new public-demo command added by the implementation

Expected: PASS and docs match observed behavior.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/demo-walkthrough.md docs/project-positioning.md docs/security-model.md docs/runbooks/public-bridge-ops.md
git commit -m "docs: publish public bridge demo path"
```

## Optional Phase K: Real Osmosis Wallet Delivery

Only do this if the requirement is specifically:
`Sepolia ETH or ERC-20 -> real Osmosis wallet`

### Task K1: Add real IBC from AegisLink to Osmosis testnet

**Files:**
- Create: `deploy/testnet/ibc/`
- Modify: `chain/aegislink/x/ibcrouter/keeper/`
- Create: `tests/e2e/osmosis_wallet_delivery_test.go`

- [ ] **Step 1: Write failing IBC wallet-delivery tests**

Cover:
- bridged asset held on AegisLink can be sent over real IBC
- Osmosis wallet receives the IBC denomination
- timeout and ack paths remain correct

- [ ] **Step 2: Run focused tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestOsmosisWalletDelivery'`
Expected: FAIL because the repo does not yet connect to real Osmosis testnet.

- [ ] **Step 3: Implement minimal real IBC transport**

Use Hermes first. Keep the asset bridge stable beneath it.

- [ ] **Step 4: Re-run tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestOsmosisWalletDelivery'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add deploy/testnet/ibc chain/aegislink/x/ibcrouter tests/e2e
git commit -m "feat: deliver bridged assets to osmosis wallets"
```

## Exit criteria

This plan is successful when:

- native Sepolia ETH can be deposited into the bridge and represented in a real Cosmos wallet
- Sepolia ERC-20 assets can be deposited into the bridge and represented in a real Cosmos wallet
- the bridged asset identity stays canonical instead of silently turning into a swap output
- users can redeem bridged ETH and bridged ERC-20 back to Sepolia
- operator docs explain the custody and trust model honestly
- the repo can demonstrate a public testnet flow rather than only a local harness flow

For the stronger optional finish line:

- a real Osmosis wallet can receive the bridged asset over real IBC

## Recommended execution order

Use this exact order:

1. Task G1: generic asset model
2. Task G2: native ETH support
3. Task G3: canonical bridged mint and burn logic
4. Task H1: real wallet-holding balance model
5. Task H2: public AegisLink testnet bootstrap
6. Task I1: Sepolia deployment and asset registration
7. Task I2: public wallet delivery
8. Task J1: redeem back to Sepolia
9. Task J2: public demo docs
10. Task K1: optional Osmosis wallet delivery

Do not start real Osmosis wallet delivery before the bridge can already deliver to a real AegisLink wallet and redeem back to Sepolia. Do not mix swaps into the core bridge milestone. Keep the bridge asset-preserving first.
