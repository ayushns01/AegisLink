# AegisLink Real IBC Demo Node Design

## Summary

The next honest milestone for AegisLink is not another local harness seam. It is a real single-validator demo chain that exposes the minimum networked surfaces required for IBC tooling to talk to it: Tendermint or CometBFT RPC, gRPC, and event streaming. That node should still be easy to operate locally, so the target is a one-command bootstrap that can be switched on for demos and switched off afterward.

This design keeps AegisLink meaningful as the bridge zone. Sepolia remains the canonical source of deposits and releases, AegisLink remains the custody-accounting and policy boundary, and Osmosis receives assets through a real ICS20 transfer path instead of a custom broadcaster shortcut.

## Goals

- Make AegisLink real enough for `rly` or Hermes to treat it like a normal IBC-capable chain.
- Preserve AegisLink as the canonical bridge zone instead of bypassing it with a direct Osmosis broadcaster.
- Keep the demo operational model simple: start one local command, relay while it is running, stop it after the demo.
- Reuse as much of the current bridge and policy logic as possible instead of rewriting the project from scratch.
- End with an honest `Sepolia -> AegisLink -> Osmosis wallet` architecture story.

## Non-Goals

- A multi-validator or production-grade public network.
- Chain-registry publication or wallet-listing work before the node itself is real.
- A trustless Ethereum light client path.
- Replacing every existing local-harness path immediately.
- Making the first real-node slice cover every existing route action and governance surface.

## Current Constraint

The current `aegislinkd` runtime is store-backed and useful, but it is still a command-driven single-process app shell. It does not expose real network endpoints or `ibc-go` modules. Hermes, `rly`, and similar relayers can only move packets for a chain that serves real state, headers, queries, transactions, and events over the expected Cosmos stack.

Changing relayers does not remove that requirement. Hermes is optional; a relayer is not. Official Hermes documentation still expects per-chain RPC, gRPC, and event sources, and current Osmosis metadata should come from the Cosmos chain-registry rather than from stale checked-in assumptions.

## Design Principles

- Prefer a real single-validator path over another simulation seam.
- Keep the current harness alive while the real node is brought up, so we do not freeze all existing bridge work.
- Start with the narrowest useful IBC feature set: ICS20 transfers only.
- Optimize for operator startup simplicity before chasing full protocol breadth.
- Keep docs honest at every step. Do not claim live Osmosis delivery until the real transfer proof exists.

## Recommended Architecture

### 1. Add a Real Demo-Node Path Inside AegisLink

Instead of pretending the current runtime is already a chain, add a real demo-node path that lives alongside the current harness during the transition. The `aegislinkd` CLI remains the operator entrypoint, but it gains a real networked mode backed by Cosmos SDK and `ibc-go` components instead of only the current local block loop.

The practical goal is:

- one single-validator node
- one local home directory
- one real set of RPC, gRPC, and REST surfaces
- one chain ID stable enough for a relayer configuration

This path should be honest enough for `rly` to connect without special-casing AegisLink as a fake chain.

### 2. Reuse Bridge-Zone Domain Logic Where Possible

The current keeper logic for assets, bridge accounting, bank balances, limits, pause controls, and route profiles should remain the domain source of truth. The real node path should reuse or adapt those modules instead of inventing a second bridge brain.

That means the networked app should preserve:

- asset registry semantics
- bridge mint or burn accounting
- wallet balance ownership
- route-profile policy checks
- withdrawal state that later drives Sepolia release

The networked path is a new execution surface, not a new product concept.

### 2a. Define a State-Migration Boundary Up Front

The real node path needs an explicit migration boundary so the current store-backed bridge state can become honest SDK module state instead of a parallel shadow store. The migration rules for the first slice should be:

- registry assets become genesis or bootstrap entries for the real registry module
- bridged wallet balances become bank genesis balances on the real node
- rate limits, pause flags, signer-set metadata, and withdrawal records become module-owned state, not sidecar JSON fixtures
- old harness-specific persistence remains only for the current local test harness, not for the real node path

The first implementation does not need a fully general migration engine, but it does need a documented rule for which current state is bootstrapped into the real node and which state remains harness-only. Without that boundary, “reusing the keepers” is too vague to implement safely.

### 3. Start with ICS20 Transfer Only

For live Osmosis delivery, the first real node slice only needs:

- bank balances that hold bridged `ueth` and later ERC-20 representations
- `ibc-go` transfer module wiring
- enough auth or tx flow to initiate a transfer
- relayer compatibility with a real packet lifecycle

This keeps the first transfer path intentionally small:

`Sepolia deposit -> AegisLink bridged balance -> ICS20 transfer -> osmo1 recipient`

Route actions like swaps and stakes remain later work after the plain asset-preserving transfer lands.

### 4. Prefer `rly` First, Keep Hermes as a Compatibility Fallback

The first real relayer target should be `rly`, not because Hermes is wrong, but because `rly` is a practical fit for a minimal local demo-node path and standard `ibc-go` compatibility. Hermes remains valid, but it is not the only honest choice.

The design should treat relayer choice like this:

- default: `rly`
- fallback: Hermes if the final chain shape aligns better there
- escape hatch: a thin AegisLink-specific relayer only if the networked node exposes a real chain surface but standard relayers still cannot integrate cleanly

### 5. One-Command Demo Bootstrap

The operator experience should be:

```bash
scripts/testnet/start_aegislink_ibc_demo.sh
```

That command should:

- initialize or reuse the AegisLink real-node home
- start the single-validator node
- wait for health on RPC and gRPC
- seed bridge assets and IBC route profiles
- generate or refresh the `rly` path config
- either start the relayer in a managed child process or print the exact follow-up relayer command as a second explicit step

Relayer ownership needs to stay explicit. The bootstrap is allowed to manage `rly`, but if it does, it also needs to own the corresponding teardown path and pid tracking. If the first slice keeps relayer startup as a second command for simplicity, the docs and scripts must say that clearly instead of implying a hidden always-on relay.

The matching stop flow can remain simple for now:

```bash
scripts/testnet/stop_aegislink_ibc_demo.sh
```

The stop helper should be idempotent and responsible for all processes the bootstrap owns, including the relayer if it was launched by the bootstrap.

## Network Surfaces Required

The real demo node must serve:

- RPC for chain state and headers
- gRPC for SDK and IBC queries
- event or WebSocket support for relayer subscriptions
- transaction submission surfaces for transfer initiation and relayed packet handling

The repo should stop calling these merely “intended endpoints” once the real-node path exists. They should become actual operator endpoints that can be probed and used by relayers.

## Data Flow

### Deposit to AegisLink

1. User deposits native ETH or an ERC-20 on Sepolia.
2. The bridge relayer observes and verifies the deposit.
3. AegisLink accepts the claim and mints the bridged balance to the user’s Bech32 account on the real AegisLink node.

### AegisLink to Osmosis

1. The user or operator initiates an ICS20 transfer from AegisLink using a seeded route profile or direct transfer flow.
2. The packet is committed to the AegisLink chain.
3. `rly` or Hermes relays the packet to Osmosis.
4. Osmosis credits the `osmo1...` wallet with the IBC denomination.

## Milestones

To keep public claims honest, this work should be measured in three separate milestones:

### Milestone 1: Real Node Readiness

- AegisLink starts as a real single-validator node
- RPC, gRPC, and event surfaces are live
- bank and bridge state seed into the real node path correctly

### Milestone 2: Local IBC Readiness

- `rly` or Hermes can connect to the demo node
- local packet creation, relay, acknowledgement, and timeout paths are provable
- route-profile seeding works against the real node

### Milestone 3: Live Osmosis Delivery

- the real node can send a small bridged asset amount to a real `osmo1...` wallet on Osmosis testnet
- only at this point should the repo claim live Osmosis wallet delivery

### Redeem Back to Sepolia

The existing redeem flow remains important. The live demo should eventually support:

1. bridged asset held on AegisLink
2. burn or withdrawal record created there
3. relayer submits the release back to Sepolia

That path remains separate from the Osmosis delivery proof, but both should still use AegisLink as the bridge-zone authority.

## File and Package Strategy

The fastest honest approach is not to delete the current harness first. Instead:

- keep the existing store-backed harness paths for current local tests
- add a real demo-node app path under `chain/aegislink`
- add testnet bootstrap scripts under `scripts/testnet`
- add relayer bootstrap assets under `deploy/testnet/ibc`

This lets the current bridge continue to function while the real-node slice comes online incrementally.

## Testing Strategy

### Local deterministic tests

- app construction and config validation for the real-node path
- genesis bootstrap and endpoint readiness
- route-profile seeding into the real node
- transfer initiation against the real app path

### Local relayer integration

- `rly` or Hermes config generation
- path creation against the AegisLink demo node and Osmosis metadata fixtures
- ICS20 packet and acknowledgement lifecycle tests

### Gated live tests

- only run when real Osmosis testnet credentials and endpoints are present
- send a small bridged asset amount to the target `osmo1...` wallet
- query and verify receipt

## Risks

- The current custom keepers may not map cleanly into a real SDK app without some restructuring.
- `ibc-go` integration can become a large dependency jump if we try to wire too many modules at once.
- A one-command local demo bootstrap can still be fragile if it tries to hide too much relayer complexity.
- Osmosis testnet metadata changes over time, so the bootstrap should pull from chain-registry-style config rather than hardcode long-lived assumptions.

## Recommended Execution Order

1. Build the minimal real AegisLink demo-node app and startup path.
2. Expose actual health-checked RPC and gRPC endpoints.
3. Wire the transfer module and a minimal send path.
4. Add `rly` bootstrap assets and local packet tests.
5. Prove one live Osmosis wallet receipt.

## Conclusion

The way out is not another shortcut. It is a narrower but honest chain implementation: a real single-validator AegisLink demo node that can be switched on for a live demo and switched off afterward. Once that exists, Osmosis delivery can happen through real IBC instead of through a path that undermines AegisLink’s role as the bridge zone.
