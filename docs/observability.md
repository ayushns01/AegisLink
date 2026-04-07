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

## Metrics surfaces available now

The repo now has the first real Prometheus-style inspection surfaces:

- `aegislinkd query metrics`
  Exposes processed claims, failed claims, pending transfers, and timed-out transfers from the runtime snapshot.
- mock target `/metrics`
  Exposes destination packet, execution, ready-ack, and swap-failure counts as Prometheus text.
- `bridge-relayer` and `route-relayer` with `AEGISLINK_PRINT_METRICS=1`
  Emit a one-shot Prometheus text snapshot after a run so operators can capture worker metrics during local testing or scripts.

## Structured logs available now

The current repo now emits structured JSON logs for the main operator surfaces:

- `aegislinkd`
  - `runtime_init`
  - `runtime_start`
  - `runtime_status`
  - `command_failed`
- `bridge-relayer`
  - `run_start`
  - `run_complete`
  - `run_failed`
- `route-relayer`
  - `run_start`
  - `run_complete`
  - `run_failed`
- `mock-osmosis-target`
  - `server_start`
  - `server_stopped`
  - `startup_failed`

These logs now carry the fields that matter during local operation:

- chain ID
- home dir and runtime paths
- configured signer count and required threshold
- enabled route IDs
- deposit and withdrawal run summary counts
- route acknowledgement and delivery summary counts
- mock target address, mode, and pool count

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

## Run summaries available now

The operator-facing binaries now emit short summaries instead of only succeeding silently:

- `aegislinkd start`
  Reports chain ID, home dir, module count, signer count, and enabled routes.
- `bridge-relayer`
  Reports deposit observations, duplicate suppression, submit attempts, withdrawal observations, and release attempts.
- `route-relayer`
  Reports ready acknowledgements, completed or failed acknowledgements, observed transfers, delivered transfers, and received-only deliveries.
- `mock-osmosis-target`
  Reports startup mode, listen address, persisted state path, and pool count.

The current demo-facing status surfaces also expose richer hardening counters:

- `aegislinkd query status`
  Includes `processed_claims`, `failed_claims`, `pending_transfers`, `failed_transfers`, and `timed_out_transfers`.
- mock target `/status`
  Includes acknowledgement counts plus `swap_failures` so destination execution failures are visible without opening execution receipts first.

## Dashboards to prepare

- `bridge health`
  Relayer lag, pending claims, submission retries, paused state.
- `safety controls`
  Replay rejections, rate-limit events, registry changes, attester set changes.
- `routing`
  AegisLink to Osmosis route volume, failures, timeouts, and active channels.
- `demo target state`
  Mock target `/status`, `/packets`, `/executions`, `/pools`, `/balances`, and `/swaps` during recruiter demos or local debugging.

## Local monitoring stack

The repo now includes the first local monitoring scaffold:

- [deploy/monitoring/prometheus.yml](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/deploy/monitoring/prometheus.yml)
  Prometheus scrape config for the destination `/metrics` endpoint.
- [deploy/monitoring/grafana/provisioning/datasources/prometheus.yml](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/deploy/monitoring/grafana/provisioning/datasources/prometheus.yml)
  Provisioned Prometheus datasource.
- [deploy/monitoring/grafana/provisioning/dashboards/dashboards.yml](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/deploy/monitoring/grafana/provisioning/dashboards/dashboards.yml)
  Dashboard loader configuration.
- [deploy/monitoring/grafana/dashboards/aegislink-overview.json](/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/deploy/monitoring/grafana/dashboards/aegislink-overview.json)
  First operator dashboard for destination packets, executions, ready acknowledgements, and swap failures.

Use `make monitor` to bring up Prometheus, Grafana, and the destination target together.

## Alert ideas

- relayer has stopped observing new Ethereum blocks
- claim retries exceed threshold
- replay rejections spike unexpectedly
- rate-limit rejections spike unexpectedly
- Osmosis route failures or timeouts exceed threshold
- pause flag changes outside maintenance windows

## Operator rule

If an alert cannot be mapped to a runbook action, the observability plan is incomplete. Every important alert should point to either the pause-and-recovery or the upgrade-and-rollback runbook.
