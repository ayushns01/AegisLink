# AegisLink Step-by-Step Roadmap

AegisLink is an Ethereum-to-Cosmos interoperability layer. Phase 1 builds a custom Cosmos-SDK bridge zone that can verify Ethereum-originated actions with a practical security model. Phase 2 extends that bridge zone so assets can move onward to Osmosis over IBC for swaps and liquidity.

## Current Checkpoint

As of April 5, 2026, the repository has already completed the foundation through the latest route-execution milestone:

- repo bootstrap and shared message model
- chain safety modules for registry, limits, and pause control
- bridge verification and accounting state machine on the Cosmos side
- Ethereum gateway and verifier contracts
- relayer pipeline with durable replay state, command-backed AegisLink integration, and RPC-backed Ethereum observation and release execution
- end-to-end proof of deposit, mint, burn, and release across the local bridge loop
- `ibcrouter` route management on the AegisLink side
- runtime query and tx surfaces for route initiation, completion, failure, timeout, and refund
- a dedicated `route-relayer` and `mock-osmosis-target` service pair for local routed-transfer handoff
- packetized local route delivery with asynchronous acknowledgement handling
- destination-side packet receipts, denom-trace-style metadata, recipient balances, configurable multi-pool swap execution records, fee-aware pricing, and execution-driven failure handling on the mock Osmosis target
- route-focused end-to-end tests, including a live Ethereum deposit that becomes a completed Osmosis-style transfer record through that local target

The next roadmap milestone stays inside `Phase 5: Route Assets To Osmosis`, but now the focus is deeper realism again: move from the current local route target into a fuller local IBC or Osmosis harness. A parallel hardening milestone is still recommended before or alongside that work: deepen the AegisLink runtime from a persistent shell into a fuller Cosmos node experience.

## How The System Fits Together

Think of AegisLink as four cooperating parts:

1. Ethereum contracts emit and verify bridge events.
2. A Cosmos-SDK bridge zone receives attestations and mints or unlocks assets on the Cosmos side.
3. A relayer watches both chains, collects threshold signatures, and submits bridge messages.
4. Safety controls protect the system with replay protection, rate limits, pause switches, and an asset registry.

The first version does not need a perfect Ethereum light client. It needs a clear trust model, a deterministic asset lifecycle, and tests that prove each safety rule is enforced.

## Terms To Know

- Bridge zone: the custom Cosmos chain that is the home of AegisLink state and controls.
- Attestation: a signed statement that enough relayers agree on an Ethereum event or Cosmos action.
- Replay protection: a way to reject the same bridge message twice.
- Asset registry: the allowlist and metadata store for assets AegisLink supports.
- Pause controls: emergency switches that stop minting, unlocking, or outbound transfers.
- IBC routing: the mechanism that lets assets move from the bridge zone to Osmosis.

## Phase 0: Understand The Problem Before Building

Start with the smallest possible mental model.

- Verify what "bridge" means for this project: lock and mint, burn and release, or a hybrid flow.
- Write down which side is the source of truth for each asset.
- Identify which behaviors must be trusted in v1 and which ones are deferred to v2.
- Decide which chain actions are forbidden when the system is paused.

Checklist:

- [ ] Define the supported asset flow for ETH to Cosmos in v1.
- [ ] Define the reverse flow for Cosmos to Ethereum in v1.
- [ ] Write the trust assumptions in plain language.
- [ ] Write the failure modes that must halt the system.

## Phase 1: Build The Foundation

This phase creates the structure that every later milestone depends on.

Milestone 1: Repo and local development basics

- Create the repository layout for the Cosmos chain, Ethereum contracts, relayer, and tests.
- Set up consistent formatting and linting rules.
- Add local development commands for booting the chain, an Ethereum devnet, and the relayer.
- Make sure a new engineer can clone the repo and run one command to see the system start.

Checklist:

- [ ] Add the root workspace files.
- [ ] Add the chain, contract, and relayer directories.
- [ ] Add a `make` command for formatting, testing, and local dev.
- [ ] Add a documented local bootstrap path.

Milestone 2: Shared types and message shape

- Define the protobuf messages and event payloads used across chains.
- Include message IDs, chain IDs, nonces, asset IDs, and expiration fields.
- Decide which fields are signed, hashed, or derived.
- Keep the message format stable so relayers and chain modules do not drift.

Checklist:

- [ ] Define protobuf schemas for bridge requests and attestations.
- [ ] Define typed errors for invalid asset, invalid signature, expired proof, and replayed message.
- [ ] Add tests for message encoding and decoding.
- [ ] Confirm every message has a unique replay key.

## Phase 2: Ship The Bridge Zone Core

This is the first major implementation milestone. The chain must be able to accept approved bridge actions and reject unsafe ones.

Milestone 3: Asset registry

- Support registration of assets that AegisLink is allowed to bridge.
- Store denomination metadata, decimals, origin chain, and status.
- Reject unknown or disabled assets.
- Keep the registry simple and auditable.

Checklist:

- [ ] Add asset registration and update messages.
- [ ] Add tests for duplicate registration and invalid metadata.
- [ ] Add tests for disabling an asset.

Milestone 4: Replay protection and rate limits

- Track every processed bridge event by nonce or message hash.
- Reject duplicate processing on both inbound and outbound paths.
- Add per-asset and per-route limits.
- Use clear error messages so operators can understand rejections.

Checklist:

- [ ] Add a replay store keyed by message identity.
- [ ] Add a rate-limit module for inbound and outbound actions.
- [ ] Add tests for duplicate submissions and over-limit behavior.

Milestone 5: Pause controls and emergency operations

- Allow governance or an authorized admin path to pause sensitive flows.
- Separate pause states for minting, unlocking, and routing if needed.
- Make pause behavior visible in query endpoints and logs.

Checklist:

- [ ] Add pause and unpause messages.
- [ ] Add tests that prove paused flows are rejected.
- [ ] Add operator queries for current pause state.

## Phase 3: Add The Ethereum Side

Ethereum should expose a small, reviewable surface area rather than a large contract suite.

Milestone 6: Bridge contracts

- Implement the Ethereum contracts that emit bridge events and validate the attestation workflow.
- Keep the contract surface narrow so audits are easier.
- Use explicit role separation for admins, pausers, and verifiers.

Checklist:

- [ ] Add the core bridge gateway contract.
- [ ] Add the attestation verifier contract or verifier library.
- [ ] Add tests for accepted, rejected, expired, and replayed proofs.
- [ ] Add pause tests for the contract side.

Milestone 7: Relayer

- Build the service that watches Ethereum, gathers threshold attestations, and submits signed bridge messages.
- Add backoff, retries, and durable checkpointing.
- Make the relayer idempotent so restarts do not duplicate work.
- In the current repository, the relayer already runs against the persistent AegisLink runtime and live Anvil-backed Ethereum paths in end-to-end tests. File-backed adapters remain as lower-fidelity fallbacks for focused local fixtures.

Checklist:

- [ ] Add event watchers for the Ethereum bridge contracts.
- [ ] Add Cosmos transaction submission paths.
- [ ] Add checkpoint storage for processed events.
- [ ] Add relayer tests for restart and duplicate-event handling.

## Phase 4: Prove The Full Loop

The project should not move forward until the full bridge round-trip works in local development.

Milestone 8: End-to-end localnet

- Start a local Ethereum devnet, an AegisLink node, and the relayer together.
- Demonstrate a full inbound bridge flow.
- Demonstrate a full outbound bridge flow.
- Verify logs and state transitions at each step.

Checklist:

- [ ] Add an end-to-end test harness.
- [ ] Add a scripted local demo for the happy path.
- [ ] Add a scripted local demo for duplicate and paused paths.

Current status:

- implemented for the full local bridge loop, including live Ethereum deposit observation and live Ethereum release execution in e2e tests

## Phase 5: Route Assets To Osmosis

Only after the bridge zone is stable should assets be routed further through IBC.

Current status:

- implemented: route allowlist, pending and completed route state, acknowledgement-failure state, timeout state, refund state, and CLI query or tx handling for those transitions
- implemented: a dedicated route-relayer and mock target that drive route completion or recovery instead of the main flow completing routes by hand
- implemented: local routed-flow proof from live Ethereum deposit to completed AegisLink-side Osmosis-style transfer record
- still pending: a fuller local IBC channel or Osmosis node path instead of the current local target harness

Milestone 9: IBC plumbing

- Connect the bridge zone to Osmosis with the intended channel configuration.
- Define which bridged assets can move onward to Osmosis.
- Preserve the same safety posture while crossing IBC.

Checklist:

- [ ] Add IBC channel configuration beyond the current route metadata model.
- [ ] Add transfer and acknowledgement tests against a fuller local IBC or Osmosis environment.
- [ ] Confirm asset metadata survives the route.

Milestone 10: Osmosis liquidity and swap flow

- Decide the first Osmosis use case: swap, liquidity provision, or both.
- Add operational controls so the routing path can be limited or paused separately.
- Validate that the user-facing path is simple.

Checklist:

- [ ] Add the first Osmosis routing flow.
- [ ] Add tests for failed acknowledgements and rollback behavior.
- [ ] Add documentation for the supported user journey.

## Phase 6: Harden For Recruiter-Grade Quality

This phase is about making the project look and feel like serious infrastructure.

- Write a threat model and trust-assumption document.
- Add observability for relayer health, bridge volume, and rejected messages.
- Add a release checklist with upgrade steps and rollback steps.
- Document exactly which guarantees v1 provides and which ones wait for v2.

Final checklist:

- [ ] Write a clear security model document.
- [ ] Add monitoring and alerting notes.
- [ ] Add runbooks for pause, recovery, and upgrade.
- [ ] Add a v2 roadmap for the Ethereum light-client verifier.
