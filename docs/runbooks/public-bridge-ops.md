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
- a production-grade repeated-run service; the live frontend-to-Osmosis path is strongest on a fresh backend launch, and repeat-run hardening is still ongoing

## Fast Start

For the repeatable local operator flow, you can now start a fresh backend stack with one command:

```bash
bash scripts/testnet/start_public_bridge_backend.sh
```

That launcher will:

- stop the older local demo-node and public bridge relayer processes
- create a fresh AegisLink home under `/tmp`
- reuse a persistent relayer home under `$HOME/.aegislink-live-rly` by default
- seed bridge assets and the public Osmosis route profile
- start the demo node
- create the required `rly` keys if they do not exist yet
- create and link a fresh AegisLink <-> Osmosis testnet path
- update the route profile to the live channel
- start the public bridge relayer with auto-delivery enabled
- lift stale auto-delivery timeout heights against the current Osmosis testnet LCD height before submitting packets

On the very first run, if the Osmosis relayer key is not funded yet, the launcher now prints the generated `osmo1...` address and exits cleanly. Fund that address with testnet OSMO once, then rerun the same command. After that, the backend path is a single command.

If your local `.env.public-bridge.local` still contains an old value like `AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT=120`, the live relayer now treats that as a floor, not a fixed timeout. On fresh runs it raises stale values to `current Osmosis height + buffer`, which avoids immediate timeout rejection on newer Osmosis testnet heights.

When it completes, it prints the active:

- AegisLink home
- ready file
- node log
- relayer log
- relayer home
- relayer path name

The frontend stays the second command:

```bash
cd web
npm run dev
```

From there, the user flow is:

- connect a Sepolia wallet
- open `AegisLink -> Transfer`
- choose the live Osmosis destination
- enter the `osmo1...` recipient
- submit the ETH bridge deposit

The backend then owns:

- deposit observation on Sepolia
- claim submission into AegisLink
- delivery-intent tracking
- automatic IBC handoff and packet flush toward Osmosis
- bridge-status reporting back to the frontend

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
scripts/testnet/start_aegislink_ibc_demo.sh /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd demo-node status --home /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd demo-node balances --home /tmp/aegislink-public-home --address <bech32-wallet>
go run ./chain/aegislink/cmd/aegislinkd demo-node transfers --home /tmp/aegislink-public-home
```

While the demo node is running, prefer the `demo-node ...` inspection commands above. The older store-backed `query ...` commands reopen the local runtime directly and are best used while the node is stopped.

The demo-node startup wrapper keeps the command surface simple:

- bootstraps the home if it does not exist yet
- binds the local RPC and gRPC listeners
- writes the ready-state file under `<home>/data/demo-node-ready.json`
- keeps running in the foreground until you stop it

Useful local overrides:

- `AEGISLINK_DEMO_NODE_RPC_ADDRESS`
- `AEGISLINK_DEMO_NODE_GRPC_ADDRESS`
- `AEGISLINK_DEMO_NODE_ABCI_ADDRESS`
- `AEGISLINK_DEMO_NODE_READY_FILE`
- `AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS`
- `AEGISLINK_PUBLIC_IBC_AEGISLINK_HOME`

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

For the current real-node path, prefer starting the demo node first and letting the relayer target its ready file through `AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS`. That keeps claim submission and withdrawal queries on the running Comet-backed surface instead of reopening the store directly.

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

## Phase K live public IBC path

The public Osmosis-delivery path uses the separate IBC env file:

```bash
cp .env.public-ibc.local.example .env.public-ibc.local
set -a; source .env.public-ibc.local; set +a
```

Bootstrap the manifest and seed the AegisLink route profile like this:

```bash
scripts/testnet/bootstrap_public_ibc.sh
scripts/testnet/bootstrap_rly_path.sh
scripts/testnet/seed_public_ibc_route.sh /tmp/aegislink-public-home
go run ./chain/aegislink/cmd/aegislinkd query route-profiles --home /tmp/aegislink-public-home
```

If the demo node is already running, prefer pointing `bootstrap_rly_path.sh` at its ready file:

- `AEGISLINK_RLY_SOURCE_READY_FILE=/tmp/aegislink-public-home/data/demo-node-ready.json`

That lets the `rly` bootstrap reuse the live demo-node RPC, WebSocket, and gRPC endpoints automatically instead of hand-copying source addresses.
The ready file now prefers the real Comet RPC surface for `rly` and keeps the custom demo-node HTTP API separate for balances and transfer inspection.

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

That file and bootstrap flow now support a real live IBC leg for the current repo scope:

- the single-validator AegisLink demo node can be linked to Osmosis testnet through `rly`
- the live path can open a real connection and channel and deliver `ueth` into a real `osmo1...` wallet
- the generated `rly` config/path artifacts are still bootstrap inputs, because client, connection, and channel ids remain run-specific
- the public bridge relayer can now also consume a frontend-registered delivery intent and drive the same handoff automatically on a fresh backend launch

One verified live shape is:

```bash
./bin/relayer paths new aegislink-public-testnet-1 osmo-test-5 live-osmo-v13 --home /tmp/aegislink-live-rly-v3
./bin/relayer transact link live-osmo-v13 --home /tmp/aegislink-live-rly-v3 --override --debug --log-level debug
./bin/relayer transact transfer aegislink-public-testnet-1 osmo-test-5 1000000000000ueth osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8 channel-0 --path live-osmo-v13 --debug --log-level debug --home /tmp/aegislink-live-rly-v3
./bin/relayer transact flush live-osmo-v13 channel-0 --debug --log-level debug --home /tmp/aegislink-live-rly-v3
```

The corresponding destination receipt can be checked with the public Osmosis LCD:

```bash
curl -sS https://lcd.osmotest5.osmosis.zone/cosmos/bank/v1beta1/balances/osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8
curl -sS https://lcd.osmotest5.osmosis.zone/ibc/apps/transfer/v1/denom_traces/F656E5CA82F49EB267E5A2D73576FA033F1ABD43A41EF7C9B18F87218ACDD75D
```

That live proof is intentionally narrow and honest:

- it proves a fresh frontend-driven `Sepolia -> AegisLink -> Osmosis` delivery over real IBC
- it does not yet claim that repeated long-lived backend sessions are fully hardened for endless sequential demo runs without operator restarts

## Intended endpoints

- Demo-node API RPC: `http://127.0.0.1:26657`
- Comet RPC: `http://127.0.0.1:27657`
- gRPC: `127.0.0.1:9090`
- ABCI: `tcp://127.0.0.1:26658`
- REST: `http://127.0.0.1:1317`

These are documented operator targets for the single-validator demo node. Treat them as local public-testnet-shaped endpoints rather than a hosted network promise.
