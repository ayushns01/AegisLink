# AegisLink Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working AegisLink foundation: the repo skeleton, shared bridge message schemas, and the Cosmos-side safety core for registry, pause controls, rate limits, and replay protection.

**Architecture:** This starter plan intentionally focuses on the AegisLink chain before Ethereum contracts or the full relayer. The reason is simple: the chain is where AegisLink records state, enforces policy, and prevents replay. If that boundary is not clear first, everything built later becomes harder to reason about.

**Tech Stack:** Go, Cosmos SDK v0.53.x, CometBFT, Protobuf, buf, Make, Solidity/Foundry placeholders for later tasks.

---

## Why This Is The Right Starting Point

Do not start with the Ethereum contract or the relayer.

Start with the chain foundation because:

- the chain is the accounting boundary
- replay protection lives there
- asset registration lives there
- rate limits and pause controls live there
- later Ethereum and relayer work become much easier once the destination state machine exists

In short:

- `first` build the place that decides whether a cross-chain claim is valid
- `then` build the components that feed claims into it

## Expected Repo Structure For This First Sprint

This plan only needs the files required for the first milestone:

- `go.work`
- `Makefile`
- `buf.yaml`
- `buf.gen.yaml`
- `.gitignore`
- `chain/aegislink/go.mod`
- `chain/aegislink/api/`
- `chain/aegislink/cmd/aegislinkd/main.go`
- `chain/aegislink/app/app.go`
- `chain/aegislink/app/config.go`
- `chain/aegislink/x/bridge/module.go`
- `chain/aegislink/x/bridge/types/keys.go`
- `chain/aegislink/x/bridge/types/errors.go`
- `chain/aegislink/x/bridge/keeper/keeper.go`
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
- `proto/aegislink/bridge/v1/bridge.proto`
- `proto/aegislink/registry/v1/registry.proto`
- `proto/aegislink/limits/v1/limits.proto`

## What You Are Learning In This Sprint

- how a Cosmos-SDK app is wired together
- why bridge systems need replay protection
- why asset registration must be explicit
- why pause and rate-limit logic belong in the chain, not only in the relayer
- how cross-chain message schemas are defined before cross-chain logic is implemented

## Tiny Glossary

- `claim ID`: the unique fingerprint for one bridge event
- `attestation metadata`: signer and proof information attached to a claim
- `codec wiring`: the setup that lets the chain encode and decode its own types
- `route flags`: policy fields that say whether an asset may be forwarded later

## Task 1: Bootstrap The Workspace

**Files:**
- Create: `go.work`
- Create: `Makefile`
- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create: `.gitignore`
- Create: `chain/aegislink/go.mod`

**What is happening:** You are creating the repo skeleton so every later file has a predictable home.

**Why this matters:** A bridge project has multiple moving parts. If the repo shape is messy at the start, the complexity multiplies later.

- [ ] **Step 1: Create the top-level folders**

Create: `chain/aegislink`, `proto/aegislink`, `contracts/ethereum`, `relayer`, `tests/e2e`

- [ ] **Step 2: Add root workspace files**

Create the root files listed above so the repo has a stable toolchain layout.

Initialize `chain/aegislink/go.mod` so the chain code has a real Go module from day one.

Write a real `buf.yaml` and `buf.gen.yaml` that generate Go code into `chain/aegislink/api`.

- [ ] **Step 3: Add minimal make targets**

Add: `make format`, `make proto`, `make test`, `make check`

- [ ] **Step 4: Run the first shape check**

Run: `find chain proto contracts relayer tests -maxdepth 3 -type d | sort`

Expected: top-level project folders exist

- [ ] **Step 5: Commit**

Run:
```bash
git add go.work Makefile buf.yaml buf.gen.yaml .gitignore chain/aegislink/go.mod
git commit -m "chore: bootstrap aegislink workspace"
```

## Task 2: Define The Shared Bridge Message Model

**Files:**
- Create: `proto/aegislink/bridge/v1/bridge.proto`
- Create: `proto/aegislink/registry/v1/registry.proto`
- Create: `proto/aegislink/limits/v1/limits.proto`
- Create: `chain/aegislink/x/bridge/types/keys.go`
- Create: `chain/aegislink/x/bridge/types/errors.go`
- Create: `chain/aegislink/x/registry/types/asset.go`
- Create: `chain/aegislink/x/limits/types/limits.go`

**What is happening:** You are defining the language the bridge will use before you write the logic that consumes it.

**Why this matters:** Cross-chain systems break when one component thinks a message means one thing and another component thinks it means something else.

**Source of truth rule:** The `.proto` files are the source of truth. The handwritten Go files in `types/` should only add helpers and validation around the generated types.

- [ ] **Step 1: Write the bridge proto schema**

Include at least:
- claim ID
- source chain ID
- tx hash
- log index
- asset ID
- amount
- recipient
- attestation metadata

- [ ] **Step 2: Write the registry proto schema**

Include asset metadata, status, decimals, origin chain, and route flags.

- [ ] **Step 3: Write the limits proto schema**

Include per-asset and per-route rate-limit fields.

- [ ] **Step 4: Add type helpers for replay identity and route checks**

Implement claim-key and replay-key helpers in `chain/aegislink/x/bridge/types/keys.go`.

Add validation helpers that explain route flags in plain language and reject invalid route configurations.

- [ ] **Step 5: Write the first cheap tests**

Add tests for:
- deterministic claim-key derivation
- missing-field rejection
- invalid route-flag combinations

- [ ] **Step 6: Run schema generation**

Run: `buf lint && buf generate`

Expected: schemas lint successfully and generate stable outputs under `chain/aegislink/api`

- [ ] **Step 7: Commit**

Run:
```bash
git add proto/aegislink chain/aegislink/api chain/aegislink/x/bridge/types chain/aegislink/x/registry/types chain/aegislink/x/limits/types
git commit -m "feat: define aegislink message and registry schemas"
```

## Task 3: Scaffold The AegisLink Chain App

**Files:**
- Modify: `chain/aegislink/go.mod`
- Create: `chain/aegislink/cmd/aegislinkd/main.go`
- Create: `chain/aegislink/app/app.go`
- Create: `chain/aegislink/app/config.go`
- Create: `chain/aegislink/x/bridge/module.go`
- Create: `chain/aegislink/x/registry/module.go`
- Create: `chain/aegislink/x/limits/module.go`
- Create: `chain/aegislink/x/pauser/module.go`

**What is happening:** You are creating the chain shell that will later host the bridge logic.

**Why this matters:** Before writing keeper logic, you need the app boundary and module entry points in place.

- [ ] **Step 1: Pin the chain dependencies**

Update `chain/aegislink/go.mod` to pin Cosmos SDK v0.53.x and the matching dependencies needed for a minimal app skeleton.

- [ ] **Step 2: Create the daemon entrypoint**

Create `chain/aegislink/cmd/aegislinkd/main.go` with the minimal application startup wiring.

- [ ] **Step 3: Create the app wiring**

Create `chain/aegislink/app/app.go` and register the placeholder modules.

- [ ] **Step 4: Add the app config**

Create `chain/aegislink/app/config.go` with the initial module and codec wiring.

- [ ] **Step 5: Run a deterministic compile check**

Run: `go test ./chain/aegislink/...`

Expected: PASS for the scaffolded app and module packages, even though keeper logic is still minimal

- [ ] **Step 6: Commit**

Run:
```bash
git add chain/aegislink/go.mod chain/aegislink/cmd chain/aegislink/app chain/aegislink/x
git commit -m "feat: scaffold aegislink chain app"
```

## Task 4: Implement The Asset Registry First

**Files:**
- Create: `chain/aegislink/x/registry/keeper/keeper.go`
- Create: `chain/aegislink/x/registry/keeper/keeper_test.go`
- Modify: `chain/aegislink/x/registry/module.go`
- Modify: `chain/aegislink/x/registry/types/asset.go`

**What is happening:** You are adding the allowlist that decides which assets the system is even allowed to touch.

**Why this matters:** A serious bridge never starts by saying "support every token." It starts by controlling the blast radius.

**Authority note:** In sprint 1, keep authority simple. Use one explicit admin authority placeholder for registry writes and test unauthorized-caller rejection.

- [ ] **Step 1: Write failing tests for registration behavior**

Cover:
- valid registration
- duplicate asset registration
- invalid decimals or metadata
- disabled asset rejection
- unauthorized caller rejection

- [ ] **Step 2: Run the registry tests**

Run: `go test ./chain/aegislink/x/registry/...`

Expected: FAIL until keeper logic exists

- [ ] **Step 3: Implement the minimal keeper**

Add store access and validation logic for asset registration and lookup.

- [ ] **Step 4: Re-run the registry tests**

Run: `go test ./chain/aegislink/x/registry/...`

Expected: PASS

- [ ] **Step 5: Commit**

Run:
```bash
git add chain/aegislink/x/registry
git commit -m "feat: add aegislink asset registry"
```

## Task 5: Add Pause Controls And Rate Limits

**Files:**
- Create: `chain/aegislink/x/pauser/keeper/keeper.go`
- Create: `chain/aegislink/x/pauser/keeper/keeper_test.go`
- Create: `chain/aegislink/x/limits/keeper/keeper.go`
- Create: `chain/aegislink/x/limits/keeper/keeper_test.go`
- Modify: `chain/aegislink/x/pauser/module.go`
- Modify: `chain/aegislink/x/limits/module.go`

**What is happening:** You are adding the two emergency controls that make v1 survivable when something goes wrong.

**Why this matters:** Bridges fail in operations, not only in code. Pause and limits are how you reduce damage when assumptions break.

**Authority note:** Reuse the same simple authority placeholder here and add unauthorized-caller tests before adding any happy-path logic.

- [ ] **Step 1: Write failing tests for pause behavior**

Cover paused and unpaused actions, plus route-specific pause behavior if you support it now.

Also cover unauthorized pause attempts.

- [ ] **Step 2: Write failing tests for rate-limit behavior**

Cover under-limit, at-limit, and over-limit cases.

Also cover unauthorized limit updates.

- [ ] **Step 3: Run the pause and limits tests**

Run: `go test ./chain/aegislink/x/pauser/... ./chain/aegislink/x/limits/...`

Expected: FAIL until keeper logic exists

- [ ] **Step 4: Implement minimal pause and limit keepers**

Keep the first version simple and deterministic.

- [ ] **Step 5: Re-run the tests**

Run: `go test ./chain/aegislink/x/pauser/... ./chain/aegislink/x/limits/...`

Expected: PASS

- [ ] **Step 6: Commit**

Run:
```bash
git add chain/aegislink/x/pauser chain/aegislink/x/limits
git commit -m "feat: add aegislink pause controls and rate limits"
```

## Task 6: Implement Bridge Claim Validation And Replay Protection

**Files:**
- Create: `chain/aegislink/x/bridge/keeper/keeper.go`
- Create: `chain/aegislink/x/bridge/keeper/keeper_test.go`
- Modify: `chain/aegislink/x/bridge/module.go`
- Modify: `chain/aegislink/x/bridge/types/keys.go`
- Modify: `chain/aegislink/x/bridge/types/errors.go`

**What is happening:** You are teaching the chain how to decide whether an inbound claim is acceptable at all.

**Why this matters:** Replay protection is only meaningful if the bridge module actually checks it together with registry, pause, and limits.

- [ ] **Step 1: Write failing bridge-level validation tests**

Cover:
- first valid claim accepted
- same claim rejected on replay
- malformed claim rejected
- disabled asset rejected
- paused flow rejected
- over-limit claim rejected

- [ ] **Step 2: Run the bridge tests**

Run: `go test ./chain/aegislink/x/bridge/...`

Expected: FAIL until keeper logic exists

- [ ] **Step 3: Implement the minimal claim-validation pipeline**

Persist claim IDs, validate asset registration, enforce pause state, check limits, and reject duplicates deterministically.

- [ ] **Step 4: Re-run the bridge tests**

Run: `go test ./chain/aegislink/x/bridge/...`

Expected: PASS

- [ ] **Step 5: Commit**

Run:
```bash
git add chain/aegislink/x/bridge
git commit -m "feat: add aegislink replay protection"
```

## Task 7: Prove The Foundation Works

**Files:**
- Modify: `Makefile`
- Modify: `docs/implementation/01-step-by-step-roadmap.md`
- Modify: `docs/implementation/02-tech-stack-and-repo-plan.md`

**What is happening:** You are closing the sprint by proving the chain core is real, testable, and understandable.

**Why this matters:** A plan is only useful if the first milestone ends with evidence.

- [ ] **Step 1: Run the full chain-side test suite**

Run: `go test ./chain/aegislink/...`

Expected: PASS for registry, pauser, limits, and replay tests

- [ ] **Step 2: Run the repo-level baseline**

Run: `make check`

Expected: PASS for `buf lint`, `buf generate`, and the chain-side test suite that exists in this sprint

- [ ] **Step 3: Write a short sprint note**

Update the implementation docs with:
- what now exists
- what still does not exist
- what the next sprint will build

- [ ] **Step 4: Commit**

Run:
```bash
git add Makefile docs/implementation/01-step-by-step-roadmap.md docs/implementation/02-tech-stack-and-repo-plan.md
git commit -m "docs: record aegislink foundation milestone"
```

## What Comes Immediately After This

Only after this plan is complete should you start:

1. Ethereum gateway contracts
2. the relayer pipeline
3. full local Ethereum plus Cosmos end-to-end wiring
4. Osmosis routing

That next phase is easier because the destination chain already knows:

- which assets are valid
- whether a claim is a replay
- whether the system is paused
- whether a route exceeds limits

## Short Learning Summary

If you only remember one thing from this plan, remember this:

`A bridge is not impressive because it moves tokens. It is impressive because it rejects bad cross-chain state changes safely.`

That is why you are starting with the AegisLink chain core first.
