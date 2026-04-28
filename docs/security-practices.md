# AegisLink Security Practices

This document summarizes the security practices that are already represented in the AegisLink repository. It is intentionally implementation-focused: each item maps to code, tests, runbooks, or architecture docs that exist in this project.

For the trust model and limits of the design, read [Security model summary](security-model.md) and [Security and trust model](architecture/02-security-and-trust-model.md).

## Security posture

AegisLink v1 is positioned as a verifiable-relayer bridge with threshold attestations. The project does not claim fully trustless Ethereum verification, permissionless asset support, instant source-chain finality, or production-ready public bridge operation.

The security approach is based on explicit boundaries:

- Ethereum emits canonical bridge events.
- Relayers observe and submit evidence, but they are not treated as the sole source of truth.
- AegisLink enforces bridge accounting, registry policy, replay protection, rate limits, pause controls, signer-set checks, and route state transitions.
- Osmosis delivery is treated as an IBC route with acknowledgement, timeout, and refund handling rather than a fire-and-forget transfer.

## Practices implemented

### Explicit verifier boundary

The Ethereum gateway depends on the narrow verifier interface in [`IBridgeVerifier.sol`](../contracts/ethereum/IBridgeVerifier.sol). This keeps release verification separate from custody logic and makes the single-attester and threshold-verifier paths swappable without changing the gateway surface.

Implemented in:

- [`BridgeGateway.sol`](../contracts/ethereum/BridgeGateway.sol)
- [`BridgeVerifier.sol`](../contracts/ethereum/BridgeVerifier.sol)
- [`ThresholdBridgeVerifier.sol`](../contracts/ethereum/ThresholdBridgeVerifier.sol)
- [`docs/architecture/04-verifier-evolution.md`](architecture/04-verifier-evolution.md)

### Typed attestation digests and signature checks

Ethereum-side verification uses typed-data-style digests that bind the message ID, payload hash, expiry, chain ID, and verifying contract. Signature recovery rejects invalid `v` values, zero-address recovery, and non-low-`s` signatures.

The threshold verifier also enforces signer-set version matching, signer membership, duplicate-signer rejection, and minimum threshold before consuming a proof.

Implemented in:

- [`BridgeVerifier.sol`](../contracts/ethereum/BridgeVerifier.sol)
- [`ThresholdBridgeVerifier.sol`](../contracts/ethereum/ThresholdBridgeVerifier.sol)
- [`tests/e2e/attestation_crypto_test.go`](../tests/e2e/attestation_crypto_test.go)

### Replay protection and proof consumption

Replay prevention exists on both sides of the bridge:

- Ethereum verifier contracts mark consumed proofs by `messageId`.
- AegisLink derives deterministic claim replay keys and rejects duplicate claims before minting.
- Relayers persist checkpoints and processed keys, so a restart does not resubmit the same observed event.

Implemented in:

- [`BridgeVerifier.sol`](../contracts/ethereum/BridgeVerifier.sol)
- [`ThresholdBridgeVerifier.sol`](../contracts/ethereum/ThresholdBridgeVerifier.sol)
- [`chain/aegislink/x/bridge/keeper/keeper.go`](../chain/aegislink/x/bridge/keeper/keeper.go)
- [`relayer/internal/replay/store.go`](../relayer/internal/replay/store.go)
- [`tests/e2e/recovery_drill_test.go`](../tests/e2e/recovery_drill_test.go)

### Finality and expiry checks

Claims and attestations carry deadlines or expiries. The AegisLink keeper rejects stale deposit claims and expired attestations before accepting bridge state changes. Ethereum releases also verify expiry before proof consumption.

Implemented in:

- [`BridgeGateway.sol`](../contracts/ethereum/BridgeGateway.sol)
- [`BridgeVerifier.sol`](../contracts/ethereum/BridgeVerifier.sol)
- [`ThresholdBridgeVerifier.sol`](../contracts/ethereum/ThresholdBridgeVerifier.sol)
- [`chain/aegislink/x/bridge/keeper/verify_attestation.go`](../chain/aegislink/x/bridge/keeper/verify_attestation.go)

### Signer-set versioning and rotation

AegisLink stores versioned signer sets with activation height, optional expiry height, threshold, and normalized signer addresses. Attestations must target the active signer-set version and meet the active threshold.

Implemented in:

- [`chain/aegislink/x/bridge/keeper/signer_set.go`](../chain/aegislink/x/bridge/keeper/signer_set.go)
- [`chain/aegislink/x/bridge/keeper/verify_attestation.go`](../chain/aegislink/x/bridge/keeper/verify_attestation.go)
- [`chain/aegislink/x/bridge/keeper/signer_set_test.go`](../chain/aegislink/x/bridge/keeper/signer_set_test.go)
- [`tests/e2e/recovery_drill_test.go`](../tests/e2e/recovery_drill_test.go)

### Asset registry and custody validation

Assets must be registered and enabled before they can move through the bridge. The Ethereum gateway rejects unsupported assets, zero amounts, empty recipients, expired deposits, and non-canonical token behavior by checking balance deltas during ERC-20 deposit and release flows.

Implemented in:

- [`BridgeGateway.sol`](../contracts/ethereum/BridgeGateway.sol)
- [`chain/aegislink/x/registry/keeper/keeper.go`](../chain/aegislink/x/registry/keeper/keeper.go)
- [`chain/aegislink/x/bridge/keeper/keeper.go`](../chain/aegislink/x/bridge/keeper/keeper.go)
- [`contracts/ethereum/test/BridgeGateway.invariant.t.sol`](../contracts/ethereum/test/BridgeGateway.invariant.t.sol)

### Pause controls

The project includes pause controls at multiple layers:

- Ethereum gateway deposits and releases are blocked while paused.
- AegisLink can pause sensitive asset or flow paths through the pauser keeper.
- Recovery docs define when to pause and how to resume safely.

Implemented in:

- [`BridgeGateway.sol`](../contracts/ethereum/BridgeGateway.sol)
- [`chain/aegislink/x/pauser/keeper/keeper.go`](../chain/aegislink/x/pauser/keeper/keeper.go)
- [`docs/runbooks/pause-and-recovery.md`](runbooks/pause-and-recovery.md)
- [`tests/e2e/recovery_drill_test.go`](../tests/e2e/recovery_drill_test.go)

### Rolling-window rate limits

Bridge volume limits are persisted as rolling-window usage records instead of only checking a single transfer amount. This means cumulative transfer activity inside the active window is tracked before accepting additional claims.

Implemented in:

- [`chain/aegislink/x/limits/keeper/keeper.go`](../chain/aegislink/x/limits/keeper/keeper.go)
- [`chain/aegislink/x/limits/keeper/usage.go`](../chain/aegislink/x/limits/keeper/usage.go)
- [`chain/aegislink/x/limits/keeper/keeper_test.go`](../chain/aegislink/x/limits/keeper/keeper_test.go)
- [`tests/e2e/recovery_drill_test.go`](../tests/e2e/recovery_drill_test.go)

### Bridge accounting invariant and circuit breaker

AegisLink checks that accepted claims minus withdrawals match tracked bridged supply. If accounting state is corrupted or inconsistent, the keeper trips a persistent circuit breaker and rejects new bridge actions until operators investigate.

Implemented in:

- [`chain/aegislink/x/bridge/keeper/accounting.go`](../chain/aegislink/x/bridge/keeper/accounting.go)
- [`chain/aegislink/x/bridge/keeper/invariants.go`](../chain/aegislink/x/bridge/keeper/invariants.go)
- [`chain/aegislink/x/bridge/keeper/fuzz_test.go`](../chain/aegislink/x/bridge/keeper/fuzz_test.go)
- [`tests/e2e/recovery_drill_test.go`](../tests/e2e/recovery_drill_test.go)

### Release reentrancy guard

The Ethereum gateway uses a dedicated release reentrancy guard around withdrawal release execution. This protects the custody release path from token callback reentry during outbound transfer handling.

Implemented in:

- [`BridgeGateway.sol`](../contracts/ethereum/BridgeGateway.sol)

### Governance authority checks

Runtime policy changes are gated by configured governance authorities. Asset-status changes, limit updates, and route-policy changes record the authority that applied them instead of being direct ungated keeper edits.

Implemented in:

- [`chain/aegislink/x/governance/keeper/auth.go`](../chain/aegislink/x/governance/keeper/auth.go)
- [`chain/aegislink/x/governance/keeper/keeper.go`](../chain/aegislink/x/governance/keeper/keeper.go)
- [`chain/aegislink/x/governance/keeper/auth_test.go`](../chain/aegislink/x/governance/keeper/auth_test.go)
- [`tests/e2e/governance_auth_test.go`](../tests/e2e/governance_auth_test.go)

### IBC acknowledgement, timeout, and refund paths

Routed transfers are tracked through packet-shaped delivery, acknowledgement, timeout, and refund states. Timeout handling is explicit, and recovery drills verify that a timed-out route can move into a recoverable/refunded state.

Implemented in:

- [`chain/aegislink/x/ibcrouter/keeper/ibc_ack.go`](../chain/aegislink/x/ibcrouter/keeper/ibc_ack.go)
- [`chain/aegislink/x/ibcrouter/keeper/ibc_timeout.go`](../chain/aegislink/x/ibcrouter/keeper/ibc_timeout.go)
- [`chain/aegislink/x/ibcrouter/keeper/fuzz_test.go`](../chain/aegislink/x/ibcrouter/keeper/fuzz_test.go)
- [`docs/runbooks/incident-drills.md`](runbooks/incident-drills.md)

### Observability and incident response

The repo includes Prometheus-style metrics, runtime status surfaces, run summaries, operator runbooks, and e2e recovery drills. These make failure modes inspectable instead of leaving recovery as an informal process.

Implemented in:

- [`docs/observability.md`](observability.md)
- [`docs/runbooks/incident-drills.md`](runbooks/incident-drills.md)
- [`docs/runbooks/pause-and-recovery.md`](runbooks/pause-and-recovery.md)
- [`docs/runbooks/upgrade-and-rollback.md`](runbooks/upgrade-and-rollback.md)
- [`chain/aegislink/internal/metrics/metrics.go`](../chain/aegislink/internal/metrics/metrics.go)
- [`relayer/internal/metrics/metrics.go`](../relayer/internal/metrics/metrics.go)

## Verification coverage

Security-relevant behavior is covered across multiple test layers:

- Foundry contract tests and invariant coverage for gateway custody accounting.
- Go keeper tests for bridge verification, signer-set lifecycle, registry, limits, pauser, governance, and IBC route behavior.
- Go fuzz coverage for bridge supply safety and route refund transitions.
- End-to-end recovery drills for relayer restart replay persistence, timed-out route refund, paused asset recovery, signer-set mismatch rejection, rolling-window limit recovery, and circuit-breaker persistence.

Useful commands:

```bash
forge test --offline
go test ./chain/aegislink/...
go test ./relayer/...
cd tests/e2e && go test ./...
```

## Current limits

These practices improve the project, but they do not remove the v1 trust assumptions:

- The bridge still depends on the configured attester threshold.
- The current public demo path is not positioned as production-ready.
- The AegisLink runtime is not yet a fully networked production Cosmos chain.
- Ethereum verification is not yet a light-client proof system.
- Public repeated-run delivery still needs more long-lived backend hardening before it should be described as operationally reliable.

