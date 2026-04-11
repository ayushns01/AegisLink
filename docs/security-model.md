# AegisLink Security Model Summary

This is the operator-facing summary of AegisLink's v1 security posture. For the full trust and threat document, read [docs/architecture/02-security-and-trust-model.md](architecture/02-security-and-trust-model.md).

## Security position

AegisLink v1 is a `verifiable-relayer bridge with threshold attestations`.

That means:

- Ethereum is the source of canonical deposit and withdrawal events
- the Ethereum verifier/gateway path is intentionally immutable and non-proxy in v1, so the first release keeps the trust surface simple and avoids upgrade-admin complexity
- Ethereum attestation verification now uses typed-data-style digests with low-`s` signature enforcement, and the gateway release path rejects reentrant token callbacks
- relayers provide evidence, not absolute truth
- the bridge zone enforces replay protection, rolling-window rate limits, asset policy, and pause controls
- the system depends on the configured attester threshold and active signer set being honest and available
- the current repo now supports explicit signer-set versioning, cryptographic signer proofs, and rotation instead of assuming one fixed attester shape forever
- governance policy changes now require a configured authority instead of applying permissionlessly inside the runtime
- bridge accounting invariants can now trip a circuit breaker that blocks new flow when tracked supply no longer matches accepted claims and withdrawals

## Security invariants

These rules should always hold:

- every accepted claim has one deterministic claim ID
- every mint, burn, and unlock path is auditable
- every public redeem burns the bridged representation before release and leaves a retryable withdrawal record until Sepolia release succeeds
- every supported asset exists in the registry before use
- every route to Osmosis is explicitly enabled
- every pause state is checked before minting, unlocking, or forwarding
- every supply transition can be explained by on-chain state and accepted claims
- every bridge window limit accumulates usage over time instead of only checking a single transfer amount

## What v1 does not guarantee

Do not claim:

- fully trustless Ethereum verification
- permissionless support for arbitrary tokens
- censorship resistance against a compromised attester majority
- instant Ethereum finality
- live public Osmosis wallet delivery today
- proxy-based contract upgradeability in v1

## Main control surfaces

- `registry`
  Controls which assets and routes are enabled.
- `limits`
  Throttles bridge volume and route volume with persisted rolling-window usage.
- `pauser`
  Stops sensitive flows during incidents.
- `bridge circuit breaker`
  Rejects new bridge activity after an accounting invariant failure until operators investigate and recover.
- `claim replay store`
  Prevents double execution.
- `attester set management`
  Controls which signatures are valid evidence.
- `signer-set versioning`
  Makes attestations rejectable when they target an inactive, expired, or mismatched signer set.
- `governance authorities`
  Gate asset-policy, limit, and route-policy changes behind explicit operator identities.

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

## Upgradeability stance

v1 keeps the Ethereum verifier/gateway path non-proxy on purpose. That choice makes the release easier to reason about, keeps the on-chain trust model explicit, and avoids introducing upgrade-admin or proxy-brick risk before the system has a stronger operational track record.

If a future version ever adopts proxy-based upgradeability, it would change the trust and engineering model in concrete ways: the team would need upgrade authority, storage-layout compatibility rules, initializer/versioning discipline, and a clearer process for deciding when implementation changes are safe. That tradeoff can be reasonable later, but it belongs to a future version rather than v1.
