# AegisLink Project Positioning

This document is the short, honest description of what AegisLink is today and what it is not.

## One-line description

AegisLink is a local Ethereum-to-Cosmos bridge systems project that proves end-to-end bridge settlement, routed delivery, and destination-side execution with an explicit v1 trust model.

## Security model

- AegisLink v1 is a `verifiable-relayer bridge with threshold attestations`.
- The repository now includes both the original narrow verifier and a threshold-verifier path with signer-set rotation on Ethereum.
- The Ethereum verifier path now uses EIP-712-style attestation digests, rejects non-low-`s` signatures, and the release flow is guarded against reentrant token callbacks.
- Ethereum is the canonical source of deposit and release events.
- AegisLink attestations are now tied to explicit signer-set versions, carry cryptographic signer proofs, and bridge verification checks activation, expiry, version mismatch, and signature validity.
- AegisLink enforces replay protection, registry policy, rolling-window rate limits, pause controls, and route state transitions.
- Governance policy changes now require a configured authority and persist who applied each change.
- The bridge runtime now has an accounting invariant and visible circuit-breaker path, so corrupted supply cannot keep processing silently.
- The Ethereum verifier/gateway path is intentionally immutable and non-proxy in v1, which keeps the trust story easy to explain and avoids introducing proxy-admin or upgrade-rollback complexity too early.
- The project does not currently claim a light-client verifier or a trustless Ethereum proof system.
- The project does not currently claim proxy-based upgradeability for the Ethereum verifier/gateway path.

## What is real today

- Ethereum gateway contracts emit and consume real local events and release transactions over the Anvil-backed path.
- The bridge-relayer and route-relayer are real services with replay-aware processing and route lifecycle handling.
- The relayers now also support poll-based daemon operation with graceful shutdown and temporary-failure backoff, so the local system can be exercised as a long-running worker instead of only one-shot commands.
- AegisLink persists bridge, policy, and route state in Cosmos KV stores and exposes operator-facing `init`, `start`, `tx`, and `query` CLI commands.
- The SDK-store runtime now persists bridge, registry, limits, pauser, governance, and route state as prefix-keyed records instead of single JSON blobs, so reload behavior is per-record and the storage layout is closer to real chain state.
- Bridge and route interfaces now have generated proto surfaces and service-backed CLI response mapping inside the `chain/aegislink` module.
- Routed transfers produce packet state, execution receipts, balances, pool updates, swap records, and later acknowledgements on the destination side.
- The end-to-end local flow is exercised in tests and exposed through `make demo` and `make inspect-demo`.
- The repo now also proves a single-node real-chain flow through `make test-real-chain`.
- The repo now also proves a daemon-style single-node block loop through `make test-real-abci`, where `aegislinkd start --daemon` advances height and drains queued deposit claims through the app boundary.
- The repo now also proves a dual-runtime local route flow through `make test-real-ibc`, where `route-relayer` moves a transfer from an AegisLink home into a dedicated `osmo-locald` home through Hermes-shaped local packet and acknowledgement verbs.
- The repo now also proves a threshold-verifier path in Foundry tests and versioned signer-set enforcement on the AegisLink side.
- The runtime and CLI already expose active signer-set state and signer-set history, so the trust model is inspectable instead of buried in code.
- The repo now includes Prometheus-style metrics, a local monitoring scaffold, and codified recovery drills, so operators can inspect and rehearse failure paths instead of only reading happy-path docs.
- The app runtime now serializes mutating access behind a single boundary and has focused race-smoke coverage, so concurrent reads, deposits, and policy updates are exercised explicitly instead of relying on unsynchronized keeper maps.
- The `ibcrouter` now supports destination route profiles, so multiple destinations, route-specific asset allowlists, and memo-policy guardrails can be modeled without rewriting the core bridge lifecycle.
- The repo now also includes a minimal governance module, so asset enablement, rate-limit updates, and route-policy updates can flow through an authority-gated recorded proposal path instead of direct keeper edits.
- The route layer now supports a second concrete action beyond swaps: profile-constrained `stake` actions can execute on the dual-runtime path with recipient and validator-path hints, while unsupported actions still fail through explicit destination receipts.
- The repo now also includes a first public-testnet scaffold for AegisLink, with a reproducible bootstrap script, operator bridge settings, and documented local RPC/gRPC endpoints for wallet-balance inspection.
- The repo now also proves a public-wallet bridge loop against a Sepolia-shaped deployment path: native ETH and ERC-20 deposits can mint balances to a real Bech32 wallet on AegisLink and redeem those balances back to Ethereum through the public relayer path.

## What is still a local harness

- AegisLink is now a store-backed single-node runtime with a daemon-style block loop, but it is still not a full networked CometBFT or ABCI chain.
- State is persisted in Cosmos KV stores, but the repo still does not claim live consensus, IAVL-backed network operation, or a full BaseApp block pipeline today.
- The destination side is now a bootstrapped local runtime with its own config, state, and local IBC link metadata, but it is still not a live IBC-Go or Hermes-connected Osmosis node.
- The public AegisLink testnet scaffold is still a single-validator local devnet bootstrap, not a hosted or externally peered public network yet.
- The route path is realistic enough to exercise packet lifecycle and destination execution through Hermes-shaped local commands, but it is still a controlled local environment rather than real proof-backed IBC transport.
- Public Osmosis wallet delivery is still only scaffolded. The repo now has an explicit `deploy/testnet/ibc` landing zone and a guarded e2e seam for Phase K, but it does not claim live Hermes or Osmosis testnet connectivity yet.

## Why this still matters

- The hard bridge logic is already real: message identity, replay protection, verification boundaries, bridge accounting, route policy, and failure handling.
- The project shows that the protocol was scoped correctly before chasing full chain realism.
- The local harness keeps the architecture honest while still proving the bridge loop, route lifecycle, and destination execution semantics end to end.

## How to describe it in review or interview settings

Use phrasing like:

- `AegisLink is a runtime-backed local bridge prototype with live Ethereum integration and a realistic routed execution harness.`
- `The current repository proves the bridge and route lifecycle end to end, and the Cosmos side now persists through a single-node SDK-store runtime, but it is still not a full networked chain.`
- `The public-wallet path now proves Sepolia-shaped deposit and redeem loops into a real Bech32 wallet on AegisLink, while public Osmosis delivery remains a future IBC step.`
- `The roadmap from here is deeper networked chain realism and fuller IBC-Go or Hermes realism, with the current daemon and dual-runtime seams acting as the transition layer.`

Avoid phrasing like:

- `fully trustless bridge`
- `real Cosmos chain today`
- `live IBC integration today`

## Future roadmap

The next realism steps are:

1. Push AegisLink from the current daemon-style single-node runtime toward a real networked CometBFT or BaseApp daemon.
2. Replace the current Hermes-shaped dual-runtime route bridge with fuller IBC-Go or Hermes-backed networking.
3. Validate the monitoring stack on a Docker-enabled machine and then deepen it with more production-style metrics or alerts.
4. Expand destination integrations or route-action breadth only after the deeper runtime and networking work is credible.

## Hardening now present

- The bridge keeper now has stronger replay and supply-conservation invariant coverage plus a persisted circuit-breaker path.
- The repo now also has property-style hardening coverage: a focused Foundry gateway invariant file plus Go fuzz coverage for supply-safety and route-refund state transitions.
- The route keeper now has explicit recoverable-refund state-machine coverage.
- The Ethereum gateway now depends on a narrow verifier interface, so the current v1 verifier can be swapped more cleanly for future threshold or light-client implementations.
- The bridge keeper now tracks versioned signer sets with activation and expiry, so attestation trust assumptions are explicit and queryable.
- The limits module now persists rolling-window usage, so rate limiting reflects cumulative bridge activity instead of a single-transfer ceiling.
- Demo-facing status summaries now expose failed claims and destination swap failures instead of only happy-path counts.

## Upgradeability stance

For v1, AegisLink keeps the Ethereum verifier/gateway path non-proxy on purpose. That makes the system easier to audit, easier to describe in interviews and reviews, and less dependent on upgrade authorities or storage-layout compatibility rules.

If the project ever adopts proxy-based upgradeability in a later version, the tradeoff would be explicit: more operational flexibility in exchange for more trust assumptions, more governance/process overhead, and more room for implementation drift. That is a valid future design space, but it is not part of the v1 positioning.
