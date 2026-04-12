# Public IBC Assets

This directory is the landing zone for the optional Phase K work:

- real AegisLink -> Osmosis testnet connectivity
- Hermes configuration
- channel and client metadata for a public wallet-delivery path
- env-driven bootstrap settings for a future public IBC run

Current status:

- the checked-in assets still act as the stable landing zone for public IBC metadata and bootstrap config
- real Hermes is still not enabled by default
- the repo now does claim a live `AegisLink -> Osmosis` wallet-delivery proof through `rly` on the single-validator demo node
- the repo still does **not** claim the fully strict Sepolia-backed one-shot delivery path is finished yet

The checked-in example manifest still stays explicitly disabled by default so operators do not trigger live IBC actions accidentally.

Expected local bootstrap files:

- `.env.public-ibc.local` from [`.env.public-ibc.local.example`](../../.env.public-ibc.local.example)
- `osmosis-wallet-delivery.example.json` as the future delivery manifest
- `osmosis-wallet-delivery.json` as the operator-generated manifest written by `scripts/testnet/bootstrap_public_ibc.sh`

Bootstrap flow:

```bash
cp .env.public-ibc.local.example .env.public-ibc.local
set -a; source .env.public-ibc.local; set +a

scripts/testnet/bootstrap_public_ibc.sh
scripts/testnet/seed_public_ibc_route.sh /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd query route-profiles --home /tmp/aegislink-public-home
```

That bootstrap is intentionally honest:

- it turns the public bridge asset registry into a route-profile manifest for Osmosis-style delivery
- it seeds that route profile into the AegisLink runtime
- it still keeps `AEGISLINK_ENABLE_REAL_IBC=0` by default, so the manifest is documented and queryable without forcing live network actions during normal local setup

The env example stays disabled by default via `AEGISLINK_ENABLE_REAL_IBC=0` so it documents the operator flow without accidentally kicking off a live Osmosis or relayer run.
