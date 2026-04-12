# `rly` Bootstrap Scaffold

This directory holds the next-step relayer bootstrap assets for the real AegisLink demo node.

Current scope:

- generate reproducible `rly` config from the public IBC manifest
- keep destination-chain inputs chain-registry-compatible
- store generated files under `deploy/testnet/ibc/rly/generated/`

Current status:

- these bootstrap assets are now strong enough to support a real live `rly` path against the single-validator AegisLink demo node and Osmosis testnet
- the repo has now proved live packet relay and live Osmosis wallet delivery through that path
- the generated files are still bootstrap inputs, not permanent checked-in client/connection/channel ids

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
