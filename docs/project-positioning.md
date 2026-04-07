# AegisLink Project Positioning

This document is the short, honest description of what AegisLink is today and what it is not.

## One-line description

AegisLink is a local Ethereum-to-Cosmos bridge systems project that proves end-to-end bridge settlement, routed delivery, and destination-side execution with an explicit v1 trust model.

## Security model

- AegisLink v1 is a `verifiable-relayer bridge with threshold attestations`.
- The repository now includes both the original narrow verifier and a threshold-verifier path with signer-set rotation on Ethereum.
- Ethereum is the canonical source of deposit and release events.
- AegisLink attestations are now tied to explicit signer-set versions, and bridge verification checks activation, expiry, and version mismatch.
- AegisLink enforces replay protection, registry policy, rate limits, pause controls, and route state transitions.
- The project does not currently claim a light-client verifier or a trustless Ethereum proof system.

## What is real today

- Ethereum gateway contracts emit and consume real local events and release transactions over the Anvil-backed path.
- The bridge-relayer and route-relayer are real services with replay-aware processing and route lifecycle handling.
- AegisLink persists bridge, policy, and route state in Cosmos KV stores and exposes operator-facing `init`, `start`, `tx`, and `query` CLI commands.
- Bridge and route interfaces now have generated proto surfaces and service-backed CLI response mapping inside the `chain/aegislink` module.
- Routed transfers produce packet state, execution receipts, balances, pool updates, swap records, and later acknowledgements on the destination side.
- The end-to-end local flow is exercised in tests and exposed through `make demo` and `make inspect-demo`.
- The repo now also proves a single-node real-chain flow through `make test-real-chain`.
- The repo now also proves a dual-runtime local route flow through `make test-real-ibc`, where `route-relayer` moves a transfer from an AegisLink home into a dedicated `osmo-locald` home.
- The repo now also proves a threshold-verifier path in Foundry tests and versioned signer-set enforcement on the AegisLink side.
- The runtime and CLI already expose active signer-set state and signer-set history, so the trust model is inspectable instead of buried in code.

## What is still a local harness

- AegisLink is now a store-backed single-node runtime, but it is still not a full networked CometBFT or ABCI chain.
- State is persisted in Cosmos KV stores, but the repo still does not claim live consensus, IAVL-backed network operation, or real IBC today.
- The destination side is now a bootstrapped local runtime with its own config and state, but it is still not a live IBC-Go or Hermes-connected Osmosis node.
- The route target is realistic enough to exercise packet lifecycle and destination execution, but it is still a controlled local environment.

## Why this still matters

- The hard bridge logic is already real: message identity, replay protection, verification boundaries, bridge accounting, route policy, and failure handling.
- The project shows that the protocol was scoped correctly before chasing full chain realism.
- The local harness keeps the architecture honest while still proving the bridge loop, route lifecycle, and destination execution semantics end to end.

## How to describe it in review or interview settings

Use phrasing like:

- `AegisLink is a runtime-backed local bridge prototype with live Ethereum integration and a realistic routed execution harness.`
- `The current repository proves the bridge and route lifecycle end to end, and the Cosmos side now persists through a single-node SDK-store runtime, but it is still not a full networked chain.`
- `The roadmap from here is deeper networked chain realism and fuller IBC-Go or Hermes realism, with the verifier boundary now documented explicitly.`

Avoid phrasing like:

- `fully trustless bridge`
- `real Cosmos chain today`
- `live IBC integration today`

## Future roadmap

The next realism steps are:

1. Push AegisLink from the current single-node runtime toward a real networked chain daemon.
2. Replace the current dual-runtime route bridge with fuller IBC-Go or Hermes-backed networking.
3. Add stronger metrics, dashboards, and operator recovery surfaces.
4. Only after that, spend time on optional verifier and production-style hardening.

## Hardening now present

- The bridge keeper now has stronger replay and supply-conservation invariant coverage.
- The route keeper now has explicit recoverable-refund state-machine coverage.
- The Ethereum gateway now depends on a narrow verifier interface, so the current v1 verifier can be swapped more cleanly for future threshold or light-client implementations.
- The bridge keeper now tracks versioned signer sets with activation and expiry, so attestation trust assumptions are explicit and queryable.
- Demo-facing status summaries now expose failed claims and destination swap failures instead of only happy-path counts.
