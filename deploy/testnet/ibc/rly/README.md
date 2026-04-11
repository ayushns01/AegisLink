# `rly` Bootstrap Scaffold

This directory holds the next-step relayer bootstrap assets for the real AegisLink demo node.

Current scope:

- generate reproducible `rly` config from the public IBC manifest
- keep destination-chain inputs chain-registry-compatible
- store generated files under `deploy/testnet/ibc/rly/generated/`

Current non-goal:

- this does **not** claim live packet relay or live Osmosis delivery yet

Bootstrap flow:

```bash
cp .env.public-ibc.local.example .env.public-ibc.local
set -a; source .env.public-ibc.local; set +a

scripts/testnet/bootstrap_public_ibc.sh
scripts/testnet/bootstrap_rly_path.sh
```

Generated outputs:

- `generated/config.yaml`
- `generated/paths/<path-name>.json`

These are operator-friendly bootstrap artifacts for the next local packet-lifecycle milestone. They are intentionally generated from manifest and metadata inputs instead of hardcoding chain values in multiple places.
