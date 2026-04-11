# Public IBC Scaffold

This directory is the landing zone for the optional Phase K work:

- real AegisLink -> Osmosis testnet connectivity
- Hermes configuration
- channel and client metadata for a public wallet-delivery path
- env-driven bootstrap settings for a future public IBC run

Current status:

- the scaffold exists so operators have a stable place for future public IBC assets
- real Hermes or IBC-Go connectivity is **not** enabled by default
- the repo does **not** claim public Osmosis wallet delivery is live yet

The checked-in example manifest stays explicitly disabled until a real public IBC path is wired and verified.

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
- it still keeps `AEGISLINK_ENABLE_REAL_IBC=0` by default, so the manifest is documented and queryable without pretending live Hermes or Osmosis connectivity already exists

The env example stays disabled by default via `AEGISLINK_ENABLE_REAL_IBC=0` so it documents the future operator flow without accidentally implying a live Osmosis or Hermes deployment.
