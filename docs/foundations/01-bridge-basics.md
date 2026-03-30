# Bridge Basics

This document explains what a bridge is, why bridges are hard, and how to think about AegisLink without getting lost in buzzwords.

## What a bridge does

A blockchain bridge moves value or verified intent from one chain to another. It does not magically teleport tokens. It coordinates state changes across two systems that do not natively share state.

In practice, a bridge usually does one or more of these things:

- lock an asset on the source chain and mint a representation on the destination chain
- burn a representation on the source chain and unlock the canonical asset on the destination chain
- transmit a verified cross-chain message that tells the destination chain what happened elsewhere

The hard part is not transferring a token. The hard part is making the destination chain trust that the source-chain event really happened.

## The real question: who or what is trusted

Every bridge has a trust model. Senior engineers care about this more than the UI.

Common models:

- `single operator`
  One relayer or admin signs off on transfers. Easy to build, weak to trust.
- `multisig or committee`
  A group of signers attests to events. Better than one key, but still depends on operator honesty and key security.
- `optimistic verification`
  A claim is accepted unless challenged during a dispute window.
- `light-client verification`
  The destination chain verifies the source chain more directly through headers and proofs.

The more a bridge depends on human-controlled signers, the less trust-minimized it is. The more it depends on direct proof verification, the stronger it is in principle, but the harder it is to build.

## Common bridge asset models

### Lock and mint

The source-chain asset is locked in escrow. A wrapped or represented version is minted on the destination chain.

Good for:

- canonical assets that must stay native on the source chain
- simple one-direction bridge flows

Risks:

- escrow contract compromise
- wrapped asset accounting mistakes

### Burn and release

A representation asset is burned on one side so the canonical asset can be released on the other side.

Good for:

- reversing a lock-and-mint flow
- preventing supply inflation

Risks:

- replay bugs
- release logic bugs

### Burn and mint

An issuer-controlled asset can be burned on one chain and minted on another.

Good for:

- controlled asset deployments
- cleaner multi-chain supply management

Risks:

- issuer or mint authority compromise
- broken supply accounting

### Message passing

Instead of thinking only in terms of tokens, the bridge carries a verified message such as "this deposit happened" or "this burn is final."

Good for:

- protocol-shaped bridge design
- future extensibility

Risks:

- message replay
- bad destination handlers
- overcomplicated routing

## Why bridges fail

Bridges usually fail for one of four reasons:

- `bad trust assumptions`
  The system quietly depends on a small set of keys or operators.
- `broken accounting`
  Supply or escrow logic gets out of sync across chains.
- `replay or uniqueness bugs`
  A valid event is accepted more than once.
- `operational failure`
  Keys, relayers, monitoring, or pause procedures fail when the system is under stress.

Many hacked bridges were not broken because "cross-chain is impossible." They were broken because the verification and operational boundaries were weak.

## What makes AegisLink different

AegisLink is stronger than a toy bridge because it is built around a `bridge zone`:

- Ethereum is the source of canonical events.
- A custom Cosmos-SDK chain becomes the place where claims are verified, recorded, and policy-gated.
- Only after the asset exists safely on the bridge zone does it move onward to Osmosis through IBC.

This gives the system a clear shape:

- Ethereum side: canonical asset entry and exit
- AegisLink: verification, accounting, replay protection, limits, pause controls
- Osmosis: real downstream utility

That shape is important. It means you solve the hardest cross-ecosystem boundary once, then reuse Cosmos-native interoperability for the rest.

## Why the v1 trust model is still respectable

AegisLink v1 is not pretending to be fully trustless. It uses a `verifiable-relayer` model with threshold attestations.

That is still respectable because:

- the assumptions are explicit
- the replay and accounting rules are on-chain
- the modules are separated cleanly
- the design leaves room for a stronger Ethereum light-client verifier later

What senior reviewers dislike is not an honest v1 trust model. What they dislike is a weak system being marketed as stronger than it really is.

## What to learn before implementing

If you are building AegisLink from scratch, learn these in order:

1. how Ethereum contract events work
2. how Cosmos-SDK modules store and transition state
3. how IBC moves assets between Cosmos chains
4. how replay protection and nonce design work
5. how bridge accounting prevents inflation
6. how operators pause and recover a bridge safely

After that, move into the architecture docs and the implementation roadmap.
