# AegisLink Incident Drills

These drills turn the Phase 8 recovery paths into repeatable operator exercises. Each drill is meant to be run locally before external demos or production-style testing.

## Drill 1: relayer restart with replay persistence

Goal:
- prove that restarting `bridge-relayer` does not duplicate a previously processed deposit

Steps:
1. Seed one inbound deposit fixture and one vote set.
2. Run `bridge-relayer` once and confirm the deposit reaches the Cosmos outbox.
3. Run `bridge-relayer` a second time against the same replay store.
4. Inspect the replay store and outbox contents.

Expected evidence:
- the Cosmos outbox still contains only one submission
- the replay store keeps the processed replay key
- the replay store keeps the deposit checkpoint
- the second relayer run reports no new observed deposits after restart recovery

## Drill 2: timed-out route refund

Goal:
- prove that a timed-out outbound route can be recovered safely with an explicit refund

Steps:
1. Bootstrap an AegisLink runtime with an enabled Osmosis route.
2. Initiate one outbound IBC-style transfer.
3. Run `route-relayer` against a destination target in `timeout` mode.
4. Confirm the transfer reaches `timed_out`.
5. Run `aegislinkd tx refund-ibc-transfer`.

Expected evidence:
- the transfer first moves into `timed_out`
- no destination execution is recorded for the timeout path
- the refund action moves the transfer into `refunded`
- status queries show the transfer as recoverable first, then refunded

## Drill 3: paused asset recovery

Goal:
- prove that operators can reject risky flow while paused and resume safely afterward

Steps:
1. Seed a valid inbound deposit claim and attestation.
2. Pause the affected asset on the AegisLink runtime.
3. Attempt the deposit claim and confirm rejection.
4. Unpause the asset.
5. Re-run the same valid claim.

Expected evidence:
- the first attempt is rejected with an asset-paused error
- the second attempt is accepted after the pause is removed
- operator notes record when the pause started and when it was lifted

## Drill 4: signer-set mismatch rejection

Goal:
- prove that signer-set drift is rejected until operators align the attestation version with the active signer set

Steps:
1. Seed a valid inbound claim and attestation using signer-set version `1`.
2. Rotate the active signer set on AegisLink to version `2`.
3. Attempt the version-`1` attestation and confirm rejection.
4. Update the attestation to signer-set version `2`.
5. Re-run the claim.

Expected evidence:
- the first attempt fails with a signer-set version mismatch
- the second attempt succeeds after the signer-set version is corrected
- `aegislinkd query signer-set` and `query signer-sets` explain the active version and history during the drill

## Drill cadence

Run these drills:
- before public demos that exercise failure handling
- after relayer or verifier configuration changes
- before any production-style testnet rehearsal

If the team cannot run these drills end to end, the operator story is not ready.
