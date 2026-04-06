# AegisLink Local Runtime Config

This directory holds repo-owned defaults for the real-chain bootstrap path.

Current Phase 5 scope:
- `scripts/localnet/bootstrap_real_chain.sh` initializes a single-node AegisLink home
- the home uses `sdk-store-runtime`
- e2e tests seed bridge assets and limits after bootstrap, then exercise `aegislinkd` through `start`, `tx`, and `query`

This is still a single-node runtime harness, not a full CometBFT multi-process localnet yet.
