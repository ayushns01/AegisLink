# AegisLink Observability Plan

Bridge infrastructure fails badly when operators cannot tell the difference between a stuck relayer, an invalid claim, a paused route, and a real security incident. This document defines the minimum observability layer AegisLink should have before external testing.

## Goals

- detect relayer lag and submission failure quickly
- explain why a claim was accepted or rejected
- measure bridge volume and route usage
- make pause, recovery, and upgrade decisions evidence-driven

## What to measure

### Relayer health

- latest observed Ethereum block
- latest finalized Ethereum block considered safe
- number of unsigned observations waiting for quorum
- number of pending submissions to AegisLink
- retry count by claim ID
- relayer process restarts

### Bridge-zone health

- accepted claims
- rejected claims by reason code
- replay rejections
- rate-limit rejections
- pause-state changes
- registry updates

### IBC and Osmosis routing

- number of outbound IBC transfers
- packet receive count
- packet execution count
- acknowledgement success and failure counts
- timed-out transfers
- per-asset route volume
- per-route pause events
- destination-side pool count, swap count, execution receipt count, and credited balance count during demo or localnet runs

## Metrics to expose first

- `aegislink_relayer_observed_block`
- `aegislink_relayer_submission_retries_total`
- `aegislink_claims_accepted_total`
- `aegislink_claims_rejected_total`
- `aegislink_claim_replay_rejections_total`
- `aegislink_rate_limit_rejections_total`
- `aegislink_pause_events_total`
- `aegislink_ibc_routes_total`
- `aegislink_ibc_route_failures_total`
- mock target `/status` summary plus `/packets` and `/executions` for local demo and inspection runs

## Logs that matter

Logs should always include:

- claim ID
- source chain ID
- source tx hash
- log index or nonce
- asset ID
- route ID
- decision result
- rejection reason when applicable

If logs do not carry claim identity, incident handling becomes much slower.

## Dashboards to prepare

- `bridge health`
  Relayer lag, pending claims, submission retries, paused state.
- `safety controls`
  Replay rejections, rate-limit events, registry changes, attester set changes.
- `routing`
  AegisLink to Osmosis route volume, failures, timeouts, and active channels.
- `demo target state`
  Mock target `/status`, `/packets`, `/executions`, `/pools`, `/balances`, and `/swaps` during recruiter demos or local debugging.

## Alert ideas

- relayer has stopped observing new Ethereum blocks
- claim retries exceed threshold
- replay rejections spike unexpectedly
- rate-limit rejections spike unexpectedly
- Osmosis route failures or timeouts exceed threshold
- pause flag changes outside maintenance windows

## Operator rule

If an alert cannot be mapped to a runbook action, the observability plan is incomplete. Every important alert should point to either the pause-and-recovery or the upgrade-and-rollback runbook.
