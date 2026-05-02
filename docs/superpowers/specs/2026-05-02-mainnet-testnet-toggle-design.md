# Mainnet / Testnet Toggle — Implementation Spec

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Testnet / Mainnet toggle to the TransferPage so users can bridge from Sepolia → Cosmos testnets (testnet mode) or from Ethereum mainnet → Cosmos mainnets (mainnet mode), with all UI, config, and backend wiring in place for both.

**Architecture:** A `NetworkMode = "testnet" | "mainnet"` type drives which source chain is expected and which destinations are shown. The toggle state lives in `TransferPage`; `useBridgeWallet` accepts it to evaluate `isWrongChain` against the correct chain ID. Config and deploy files are mirrored under `deploy/mainnet/` following the same structure as `deploy/testnet/`.

**Tech Stack:** React + TypeScript (frontend), wagmi v2, viem, JSON deploy configs, bash startup scripts, go-relayer (rly) chain/path configs.

---

## Files

### Modified
- `web/src/lib/evm/chains.ts` — export both chains and a helper
- `web/src/features/wallet/useBridgeWallet.ts` — accept `networkMode` param
- `web/src/lib/config/env.ts` — add mainnet gateway + explorer fields
- `web/src/features/bridge/TransferPage.tsx` — add toggle, network-filtered destinations, mainnet entries
- `web/src/features/bridge/bridge-session.ts` — add `sourceChainId: number` field
- `web/src/features/bridge/ProgressPanel.tsx` — derive "Sepolia" vs "Ethereum" label and explorer URL from `sourceChainId`
- `web/src/app/App.test.tsx` — update for toggle default state
- `web/src/features/bridge/bridge.test.tsx` — add mainnet routeId assertions

### Created
- `deploy/mainnet/ethereum/bridge-addresses.json` — mainnet gateway placeholder
- `deploy/mainnet/ethereum/bridge-assets.json` — mainnet asset registry
- `deploy/mainnet/ibc/osmosis-mainnet-wallet-delivery.json` — Osmosis mainnet route profile
- `deploy/mainnet/ibc/neutron-mainnet-wallet-delivery.json` — Neutron mainnet route profile
- `deploy/mainnet/ibc/rly/osmosis-mainnet.chain.json` — osmosis-1 relayer chain config
- `deploy/mainnet/ibc/rly/neutron-mainnet.chain.json` — neutron-1 relayer chain config
- `deploy/mainnet/ibc/rly/generated/paths/osmosis-mainnet-wallet.json` — relayer path
- `deploy/mainnet/ibc/rly/generated/paths/neutron-mainnet-wallet.json` — relayer path
- `.env.mainnet.local.example` — mainnet env template
- `scripts/mainnet/start_public_bridge_backend.sh` — mainnet backend startup

---

## Destination Data Model

Add a `network` field to `Destination` in `TransferPage.tsx`:

```ts
type Destination = {
  id: string;
  label: string;
  shortName: string;
  symbol: string;
  color: string;
  logo?: string;
  enabled: boolean;
  prefix: string;
  routeId: string;
  network: "testnet" | "mainnet";
};
```

New/updated entries in the `destinations` array:

| id | network | enabled | routeId |
|---|---|---|---|
| `osmosis-testnet-osmo` | `"testnet"` | `true` | `osmosis-public-wallet` |
| `neutron-testnet-ntrn` | `"testnet"` | `true` | `neutron-public-wallet` |
| `osmosis-mainnet-osmo` | `"mainnet"` | `true` | `osmosis-mainnet-wallet` |
| `neutron-mainnet-ntrn` | `"mainnet"` | `true` | `neutron-mainnet-wallet` |
| All others (Celestia, Injective, dYdX, Akash) | `"mainnet"` | `false` | `""` |

Testnet "coming soon" chains (Celestia mocha, Injective testnet, etc.) keep `network: "testnet"`, `enabled: false`.

---

## `chains.ts` Changes

```ts
import { sepolia, mainnet } from "wagmi/chains";

export type NetworkMode = "testnet" | "mainnet";

export const TESTNET_SOURCE_CHAIN = sepolia;   // chainId 11155111
export const MAINNET_SOURCE_CHAIN = mainnet;   // chainId 1

export function getSourceChainForMode(mode: NetworkMode) {
  return mode === "mainnet" ? mainnet : sepolia;
}
```

Remove the old `export const sourceChain = sepolia` (update all consumers).

---

## `useBridgeWallet.ts` Changes

Signature change — accept `networkMode`:

```ts
export function useBridgeWallet(networkMode: NetworkMode): BridgeWalletState
```

- `isWrongChain`: `isConnected && chain?.id !== getSourceChainForMode(networkMode).id`
- `switchToSourceChain`: switches to `getSourceChainForMode(networkMode).id`
- `connect`: connects to `getSourceChainForMode(networkMode).id`

---

## `env.ts` Changes

```ts
type FrontendEnv = {
  gatewayAddress: `0x${string}`;          // Sepolia gateway
  mainnetGatewayAddress: `0x${string}`;   // Ethereum mainnet gateway
  aegislinkDepositRecipient: string;
  statusApiBaseUrl: string;
  sepoliaExplorerBaseUrl: string;
  ethereumExplorerBaseUrl: string;
};

export const frontendEnv: FrontendEnv = {
  gatewayAddress: ...,                    // existing — unchanged
  mainnetGatewayAddress:
    (import.meta.env.VITE_BRIDGE_MAINNET_GATEWAY_ADDRESS as `0x${string}` | undefined) ??
    ("0x0000000000000000000000000000000000000000" as `0x${string}`),
  aegislinkDepositRecipient: ...,         // existing — unchanged
  statusApiBaseUrl: ...,                  // existing — unchanged
  sepoliaExplorerBaseUrl: ...,            // existing — unchanged
  ethereumExplorerBaseUrl:
    import.meta.env.VITE_ETHEREUM_EXPLORER_BASE_URL ?? "https://etherscan.io",
};
```

---

## `TransferPage.tsx` Changes

New state:

```ts
const [networkMode, setNetworkMode] = useState<NetworkMode>("testnet");
```

Derived destination lists (replace module-level `liveDestinations`/`soonDestinations`):

```ts
const networkDestinations = destinations.filter((d) => d.network === networkMode);
const liveDestinations = networkDestinations.filter((d) => d.enabled);
const soonDestinations = networkDestinations.filter((d) => !d.enabled);
```

On toggle switch — reset selected destination and recipient:

```ts
function handleNetworkModeChange(mode: NetworkMode) {
  setNetworkMode(mode);
  const firstLive = destinations.find((d) => d.network === mode && d.enabled);
  setSelectedDestinationId(firstLive?.id ?? destinations[0].id);
  setRecipient("");
  setSubmissionError(null);
  setIsDestinationMenuOpen(false);
}
```

Pass `networkMode` to `useBridgeWallet`:

```ts
const wallet = useBridgeWallet(networkMode);
```

Gateway address selection:

```ts
const gatewayAddress = networkMode === "mainnet"
  ? frontendEnv.mainnetGatewayAddress
  : frontendEnv.gatewayAddress;
```

Toggle UI — placed inside `.transfer-card__header`, below the heading row:

```tsx
<div className="network-toggle" role="group" aria-label="Network mode">
  <button
    aria-pressed={networkMode === "testnet"}
    className={`network-toggle__btn${networkMode === "testnet" ? " network-toggle__btn--active" : ""}`}
    onClick={() => handleNetworkModeChange("testnet")}
    type="button"
  >
    Testnet
  </button>
  <button
    aria-pressed={networkMode === "mainnet"}
    className={`network-toggle__btn${networkMode === "mainnet" ? " network-toggle__btn--active" : ""}`}
    onClick={() => handleNetworkModeChange("mainnet")}
    type="button"
  >
    Mainnet
  </button>
</div>
```

Update eyebrow text:

```tsx
<p className="eyebrow eyebrow--dark">
  {networkMode === "mainnet" ? "Ethereum → Cosmos" : "Sepolia → Cosmos"}
</p>
```

---

## `bridge-session.ts` Changes

Add `sourceChainId` to `BridgeSession`:

```ts
export type BridgeSession = {
  ...
  sourceChainId: number;   // 1 = mainnet, 11155111 = Sepolia
};
```

Add to `CreateBridgeSessionArgs` and `createSubmittedBridgeSession`.

Pass from `TransferPage.handleSubmit`:

```ts
setSession(createSubmittedBridgeSession({
  ...
  sourceChainId: getSourceChainForMode(networkMode).id,
}));
```

---

## `ProgressPanel.tsx` Changes

Derive source chain label and explorer URL from `session.sourceChainId`:

```ts
const sourceChainLabel = session.sourceChainId === 1 ? "Ethereum" : "Sepolia";
const sourceTxUrl = session.sourceChainId === 1
  ? `${frontendEnv.ethereumExplorerBaseUrl}/tx/${session.sourceTxHash}`
  : `${frontendEnv.sepoliaExplorerBaseUrl}/tx/${session.sourceTxHash}`;
```

Update route display:
```tsx
<span>{sourceChainLabel}</span>
<i aria-hidden="true" />
<span>{destShortName}</span>
```

---

## Deploy Config Files

### `deploy/mainnet/ethereum/bridge-addresses.json`
```json
{
  "chain_id": "1",
  "deployer_address": "",
  "verifier_address": "",
  "gateway_address": "",
  "notes": "Placeholder — deploy with scripts/mainnet/deploy_ethereum_bridge.sh before use"
}
```

### `deploy/mainnet/ethereum/bridge-assets.json`
```json
{
  "chain_id": "1",
  "assets": [
    {
      "asset_id": "eth",
      "source_chain_id": "1",
      "source_asset_kind": "native_eth",
      "denom": "ueth",
      "decimals": 18,
      "display_name": "Ether",
      "display_symbol": "ETH",
      "enabled": true
    }
  ]
}
```

### `deploy/mainnet/ibc/osmosis-mainnet-wallet-delivery.json`
```json
{
  "enabled": true,
  "source_chain_id": "aegislink-public-mainnet-1",
  "destination_chain_id": "osmosis-1",
  "provider": "hermes",
  "wallet_prefix": "osmo",
  "channel_id": "channel-public-osmosis-mainnet",
  "port_id": "transfer",
  "route_id": "osmosis-mainnet-wallet",
  "allowed_memo_prefixes": ["swap:", "stake:", "bridge:"],
  "allowed_action_types": ["swap", "stake", "bridge"],
  "assets": [
    {
      "asset_id": "eth",
      "source_denom": "ueth",
      "destination_denom": "ibc/ueth"
    }
  ]
}
```

### `deploy/mainnet/ibc/neutron-mainnet-wallet-delivery.json`
```json
{
  "enabled": true,
  "source_chain_id": "aegislink-public-mainnet-1",
  "destination_chain_id": "neutron-1",
  "provider": "hermes",
  "wallet_prefix": "neutron",
  "channel_id": "channel-public-neutron-mainnet",
  "port_id": "transfer",
  "route_id": "neutron-mainnet-wallet",
  "allowed_memo_prefixes": ["swap:", "stake:", "bridge:"],
  "allowed_action_types": ["swap", "stake", "bridge"],
  "assets": [
    {
      "asset_id": "eth",
      "source_denom": "ueth",
      "destination_denom": "ibc/ueth"
    }
  ]
}
```

### `deploy/mainnet/ibc/rly/osmosis-mainnet.chain.json`
```json
{
  "chain_name": "osmosis",
  "chain_id": "osmosis-1",
  "bech32_prefix": "osmo",
  "fees": {
    "fee_tokens": [{ "denom": "uosmo", "fixed_min_gas_price": 0.025 }]
  },
  "apis": {
    "rpc": [{ "address": "https://rpc.osmosis.zone:443" }],
    "grpc": [{ "address": "https://grpc.osmosis.zone:443" }]
  }
}
```

### `deploy/mainnet/ibc/rly/neutron-mainnet.chain.json`
```json
{
  "chain_name": "neutron",
  "chain_id": "neutron-1",
  "bech32_prefix": "neutron",
  "fees": {
    "fee_tokens": [{ "denom": "untrn", "fixed_min_gas_price": 0.025 }]
  },
  "apis": {
    "rpc": [{ "address": "https://rpc-kralum.neutron-1.neutron.org:443" }],
    "grpc": [{ "address": "https://grpc-kralum.neutron-1.neutron.org:443" }]
  }
}
```

### `deploy/mainnet/ibc/rly/generated/paths/osmosis-mainnet-wallet.json`
```json
{
  "path_name": "osmosis-mainnet-wallet",
  "src": {
    "chain_id": "aegislink-public-mainnet-1",
    "client_id": "07-tendermint-aegislink-mainnet",
    "connection_id": "connection-aegislink-mainnet",
    "port_id": "transfer",
    "channel_id": "channel-public-osmosis-mainnet"
  },
  "dst": {
    "chain_id": "osmosis-1",
    "client_id": "07-tendermint-osmosis-mainnet",
    "connection_id": "connection-osmosis-mainnet",
    "port_id": "transfer",
    "channel_id": "channel-public-osmosis-mainnet"
  },
  "manifest": {
    "source_chain_id": "aegislink-public-mainnet-1",
    "destination_chain_id": "osmosis-1",
    "channel_id": "channel-public-osmosis-mainnet",
    "port_id": "transfer",
    "route_id": "osmosis-mainnet-wallet",
    "assets": [
      { "asset_id": "eth", "source_denom": "ueth", "destination_denom": "ibc/ueth" }
    ]
  }
}
```

### `deploy/mainnet/ibc/rly/generated/paths/neutron-mainnet-wallet.json`
```json
{
  "path_name": "neutron-mainnet-wallet",
  "src": {
    "chain_id": "aegislink-public-mainnet-1",
    "client_id": "07-tendermint-aegislink-mainnet",
    "connection_id": "connection-aegislink-mainnet",
    "port_id": "transfer",
    "channel_id": "channel-public-neutron-mainnet"
  },
  "dst": {
    "chain_id": "neutron-1",
    "client_id": "07-tendermint-neutron-mainnet",
    "connection_id": "connection-neutron-mainnet",
    "port_id": "transfer",
    "channel_id": "channel-public-neutron-mainnet"
  },
  "manifest": {
    "source_chain_id": "aegislink-public-mainnet-1",
    "destination_chain_id": "neutron-1",
    "channel_id": "channel-public-neutron-mainnet",
    "port_id": "transfer",
    "route_id": "neutron-mainnet-wallet",
    "assets": [
      { "asset_id": "eth", "source_denom": "ueth", "destination_denom": "ibc/ueth" }
    ]
  }
}
```

---

## `.env.mainnet.local.example`

```bash
# Ethereum mainnet gateway (deployed contract address)
VITE_BRIDGE_MAINNET_GATEWAY_ADDRESS=0x0000000000000000000000000000000000000000

# AegisLink mainnet node
VITE_BRIDGE_STATUS_API_BASE_URL=http://localhost:1318

# Mainnet Cosmos destination overrides
AEGISLINK_PUBLIC_BACKEND_DESTINATION_RPC_ADDR=https://rpc.osmosis.zone:443
AEGISLINK_PUBLIC_BACKEND_DESTINATION_GRPC_ADDR=https://grpc.osmosis.zone:443
AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_ID=osmosis-1
AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL=https://lcd.osmosis.zone

# Funded Osmosis mainnet relayer key (OSMO for gas)
AEGISLINK_RLY_OSMOSIS_MAINNET_KEY_NAME=osmosis-mainnet-relayer-key
AEGISLINK_RLY_OSMOSIS_MAINNET_MNEMONIC=""

# Neutron mainnet overrides
AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR=https://rpc-kralum.neutron-1.neutron.org:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR=https://grpc-kralum.neutron-1.neutron.org:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL=https://rest-kralum.neutron-1.neutron.org
AEGISLINK_RLY_NEUTRON_MAINNET_KEY_NAME=neutron-mainnet-relayer-key
AEGISLINK_RLY_NEUTRON_MAINNET_MNEMONIC=""
```

---

## CSS — `.network-toggle`

Add to `global.css` inside the transfer card section:

```css
.network-toggle {
  display: flex;
  gap: 2px;
  background: rgba(255,255,255,0.06);
  border-radius: 8px;
  padding: 3px;
  width: fit-content;
}

.network-toggle__btn {
  padding: 4px 14px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: rgba(255,255,255,0.5);
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.network-toggle__btn--active {
  background: rgba(255,255,255,0.12);
  color: #fff;
}
```

---

## Tests

### `App.test.tsx`
- Assertions for "Destination chain: Osmosis Testnet (OSMO)" remain valid (default is testnet mode)
- No changes needed unless the button aria-label changes

### `bridge.test.tsx` — new test cases to add
- "passes osmosis-mainnet-wallet routeId when mainnet mode + Osmosis mainnet selected"
- "passes neutron-mainnet-wallet routeId when mainnet mode + Neutron mainnet selected"
- "resets recipient when switching network mode"
- "shows only mainnet destinations in mainnet mode"
- "shows only testnet destinations in testnet mode"

### `useBridgeWallet.test.ts` (if it exists) or inline in bridge tests
- `isWrongChain = true` when networkMode=mainnet and wallet on Sepolia
- `isWrongChain = false` when networkMode=mainnet and wallet on Ethereum mainnet

---

## `scripts/mainnet/start_public_bridge_backend.sh`

Mirrors `scripts/testnet/start_public_bridge_backend.sh` with:
- Default `DESTINATION_CHAIN_ID=osmosis-1`
- Default `DESTINATION_RPC_ADDR=https://rpc.osmosis.zone:443`
- Default `NEUTRON_RPC_ADDR=https://rpc-kralum.neutron-1.neutron.org:443`
- Sources `.env.mainnet.local` instead of `.env.public-ibc.neutron.local`
- Route IDs: `osmosis-mainnet-wallet`, `neutron-mainnet-wallet`
- Path names: `live-osmo-mainnet-$RUN_ID`, `live-ntrn-mainnet-$RUN_ID`
