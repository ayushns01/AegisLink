# Transfer Pipeline Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the main stuck-transfer failure modes in the public bridge by hardening relayer catch-up, per-event error handling, daemon retry behavior, and stale-intent handling.

**Architecture:** This slice stays inside the relayer and networked delivery-intent path. The relayer should become resilient to lag and malformed single events, while autodelivery gains enough persisted timing/state to surface and act on intents that remain in “waiting” indefinitely.

**Tech Stack:** Go, relayer runtime, networked demo-node HTTP APIs, JSON-backed delivery-intent persistence, Vitest/Go test suites

---

## File Map

- `relayer/internal/evm/rpc_source.go`
  - EVM deposit log scanning window and chunking behavior.
- `relayer/internal/evm/rpc_source_test.go`
  - Chunking behavior tests for large log ranges.
- `relayer/internal/pipeline/pipeline.go`
  - Deposit/withdrawal coordination and per-event submission loop behavior.
- `relayer/internal/pipeline/pipeline_test.go`
  - Coordinator behavior, duplicate suppression, retry handling, replay persistence.
- `relayer/internal/pipeline/daemon.go`
  - Main poll loop and retry/backoff policy.
- `relayer/internal/pipeline/daemon_test.go`
  - New daemon-specific tests for retrying non-temporary failures and backoff behavior.
- `relayer/internal/autodelivery/coordinator.go`
  - Intent waiting/claim-ready/flush behavior.
- `relayer/internal/autodelivery/coordinator_test.go`
  - Intent waiting, initiation, flush, and stale-intent handling tests.
- `relayer/internal/autodelivery/runtime.go`
  - Intent-source shape exposed to the coordinator.
- `relayer/internal/autodelivery/runtime_test.go`
  - Runtime/networked intent mapping tests.
- `chain/aegislink/networked/delivery_intent.go`
  - Persisted delivery-intent shape; add timestamps/status metadata.
- `chain/aegislink/networked/delivery_intent_test.go`
  - JSON persistence/update behavior for expanded intent records.
- `relayer/cmd/public-bridge-relayer/main.go`
  - Config plumbing and stale-intent timeout configuration.
- `relayer/cmd/public-bridge-relayer/main_test.go`
  - CLI/env parsing tests for new relayer options.

## Task 1: Widen deposit scanning so catch-up does not stall behind Sepolia

**Files:**
- Modify: `relayer/internal/evm/rpc_source.go`
- Test: `relayer/internal/evm/rpc_source_test.go`

- [ ] **Step 1: Write the failing chunking test expectation**

Update `TestRPCLogSourceChunksLargeLogRanges` so the same `100 -> 125` range is fetched in a single chunk when the widened range is in effect.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./relayer/internal/evm -run TestRPCLogSourceChunksLargeLogRanges -count=1`
Expected: FAIL because the current implementation still emits three `eth_getLogs` calls using the 10-block cap.

- [ ] **Step 3: Implement the larger scan window**

Change `maxDepositLogRange` in `relayer/internal/evm/rpc_source.go` from `10` to a production-safe value such as `2000`.

- [ ] **Step 4: Re-run the focused test**

Run: `go test ./relayer/internal/evm -run TestRPCLogSourceChunksLargeLogRanges -count=1`
Expected: PASS with one widened log-range call.

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/evm/rpc_source.go relayer/internal/evm/rpc_source_test.go
git commit -m "fix: widen deposit log scan range"
```

## Task 2: Stop single deposit/withdrawal errors from aborting the whole coordinator run

**Files:**
- Modify: `relayer/internal/pipeline/pipeline.go`
- Test: `relayer/internal/pipeline/pipeline_test.go`

- [ ] **Step 1: Write failing coordinator tests for per-event isolation**

Add tests that prove:
- one invalid deposit claim is skipped while later valid deposits still submit
- one failed withdrawal release is skipped while later valid withdrawals still release
- summary counters record skipped/failed events without aborting the whole run

- [ ] **Step 2: Run the focused pipeline tests to verify failure**

Run: `go test ./relayer/internal/pipeline -run 'TestCoordinator.*(Skip|Continue)' -count=1`
Expected: FAIL because `runDeposits`/`runWithdrawals` currently return immediately on validation/submission/release errors.

- [ ] **Step 3: Implement event-level isolation**

In `relayer/internal/pipeline/pipeline.go`:
- keep watcher failure as a run-level error
- treat per-event validation/attestation/submission/release failures as event-level failures
- log/accumulate the failure and `continue`
- add explicit summary counters for skipped/failed deposit and withdrawal events if needed

- [ ] **Step 4: Re-run the focused pipeline tests**

Run: `go test ./relayer/internal/pipeline -run 'TestCoordinator.*(Skip|Continue)' -count=1`
Expected: PASS with successful later events still processed.

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/pipeline/pipeline.go relayer/internal/pipeline/pipeline_test.go
git commit -m "fix: isolate relayer event failures"
```

## Task 3: Make daemon retries resilient instead of exiting on first non-temporary error

**Files:**
- Modify: `relayer/internal/pipeline/daemon.go`
- Create: `relayer/internal/pipeline/daemon_test.go`
- Optionally modify: `relayer/cmd/public-bridge-relayer/main.go`

- [ ] **Step 1: Write failing daemon tests**

Create `relayer/internal/pipeline/daemon_test.go` with tests that prove:
- a non-temporary error is retried instead of immediately returning
- repeated failures back off and only exit after a configured cap
- a later successful iteration resets consecutive failure tracking

- [ ] **Step 2: Run the daemon tests to verify failure**

Run: `go test ./relayer/internal/pipeline -run TestDaemon -count=1`
Expected: FAIL because `Run` currently returns immediately on any non-temporary error.

- [ ] **Step 3: Implement bounded retry semantics**

In `relayer/internal/pipeline/daemon.go`:
- track consecutive failures for all errors, not only temporary ones
- back off on both temporary and non-temporary failures
- add a config cap such as `MaxFatalRetries` before returning the error
- reset failure count after a successful iteration

If `MaxFatalRetries` needs config plumbing, add it in `relayer/cmd/public-bridge-relayer/main.go` and its tests.

- [ ] **Step 4: Re-run daemon tests**

Run: `go test ./relayer/internal/pipeline -run TestDaemon -count=1`
Expected: PASS with non-temporary failures retried and later success accepted.

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/pipeline/daemon.go relayer/internal/pipeline/daemon_test.go relayer/cmd/public-bridge-relayer/main.go relayer/cmd/public-bridge-relayer/main_test.go
git commit -m "fix: retry relayer daemon failures before exit"
```

## Task 4: Add stale-intent detection so “waiting forever” is not silent

**Files:**
- Modify: `chain/aegislink/networked/delivery_intent.go`
- Modify: `relayer/internal/autodelivery/runtime.go`
- Modify: `relayer/internal/autodelivery/coordinator.go`
- Test: `chain/aegislink/networked/delivery_intent_test.go`
- Test: `relayer/internal/autodelivery/runtime_test.go`
- Test: `relayer/internal/autodelivery/coordinator_test.go`
- Optionally modify: `relayer/cmd/public-bridge-relayer/main.go`, `relayer/cmd/public-bridge-relayer/main_test.go`

- [ ] **Step 1: Write failing tests for intent timing/state**

Add tests that prove:
- delivery intents persist `createdAt`/`updatedAt` metadata
- runtime intent mapping preserves those timestamps into the coordinator
- coordinator marks an intent as stale/failed after the waiting timeout while leaving fresh waiting intents alone

- [ ] **Step 2: Run the focused tests to verify failure**

Run:
- `go test ./chain/aegislink/networked -run TestDeliveryIntent -count=1`
- `go test ./relayer/internal/autodelivery -run TestCoordinator -count=1`
Expected: FAIL because intents do not currently persist timing metadata and the coordinator has no timeout path.

- [ ] **Step 3: Implement intent timing metadata**

In `chain/aegislink/networked/delivery_intent.go`:
- add `CreatedAt` and `UpdatedAt` fields
- preserve `CreatedAt` on updates by source tx hash
- update `UpdatedAt` on every write

In `relayer/internal/autodelivery/runtime.go` and `coordinator.go`:
- propagate timestamps into the coordinator
- add a configurable waiting timeout
- when status stays in `""`, `deposit_submitted`, or `sepolia_confirming` beyond timeout, count and surface it as stale/failed instead of silently waiting forever

- [ ] **Step 4: Re-run focused tests**

Run:
- `go test ./chain/aegislink/networked -run TestDeliveryIntent -count=1`
- `go test ./relayer/internal/autodelivery -run TestCoordinator -count=1`
Expected: PASS with fresh waits preserved and stale intents escalated.

- [ ] **Step 5: Commit**

```bash
git add chain/aegislink/networked/delivery_intent.go chain/aegislink/networked/delivery_intent_test.go relayer/internal/autodelivery/runtime.go relayer/internal/autodelivery/runtime_test.go relayer/internal/autodelivery/coordinator.go relayer/internal/autodelivery/coordinator_test.go relayer/cmd/public-bridge-relayer/main.go relayer/cmd/public-bridge-relayer/main_test.go
git commit -m "fix: detect stale bridge delivery intents"
```

## Task 5: Run the relayer regression suite for the whole reliability slice

**Files:**
- Test only: `relayer/internal/evm/rpc_source_test.go`
- Test only: `relayer/internal/pipeline/pipeline_test.go`
- Test only: `relayer/internal/pipeline/daemon_test.go`
- Test only: `relayer/internal/autodelivery/coordinator_test.go`
- Test only: `relayer/internal/autodelivery/runtime_test.go`
- Test only: `chain/aegislink/networked/delivery_intent_test.go`
- Test only: `relayer/cmd/public-bridge-relayer/main_test.go`

- [ ] **Step 1: Run focused backend tests**

```bash
go test ./relayer/internal/evm ./relayer/internal/pipeline ./relayer/internal/autodelivery ./chain/aegislink/networked ./relayer/cmd/public-bridge-relayer -count=1
```

Expected: PASS for the touched relayer/networked packages.

- [ ] **Step 2: Run formatting and diff sanity**

```bash
gofmt -w relayer/internal/evm/rpc_source.go relayer/internal/evm/rpc_source_test.go relayer/internal/pipeline/pipeline.go relayer/internal/pipeline/pipeline_test.go relayer/internal/pipeline/daemon.go relayer/internal/pipeline/daemon_test.go relayer/internal/autodelivery/coordinator.go relayer/internal/autodelivery/coordinator_test.go relayer/internal/autodelivery/runtime.go relayer/internal/autodelivery/runtime_test.go chain/aegislink/networked/delivery_intent.go chain/aegislink/networked/delivery_intent_test.go relayer/cmd/public-bridge-relayer/main.go relayer/cmd/public-bridge-relayer/main_test.go
git diff --check
```

- [ ] **Step 3: Commit the full slice**

```bash
git add relayer/internal/evm/rpc_source.go relayer/internal/evm/rpc_source_test.go relayer/internal/pipeline/pipeline.go relayer/internal/pipeline/pipeline_test.go relayer/internal/pipeline/daemon.go relayer/internal/pipeline/daemon_test.go relayer/internal/autodelivery/coordinator.go relayer/internal/autodelivery/coordinator_test.go relayer/internal/autodelivery/runtime.go relayer/internal/autodelivery/runtime_test.go chain/aegislink/networked/delivery_intent.go chain/aegislink/networked/delivery_intent_test.go relayer/cmd/public-bridge-relayer/main.go relayer/cmd/public-bridge-relayer/main_test.go
git commit -m "fix: harden transfer pipeline against stuck relayer flows"
```

## Execution Notes

- This plan should execute in a clean worktree, not the current dirty `about-ux` workspace.
- Do **not** mix frontend transfer UI work into this slice.
- Keep the first implementation slice focused on issues 1–4 only. Issues 5–9 are follow-up performance/UX work after the relayer stops stalling.
