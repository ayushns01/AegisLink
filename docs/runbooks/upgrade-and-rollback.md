# AegisLink Upgrade and Rollback Runbook

This runbook covers controlled upgrades to AegisLink components and what to do if the release must be rolled back.

## Upgrade targets

AegisLink upgrades may affect:

- Ethereum gateway contracts
- attester configuration
- AegisLink chain modules
- relayer configuration or binaries
- IBC route configuration
- asset registry entries

## Pre-upgrade checklist

- [ ] Confirm the release scope and expected behavior changes.
- [ ] Confirm migration steps for any chain state changes.
- [ ] Confirm compatibility between contracts, relayer, and chain message formats.
- [ ] Confirm monitoring and alerting are active.
- [ ] Confirm pause authority is available if rollback is needed.
- [ ] Confirm a rollback package exists before rollout starts.

## Safe rollout pattern

Use a staged rollout whenever possible:

1. pause the affected surface if needed
2. apply the change to the smallest environment first
3. verify claim handling, registry queries, and route status
4. resume the affected surface gradually
5. watch alerts and logs before declaring the rollout stable

## Rollback triggers

Roll back if any of the following appears:

- unexpected claim rejection patterns
- unexpected accepted claims
- message format mismatch between components
- relayer submission failure spike
- contract event shape mismatch
- IBC route failures tied to the release

## Rollback checklist

- [ ] Pause the affected surface.
- [ ] Revert the changed binary, config, or contract path as planned.
- [ ] Confirm all components are back on the expected versions.
- [ ] Re-run the most critical health checks.
- [ ] Re-run the closest matching incident drill if rollback touched relayers, signer sets, or route behavior.
- [ ] Resume only after rejection and acceptance paths look normal.

## Minimum health checks after rollback

- bridge claim acceptance and rejection counters are stable
- no unexpected replay rejections
- relayer sees current Ethereum blocks
- AegisLink accepts valid claims again
- Osmosis route status is correct for enabled assets

## Release discipline

If the team cannot explain how to roll a change back, the change is not ready for production-style testing.

Use [Incident drills](incident-drills.md) to prove the rollback target still supports the expected recovery behavior after the rollback is complete.
