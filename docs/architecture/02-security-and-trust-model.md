# AegisLink Security and Trust Model

## Purpose

This document states what AegisLink v1 does and does not trust. The goal is to be precise enough for engineering review, audits, and recruiter assessment without overstating the security properties.

## Trust Assumptions

### Ethereum

- Ethereum consensus provides the canonical source of deposit and withdrawal events.
- AegisLink v1 waits for a configurable finality window before accepting an Ethereum claim.
- The system assumes the configured finality threshold is appropriate for the chain conditions at the time of operation.

### Bridge relayers and attesters

- Relayers are not trusted individually.
- A claim is accepted only after the required threshold of attestations is present and valid.
- The attesting set is assumed to be configured correctly and to maintain the operational threshold expected by the protocol.
- If the attestation quorum is compromised, the system can be forced to accept invalid claims. That is a core v1 trust assumption, not a hidden guarantee.

### Cosmos-SDK bridge zone

- The bridge zone chain is trusted to enforce its own state machine, store claim uniqueness, and honor pause or rate-limit settings.
- The bridge zone validator set is trusted according to the normal security assumptions of the chain itself.

### Osmosis and IBC

- The bridge zone relies on standard IBC security for transfers to Osmosis.
- Once assets are on the bridge zone, the IBC channel and light-client machinery determine the security of the route to Osmosis.

## Threat Model

AegisLink should be designed against the following threats:

- forged deposit or withdrawal claims
- replay of a previously accepted claim
- Ethereum reorgs that invalidate a previously observed event
- malicious or buggy relayers
- attester collusion below the intended threshold
- misconfigured asset metadata or decimals
- unauthorized asset addition
- stale or duplicated claim submission
- rate abuse or liquidity draining
- pause bypass attempts
- IBC packet replay, timeout, or channel misrouting
- key compromise of relayer infrastructure
- operational failure such as delayed observations or partial signer outages

## Failure Modes and Mitigations

### 1. Ethereum reorg after observation

Mitigation:

- require a finality threshold before a claim is eligible
- record the source block and log index
- reject claims outside the configured finality window

Residual risk:

- a sufficiently deep reorganization relative to the chosen threshold can still invalidate the assumption

### 2. Replay of the same deposit

Mitigation:

- derive a deterministic claim key from chain ID, source tx hash, log index, and asset metadata
- store claim status in the bridge zone state machine
- reject any claim already marked processed, failed-terminal, or superseded

### 3. Forged or tampered attestation

Mitigation:

- verify the threshold signature or aggregate proof against an allowlisted attester set
- version attestation keys and rotate them by governance or protocol upgrade
- reject claims that do not satisfy the quorum and freshness checks

### 4. Unauthorized mint or unlock

Mitigation:

- only the bridge module may transition claim state into mint or release
- all mint/unlock paths must pass pause, limit, and registry checks
- use invariant tests to prove supply-accounting consistency

### 5. Stale asset metadata

Mitigation:

- version asset registry entries
- require activation heights for new registry values
- never infer decimals or denom mapping from user input

### 6. Abuse of routing to Osmosis

Mitigation:

- route only assets that are explicitly enabled
- apply per-asset and per-route rate limits
- keep the Osmosis path dependent on a live, initialized IBC channel

### 7. Operational outage

Mitigation:

- keep pause controls available to halt new mint or route actions
- separate observation, signing, and chain submission duties
- design relayers so a single instance outage does not block recovery

### 8. Key compromise

Mitigation:

- assume relayer keys are hot-wallet keys and keep funds exposure minimal
- use threshold attestations instead of a single key
- support key rotation and signer set updates

## What v1 Does Not Claim

AegisLink v1 should not claim any of the following:

- fully trustless Ethereum verification
- censorship resistance against a majority of attesters or the bridge zone validator set
- instant finality on Ethereum deposits
- support for arbitrary tokens without registration
- support for arbitrary message passing between Ethereum and Cosmos
- immunity to governance misconfiguration
- a permissionless attester set unless and until the protocol explicitly adds one

The security story is narrower and stronger: a verifiable-relayer bridge with explicit assumptions and clear operational controls.

## Security Properties to Preserve

- Every accepted claim must be uniquely identified.
- Every mint, burn, and unlock path must be auditable.
- Every supported asset must have an explicit registry entry.
- Every route to Osmosis must be policy-gated.
- Every emergency path must be faster than the normal claim path.
- Every acceptance decision should be explainable from on-chain state and relayer evidence.

## Audit-Oriented Questions

Before launch, the team should be able to answer:

- What is the exact finality threshold for each source chain?
- Which attester set is authorized for each asset or route?
- What proof format is accepted by the bridge module?
- How is replay protection encoded in state?
- What happens if the pause flag is raised mid-flow?
- What happens if Osmosis is unavailable during IBC routing?
- How does the protocol prevent supply inflation across bridge-zone and source-chain representations?

## Recommended Security Positioning

Use the following phrasing in external-facing materials:

- "AegisLink v1 is a verifiable-relayer bridge with threshold attestations."
- "AegisLink enforces replay protection, asset registration, rate limits, and pause controls."
- "AegisLink v2 is planned to replace relayer trust with an Ethereum light client verification path."

Avoid claiming that v1 is trustless or fully light-client verified. That would overstate the design.
