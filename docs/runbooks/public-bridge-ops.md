# Public Bridge Ops

This runbook explains the first public-testnet bootstrap for AegisLink and how to inspect it safely.

## Scope

This is a reproducible single-validator public devnet scaffold. It is useful for:

- bootstrapping a wallet-query-capable AegisLink home
- loading operator bridge settings consistently
- rehearsing the first public wallet-delivery flow
- rehearsing the first public redeem-back-to-Sepolia flow
- preparing Sepolia-shaped bridge deployment metadata and relayer wiring

It is not yet:

- a multi-validator public network
- real public IBC connectivity to Osmosis

## Bootstrap

```bash
scripts/testnet/bootstrap_aegislink_testnet.sh /tmp/aegislink-public-home
```

That creates:

- runtime home in `/tmp/aegislink-public-home`
- copied operator config in `/tmp/aegislink-public-home/config/operator.json`
- copied network config in `/tmp/aegislink-public-home/config/network.json`

## Start and inspect

```bash
go run ./chain/aegislink/cmd/aegislinkd start --home /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd query status --home /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd query balances --home /tmp/aegislink-public-home --address <bech32-wallet>
```

## Sepolia bridge scaffold

Start from the checked-in env examples:

```bash
cp .env.sepolia.deploy.local.example .env.sepolia.deploy.local
cp .env.public-bridge.local.example .env.public-bridge.local
```

Those `*.local` files are ignored by git, so keep your real keys there and do not commit them.

Set your public-testnet RPC and signer locally, then produce bridge metadata:

```bash
set -a; source .env.sepolia.deploy.local; set +a
scripts/testnet/deploy_sepolia_bridge.sh

set -a; source .env.sepolia.deploy.local; set +a
scripts/testnet/register_bridge_assets.sh

scripts/testnet/seed_public_bridge_assets.sh /tmp/aegislink-public-home
```

That gives you:

- deployed verifier and gateway addresses in `deploy/testnet/sepolia/bridge-addresses.json`
- an ETH-only or ETH-plus-ERC-20 registry fixture in `deploy/testnet/sepolia/bridge-assets.json`
- a bootstrapped AegisLink home whose asset registry and limits match that bridge fixture

If `AEGISLINK_SEPOLIA_ERC20_ADDRESS` is unset, the registry and seed step only load native ETH. That is the simplest first live path.

## Public relayer flow

Once the AegisLink home and seeded Sepolia bridge metadata exist, the public relayer entrypoint is:

```bash
set -a; source .env.public-bridge.local; set +a
go run ./relayer/cmd/public-bridge-relayer
```

This current repo scope is verified locally against Anvil-backed Sepolia-shaped deposits for:

- native ETH wallet delivery
- ERC-20 wallet delivery
- native ETH redeem back to Sepolia
- ERC-20 redeem back to Sepolia
- replay-safe reruns through the relayer replay store

If you want the relayer to execute Sepolia release transactions during redeem, set one of these locally before sourcing `.env.public-bridge.local`:

- `AEGISLINK_RELAYER_EVM_RELEASE_SIGNER_PRIVATE_KEY`
- `AEGISLINK_RELAYER_EVM_RELEASE_PRIVATE_KEY`

The `...SIGNER_...` form is the canonical name in the codebase. The shorter alias exists so older local env files still work.

## Phase K scaffold

The future public Osmosis-delivery path has a separate, disabled-by-default env file:

```bash
cp .env.public-ibc.local.example .env.public-ibc.local
set -a; source .env.public-ibc.local; set +a
```

Bootstrap the manifest and seed the AegisLink route profile like this:

```bash
scripts/testnet/bootstrap_public_ibc.sh
scripts/testnet/seed_public_ibc_route.sh /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd query route-profiles --home /tmp/aegislink-public-home
```

Once the profile exists, the current repo can also prove the profile-based initiation path locally:

```bash
go run ./chain/aegislink/cmd/aegislinkd tx initiate-ibc-transfer \
  --home /tmp/aegislink-public-home \
  --route-id osmosis-public-wallet \
  --asset-id eth \
  --amount 1000000000000000 \
  --receiver osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8 \
  --timeout-height 120
```

That file and bootstrap flow are only scaffolds for the future public IBC path. They keep:

- `AEGISLINK_ENABLE_REAL_IBC=0` until a live Hermes or IBC-Go path exists
- local AegisLink and Osmosis home placeholders
- a future route-relayer command shape
- a manifest pointer to `deploy/testnet/ibc/osmosis-wallet-delivery.json`

Use it to document the eventual public IBC bootstrap, not to claim live Osmosis wallet delivery today.

## Intended endpoints

- RPC: `http://127.0.0.1:26657`
- gRPC: `127.0.0.1:9090`
- REST: `http://127.0.0.1:1317`

These are documented operator targets for the scaffold. The current bootstrap remains a local single-validator devnet, so treat these as local public-testnet-shaped endpoints rather than a hosted network promise.
