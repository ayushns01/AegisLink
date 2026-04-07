# AegisLink Security Model Summary

This is the operator-facing summary of AegisLink's v1 security posture. For the full trust and threat document, read [docs/architecture/02-security-and-trust-model.md](architecture/02-security-and-trust-model.md).

## Security position

AegisLink v1 is a `verifiable-relayer bridge with threshold attestations`.

That means:

- Ethereum is the source of canonical deposit and withdrawal events
- relayers provide evidence, not absolute truth
- the bridge zone enforces replay protection, rate limits, asset policy, and pause controls
- the system depends on the configured attester threshold and active signer set being honest and available
- the current repo now supports explicit signer-set versioning and rotation instead of assuming one fixed attester shape forever

## Security invariants

These rules should always hold:

- every accepted claim has one deterministic claim ID
- every mint, burn, and unlock path is auditable
- every supported asset exists in the registry before use
- every route to Osmosis is explicitly enabled
- every pause state is checked before minting, unlocking, or forwarding
- every supply transition can be explained by on-chain state and accepted claims

## What v1 does not guarantee

Do not claim:

- fully trustless Ethereum verification
- permissionless support for arbitrary tokens
- censorship resistance against a compromised attester majority
- instant Ethereum finality

## Main control surfaces

- `registry`
  Controls which assets and routes are enabled.
- `limits`
  Throttles bridge volume and route volume.
- `pauser`
  Stops sensitive flows during incidents.
- `claim replay store`
  Prevents double execution.
- `attester set management`
  Controls which signatures are valid evidence.
- `signer-set versioning`
  Makes attestations rejectable when they target an inactive, expired, or mismatched signer set.

## Launch questions

Before any external testnet or public review, the team should be able to answer:

- what is the finality threshold for the active Ethereum network
- which attester quorum is required
- which signer-set version is active
- which assets are enabled
- which routes to Osmosis are enabled
- what happens when Osmosis is unavailable
- what triggers a pause

## Related docs

- [System architecture](architecture/01-system-architecture.md)
- [Security and trust model](architecture/02-security-and-trust-model.md)
- [Verifier evolution](architecture/04-verifier-evolution.md)
- [Observability plan](observability.md)
- [Pause and recovery runbook](runbooks/pause-and-recovery.md)
- [Upgrade and rollback runbook](runbooks/upgrade-and-rollback.md)
