# AegisLink Security Audit

This document outlines the security vulnerabilities discovered during an architectural audit of the `stg` branch. Every finding below is traced to actual source code. 

Severity levels: 🔴 Critical, 🟠 High, 🟡 Medium, 🔵 Low, ⚪ Informational.

---

## 🔴 CRITICAL — No Withdrawal Authorization Check

**File:** `chain/aegislink/x/bridge/keeper/keeper.go` (line 203)
**File:** `chain/aegislink/app/app.go` (line 381)

**The Bug:** `ExecuteWithdrawal()` accepts a `signature` parameter but **never verifies it**. The signature bytes are stored in the `WithdrawalRecord` but no cryptographic check is performed to validate that the person requesting the withdrawal actually owns the tokens.

**Impact:** Anyone who can submit a transaction to the AegisLink chain can withdraw **anyone else's tokens**. The `ownerAddress` parameter in `app.go:ExecuteWithdrawal()` is used to debit the bank balance, but there's no proof that the caller actually controls that address.

**Fix:** Verify the signature against the `ownerAddress` before allowing the withdrawal. The signature should sign a message containing `(ownerAddress, assetID, amount, recipient, deadline)`, and the recovered signer must match `ownerAddress`.

---

## 🔴 CRITICAL — No Expiry Check on Withdrawal Deadline

**File:** `chain/aegislink/x/bridge/keeper/keeper.go` (line 203)

**The Bug:** The deposit flow correctly checks `if k.currentHeight > claim.Deadline` (line 15 of `verify_attestation.go`), but the withdrawal flow **never checks if the deadline has passed**. The `deadline` field is stored but ignored.

**Impact:** A withdrawal request with an expired deadline will still be processed. If a legitimate withdrawal is meant to expire, it won't.

**Fix:** Add `if k.currentHeight > deadline { return ErrFinalityWindowExpired }` to `ExecuteWithdrawal()`.

---

## 🟠 HIGH — Owner Address Not Checked on Chain-Side Withdrawal

**File:** `chain/aegislink/app/app.go` (line 381)

**The Bug:** `app.ExecuteWithdrawal()` accepts `ownerAddress` and debits their balance, but the bridge keeper's `ExecuteWithdrawal()` has no concept of an owner. The keeper burns from the global supply pool without verifying that the specific owner actually authorized the burn.

The debit happens *after* the burn. If the bank debit fails (insufficient balance), the app rolls back. But the bridge keeper's supply burn already happened in-memory.

**Impact:** The two-phase pattern (burn then debit, rollback on failure) works today but is error-prone. Any future code path that calls `BridgeKeeper.ExecuteWithdrawal()` directly (bypassing `app.go`) would burn supply without checking ownership.

---

## 🟠 HIGH — Rate Limiter Counts Failed Transactions

**File:** `chain/aegislink/x/bridge/keeper/keeper.go` (line 186)

**The Bug:** In `ExecuteDepositClaim()`, rate limit usage is recorded *before* the deposit is accepted (and before the invariant check runs). 

**Impact:** If the deposit is rejected by the invariant check (or any future check added after `RecordTransferAtHeight`), the rate limit window still consumed that capacity. An attacker could intentionally trigger invariant failures to fill up the rate limit window, effectively DoS-ing legitimate deposits.

**Fix:** Move `RecordTransferAtHeight` to *after* `acceptDepositClaim()` and the invariant check succeed.

---

## 🟡 MEDIUM — No Contract Upgradeability (No Proxy Pattern)

**Files:** `BridgeGateway.sol`, `ThresholdBridgeVerifier.sol`

**The Issue:** Both contracts are deployed as plain contracts with no proxy pattern (UUPS, Transparent Proxy, etc.). If a critical vulnerability is found post-deployment, the only option is to deploy a brand new contract and migrate all locked funds manually.

**Impact:** In a production scenario, this could lead to weeks of downtime during migration.

---

## 🟡 MEDIUM — Single Owner Key Controls Everything on Ethereum

**Files:** `BridgeGateway.sol` (line 34), `ThresholdBridgeVerifier.sol` (line 28)

**The Issue:** Both contracts use a single `owner` address with full control (pause/unpause, add/remove supported assets, rotate signers). There is no timelock, no multisig requirement, and no way to transfer ownership.

**Impact:** If the owner key is compromised, the attacker can pause the bridge, rotate signers to their own keys (stealing all future releases), or add malicious asset configs.

---

## 🟡 MEDIUM — Withdrawal Records Grow Unboundedly

**File:** `chain/aegislink/x/bridge/keeper/keeper.go` (line 250)

**The Issue:** Every withdrawal is appended to `k.withdrawals` (a slice) and never pruned. The invariant check iterates over ALL withdrawals on every deposit and withdrawal.

**Impact:** Over time, state grows linearly. After millions of withdrawals, both persistence and invariant checks become increasingly slow, eventually degrading chain performance.

---

## 🟡 MEDIUM — Processed Claims Map Also Grows Unboundedly

**File:** `chain/aegislink/x/bridge/keeper/keeper.go` (line 62)

Same pattern as withdrawals. `processedClaims` is a map that grows forever. The invariant check iterates over every single processed claim to recompute expected supply.

---

## 🔵 LOW — RLP Encoding Treats Negative BigInts as Zero

**File:** `relayer/internal/evm/rpc_release.go` (line 360)

Negative values silently encode as `0x80` (RLP empty bytes) instead of erroring. While negative values shouldn't occur in practice, silently encoding them as zero is dangerous and could mask bugs upstream.

---

## 🔵 LOW — No Duplicate Proposal ID Check in Governance

**File:** `chain/aegislink/x/governance/keeper/keeper.go` (line 155)

Governance proposals are appended to `k.applied` without checking if a proposal with the same `ProposalID` already exists. An admin could accidentally or intentionally apply the same proposal twice.

---

## ⚪ INFORMATIONAL — Auth Check is String Comparison, Not Crypto

**File:** `chain/aegislink/x/governance/keeper/auth.go` (line 21)

The `authorize()` function just checks if a string exists in a map. There's no cryptographic proof that the caller actually controls the authority address. This is fine for the current demo-node architecture but would be a critical gap in a real multi-validator production chain.

---

## Summary Table

| # | Severity | Finding | Location |
|:--|:---------|:--------|:---------|
| 1 | 🔴 Critical | No signature verification on withdrawals | `x/bridge/keeper/keeper.go:203` |
| 2 | 🔴 Critical | Withdrawal deadline never checked | `x/bridge/keeper/keeper.go:203` |
| 3 | 🟠 High | Owner address not cryptographically proven | `app/app.go:381` |
| 4 | 🟠 High | Rate limiter counts failed transactions | `x/bridge/keeper/keeper.go:186` |
| 5 | 🟡 Medium | No contract upgradeability | `BridgeGateway.sol` |
| 6 | 🟡 Medium | Single owner key, no timelock/multisig | `BridgeGateway.sol:34` |
| 7 | 🟡 Medium | Unbounded withdrawal storage growth | `x/bridge/keeper/keeper.go:250` |
| 8 | 🟡 Medium | Unbounded processed claims growth | `x/bridge/keeper/keeper.go:62` |
| 9 | 🔵 Low | RLP silently encodes negatives as zero | `rpc_release.go:360` |
| 10 | 🔵 Low | No duplicate proposal ID check | `governance/keeper/keeper.go:155` |
| 11 | ⚪ Info | Auth is string matching, not cryptographic | `governance/keeper/auth.go:21` |
