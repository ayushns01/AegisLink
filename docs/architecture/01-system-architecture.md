# AegisLink System Architecture

## Overview

AegisLink is an Ethereum-to-Cosmos interoperability layer designed as a protocol, not as a single-purpose app. In v1, it uses a verifiable-relayer model: Ethereum events are observed by an off-chain relayer set, converted into threshold attestations, and then verified by a Cosmos-SDK chain that acts as the bridge zone. In phase 2, the bridge zone routes assets onward to Osmosis over IBC for swaps and liquidity.

The bridge zone is the accounting and policy boundary. It is not just a message sink. It owns asset registration, replay protection, mint/burn or lock/unlock accounting, rate limits, and pause controls. It is the place where cross-chain claims become state changes.

## Goals

- Make the bridge easy to reason about as a system of bounded components.
- Separate observation, verification, policy enforcement, and token movement.
- Keep the v1 trust model explicit and narrow.
- Leave a clean path to replace the attestation model with an Ethereum light client in v2.

## Core Components

### Ethereum side

- `BridgeGateway` contracts emit canonical deposit events and execute attested releases back on Ethereum.
- `AssetRegistry` stores Ethereum-side metadata for supported assets.
- `PauseController` can halt new deposits, withdrawals, or both.
- `ThresholdAttestation` references are consumed by the bridge zone as proof artifacts.

### Off-chain relayer layer

- Watches Ethereum logs and finality signals.
- Builds observation bundles keyed by chain ID, tx hash, log index, and asset metadata.
- Collects or aggregates threshold attestations from authorized signers.
- Submits verified claims to the bridge zone.
- Handles routed-transfer handoff from AegisLink into downstream targets.
- Can be fully replaced in v2 by a light-client verification path, but remains the v1 execution path.

Current implementation note:

- As of April 5, 2026, the repository implements the relayer as a real pipeline with replay persistence, forward and reverse-path processing, command-backed AegisLink runtime integration, RPC-backed Ethereum observation and release execution, plus a separate local route-relayer for Osmosis-style handoff.
- File-backed adapters still exist as local fallbacks, but they are no longer the highest-fidelity execution path in the repository.

### Bridge zone on Cosmos-SDK

- `bridge` module: verifies attestations, enforces replay protection, mints or releases representation assets, and tracks claim status.
- `registry` module: stores supported assets, decimal metadata, canonical denominations, and governance status.
- `limits` module: enforces per-asset and per-route throttles.
- `pauser` module: exposes chain-wide and asset-scoped emergency controls.
- `ibcrouter` module: tracks eligible routed transfers after they exist on the bridge zone, including pending, completed, failed, timed-out, and refunded states on the current local runtime path.

### Osmosis route

- The bridge zone is the source chain for phase 2 IBC transfers.
- Assets move from bridge-zone denominations into Osmosis through a predefined IBC channel.
- Osmosis receives them as standard IBC assets and can route them into swaps or liquidity pools.
- In the current repository checkpoint, the route lifecycle is implemented and queryable through the AegisLink runtime CLI, and a separate local route-relayer can drive those transfers against a lightweight target service.
- That target now persists packet receipts, denom-trace-style metadata, asynchronous acknowledgements, recipient balances, configurable multi-pool swap execution records, fee-aware pricing, and execution-driven acknowledgement failures derived from routed packets.
- The full live IBC channel or local Osmosis stack is still a later extension.

## Message Interfaces

### Deposit claim

A deposit claim is the unit of work submitted to the bridge zone. It should include:

- source chain ID
- source contract address
- destination chain ID
- source transaction hash
- log index
- asset identifier
- amount
- recipient
- bridge nonce or unique claim key
- attestation set or aggregated proof

### Withdrawal claim

A withdrawal claim is the reverse direction:

- bridge-zone transaction hash
- burn or escrow event
- destination Ethereum address
- asset identifier
- amount
- withdrawal nonce
- attestation set or proof payload

### IBC transfer packet

For phase 2, the bridge zone emits standard IBC transfer packets with:

- source denom
- destination denom trace
- receiver on Osmosis
- timeout height or timestamp
- memo if needed for routing or observability

## Message Lifecycle

```mermaid
sequenceDiagram
    participant U as User
    participant E as Ethereum BridgeGateway
    participant R as Relayer
    participant C as Cosmos AegisLink
    participant O as Osmosis

    U->>E: Deposit asset
    E-->>R: Emit deposit event
    R->>R: Wait finality window
    R->>R: Build observation bundle
    R->>R: Collect threshold attestations
    R->>C: Submit verified claim
    C->>C: Verify attestation, replay key, limits, pause
    C->>C: Mint voucher or release asset
    C->>O: IBC transfer optional
    O-->>U: Asset available for swap/liquidity
```

1. A user deposits an approved asset into the Ethereum gateway contract.
2. The gateway emits an event with enough data to reconstruct the claim.
3. Relayers wait for the configured finality depth and collect threshold attestations.
4. The bridge zone verifies the claim, checks uniqueness, and enforces policy.
5. The bridge zone mints a representation asset or unlocks the canonical asset on Cosmos.
6. If the route is enabled, the bridge zone forwards the asset to Osmosis over IBC.
7. The claim transitions to a terminal state and cannot be replayed.

In the current local milestone, the reverse direction is also proven end to end:

1. A withdrawal is executed on AegisLink and burns the bridged representation.
2. The relayer observes the persisted withdrawal record from the AegisLink runtime.
3. The relayer submits a real `release` transaction to Ethereum over JSON-RPC.
4. The gateway verifies the attestation, releases the canonical asset, and marks the proof consumed.

In the current routing milestone, the onward route is also proven in a controlled local form:

1. A live Ethereum deposit can be observed over RPC and minted onto AegisLink.
2. AegisLink can initiate an outbound Osmosis-style transfer through `ibcrouter`.
3. A separate `route-relayer` can read pending transfers from AegisLink and submit packet-shaped deliveries to a local target.
4. The target stores the receive-side state first, then exposes a later acknowledgement that drives completed, acknowledgement-failed, or timed-out state on the AegisLink side.
5. Refund and transfer state remain queryable from the persisted runtime.

## Asset Lifecycle

AegisLink supports a constrained set of assets, each with an explicit lifecycle.

1. The asset is registered with canonical metadata, decimals, source chain, and policy flags.
2. The first deposit creates a bridge-zone representation or accounting entry.
3. The representation can move within the bridge zone subject to rate limits and pause state.
4. A supported route can forward the asset to Osmosis over IBC.
5. A withdrawal burns the bridge-zone representation or returns an escrowed balance.
6. An attested claim releases the asset on Ethereum.

The important property is that every asset is always in one of a few well-defined states: locked, represented, routed over IBC, escrowed, burned, or released. There should be no hidden balance state.

## Bridge Zone Role

The bridge zone is the trust boundary where external claims become local state changes.

- It verifies that a claim is authorized by the expected threshold.
- It decides whether the claim is eligible for minting, unlock, or routing.
- It prevents duplicate execution using unique claim keys.
- It applies operational controls such as pause flags and rate limits.
- It serves as the bridge zone for phase 2 IBC routing, so Osmosis sees only the bridge zone as the source chain.

This role makes the bridge zone the protocol's accounting center, which keeps the system auditable and easier to extend later.

## Recommended Repo and Service Boundaries

For a recruiter-grade repository, keep the boundaries obvious. In the current repo they map to:

- `contracts/ethereum/`: gateway contracts, verifier logic, deployment script, and Foundry tests.
- `chain/aegislink/app/`: Cosmos app shell and top-level chain configuration.
- `chain/aegislink/x/bridge/`: claim verification, replay checks, state transitions, and accounting.
- `chain/aegislink/x/registry/`: asset registry and canonical asset metadata.
- `chain/aegislink/x/limits/`: per-asset throttles and limit policy.
- `chain/aegislink/x/pauser/`: emergency stop controls.
- `relayer/internal/evm/`: Ethereum-side event observation and live release execution, with file-backed fallbacks for local fixtures.
- `relayer/internal/cosmos/`: Cosmos-side withdrawal observation and claim submission handling, including command-backed runtime integration.
- `relayer/internal/attestations/`: vote collection and quorum assembly.
- `relayer/internal/replay/`: durable replay and checkpoint state.
- `relayer/internal/pipeline/`: forward and reverse bridge orchestration.
- `relayer/internal/route/`: pending-route polling, target delivery, and completion or failure handling for Osmosis-style transfers.
- `relayer/cmd/route-relayer/`: local executable that drives route lifecycle transitions from pending to completed or recoverable states.
- `relayer/cmd/mock-osmosis-target/`: lightweight local HTTP service used to simulate a downstream route target during e2e tests and devnet runs, including receive-side balances, configurable pools, and swap records.
- `proto/`: shared message schemas and cross-component identifiers.
- `docs/`: architecture, security, and implementation specs.
- `tests/e2e/osmosis_route_test.go`: routed-flow proofs for local-target completion, timeout, and refund behavior.

Prefer a monorepo layout if the team is small, but keep service boundaries explicit so the relayer can be swapped independently of the chain modules.

## Interface Rules

- All cross-chain claims must be keyed by a unique, deterministic claim ID.
- The bridge zone must reject any claim that does not prove finality according to the configured policy.
- Asset metadata must be versioned, not overwritten in place.
- Pause and limit decisions must be checked before minting, burning, or IBC forwarding.
- Any route to Osmosis must be gated by a chain-owned allowlist and an initialized IBC channel in the final system; the current local runtime already enforces the allowlist, explicit route state transitions, and a separate handoff boundary between AegisLink and the route target.

## v1 to v2 Direction

v1 assumes a verifiable relayer with threshold attestations. v2 should preserve the same message model but replace the proof source with an Ethereum light client verifier on the bridge zone or an equivalent on-chain verification path. The architecture above is intentionally structured so the claim interface does not need to change when the trust model improves.
