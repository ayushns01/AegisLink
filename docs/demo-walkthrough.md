# AegisLink Demo Walkthrough

This is the fastest recruiter-friendly path through the repo.

## Quick run

From the repository root:

```bash
make demo
```

For the inspection-oriented path:

```bash
make inspect-demo
```

The demo runs two proof paths:

1. A live local Ethereum deposit that is observed over RPC, relayed into AegisLink, and routed onward through the Osmosis-style target.
2. A configurable alternate-pool route that proves the destination side is not hardcoded to one swap output.

For the visual version of this flow, use [Current flow diagrams](architecture/03-current-flow-diagrams.md).

## What the demo proves

- Ethereum is not mocked away. Deposits come from the local Anvil-backed path.
- AegisLink is not just a contract wrapper. It owns bridge accounting, routing, and policy.
- Outbound routing is not just a status flip. The destination target persists packet receipt, executes pool-backed swaps, and later produces an acknowledgement.
- The destination side is queryable through `/status`, `/packets`, `/executions`, `/pools`, `/balances`, and `/swaps`.
- Route intent is richer than a single happy-path swap. The target can honor `min_out`, recipient override, and path metadata, and it can fail cleanly on unsupported actions.

## Short demo transcript

1. Run `make demo`.
   Say: `This proves a live Ethereum deposit becomes an AegisLink settlement, then a routed destination-side execution.`
2. Run `make inspect-demo`.
   Say: `Now I am showing the receiver-side evidence, not just the final transfer status.`
3. Point at the packet and execution lifecycle.
   Say: `The route target stores receipt, execution, and later acknowledgement as separate states.`
4. Point at the pool and balance surfaces.
   Say: `The destination side is not a stub. It tracks balances, pools, swap outputs, and failure reasons.`

## Route lifecycle to point at

During the inspection path, call out the destination-side lifecycle explicitly:

- `received`: the target accepted and persisted the routed packet
- `executed`: the target finished credit or swap execution
- `ack_ready`: a destination result is ready for the route-relayer to pick up
- `ack_relayed`: the acknowledgement has been consumed and confirmed back to AegisLink

Those states are what make the local harness feel closer to real interchain delivery than a single callback.

## What to point at during the demo

- `Ethereum deposit`
  Point at the Anvil-backed source path and explain that the deposit is observed over live local RPC.
- `AegisLink status`
  Point at `aegislinkd query status` and explain that AegisLink owns bridge, policy, and route state.
- `Route state`
  Point at `/packets` and `/executions` and call out `received`, `executed`, `ack_ready`, and `ack_relayed`.
- `Destination balances and swap output`
  Point at `/balances`, `/pools`, and `/swaps` to show that the target actually executes value movement.

## What to say while showing it

- `Ethereum emits the canonical source event.`
- `The relayer carries evidence into AegisLink.`
- `AegisLink mints and routes according to policy.`
- `The mock Osmosis target persists the packet, executes the destination-side swap, and exposes both packet and execution state.`
- `The route can also fail for execution reasons like missing pool or min_out, not only transport reasons.`
- `The route timeout path is recoverable on AegisLink, so the demo covers both success and refund-safe failure.`

## Important honesty line

This is still a strong local prototype, not a production bridge:

- Ethereum observation and release are live locally.
- AegisLink is a persistent runtime, not yet a full networked Cosmos node.
- The Osmosis side is an `osmosis-lite` harness, not a full real IBC-connected Osmosis node.

That honesty makes the project stronger, not weaker.
