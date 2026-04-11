# Public Bridge Ops

This runbook explains the first public-testnet bootstrap for AegisLink and how to inspect it safely.

## Scope

This is a reproducible single-validator public devnet scaffold. It is useful for:

- bootstrapping a wallet-query-capable AegisLink home
- loading operator bridge settings consistently
- rehearsing the first public wallet-delivery flow
- preparing Sepolia-shaped bridge deployment metadata and relayer wiring

It is not yet:

- a multi-validator public network
- live Sepolia delivery
- public IBC connectivity

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

Set your public-testnet RPC and signer locally, then produce bridge metadata:

```bash
AEGISLINK_SEPOLIA_RPC_URL=https://your-sepolia-rpc \
AEGISLINK_SEPOLIA_PRIVATE_KEY=0xyourprivatekey \
scripts/testnet/deploy_sepolia_bridge.sh

AEGISLINK_SEPOLIA_ERC20_ADDRESS=0xyourtoken \
scripts/testnet/register_bridge_assets.sh
```

That gives you:

- deployed verifier and gateway addresses in `deploy/testnet/sepolia/bridge-addresses.json`
- an ETH plus ERC-20 registry fixture in `deploy/testnet/sepolia/bridge-assets.json`

## Public relayer flow

Once the AegisLink home and Sepolia bridge metadata exist, the public relayer entrypoint is:

```bash
AEGISLINK_RELAYER_EVM_RPC_URL=https://your-sepolia-rpc \
AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS=0xyourverifier \
AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS=0xyourgateway \
AEGISLINK_RELAYER_AEGISLINK_CMD=go \
AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS="run ./chain/aegislink/cmd/aegislinkd --home /tmp/aegislink-public-home" \
go run ./relayer/cmd/public-bridge-relayer
```

This current repo scope is verified locally against Anvil-backed Sepolia-shaped deposits for:

- native ETH wallet delivery
- ERC-20 wallet delivery
- replay-safe reruns through the relayer replay store

## Intended endpoints

- RPC: `http://127.0.0.1:26657`
- gRPC: `127.0.0.1:9090`
- REST: `http://127.0.0.1:1317`

These are documented operator targets for the scaffold. The current bootstrap remains a local single-validator devnet, so treat these as local public-testnet-shaped endpoints rather than a hosted network promise.
