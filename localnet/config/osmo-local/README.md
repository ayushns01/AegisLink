# Osmo Local Runtime

This directory documents the Phase 6 local destination-chain runtime.

Current scope:
- `scripts/localnet/bootstrap_destination_chain.sh` initializes an `osmo-locald` home
- the runtime persists packet, execution, pool, balance, and ack state locally
- the Phase 6 "real" route path uses `route-relayer` plus `osmo-locald`, not the old HTTP mock target

This is a local destination-chain runtime milestone, not a full Osmosis or IBC-Go network yet.
