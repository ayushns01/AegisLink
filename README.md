# AegisLink

AegisLink is a local Ethereum-to-Cosmos bridge systems project that proves deposit verification, bridge accounting, routed delivery, and destination-side execution end to end.

It is designed like a protocol, not like a token-transfer demo: Ethereum emits canonical bridge events, AegisLink owns bridge policy and accounting, and routed assets can execute destination-side actions through an Osmosis-style harness.

## In one minute

This repository is meant to show:

- explicit trust assumptions
- clean accounting boundaries
- replay protection and rate limits
- clear module and service separation
- a practical v1 architecture with a light-client roadmap

## What is real today

- Ethereum deposit observation and release execution run through the live local Anvil path.
- AegisLink owns bridge, registry, limits, pauser, and route state in a persistent runtime with `init`, `start`, and `query status`.
- The bridge-relayer and route-relayer are real services with replay persistence and route lifecycle handling.
- Routed transfers go through packet-shaped delivery, destination-side execution, later acknowledgement, and explicit completion, failure, timeout, or refund handling.
- The destination target tracks packets, execution receipts, balances, pools, swaps, and acknowledgement state through public inspection endpoints.

## What is a local harness today

- AegisLink is a persistent Cosmos-inspired runtime, not yet a full networked CometBFT or ABCI chain.
- The Osmosis side is an `osmosis-lite` receiver, not a live IBC-connected Osmosis node.
- The verifier model is still a v1 verifiable-relayer plus threshold-attestation path, not a light client.

## Why this project is not a toy

- It uses a dedicated Cosmos bridge zone instead of wiring Ethereum directly into a single destination app.
- It separates observation, verification, policy enforcement, settlement, and routing.
- It proves the full local bridge loop in both directions instead of stopping at inbound minting.
- It treats destination execution as first-class state, including async acknowledgements, swap failures, and refund-safe timeout handling.
- It is honest about the trust model and runtime limits instead of pretending the local harness is a production chain.

## Architecture snapshot

```mermaid
flowchart LR
    U["User"] --> E["Ethereum Gateway"]
    E --> BR["Bridge Relayer"]
    BR --> A["AegisLink Runtime"]
    A --> RR["Route Relayer"]
    RR --> O["Osmosis-lite Target"]
    O --> RR
    RR --> A
    A --> BR
    BR --> E
```

Use [Current flow diagrams](docs/architecture/03-current-flow-diagrams.md) for the fuller end-to-end view and the route lifecycle diagram.

## Documentation map

Start here if you want the basics:

- [Bridge basics](docs/foundations/01-bridge-basics.md)
- [Ethereum, Cosmos, IBC, and Osmosis primer](docs/foundations/02-eth-cosmos-primer.md)

Read these for the protocol design:

- [System architecture](docs/architecture/01-system-architecture.md)
- [Current flow diagrams](docs/architecture/03-current-flow-diagrams.md)
- [Security and trust model](docs/architecture/02-security-and-trust-model.md)
- [Project positioning](docs/project-positioning.md)
- [Architecture spec](docs/superpowers/specs/2026-03-28-eth-cosmos-aegislink-design.md)

Use these to build or review the project step by step:

- [Step-by-step roadmap](docs/implementation/01-step-by-step-roadmap.md)
- [Tech stack and repo plan](docs/implementation/02-tech-stack-and-repo-plan.md)
- [0-to-100 execution plan](docs/superpowers/plans/2026-03-30-aegislink-0-to-100-implementation.md)
- [Final stretch plan](docs/superpowers/plans/2026-04-05-aegislink-final-stretch-plan.md)
- [Future realism plan](docs/superpowers/plans/2026-04-06-aegislink-future-realism-plan.md)
- [Initial implementation plan, historical](docs/superpowers/plans/2026-03-28-eth-cosmos-aegislink-implementation.md)

Use these for operational and launch thinking:

- [Security model summary](docs/security-model.md)
- [Observability plan](docs/observability.md)
- [Demo walkthrough](docs/demo-walkthrough.md)
- [Pause and recovery runbook](docs/runbooks/pause-and-recovery.md)
- [Upgrade and rollback runbook](docs/runbooks/upgrade-and-rollback.md)

## What AegisLink v1 should say publicly

Use phrasing like:

- "AegisLink v1 is a verifiable-relayer bridge with threshold attestations."
- "AegisLink enforces replay protection, asset registration, rate limits, and pause controls."
- "AegisLink has a roadmap toward stronger Ethereum verification."

Do not describe v1 as fully trustless or fully light-client verified.

## Five-minute demo

If you want the fastest way to show the project working locally, run:

```bash
make demo
```

If you want the inspection-focused path that exercises the public target surfaces:

```bash
make inspect-demo
```

That demo exercises:

- a live local Ethereum deposit
- relayer submission into AegisLink
- outbound routing into the Osmosis-style target
- destination-side packet receipt, execution, and swap lifecycle
- public target queries for packets, executions, pools, balances, and swaps

For the full walkthrough, use [Demo walkthrough](docs/demo-walkthrough.md).
For the honest reviewer framing, use [Project positioning](docs/project-positioning.md).

## Runtime commands

`aegislinkd` now has a more node-like local runtime surface:

```bash
go run ./chain/aegislink/cmd/aegislinkd init --home /tmp/aegislink-home --chain-id aegislink-devnet-1 --runtime-mode sdk-store-runtime
go run ./chain/aegislink/cmd/aegislinkd start --home /tmp/aegislink-home
go run ./chain/aegislink/cmd/aegislinkd query status --home /tmp/aegislink-home
make test-real-chain
```

That flow creates and uses:

- a runtime config file
- a runtime genesis file
- a Cosmos KV-store-backed runtime store
- service-backed `tx` and `query` command paths

## Current checkpoint

As of April 6, 2026:

- the live local Ethereum bridge loop is proven end to end
- Phase 5 is now complete as a single-node SDK-store runtime milestone: AegisLink has store-backed keeper persistence, generated bridge or route proto surfaces, service-backed CLI responses, and a real-chain bootstrap or e2e proof through `aegislinkd init`, `start`, `tx`, and `query`
- Phase 1 of the fuller route-harness plan is complete
- Phase 3 runtime and operator surfaces now include structured startup and run logs plus clearer runtime validation
- Phase 4 hardening now adds stronger replay and supply invariants, a narrow verifier interface, and demo-facing failure counters
- the routed side now has explicit packet, execution, and acknowledgement lifecycle state
- the next roadmap focus is deeper chain realism beyond the SDK-store runtime, not basic bridge invention

The current repo shape is:

- [chain/aegislink](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/chain/aegislink): persistent runtime, bridge state machine, safety modules, and route lifecycle handling
- [contracts/ethereum](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/contracts/ethereum): Ethereum event source and release verification contracts
- [relayer](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/relayer): observation, attestation, replay, live forward or reverse bridge pipeline, and route-target handoff services

Fresh verification checkpoints that already pass in this repo:

- `go test ./chain/aegislink/...`
- `forge test --offline`
- `go test ./relayer/...`
- `cd tests/e2e && go test ./...`

The local route-harness, operator-surface, and SDK-store runtime phases are now in place. The next active roadmap work is the deeper realism layer after that milestone: moving from the current single-node runtime into a more networked Cosmos experience and then deeper verifier hardening.
