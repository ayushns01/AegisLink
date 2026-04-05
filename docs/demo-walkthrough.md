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

## What the demo proves

- Ethereum is not mocked away. Deposits come from the local Anvil-backed path.
- AegisLink is not just a contract wrapper. It owns bridge accounting, routing, and policy.
- Outbound routing is not just a status flip. The destination target persists packet receipt, executes pool-backed swaps, and later produces an acknowledgement.
- The destination side is queryable through `/status`, `/packets`, `/executions`, `/pools`, `/balances`, and `/swaps`.

## Route lifecycle to point at

During the inspection path, call out the destination-side lifecycle explicitly:

- `received`: the target accepted and persisted the routed packet
- `executed`: the target finished credit or swap execution
- `ack_ready`: a destination result is ready for the route-relayer to pick up
- `ack_relayed`: the acknowledgement has been consumed and confirmed back to AegisLink

Those states are what make the local harness feel closer to real interchain delivery than a single callback.

## What to say while showing it

- `Ethereum emits the canonical source event.`
- `The relayer carries evidence into AegisLink.`
- `AegisLink mints and routes according to policy.`
- `The mock Osmosis target persists the packet, executes the destination-side swap, and exposes both packet and execution state.`
- `The route can also fail for execution reasons like missing pool or min_out, not only transport reasons.`

## Important honesty line

This is still a strong local prototype, not a production bridge:

- Ethereum observation and release are live locally.
- AegisLink is a persistent runtime, not yet a full networked Cosmos node.
- The Osmosis side is an `osmosis-lite` harness, not a full real IBC-connected Osmosis node.

That honesty makes the project stronger, not weaker.
