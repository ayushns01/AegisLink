# AegisLink — Getting Started

This guide walks you through running the full AegisLink cross-chain bridge stack locally: backend node, IBC relayer, and the frontend UI.

---

## Prerequisites

Make sure you have the following installed and set up before proceeding.

### Required tools

| Tool | Purpose |
|------|---------|
| `go` (1.21+) | Building the AegisLink node and relayer |
| `node` (18+) + `npm` | Running the frontend |
| `cast` (from Foundry) | Setting the EVM deposit checkpoint at startup |
| `curl` | Health checks during startup |

### Required binaries (pre-built)

The relayer binary must be built before running the backend:

```bash
# From the repo root
go build -o bin/relayer ./relayer/cmd/rly
```

---

## Environment Files

The backend requires three `.env` files at the repo root. These are **not committed** to git — you must create them manually.

### 1. `.env.sepolia.deploy.local`

Contains the deployed Sepolia contract addresses and RPC endpoint.

```bash
AEGISLINK_SEPOLIA_RPC_URL=https://eth-sepolia.g.alchemy.com/v2/<YOUR_KEY>
AEGISLINK_BRIDGE_CONTRACT_ADDRESS=0x...
```

### 2. `.env.public-bridge.local`

Contains relayer config and signing keys.

```bash
AEGISLINK_RELAYER_EVM_RPC_URL=https://eth-sepolia.g.alchemy.com/v2/<YOUR_KEY>
AEGISLINK_RLY_SOURCE_KEY_NAME=aegislink-demo
AEGISLINK_RLY_DESTINATION_KEY_NAME=osmosis-demo
AEGISLINK_RLY_DESTINATION_MNEMONIC="your twenty four word osmosis relayer mnemonic here"
```

### 3. `.env.public-ibc.local`

Contains IBC configuration for the Osmosis testnet route.

```bash
AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES=swap:,stake:,bridge:
AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES=swap,stake,bridge
```

---

## Fund the Osmosis Relayer Key

The IBC relayer needs real testnet OSMO to pay gas for channel handshakes and packet relay.

**This only needs to be done once.**

1. Run the backend once — it will fail and print the Osmosis address that needs funding:
   ```
   osmosis relayer key is not funded yet
   Fund this address with testnet OSMO once, then rerun this same command:
     osmo1xxxxxxxxxxxxxxxxxxxx
   ```
2. Go to the [Osmosis testnet faucet](https://faucet.osmotest5.osmosis.zone/) and fund that address.
3. Re-run the backend — it will proceed normally.

---

## Starting the Backend

From the repo root, run:

```bash
./scripts/testnet/start_public_bridge_backend.sh
```

### What this script does (in order)

1. **Bootstraps** a fresh AegisLink demo home directory under `/tmp/`
2. **Seeds** bridge assets (`eth`) and the public IBC route manifest
3. **Starts** the local AegisLink demo node (block production begins)
4. **Funds** the source relayer key on the local chain
5. **Performs IBC handshakes** — creates client → connection → channel against `osmo-test-5` (this is the slow part, ~60–90 seconds)
6. **Sets the route profile** — enables `osmosis-public-wallet` for ETH bridging
7. **Starts the bridge relayer loop** — polls Sepolia for deposits, relays IBC packets
8. Prints **Backend ready** and exits — both node and relayer continue running in the background

### Expected output

```
+ bash scripts/testnet/bootstrap_aegislink_testnet.sh ...
{"status":"bootstrapped", ...}
+ bash scripts/testnet/seed_public_bridge_assets.sh ...
{"status":"seeded", ...}
+ starting demo node
+ ./bin/relayer transact link live-osmo-ui-... ...
[IBC handshake logs — takes ~60s]
Found termination condition for channel handshake
+ starting public bridge relayer

Backend ready.
Home:          /tmp/aegislink-public-home-ui-<RUN_ID>
Node log:      /tmp/aegislink-public-backend-<RUN_ID>/node.log
Relayer log:   /tmp/aegislink-public-backend-<RUN_ID>/relayer.log
Status file:   /tmp/aegislink-public-backend-<RUN_ID>/status.json
```

### Why the IBC handshake looks "stuck"

The relayer is doing real TCP/HTTPS handshakes against the public Osmosis testnet RPC. It is not frozen. Common benign messages during this phase:

- `could not find results for height #N` — transient, recovers automatically
- `context canceled` at the end of linking — expected, the linker shuts itself down after success

### Process lifecycle (post our fix)

After `Backend ready` is printed, **the script exits cleanly**. The node and relayer continue running as fully detached background processes (`nohup` + `disown`). They will survive terminal closure and are not tied to any shell session.

---

## Stopping the Backend

```bash
pkill -f start_aegislink_ibc_demo.sh
pkill -f public-bridge-relayer
pkill -f 'aegislinkd demo-node start'
```

Or use this single one-liner:

```bash
pkill -f start_aegislink_ibc_demo.sh 2>/dev/null || true; \
pkill -f public-bridge-relayer 2>/dev/null || true; \
pkill -f 'aegislinkd demo-node start' 2>/dev/null || true
```

Verify everything is stopped:

```bash
ps aux | grep -E 'aegislink|relayer|aegislinkd' | grep -v grep
# Should return nothing
```

---

## Starting the Frontend

In a separate terminal:

```bash
cd web
npm install       # first time only
npm run dev
```

Open [http://localhost:5173](http://localhost:5173).

### Preview pages

| URL | Description |
|-----|-------------|
| `/` | Landing page with bridge UI |
| `/abc` | Wormhole visualization preview (isolated) |

---

## Checking Backend Health

After `Backend ready`, verify the node is live:

```bash
curl http://127.0.0.1:26657/healthz
```

Check the relayer is looping:

```bash
tail -f /tmp/aegislink-public-backend-<RUN_ID>/relayer.log | grep run_complete
```

You should see `run_complete` entries every ~4 seconds.

---

## Common Failure Modes

| Symptom | Cause | Fix |
|---------|-------|-----|
| `osmosis relayer key is not funded yet` | Osmosis testnet address has 0 OSMO | Fund via faucet (one-time) |
| IBC link hangs > 5 minutes | Osmosis testnet RPC is congested | Wait, or retry — it has built-in retry logic |
| `Backend ready` but health check fails | Old bug (fixed) — was a `disown` issue | No longer occurs after process lifecycle fix |
| Relayer log shows no `run_complete` | Relayer crashed on startup | Check relayer log: `cat /tmp/aegislink-public-backend-<RUN_ID>/relayer.log` |
| Port already in use (26657/27657/9090) | Previous run not killed | Run the stop commands above, then retry |

---

## Runtime File Reference

All runtime files are ephemeral (under `/tmp/`) and are recreated fresh on each backend start.

| File | Contents |
|------|---------|
| `status.json` | PIDs, log paths, RPC addresses for the current run |
| `node.log` | AegisLink node output (block production, transactions) |
| `relayer.log` | Bridge relayer loop output (deposits, IBC packets) |
| `replay.json` | EVM deposit checkpoint (prevents re-processing old events) |
| `attestations.json` | Verifier attestation state |

---

## Architecture Overview

```
Sepolia (ETH testnet)
    │  User deposits ETH to bridge contract
    ▼
AegisLink Node (local)
    │  Relayer polls Sepolia, submits deposit proofs
    │  Node produces blocks, updates chain state
    ▼
IBC Channel (channel-0)
    │  Packets relayed to Osmosis via go-relayer
    ▼
Osmosis (osmo-test-5)
    │  ibc/ueth credited to destination wallet
    ▼
Frontend polls /api/bridge/status every 4s
    │  Updates wormhole visualization in real time
```
