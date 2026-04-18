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

For the newer dual-runtime route path:

```bash
make real-demo
make inspect-real-demo
```

The demo runs two proof paths:

1. A live local Ethereum deposit that is observed over RPC, relayed into AegisLink, and routed onward through the Osmosis-style target.
2. A configurable alternate-pool route that proves the destination side is not hardcoded to one swap output.

The real-route demo adds a third proof path:

3. A dual-runtime route where AegisLink and `osmo-locald` each boot from their own home directories and `route-relayer` moves a transfer across that boundary through command-backed packet and acknowledgement handling.

The Phase E runtime path adds a fourth proof path:

4. A daemon-style AegisLink node loop where `aegislinkd start --daemon` advances height automatically and drains queued deposit claims through the same application boundary used by the rest of the runtime.

The public-wallet bridge path adds a fifth proof path:

5. A Sepolia-shaped deposit and redeem loop where `public-bridge-relayer` can mint bridged wallet balances on AegisLink and later release the corresponding asset back to Ethereum.

The frontend branch adds a sixth proof path:

6. A browser-driven `Sepolia -> AegisLink -> Osmosis` run where the user connects a wallet in `web/`, submits the deposit from the UI, follows live stage-by-stage bridge progress, and ends on an Osmosis receipt with a destination transaction link.

For the visual version of this flow, use [Current flow diagrams](architecture/03-current-flow-diagrams.md).

## What the demo proves

- Ethereum is not mocked away. Deposits come from the local Anvil-backed path.
- AegisLink is not just a contract wrapper. It owns bridge accounting, routing, and policy.
- Outbound routing is not just a status flip. The destination target persists packet receipt, executes pool-backed swaps, and later produces an acknowledgement.
- The destination side is queryable through `/status`, `/packets`, `/executions`, `/pools`, `/balances`, and `/swaps`.
- Route intent is richer than a single happy-path swap. The target can honor `min_out`, recipient override, and path metadata, and it can fail cleanly on unsupported actions.
- The repo now also proves a real destination-runtime path through `make real-demo`, where the route no longer depends on the old HTTP target entrypoint.
- The repo now also proves a Hermes-shaped local packet flow, where the route path explicitly relays `recv-packet` and later `acknowledge-packet` across separate runtime homes.
- The repo now also proves a daemon-style node lifecycle through `make test-real-abci`, so the height advance is not only a manual setter path anymore.
- The repo now also proves a public-wallet bridge loop locally against Anvil-backed Sepolia-shaped contracts, including both deposit-to-wallet and redeem-back-to-Ethereum paths for native ETH and ERC-20.
- The repo now also proves a browser-first public wallet path, where `web/` drives the Sepolia deposit and the backend stack can complete a fresh run into a real Osmosis testnet wallet.

## Short demo transcript

1. Run `make demo`.
   Say: `This proves a live Ethereum deposit becomes an AegisLink settlement, then a routed destination-side execution.`
2. Run `make inspect-demo`.
   Say: `Now I am showing the receiver-side evidence, not just the final transfer status.`
3. Point at the packet and execution lifecycle.
   Say: `The route target stores receipt, execution, and later acknowledgement as separate states.`
4. Point at the pool and balance surfaces.
   Say: `The destination side is not a stub. It tracks balances, pools, swap outputs, and failure reasons.`
5. Run `make real-demo`.
   Say: `Now I am proving the route against a separate destination runtime home, not just the earlier HTTP harness path.`
6. Run `make test-real-abci`.
   Say: `Now I am proving that AegisLink can queue a deposit claim, advance blocks automatically, and apply the queued claim through the runtime loop.`
7. Run `./scripts/testnet/start_public_bridge_backend.sh` and `cd web && npm run dev`.
   Say: `Now I am showing the same system through the user-facing bridge surface instead of only through operator CLI commands, the backend lifts stale Osmosis timeout heights automatically on fresh runs, and the UI ends on the destination receipt instead of stopping at a pending label.`

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
- `The Phase 6 and Phase E path uses the dedicated osmo-locald runtime and Hermes-shaped route-relayer command path instead of the earlier HTTP-only target.`
- `The route can also fail for execution reasons like missing pool or min_out, not only transport reasons.`
- `The route timeout path is recoverable on AegisLink, so the demo covers both success and refund-safe failure.`
- `The AegisLink daemon loop can queue and later apply deposits, so the runtime now looks more like a single-node chain loop than a pure request-response shell.`
- `The public bridge path preserves asset identity, so the wallet receives bridged ETH or bridged ERC-20 first and only later redeems back to Ethereum instead of silently swapping assets.`
- `The frontend is not a mock shell. It sends a real Sepolia deposit transaction, registers a delivery intent, tracks the bridge session through the local status surface, and links the completed destination receipt directly.`

## Important honesty line

This is still a strong local prototype, not a production bridge:

- Ethereum observation and release are live locally.
- AegisLink is a persistent single-node runtime with a daemon block loop, not yet a full networked Cosmos node.
- The original destination-side walkthrough still uses its own bootstrapped runtime home and Hermes-shaped local packet flow for the local demo path.
- Real Osmosis wallet delivery is now live through the public frontend-driven path too, but the operator should still treat it as demo-grade and prefer a fresh backend launch for clean repeated verification runs.

That honesty makes the project stronger, not weaker.

## Demo failure troubleshooting

- `make demo fails before the route target starts`
  Check the relayer stderr logs first. The bridge-relayer and route-relayer now emit structured JSON summaries that show whether the failure happened on the Ethereum observation side, the AegisLink submission side, or the route acknowledgement side.
- `AegisLink looks healthy but the route does not complete`
  Inspect `/status`, `/packets`, and `/executions` on the mock target. If the packet is `received` but not `executed`, the failure is on the destination execution side. If it is `ack_ready` but not `ack_relayed`, the route-relayer is the likely bottleneck.
- `The destination swap failed`
  Check `/swaps`, `/pools`, and `/executions` for the execution error. Common causes are unsupported route intent, missing pool liquidity, or `min_out` not being met.
- `The route timed out`
  Confirm AegisLink shows the transfer as `timed_out`, then demonstrate the refund-safe path instead of trying to force a completion.
