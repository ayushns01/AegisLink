# AegisLink Cross-Chain Bridge — Security Review

**Date:** 2026-04-28  
**Scope:** Full codebase audit (`chain/`, `contracts/`, `relayer/`, `deploy/`)  
**Severity scale:** Critical → High → Medium → Low → Informational

---

## Summary

The bridge architecture is well-structured with multi-signature attestation, rate limiting, and a pause mechanism. However, several vulnerabilities range from critical (funds can be double-spent or fully stolen) to low (operational hardening gaps). They are listed below in descending severity.

---

## CRITICAL

### 1. No On-Chain Replay Protection — Double-Spend Risk

**File:** `chain/aegislink/x/bridge/keeper/verify_attestation.go`

`verifyDepositClaim` verifies signatures and quorum but never checks whether a `MessageID` has already been successfully processed. The only replay guard lives off-chain in `relayer/internal/replay/store.go` (a JSON file). This means:

- A second relayer instance, a restart after crash, or corruption of `replay-store.json` will re-submit the same deposit claim.
- The chain will accept it again, minting/releasing tokens a second time — a classic double-spend.

**Fix:** Maintain a committed, on-chain set of consumed `MessageID`s. Before finalizing a claim, assert the ID is absent; after finalizing, insert it. The set can be pruned after `Expiry` height.

---

### 2. Trivially Known Hardcoded Private Keys in Production Source

**File:** `chain/aegislink/x/bridge/types/proof.go` (lines 23–30)

```go
var defaultHarnessSignerPrivateKeys = []string{
    "0000000000000000000000000000000000000000000000000000000000000001",
    "0000000000000000000000000000000000000000000000000000000000000002",
    ...
    "0000000000000000000000000000000000000000000000000000000000000006",
}
```

These are the secp256k1 private keys for integers 1–6 — the first keys any attacker would try. They are exported via `DefaultHarnessSignerAddresses()` and `DefaultHarnessSignerPrivateKeys()`, making them trivially available. If a staging or testnet deployment is initialized with these signer addresses (a common shortcut), an attacker can forge any attestation, bypassing the entire multi-sig model and draining the bridge.

**Fix:** Remove these from the production package entirely. Move them to a `_test` package or an internal `testutil` package with a build tag that cannot be imported from production paths.

---

## HIGH

### 3. Ambiguous Signing Digest — Hash Collision Attack

**File:** `chain/aegislink/x/bridge/types/proof.go` (lines 67–74)

```go
payload := strings.Join([]string{
    "aegislink.attestation.v1",
    strings.TrimSpace(a.MessageID),
    strings.ToLower(strings.TrimSpace(a.PayloadHash)),
    fmt.Sprintf("%d", a.Threshold),
    fmt.Sprintf("%d", a.Expiry),
    fmt.Sprintf("%d", a.SignerSetVersion),
}, "|")
```

The `|` pipe is used as a delimiter but is **not escaped** from the fields. A `MessageID` containing `|` produces the same digest as a different `(MessageID, PayloadHash)` pair. For example:

- `MessageID = "abc|0xdeadbeef"`, `PayloadHash = "xyz"` → same digest as
- `MessageID = "abc"`, `PayloadHash = "0xdeadbeef|xyz"`

An attacker who can influence message IDs can craft collisions, potentially reusing a valid quorum signature for a different payload.

**Fix:** Either length-prefix each field (`<len>:<value>`) or use a canonical serialization format like ABI encoding or protobuf. Never use an unescaped delimiter on untrusted inputs.

---

### 4. Governance Proposals Have No Idempotency / Replay Protection

**File:** `chain/aegislink/x/governance/keeper/keeper.go`

`ApplyAssetStatusProposal`, `ApplyLimitUpdateProposal`, and `ApplyRoutePolicyUpdateProposal` do not check whether a `ProposalID` has already been applied. An authorized governance address can submit the same `ProposalID` repeatedly, executing the state change multiple times. For limit updates with a cumulative effect, or for enable/disable toggling, this is a meaningful exploit surface.

**Fix:** Before applying, check `k.applied` for an existing record with the same `ProposalID` and return an error if found. The applied list is already persisted, so this check is straightforward.

---

### 5. File-Based Vote Source Trusts Unsigned Signer Claims

**File:** `relayer/internal/attestations/file_source.go`

`FileVoteSource.Votes()` reads votes from a plain JSON file and returns the `Signer` field verbatim with no cryptographic verification:

```go
votes = append(votes, Vote{
    Signer: vote.Signer,   // trusted without signature check
    Expiry: vote.Expiry,
})
```

Any process that can write to the attestation file path can inject a vote for any signer address, including active quorum members. Combined with the bridge relayer constructing attestations from these votes, this effectively allows forging multi-sig attestations by file manipulation alone.

**Fix:** Each vote in the file should include a signature over `(MessageID, PayloadHash, Expiry)`. The vote source must verify this signature against the claimed `Signer` address before returning the vote.

---

### 6. `WindowSeconds` Used as Block Count — Rate Limit Bypass

**File:** `chain/aegislink/x/limits/keeper/usage.go` (line 103)

```go
if atHeight >= usage.WindowStart + limit.WindowSeconds {
```

`WindowSeconds` is added to a block height (`atHeight`). Block height is dimensionless (a count of blocks), not seconds. If the chain produces one block every 6 seconds and an operator configures `WindowSeconds: 3600` (intending a 1-hour window), the actual window is 6 hours. Conversely, a fast chain makes the window much shorter, potentially allowing rate-limit bypass.

**Fix:** Rename the field to `WindowBlocks` and document the unit clearly, or convert seconds to blocks using a known block time constant. Either way, ensure the type and unit are consistent throughout.

---

## MEDIUM

### 7. Case-Sensitive Duplicate Check in `SignerSet.ValidateBasic`

**File:** `chain/aegislink/x/bridge/keeper/signer_set.go` (lines 36–48)

`ValidateBasic` checks for duplicate signers using the raw (case-preserved) string, but `normalizeSignerSet` later lowercases all addresses. This means `"0xABCD"` and `"0xabcd"` pass the duplicate check in `ValidateBasic` but collapse to the same address after normalization, effectively reducing the signer set size and potentially lowering the effective threshold without detection.

**Fix:** Normalize (lowercase + trim) signer addresses inside `ValidateBasic` before inserting into the `seen` map, consistent with `NormalizeSignerAddress`.

---

### 8. No HTTPS Enforcement on HTTP Transfer Target

**File:** `relayer/internal/route/http_target.go`

`NewHTTPTarget` accepts any `baseURL`, including plain `http://`. Transfer data and acknowledgment records are sent over the network in cleartext if a non-TLS URL is configured. An on-path attacker can read or modify transfer payloads.

**Fix:** Validate that `baseURL` starts with `https://` at construction time, or reject insecure schemes. For internal network deployments, document the TLS requirement explicitly.

---

### 9. No Size Limit on Attestation Proof Array — DoS Risk

**File:** `chain/aegislink/x/bridge/keeper/verify_attestation.go`

The `verifyDepositClaim` iterates over every entry in `attestation.Proofs` doing an ECDSA recovery per entry. There is no cap on the number of proofs a submitter can include. A malicious relayer can submit thousands of proofs, causing the chain node to spend excessive CPU per transaction — a vector for resource exhaustion / DoS.

**Fix:** Enforce `len(attestation.Proofs) <= len(activeSignerSet.Signers)` in `ValidateBasic` (or in the keeper before verification), since extra proofs beyond the signer set size are useless.

---

## LOW

### 10. Default Grafana Credentials (`admin` / `admin`)

**File:** `docker-compose.yml` (lines 89–90)

```yaml
GF_SECURITY_ADMIN_USER: admin
GF_SECURITY_ADMIN_PASSWORD: admin
```

If the monitoring stack is deployed to any non-local environment without rotating these credentials, the Grafana dashboard is publicly accessible. Dashboards expose operational metrics that help an attacker time attacks around low-liquidity windows or relay delays.

**Fix:** Remove hardcoded credentials from `docker-compose.yml`. Require secrets to be injected via environment variables or a secrets manager at deployment time.

---

### 11. Replay Store Written to `/tmp` by Default

**File:** `relayer/internal/replay/store.go` (line 35)

```go
path = filepath.Join(os.TempDir(), defaultStoreFile)
```

If no explicit path is configured, the replay deduplication store lands in `/tmp`, which is typically ephemeral and may be wiped on reboot or by the OS. Losing this file removes the off-chain replay guard, and — given the missing on-chain replay protection (issue #1) — would allow all past deposits to be re-submitted.

**Fix:** Require an explicit `REPLAY_STORE_PATH` configuration value; fail fast if it is absent rather than silently defaulting to `/tmp`.

---

## Informational

- **`IBridgeVerifier.sol`** is an interface only — the concrete verifier implementation is not present in this repository. Ensure the implementation correctly guards against signature malleability (non-canonical `s` values in ECDSA) and validates the `expiry` parameter on-chain.
- **`ibcTransferSequence`** parses sequence numbers from a string-split of `transferID`. Malformed IDs (e.g., empty string, no numeric suffix) return `ErrInvalidTransfer`, which is correct, but the error path should be tested under fuzzing given IDs originate from external chains.
- **Missing rate-limit enforcement before IBC dispatch in `ibcrouter`** — confirm that `CheckTransferAtHeight` is always called and cannot be bypassed via the IBC module's callback path.

---

## Severity Summary

| # | Title | Severity |
|---|-------|----------|
| 1 | No on-chain replay protection (double-spend) | **Critical** |
| 2 | Hardcoded sequential private keys in production source | **Critical** |
| 3 | Ambiguous signing digest — hash collision via `\|` delimiter | **High** |
| 4 | Governance proposals lack idempotency / replay protection | **High** |
| 5 | File-based vote source trusts unsigned signer claims | **High** |
| 6 | `WindowSeconds` used as block count — rate limit bypass | **High** |
| 7 | Case-sensitive signer duplicate check in `ValidateBasic` | **Medium** |
| 8 | No HTTPS enforcement on HTTP transfer target | **Medium** |
| 9 | No size limit on proof array — DoS via ECDSA loop | **Medium** |
| 10 | Default Grafana `admin/admin` credentials | **Low** |
| 11 | Replay store defaults to ephemeral `/tmp` | **Low** |
