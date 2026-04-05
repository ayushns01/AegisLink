# AegisLink Demo Walkthrough

This is the fastest recruiter-friendly path through the repo.

## Quick run

From the repository root:

```bash
make demo
```

The demo runs two proof paths:

1. A live local Ethereum deposit that is observed over RPC, relayed into AegisLink, and routed onward through the Osmosis-style target.
2. A configurable alternate-pool route that proves the destination side is not hardcoded to one swap output.

## What the demo proves

- Ethereum is not mocked away. Deposits come from the local Anvil-backed path.
- AegisLink is not just a contract wrapper. It owns bridge accounting, routing, and policy.
- Outbound routing is not just a status flip. The destination target executes pool-backed swaps.
- The destination side is queryable through `/pools`, `/balances`, and `/swaps`.

## What to say while showing it

- `Ethereum emits the canonical source event.`
- `The relayer carries evidence into AegisLink.`
- `AegisLink mints and routes according to policy.`
- `The mock Osmosis target executes the destination-side swap and exposes the resulting state.`
- `The route can also fail for execution reasons like missing pool or min_out, not only transport reasons.`

## Important honesty line

This is still a strong local prototype, not a production bridge:

- Ethereum observation and release are live locally.
- AegisLink is a persistent runtime, not yet a full networked Cosmos node.
- The Osmosis side is an `osmosis-lite` harness, not a full real IBC-connected Osmosis node.

That honesty makes the project stronger, not weaker.
