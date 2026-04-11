# AegisLink Public Testnet Scaffold

This directory holds the first reproducible public-testnet bootstrap assets for AegisLink.

- `operator.json` defines the bridge-facing runtime settings the operator should load.
- `network.json` documents the intended RPC, gRPC, and REST endpoints for the single-validator devnet bootstrap.

This scaffold is intentionally small:

- one validator
- one runtime home
- operator-facing bridge settings
- wallet balance query support

It is not yet a multi-validator public network.
