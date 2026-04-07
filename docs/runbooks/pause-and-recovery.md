# AegisLink Pause and Recovery Runbook

This runbook explains when to pause AegisLink, what to check during the incident, and how to recover safely.

## When to pause

Pause the bridge when any of the following happens:

- invalid claims appear accepted or close to acceptance
- relayer keys are suspected compromised
- replay protection behaves unexpectedly
- asset registry data is incorrect
- rate-limit behavior is broken
- the Osmosis route is failing in a way that risks funds or accounting confusion

## Pause scope

AegisLink should support targeted pauses when possible:

- pause deposits only
- pause withdrawals only
- pause specific assets
- pause routing to Osmosis only
- pause the full bridge if the issue is systemic

Use the smallest pause that removes risk, but choose a full pause if the blast radius is unclear.

## Immediate response checklist

- [ ] Confirm the alert or anomaly with logs and state queries.
- [ ] Identify the affected asset, route, and claim IDs.
- [ ] Pause the smallest safe scope.
- [ ] Record the incident start time and operator responsible.
- [ ] Stop automated retries if they are making the state noisier.
- [ ] Preserve logs, metrics, and signer evidence.

Use the fastest inspection surfaces first:

- `aegislinkd query status`
  Confirms current chain ID, runtime paths, signer threshold, processed claim count, and enabled routes.
- bridge-relayer structured logs
  Confirm whether deposits or withdrawals were observed, retried, and completed during the last run.
- route-relayer structured logs
  Confirm whether the route is stuck before delivery, after delivery, or after acknowledgement became ready.
- mock target `/status`, `/packets`, and `/executions`
  Confirm whether the destination actually received the packet, executed it, or produced a failure or timeout outcome.
- [Incident drills](incident-drills.md)
  Use the codified restart, timeout-refund, pause, and signer-set mismatch drills when the team needs to rehearse or validate recovery steps before resuming service.

## Investigation checklist

- [ ] Determine whether the issue is verification, accounting, routing, or infrastructure.
- [ ] Check whether any invalid claim was actually executed.
- [ ] Check whether any supported asset needs to be disabled in the registry.
- [ ] Check whether the attester set or relayer config changed recently.
- [ ] Check whether IBC acknowledgements or timeouts are involved.

Practical triage split:

- `verification or accounting issue`
  Look at `aegislinkd` status plus bridge-relayer summaries first.
- `route issue`
  Look at route-relayer summaries plus mock target packet or execution state first.
- `timeout or refund issue`
  Confirm whether the transfer is still `ack_ready` on the target or already `timed_out` on AegisLink before taking recovery action.

## Recovery checklist

Only resume when the root cause is understood and the control path is tested.

- [ ] Fix or isolate the root cause.
- [ ] Validate the fix in a local or staging environment if possible.
- [ ] Reconfirm pause scope and affected routes.
- [ ] Confirm the registry and attester set are correct.
- [ ] Re-enable flows gradually.
- [ ] Monitor accepted and rejected claims closely after resume.

## Post-incident output

Every pause should produce:

- a short incident summary
- affected assets and routes
- claim IDs involved
- root cause
- mitigation taken
- follow-up engineering task list

If the team cannot produce this output, the pause process is incomplete.

Use [Incident drills](incident-drills.md) after the incident when a fix changes operator behavior, so the same failure path is rehearsed before the next rollout.
