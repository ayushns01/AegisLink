# AegisLink Gap Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the highest-impact security, correctness, and realism gaps identified in the evaluation by adding cryptographic attestation verification, governance authorization, real rate limiting, invariant guardrails, scalable state layout, safer relayer operation, and a clearer path to real chain or IBC integration.

**Architecture:** Fix the gaps in dependency order. First harden trust and policy enforcement inside the current runtime so the bridge logic is no longer self-asserted. Then replace the current JSON-blob state persistence with prefix-keyed storage and add stronger invariants, race coverage, and daemon behavior. Only after the local trust and state model are sound should the project move into full `BaseApp` or `ABCI` work and real IBC-Go or Hermes networking.

**Tech Stack:** Go, Cosmos SDK store APIs, go-ethereum crypto, Foundry, Prometheus, Docker Compose, current AegisLink runtime and relayers.

---

## Scope notes

This plan intentionally fixes the **real current gaps** and skips a few evaluation items that are already partially addressed:

- the repo already has some invariant-style tests, but not fuzz or formal-style invariant coverage
- the repo is not in-memory anymore, but it still is not a real consensus-driven node
- the destination path is dual-runtime today, but it is still not real IBC-Go or Hermes

The phases below are ordered by impact:

1. trust-model fixes
2. policy and accounting fixes
3. state-layout and concurrency fixes
4. relayer runtime fixes
5. real node and IBC realism
6. Solidity hardening follow-through

## File structure and responsibility map

These are the main files this remediation plan expects to touch.

- Modify: `chain/aegislink/x/bridge/types/attestation.go`
  Extend the attestation payload from signer names to signer proofs or signatures.
- Create: `chain/aegislink/x/bridge/types/proof.go`
  Signature-proof types and helpers for canonical attestation verification.
- Create: `chain/aegislink/x/bridge/keeper/verify_signature.go`
  Go-side cryptographic attestation verification using signer-set metadata.
- Modify: `chain/aegislink/x/bridge/keeper/verify_attestation.go`
  Replace string-set matching with real signature verification.
- Modify: `chain/aegislink/x/bridge/keeper/signer_set.go`
  Store signer public identity in a form usable for verification.
- Modify: `proto/aegislink/bridge/v1/bridge.proto`
  Add signature-proof fields to the wire format.
- Modify: `relayer/internal/attestations/collector.go`
  Produce real signed attestations instead of only signer-name bundles.
- Modify: `relayer/internal/config/config.go`
  Add signer-key configuration for local attestation signing.
- Modify: `chain/aegislink/x/governance/keeper/keeper.go`
  Add auth or guardian enforcement for proposal application.
- Create: `chain/aegislink/x/governance/keeper/auth.go`
  Shared auth helpers for governance proposal application.
- Modify: `chain/aegislink/app/config.go`
  Add guardian or governance authority config.
- Modify: `chain/aegislink/x/limits/keeper/keeper.go`
  Replace ceiling-only checks with persisted time-window or token-bucket tracking.
- Create: `chain/aegislink/x/limits/keeper/usage.go`
  Sliding-window or token-bucket usage tracking and persistence logic.
- Create: `chain/aegislink/x/bridge/keeper/invariants.go`
  Runtime accounting invariant checks and helper functions.
- Modify: `chain/aegislink/internal/sdkstore/jsonstore.go`
  Stop storing whole-module JSON blobs behind a single key.
- Create: `chain/aegislink/internal/sdkstore/prefixstore.go`
  Prefix-keyed helpers for per-record persistence.
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
  Move claim, signer-set, supply, and withdrawal persistence to prefix-keyed entries.
- Modify: `chain/aegislink/x/registry/keeper/keeper.go`
  Move asset storage to prefix-keyed entries.
- Modify: `chain/aegislink/x/limits/keeper/keeper.go`
  Move limits and usage history to prefix-keyed entries.
- Modify: `chain/aegislink/x/pauser/keeper/keeper.go`
  Move pause state to prefix-keyed entries.
- Modify: `chain/aegislink/x/ibcrouter/keeper/keeper.go`
  Move route, route-profile, and transfer state to prefix-keyed entries.
- Modify: `chain/aegislink/x/governance/keeper/keeper.go`
  Move applied proposals to prefix-keyed entries.
- Modify: `relayer/internal/pipeline/pipeline.go`
  Add long-running loop support and health or lag summary hooks.
- Create: `relayer/internal/pipeline/daemon.go`
  Persistent daemon runner, backoff, poll interval, graceful shutdown.
- Modify: `relayer/cmd/bridge-relayer/main.go`
  Add daemon mode, health endpoint, and lag metrics output.
- Modify: `relayer/cmd/route-relayer/main.go`
  Add daemon mode, health endpoint, and lag metrics output.
- Create: `tests/e2e/race_smoke_test.go`
  Simple concurrent submission smoke coverage.
- Create: `tests/e2e/governance_auth_test.go`
  Auth and rejection coverage for governance changes.
- Create: `tests/e2e/attestation_crypto_test.go`
  Real signed attestation flow against the Go bridge keeper.
- Create: `tests/e2e/real_abci_chain_test.go`
  Real node lifecycle once `BaseApp` or `ABCI` work starts.
- Create: `tests/e2e/real_hermes_ibc_test.go`
  Real IBC route flow once Hermes or IBC-Go wiring lands.
- Modify: `contracts/ethereum/BridgeGateway.sol`
  EIP-712 digest path, low-s enforcement through verifier boundary if needed, and reentrancy guard.
- Modify: `contracts/ethereum/ThresholdBridgeVerifier.sol`
  Typed-data digest path and stricter signature checks.
- Create: `contracts/ethereum/test/BridgeGateway.invariant.t.sol`
  Foundry invariant tests for gateway balance or release safety.
- Modify: `docs/project-positioning.md`
  Keep “what is real vs not” honest as each gap closes.
- Modify: `docs/security-model.md`
  Update trust model once the Go side stops trusting signer-name lists.
- Modify: `docs/observability.md`
  Document long-running relayer health and lag metrics.

## Phase A: Trust and Policy Hardening

Status as of April 8, 2026:

- Task A1 is complete for the current repo scope: Go-side bridge verification now requires cryptographic signer proofs, the relayer collector signs local harness attestations with configured keys, and focused keeper plus e2e coverage exist.
- Task A2 is complete for the current repo scope: governance policy changes now require configured authorities, proposal records persist `applied_by`, and focused keeper plus e2e coverage exist.

### Task A1: Add cryptographic attestation verification on the Go side

**Files:**
- Modify: `chain/aegislink/x/bridge/types/attestation.go`
- Create: `chain/aegislink/x/bridge/types/proof.go`
- Modify: `chain/aegislink/x/bridge/keeper/verify_attestation.go`
- Create: `chain/aegislink/x/bridge/keeper/verify_signature.go`
- Modify: `chain/aegislink/x/bridge/keeper/signer_set.go`
- Modify: `proto/aegislink/bridge/v1/bridge.proto`
- Modify: `relayer/internal/attestations/collector.go`
- Modify: `relayer/internal/config/config.go`
- Test: `chain/aegislink/x/bridge/keeper/verify_attestation_test.go`
- Test: `tests/e2e/attestation_crypto_test.go`

- [x] **Step 1: Write failing keeper tests**

Cover:
- attestation with signer names but no signatures is rejected
- signature over the wrong payload hash is rejected
- signer not in active signer set is rejected
- threshold is met only when valid signatures from the active set are present

- [x] **Step 2: Run the focused keeper tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper -run 'TestVerifyAttestation'`
Expected: FAIL because the keeper still only counts signer names.

- [x] **Step 3: Extend attestation types and proto**

Add:
- canonical proof entries that carry signer identity plus signature bytes
- signer-set representation that can verify Ethereum-style secp256k1 identities

- [x] **Step 4: Implement Go-side signature verification**

Use go-ethereum crypto helpers so the bridge keeper verifies the attestation payload hash against actual signatures instead of trusting self-reported signer strings.

- [x] **Step 5: Update the relayer collector**

Make the collector produce signatures using configured local signer keys for the local harness.

- [x] **Step 6: Re-run focused tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper -run 'TestVerifyAttestation'`
Expected: PASS

- [x] **Step 7: Add e2e proof**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestAttestationCrypto'`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add proto chain/aegislink/x/bridge relayer/internal/attestations relayer/internal/config tests/e2e
git commit -m "feat: verify bridge attestations cryptographically"
```

### Task A2: Add governance authorization

**Files:**
- Modify: `chain/aegislink/x/governance/keeper/keeper.go`
- Create: `chain/aegislink/x/governance/keeper/auth.go`
- Modify: `chain/aegislink/app/config.go`
- Modify: `chain/aegislink/app/app.go`
- Modify: `chain/aegislink/cmd/aegislinkd/main.go`
- Test: `chain/aegislink/x/governance/keeper/keeper_test.go`
- Test: `tests/e2e/governance_auth_test.go`

- [x] **Step 1: Write failing governance auth tests**

Cover:
- unauthorized proposal application is rejected
- authorized guardian proposal is accepted
- governance state records who applied the change

- [x] **Step 2: Run the focused governance tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/governance/...`
Expected: FAIL because proposal application is currently permissionless.

- [x] **Step 3: Implement the smallest useful auth model**

Use a configured guardian or authority set first. Do not build voting yet unless needed.

- [x] **Step 4: Wire authority config into runtime and CLI**

Add config-driven guardian identities and make the CLI require them for proposal application.

- [x] **Step 5: Re-run tests**

Run:
- `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/governance/...`
- `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestGovernanceAuth'`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add chain/aegislink/x/governance chain/aegislink/app chain/aegislink/cmd tests/e2e
git commit -m "feat: authorize governance policy changes"
```

## Phase B: Accounting and Rate-Limit Correctness

### Task B1: Implement real time-windowed rate limiting

**Files:**
- Modify: `chain/aegislink/x/limits/keeper/keeper.go`
- Create: `chain/aegislink/x/limits/keeper/usage.go`
- Modify: `chain/aegislink/x/limits/types/limits.go`
- Test: `chain/aegislink/x/limits/keeper/keeper_test.go`
- Test: `tests/e2e/recovery_drill_test.go`

- [ ] **Step 1: Write failing limit-window tests**

Cover:
- multiple transfers within a window accumulate
- transfer after the window expires is allowed again
- limits are enforced separately per asset

- [ ] **Step 2: Run the focused limits tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/limits/keeper -run 'TestCheckTransfer'`
Expected: FAIL because the keeper only compares amount to `MaxAmount`.

- [ ] **Step 3: Implement persisted usage tracking**

Choose one:
- sliding-window history
- token bucket

Prefer token bucket if you want simpler O(1) writes.

- [ ] **Step 4: Re-run tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/limits/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/limits
git commit -m "feat: add windowed bridge rate limiting"
```

### Task B2: Add runtime invariant checks and bridge circuit breakers

**Files:**
- Create: `chain/aegislink/x/bridge/keeper/invariants.go`
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
- Modify: `chain/aegislink/x/bridge/keeper/accounting.go`
- Modify: `chain/aegislink/app/app.go`
- Test: `chain/aegislink/x/bridge/keeper/keeper_test.go`
- Test: `tests/e2e/recovery_drill_test.go`

- [ ] **Step 1: Write failing invariant tests**

Cover:
- minted minus burned equals tracked supply
- invariant failure pauses or rejects further flows
- `burnRepresentation` rejects underflow even if called incorrectly

- [ ] **Step 2: Run focused bridge invariant tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper -run 'TestBridgeSupply|TestInvariant'`
Expected: FAIL because the keeper does not enforce invariants centrally.

- [ ] **Step 3: Implement invariant helpers**

Add:
- `CheckAccountingInvariant()`
- safe burn helper that cannot silently go negative
- optional auto-pause or circuit-breaker behavior on invariant failure

- [ ] **Step 4: Re-run tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/x/bridge chain/aegislink/app tests/e2e
git commit -m "feat: add bridge accounting invariants"
```

## Phase C: State Layout and Concurrency Safety

### Task C1: Replace whole-module JSON blobs with prefix-keyed storage

**Files:**
- Modify: `chain/aegislink/internal/sdkstore/jsonstore.go`
- Create: `chain/aegislink/internal/sdkstore/prefixstore.go`
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
- Modify: `chain/aegislink/x/registry/keeper/keeper.go`
- Modify: `chain/aegislink/x/limits/keeper/keeper.go`
- Modify: `chain/aegislink/x/pauser/keeper/keeper.go`
- Modify: `chain/aegislink/x/ibcrouter/keeper/keeper.go`
- Modify: `chain/aegislink/x/governance/keeper/keeper.go`
- Test: `chain/aegislink/x/...`

- [ ] **Step 1: Write failing persistence tests for per-record storage**

Cover:
- claims can be loaded individually
- routes and transfers can be iterated without loading all module state
- restarting the runtime preserves per-record state

- [ ] **Step 2: Run focused persistence tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/... -run 'TestSDKKeeper|TestStoreKeeper'`
Expected: FAIL because state is still persisted as whole-module JSON.

- [ ] **Step 3: Add prefix-keyed helpers**

Use one key per:
- claim
- signer set
- limit
- pause flag
- route
- route profile
- transfer
- governance proposal record

- [ ] **Step 4: Migrate keepers incrementally**

Keep public keeper APIs stable while moving storage internals.

- [ ] **Step 5: Re-run tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add chain/aegislink/internal/sdkstore chain/aegislink/x
git commit -m "refactor: move chain state to prefix-keyed stores"
```

### Task C2: Add concurrency safety and race coverage

**Files:**
- Modify: `chain/aegislink/x/bridge/keeper/keeper.go`
- Modify: `chain/aegislink/x/registry/keeper/keeper.go`
- Modify: `chain/aegislink/x/limits/keeper/keeper.go`
- Modify: `chain/aegislink/x/pauser/keeper/keeper.go`
- Modify: `chain/aegislink/x/ibcrouter/keeper/keeper.go`
- Modify: `chain/aegislink/x/governance/keeper/keeper.go`
- Create: `tests/e2e/race_smoke_test.go`

- [ ] **Step 1: Write failing concurrent access tests**

Cover:
- concurrent deposit submissions
- concurrent route updates and reads
- concurrent governance application and query

- [ ] **Step 2: Run race checks**

Run: `GOCACHE=/tmp/aegislink-gocache go test -race ./chain/aegislink/...`
Expected: FAIL or report races until synchronization is added.

- [ ] **Step 3: Add synchronization or single-threaded access boundary**

Choose one:
- keeper-level mutexes
- app-level serialized execution boundary

Prefer app-level serialization if the long-term goal is real tx delivery through one runtime.

- [ ] **Step 4: Re-run race checks**

Run: `GOCACHE=/tmp/aegislink-gocache go test -race ./chain/aegislink/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink tests/e2e
git commit -m "fix: add runtime concurrency safety"
```

## Phase D: Relayer Runtime Hardening

### Task D1: Convert relayers from `RunOnce` workers into long-running daemons

**Files:**
- Modify: `relayer/internal/pipeline/pipeline.go`
- Create: `relayer/internal/pipeline/daemon.go`
- Modify: `relayer/internal/route/relay.go`
- Modify: `relayer/cmd/bridge-relayer/main.go`
- Modify: `relayer/cmd/route-relayer/main.go`
- Modify: `docs/observability.md`
- Test: `relayer/internal/pipeline/pipeline_test.go`
- Test: `relayer/internal/route/relay_test.go`

- [ ] **Step 1: Write failing daemon-loop tests**

Cover:
- repeated polling with no duplicate reprocessing
- backoff after temporary failure
- graceful shutdown on context cancel

- [ ] **Step 2: Run focused relayer tests**

Run:
- `GOCACHE=/tmp/aegislink-gocache go test ./relayer/internal/pipeline -run 'TestCoordinator'`
- `GOCACHE=/tmp/aegislink-gocache go test ./relayer/internal/route -run 'TestRelayer'`

Expected: FAIL because the relayers are currently one-shot workers.

- [ ] **Step 3: Implement daemon mode**

Add:
- `--loop`
- poll interval
- health summary
- graceful shutdown

- [ ] **Step 4: Re-run tests**

Run: `GOCACHE=/tmp/aegislink-gocache go test ./relayer/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add relayer docs/observability.md
git commit -m "feat: run bridge and route relayers as daemons"
```

### Task D2: Add fuzz and invariant coverage

**Files:**
- Create: `contracts/ethereum/test/BridgeGateway.invariant.t.sol`
- Modify: `contracts/ethereum/test/BridgeGateway.t.sol`
- Create: `chain/aegislink/x/bridge/keeper/fuzz_test.go`
- Modify: `Makefile`

- [ ] **Step 1: Write failing invariant and fuzz scaffolds**

Cover:
- gateway balance never releases more than attested deposits allow
- bridge supply never goes negative
- route refund state machine never skips directly from pending to refunded

- [ ] **Step 2: Run fuzz and invariant commands**

Run:
- `cd contracts/ethereum && forge test --match-path test/BridgeGateway.invariant.t.sol`
- `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/x/bridge/keeper -run 'Fuzz'`

Expected: FAIL until the scaffolds and invariants are added.

- [ ] **Step 3: Implement the smallest useful invariant set**

- [ ] **Step 4: Re-run**

Run:
- `cd contracts/ethereum && forge test`
- `GOCACHE=/tmp/aegislink-gocache go test ./chain/aegislink/...`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add contracts/ethereum chain/aegislink Makefile
git commit -m "test: add bridge fuzz and invariant coverage"
```

## Phase E: Real Node and IBC Realism

### Task E1: Replace manual-height runtime with real `BaseApp` or `ABCI` lifecycle

**Files:**
- Create: `chain/aegislink/cmd/aegislinkd/cmd/root.go`
- Create: `chain/aegislink/cmd/aegislinkd/cmd/start.go`
- Create: `chain/aegislink/cmd/aegislinkd/cmd/init.go`
- Modify: `chain/aegislink/app/app.go`
- Modify: `chain/aegislink/app/store_runtime.go`
- Create: `tests/e2e/real_abci_chain_test.go`

- [ ] **Step 1: Write failing runtime-node tests**

Cover:
- block height advances from the runtime, not manual setters
- tx delivery goes through a real application boundary
- startup and shutdown match a real daemon lifecycle

- [ ] **Step 2: Run focused runtime tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestRealABCIChain'`
Expected: FAIL because the runtime is still not consensus-driven.

- [ ] **Step 3: Add real node command layout and block-driven height updates**

- [ ] **Step 4: Re-run tests**

Expected: PASS once the node lifecycle is real enough for the targeted scope.

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink tests/e2e
git commit -m "feat: add real aegislink node lifecycle"
```

### Task E2: Replace dual-runtime route bridge with real IBC-Go or Hermes-backed networking

**Files:**
- Modify: `chain/aegislink/x/ibcrouter/keeper/`
- Modify: `localnet/compose/real-chains.yml`
- Modify: `scripts/localnet/bootstrap_ibc.sh`
- Create: `tests/e2e/real_hermes_ibc_test.go`
- Modify: `docs/demo-walkthrough.md`

- [ ] **Step 1: Write failing real-IBC tests**

Cover:
- channel handshake
- packet relay
- timeout
- acknowledgement completion

- [ ] **Step 2: Run focused IBC tests**

Run: `cd tests/e2e && GOCACHE=/tmp/aegislink-gocache go test ./... -run 'TestRealHermesIBC'`
Expected: FAIL because the route path still shells between local runtimes.

- [ ] **Step 3: Implement minimal real IBC transport**

Use Hermes first. Keep the route action layer stable above transport.

- [ ] **Step 4: Re-run tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink localnet scripts tests/e2e docs/demo-walkthrough.md
git commit -m "feat: route assets over real local IBC"
```

## Phase F: Solidity Security Follow-through

### Task F1: Add typed-data signing, stricter signature checks, and release hardening

**Files:**
- Modify: `contracts/ethereum/BridgeGateway.sol`
- Modify: `contracts/ethereum/BridgeVerifier.sol`
- Modify: `contracts/ethereum/ThresholdBridgeVerifier.sol`
- Modify: `contracts/ethereum/test/BridgeGateway.t.sol`
- Modify: `contracts/ethereum/test/ThresholdBridgeVerifier.t.sol`

- [ ] **Step 1: Write failing Solidity hardening tests**

Cover:
- low-s signatures only
- EIP-712 typed digest compatibility
- reentrancy attempt on release path is blocked

- [ ] **Step 2: Run focused Foundry tests**

Run: `cd contracts/ethereum && forge test --match-test 'test.*(Typed|LowS|Reentrant)'`
Expected: FAIL because those checks are not fully enforced today.

- [ ] **Step 3: Implement minimal contract hardening**

- [ ] **Step 4: Re-run Foundry tests**

Run: `cd contracts/ethereum && forge test --offline`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add contracts/ethereum
git commit -m "feat: harden ethereum verifier and release flow"
```

### Task F2: Decide whether upgradeability belongs in v2

**Files:**
- Modify: `docs/security-model.md`
- Modify: `docs/project-positioning.md`
- Optional modify: `contracts/ethereum/` if upgradeability is actually adopted

- [ ] **Step 1: Write the decision note**

Document:
- why immutable verifier binding is acceptable or not for v1
- what a proxy or upgrade path would mean for trust
- whether the project should keep immutable contracts for clarity

- [ ] **Step 2: If and only if the project opts in, implement proxy tests**

Run: `cd contracts/ethereum && forge test`

- [ ] **Step 3: Commit**

```bash
git add docs/security-model.md docs/project-positioning.md contracts/ethereum
git commit -m "docs: clarify ethereum upgrade strategy"
```

## Exit criteria

This remediation roadmap is successful when:

- the Go-side bridge no longer trusts signer-name lists and verifies real signatures
- governance changes require explicit authority
- rate limiting is windowed or bucket-based rather than a simple ceiling
- accounting invariants are enforced and surfaced operationally
- chain state is persisted as prefix-keyed records instead of whole-module JSON blobs
- race tests pass for the runtime boundary
- relayers can run continuously and expose health or lag summaries
- the next realism step to a full node and real IBC is explicit and test-driven

## Recommended execution order

Use this exact order:

1. Task A1: cryptographic attestation verification
2. Task A2: governance authorization
3. Task B1: time-windowed rate limiting
4. Task B2: invariant enforcement and safe burn path
5. Task C1: prefix-keyed state layout
6. Task C2: concurrency safety and race coverage
7. Task D1: relayer daemonization
8. Task D2: fuzz and invariant coverage
9. Task E1: real node lifecycle
10. Task E2: real IBC-Go or Hermes path
11. Task F1: Solidity signature and release hardening
12. Task F2: upgradeability decision

Do not start the real IBC phase before the trust model and state model are fixed. Do not add proxy complexity before deciding whether v1 clarity is more important than upgradeability.
