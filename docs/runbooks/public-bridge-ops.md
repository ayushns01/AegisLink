# Public Bridge Ops

This runbook explains the first public-testnet bootstrap for AegisLink and how to inspect it safely.

## Scope

This is a reproducible single-validator public devnet scaffold. It is useful for:

- bootstrapping a wallet-query-capable AegisLink home
- loading operator bridge settings consistently
- rehearsing the first public wallet-delivery flow

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

## Intended endpoints

- RPC: `http://127.0.0.1:26657`
- gRPC: `127.0.0.1:9090`
- REST: `http://127.0.0.1:1317`

These are documented operator targets for the scaffold. The current bootstrap remains a local single-validator devnet, so treat these as local public-testnet-shaped endpoints rather than a hosted network promise.
