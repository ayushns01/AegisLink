# AegisLink Verifier Evolution

This document explains what verifier model AegisLink has today, what is already replaceable, and what still belongs to future work.

## Current boundary

The Ethereum gateway depends on the narrow verifier interface in [IBridgeVerifier.sol](../../contracts/ethereum/IBridgeVerifier.sol).

That matters because the gateway does not need to know whether a proof came from:

- one attester
- a threshold signer set
- some future proof system

The repo now includes two concrete verifier paths:

- [BridgeVerifier.sol](../../contracts/ethereum/BridgeVerifier.sol)
  This is the simpler v1-compatible verifier.
- [ThresholdBridgeVerifier.sol](../../contracts/ethereum/ThresholdBridgeVerifier.sol)
  This adds threshold enforcement, duplicate-signer rejection, signer-set versioning, and signer rotation.

On the AegisLink side, attestations now carry `signer_set_version`, and bridge verification checks:

- active signer-set lookup
- activation height
- optional expiry height
- signer-set version mismatch

That logic lives in [signer_set.go](../../chain/aegislink/x/bridge/keeper/signer_set.go) and [verify_attestation.go](../../chain/aegislink/x/bridge/keeper/verify_attestation.go).

## What is replaceable today

These parts are already replaceable without redesigning the whole bridge:

- the concrete Ethereum verifier behind `IBridgeVerifier`
- the active signer set and signer-set history on AegisLink
- the relayer-produced attestation payload, as long as it keeps message ID, payload hash, expiry, and signer-set version semantics

This is a good boundary for the current stage of the repo.

## What is still coupled today

Some coupling is still intentional:

- the relayer still assumes a threshold-attestation workflow
- the AegisLink keeper still reasons about named signers and quorum
- the proof blob passed to the gateway is still verifier-specific

So the verifier boundary is meaningfully cleaner now, but it is not yet a generic proof system.

## Near-term path: threshold attestation

This is the strongest path already implemented in code.

What it gives:

- multiple signers instead of one
- threshold enforcement
- explicit signer-set versioning
- signer rotation without changing the gateway interface

What it does not give:

- trustless Ethereum verification
- censorship resistance against a compromised signer threshold
- removal of relayer-trust assumptions

This is the right honest near-term security story for the current repository.

## Medium-term path: optimistic verification

An optimistic bridge direction would mean:

- provisional acceptance
- a challenge window
- a fraud or slash path on invalid claims

Why it is interesting:

- lower verification cost on Ethereum
- less dependence on a fixed signer threshold

Why it is not built here yet:

- it needs dispute semantics, incentives, and fraud-proof design
- the current repo does not model challenge games or bonded operators

This is future design work, not a current feature.

## Longer-term path: Ethereum light client

A light-client path would aim to replace signer-trust assumptions with direct proof verification tied to Ethereum consensus.

What it would change:

- relayers become delivery agents rather than trust-bearing attesters
- AegisLink would verify Ethereum state or receipts more directly
- the trust story shifts from signer honesty to light-client correctness and update liveness

What it would still require:

- careful light-client design
- acceptable verification cost
- stronger update and recovery flows

This is the most trust-minimizing direction, but also the most complex one.

## Honest recommendation

For this repo, the right order is:

1. keep the threshold verifier as the realistic hardening path
2. keep signer-set lifecycle explicit and inspectable
3. treat optimistic and light-client verification as documented future evolution, not implied current features

That keeps the project honest while still showing a believable security roadmap.
