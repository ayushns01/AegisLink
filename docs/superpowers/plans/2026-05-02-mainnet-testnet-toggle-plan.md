# Mainnet / Testnet Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Testnet/Mainnet toggle to TransferPage so users can bridge Sepolia → Cosmos testnets or Ethereum mainnet → Cosmos mainnets, with all UI, config, and backend wiring in place for both modes.

**Architecture:** `NetworkMode = "testnet" | "mainnet"` drives which source chain is expected and which destinations are shown. Toggle state lives in `TransferPage`; `useBridgeWallet` accepts `networkMode` to derive `isWrongChain` against the correct chain ID. Deploy configs mirror the testnet structure under `deploy/mainnet/`.

**Tech Stack:** React 18, TypeScript, wagmi v2, viem, Vitest + @testing-library/react, bash, JSON.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `web/src/lib/evm/chains.ts` | Modify | Export `NetworkMode`, both source chains, `getSourceChainForMode` |
| `web/src/main.tsx` | Modify | Register both chains in wagmi config |
| `web/src/features/wallet/useBridgeWallet.ts` | Modify | Accept `networkMode` param, derive correct chain |
| `web/src/lib/config/env.ts` | Modify | Add `mainnetGatewayAddress` + `ethereumExplorerBaseUrl` |
| `web/src/features/bridge/bridge-session.ts` | Modify | Add `sourceChainId: number` field |
| `web/src/features/bridge/ProgressPanel.tsx` | Modify | Use `sourceChainId` for source label + explorer URL; add Neutron mintscan |
| `web/src/features/bridge/TransferPage.tsx` | Modify | Toggle UI, `network` field on destinations, mainnet entries, network-filtered lists |
| `web/src/styles/global.css` | Modify | Add `.network-toggle` styles |
| `web/src/features/bridge/bridge.test.tsx` | Modify | Update stale test, add network toggle + mainnet route tests |
| `web/src/app/App.test.tsx` | Modify | Update `useBridgeWallet` mock to accept args |
| `deploy/mainnet/ethereum/bridge-addresses.json` | Create | Mainnet gateway placeholder |
| `deploy/mainnet/ethereum/bridge-assets.json` | Create | Mainnet asset registry |
| `deploy/mainnet/ibc/osmosis-mainnet-wallet-delivery.json` | Create | Osmosis mainnet route profile |
| `deploy/mainnet/ibc/neutron-mainnet-wallet-delivery.json` | Create | Neutron mainnet route profile |
| `deploy/mainnet/ibc/rly/osmosis-mainnet.chain.json` | Create | osmosis-1 relayer chain config |
| `deploy/mainnet/ibc/rly/neutron-mainnet.chain.json` | Create | neutron-1 relayer chain config |
| `deploy/mainnet/ibc/rly/generated/paths/osmosis-mainnet-wallet.json` | Create | Osmosis mainnet relayer path |
| `deploy/mainnet/ibc/rly/generated/paths/neutron-mainnet-wallet.json` | Create | Neutron mainnet relayer path |
| `.env.mainnet.local.example` | Create | Mainnet env template |
| `scripts/mainnet/start_public_bridge_backend.sh` | Create | Mainnet backend startup script |

---

### Task 1: `chains.ts` — NetworkMode type + dual source chain exports

**Files:**
- Modify: `web/src/lib/evm/chains.ts`

- [ ] **Step 1: Replace the file contents**

```ts
import { sepolia, mainnet } from "wagmi/chains";

export type NetworkMode = "testnet" | "mainnet";

export const sourceChain = sepolia; // kept for backward compat
export const TESTNET_SOURCE_CHAIN = sepolia; // chainId 11155111
export const MAINNET_SOURCE_CHAIN = mainnet;  // chainId 1

export function getSourceChainForMode(mode: NetworkMode) {
  return mode === "mainnet" ? mainnet : sepolia;
}
```

- [ ] **Step 2: Run the full test suite to confirm nothing breaks**

```bash
cd web && npx vitest run 2>&1 | tail -20
```

Expected: same pass/fail count as before (no regressions).

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/evm/chains.ts
git commit -m "feat: export NetworkMode type and dual source chains"
```

---

### Task 2: `main.tsx` — register both chains in wagmi

**Files:**
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Update wagmi config to include both chains**

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider, createConfig, http } from "wagmi";
import { injected } from "wagmi/connectors";
import { App } from "./app/App";
import { TESTNET_SOURCE_CHAIN, MAINNET_SOURCE_CHAIN } from "./lib/evm/chains";
import "./styles/tokens.css";
import "./styles/global.css";

const queryClient = new QueryClient();
const config = createConfig({
  chains: [TESTNET_SOURCE_CHAIN, MAINNET_SOURCE_CHAIN],
  connectors: [injected()],
  transports: {
    [TESTNET_SOURCE_CHAIN.id]: http(),
    [MAINNET_SOURCE_CHAIN.id]: http(),
  },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <WagmiProvider config={config}>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </WagmiProvider>
  </React.StrictMode>,
);
```

- [ ] **Step 2: Run tests**

```bash
cd web && npx vitest run 2>&1 | tail -10
```

Expected: no regressions.

- [ ] **Step 3: Commit**

```bash
git add web/src/main.tsx
git commit -m "feat: register Ethereum mainnet in wagmi config alongside Sepolia"
```

---

### Task 3: `useBridgeWallet.ts` — accept networkMode param

**Files:**
- Modify: `web/src/features/wallet/useBridgeWallet.ts`

- [ ] **Step 1: Update the hook to accept and use `networkMode`**

```ts
import { useMemo } from "react";
import {
  useAccount,
  useConnect,
  useDisconnect,
  useSwitchChain,
} from "wagmi";
import { type NetworkMode, getSourceChainForMode } from "../../lib/evm/chains";

export type BridgeWalletState = {
  address?: string;
  chainId?: number;
  connectionError?: string;
  hasInjectedWallet: boolean;
  isConnected: boolean;
  isConnecting: boolean;
  isWrongChain: boolean;
  chainName?: string;
  connect: () => Promise<void>;
  disconnect: () => void;
  switchToSourceChain: () => Promise<void>;
};

export function useBridgeWallet(networkMode: NetworkMode = "testnet"): BridgeWalletState {
  const { address, chain, isConnected } = useAccount();
  const { connectAsync, connectors, error, isPending } = useConnect();
  const { disconnect } = useDisconnect();
  const { switchChainAsync } = useSwitchChain();
  const sourceChain = getSourceChainForMode(networkMode);
  const injectedConnector = connectors.find(
    (connector) => connector.type === "injected",
  );
  const hasInjectedWallet =
    Boolean(injectedConnector) ||
    (typeof window !== "undefined" &&
      Boolean((window as Window & { ethereum?: unknown }).ethereum));

  return useMemo(
    () => ({
      address,
      chainId: chain?.id,
      connectionError: error?.message,
      hasInjectedWallet,
      isConnected,
      isConnecting: isPending,
      isWrongChain: Boolean(isConnected && chain?.id !== sourceChain.id),
      chainName: chain?.name,
      connect: async () => {
        if (!injectedConnector) {
          throw new Error("No wallet extension is available.");
        }
        await connectAsync({
          connector: injectedConnector,
          chainId: sourceChain.id,
        });
      },
      disconnect,
      switchToSourceChain: async () => {
        await switchChainAsync({ chainId: sourceChain.id });
      },
    }),
    [
      address,
      chain?.id,
      chain?.name,
      connectAsync,
      connectors,
      disconnect,
      error?.message,
      hasInjectedWallet,
      injectedConnector,
      isConnected,
      isPending,
      sourceChain.id,
      switchChainAsync,
    ],
  );
}
```

- [ ] **Step 2: Run tests**

```bash
cd web && npx vitest run 2>&1 | tail -10
```

Expected: no regressions (existing tests mock this hook entirely).

- [ ] **Step 3: Commit**

```bash
git add web/src/features/wallet/useBridgeWallet.ts
git commit -m "feat: useBridgeWallet accepts networkMode param"
```

---

### Task 4: `env.ts` — mainnet gateway + explorer fields

**Files:**
- Modify: `web/src/lib/config/env.ts`

- [ ] **Step 1: Add mainnet fields to FrontendEnv**

```ts
import bridgeAddresses from "../../../../deploy/testnet/sepolia/bridge-addresses.json";

type FrontendEnv = {
  gatewayAddress: `0x${string}`;
  mainnetGatewayAddress: `0x${string}`;
  aegislinkDepositRecipient: string;
  statusApiBaseUrl: string;
  sepoliaExplorerBaseUrl: string;
  ethereumExplorerBaseUrl: string;
};

const defaultAegislinkDepositRecipient =
  "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4";

export const frontendEnv: FrontendEnv = {
  gatewayAddress:
    (import.meta.env.VITE_BRIDGE_GATEWAY_ADDRESS as `0x${string}` | undefined) ??
    (bridgeAddresses.gateway_address as `0x${string}`),
  mainnetGatewayAddress:
    (import.meta.env.VITE_BRIDGE_MAINNET_GATEWAY_ADDRESS as `0x${string}` | undefined) ??
    ("0x0000000000000000000000000000000000000000" as `0x${string}`),
  aegislinkDepositRecipient:
    import.meta.env.VITE_BRIDGE_AEGISLINK_DEPOSIT_RECIPIENT ??
    defaultAegislinkDepositRecipient,
  statusApiBaseUrl: import.meta.env.VITE_BRIDGE_STATUS_API_BASE_URL ?? "",
  sepoliaExplorerBaseUrl:
    import.meta.env.VITE_SEPOLIA_EXPLORER_BASE_URL ?? "https://sepolia.etherscan.io",
  ethereumExplorerBaseUrl:
    import.meta.env.VITE_ETHEREUM_EXPLORER_BASE_URL ?? "https://etherscan.io",
};
```

- [ ] **Step 2: Run tests**

```bash
cd web && npx vitest run 2>&1 | tail -10
```

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/config/env.ts
git commit -m "feat: add mainnetGatewayAddress and ethereumExplorerBaseUrl to frontendEnv"
```

---

### Task 5: `bridge-session.ts` — add sourceChainId

**Files:**
- Modify: `web/src/features/bridge/bridge-session.ts`

- [ ] **Step 1: Add `sourceChainId` to the session type and constructor**

```ts
export type BridgeSessionStatus =
  | "deposit_submitted"
  | "sepolia_confirming"
  | "aegislink_processing"
  | "osmosis_pending"
  | "completed"
  | "failed";

export type BridgeSession = {
  amountEth: string;
  destinationChain: string;
  recipient: string;
  sourceAddress: string;
  sourceTxHash: string;
  sourceChainId: number;
  status: BridgeSessionStatus;
  createdAt: number;
  destinationTxHash?: string;
  destinationTxUrl?: string;
  errorMessage?: string;
};

type CreateBridgeSessionArgs = {
  amountEth: string;
  destinationChain: string;
  recipient: string;
  sourceAddress: string;
  sourceTxHash: string;
  sourceChainId: number;
  createdAt?: number;
};

export function createSubmittedBridgeSession({
  amountEth,
  destinationChain,
  recipient,
  sourceAddress,
  sourceTxHash,
  sourceChainId,
  createdAt = Date.now(),
}: CreateBridgeSessionArgs): BridgeSession {
  return {
    amountEth,
    destinationChain,
    recipient,
    sourceAddress,
    sourceTxHash,
    sourceChainId,
    status: "deposit_submitted",
    createdAt,
  };
}
```

- [ ] **Step 2: Run tests**

```bash
cd web && npx vitest run 2>&1 | tail -10
```

Expected: TypeScript will error on callers of `createSubmittedBridgeSession` that don't pass `sourceChainId` yet — that's expected; they get fixed in Task 7.

- [ ] **Step 3: Commit**

```bash
git add web/src/features/bridge/bridge-session.ts
git commit -m "feat: add sourceChainId to BridgeSession"
```

---

### Task 6: `ProgressPanel.tsx` — network-aware source label + explorer URL

**Files:**
- Modify: `web/src/features/bridge/ProgressPanel.tsx`

- [ ] **Step 1: Update source tx link and route label to use `session.sourceChainId`**

Replace the source transaction proof card block and the route manifest span. Full updated file:

```tsx
import { frontendEnv } from "../../lib/config/env";
import type { BridgeSession } from "./bridge-session";
import { deriveTransferProgressModel, type TransferVisualStageId } from "./transfer-progress";
import { BridgeWormholeScene, DESTINATION_RIGHT_RGB } from "./BridgeWormholeScene";

function resolveDestinationKey(destinationChain: string): string {
  return destinationChain.split(" ")[0].toLowerCase();
}

function resolveDestinationShortName(destinationChain: string): string {
  return destinationChain.split(" ")[0];
}

type ProgressPanelProps = {
  isPolling?: boolean;
  onReset: () => void;
  pollError?: string | null;
  session: BridgeSession;
};

export function ProgressPanel({
  isPolling = false,
  onReset,
  pollError = null,
  session,
}: ProgressPanelProps) {
  const destinationTxUrl = resolveDestinationTxUrl(session);
  const progress = deriveTransferProgressModel(session, isPolling);
  const currentStageId =
    (progress.stages.find((stage) => stage.state === "current")?.id ?? "sepolia") as TransferVisualStageId;
  const destKey = resolveDestinationKey(session.destinationChain);
  const destShortName = resolveDestinationShortName(session.destinationChain);
  const rightRgb = DESTINATION_RIGHT_RGB[destKey] ?? DESTINATION_RIGHT_RGB.osmosis;
  const sourceChainLabel = session.sourceChainId === 1 ? "Ethereum" : "Sepolia";
  const sourceTxUrl = session.sourceChainId === 1
    ? `${frontendEnv.ethereumExplorerBaseUrl}/tx/${session.sourceTxHash}`
    : `${frontendEnv.sepoliaExplorerBaseUrl}/tx/${session.sourceTxHash}`;

  return (
    <div className="transfer-card transfer-card--progress transfer-card--progress-expanded transfer-card--progress-obsidian transfer-card--progress-contained">
      <div className="progress-shell progress-shell--ignited">
        <div className="progress-shell__top progress-summary-bar">
          <div className="progress-manifest">
            <p className="eyebrow">Bridge Session</p>
            <h2>Transfer in progress</h2>
            <small>Transfer route</small>
            <strong>{session.amountEth} ETH</strong>
            <div className="progress-manifest__route" aria-label={`${sourceChainLabel} to ${destShortName} route`}>
              <span>{sourceChainLabel}</span>
              <i aria-hidden="true" />
              <span>{destShortName}</span>
            </div>
            <p>{session.recipient}</p>
          </div>

          <div className="progress-live">
            <small>{progress.sceneLabel}</small>
            <h3>{progress.headline}</h3>
            <p>{progress.summary}</p>
            <div className="wallet-chip wallet-chip--progress wallet-chip--progress-live">
              {progress.chipLabel}
            </div>
          </div>
        </div>

        <BridgeWormholeScene
          activeStageId={currentStageId}
          stages={progress.stages}
          rightRgb={rightRgb}
        />

        <div className="progress-proof-grid">
          <div className="progress-proof-card">
            <small>Source transaction</small>
            <a
              className="tx-link"
              href={sourceTxUrl}
              rel="noreferrer"
              target="_blank"
            >
              {shortHash(session.sourceTxHash)}
            </a>
            <span>{session.sourceAddress}</span>
          </div>

          <div className="progress-proof-card">
            <small>Destination receipt</small>
            {session.destinationTxHash ? (
              destinationTxUrl ? (
                <a
                  className="tx-link"
                  href={destinationTxUrl}
                  rel="noreferrer"
                  target="_blank"
                >
                  {shortHash(session.destinationTxHash)}
                </a>
              ) : (
                <strong>{shortHash(session.destinationTxHash)}</strong>
              )
            ) : (
              <strong>Waiting for final destination hash</strong>
            )}
            <span>
              {session.destinationTxHash
                ? "Confirmed by the configured bridge status source."
                : `This appears as soon as the operator tracking endpoint observes the ${destShortName} receipt.`}
            </span>
          </div>
        </div>

        {pollError ? <p className="progress-alert">{pollError}</p> : null}

        <div className="progress-actions">
          <button className="secondary-cta" onClick={onReset} type="button">
            Start New Transfer
          </button>
        </div>
      </div>
    </div>
  );
}

function shortHash(hash: string) {
  return `${hash.slice(0, 10)}...${hash.slice(-8)}`;
}

function resolveDestinationTxUrl(session: BridgeSession) {
  if (!session.destinationTxHash) {
    return undefined;
  }
  if (session.destinationTxUrl) {
    return normalizeDestinationTxUrl(session.destinationTxUrl);
  }
  if (session.destinationChain === "Osmosis Testnet (OSMO)") {
    return `https://www.mintscan.io/osmosis-testnet/tx/${session.destinationTxHash}`;
  }
  if (session.destinationChain === "Osmosis Mainnet (OSMO)") {
    return `https://www.mintscan.io/osmosis/tx/${session.destinationTxHash}`;
  }
  if (session.destinationChain === "Neutron Testnet (NTRN)") {
    return `https://www.mintscan.io/neutron-testnet/tx/${session.destinationTxHash}`;
  }
  if (session.destinationChain === "Neutron Mainnet (NTRN)") {
    return `https://www.mintscan.io/neutron/tx/${session.destinationTxHash}`;
  }
  return undefined;
}

function normalizeDestinationTxUrl(url: string) {
  return url
    .replace("https://www.mintscan.io/osmosis-testnet/txs/", "https://www.mintscan.io/osmosis-testnet/tx/")
    .replace("https://www.mintscan.io/osmosis/txs/", "https://www.mintscan.io/osmosis/tx/");
}
```

- [ ] **Step 2: Run tests**

```bash
cd web && npx vitest run 2>&1 | tail -10
```

- [ ] **Step 3: Commit**

```bash
git add web/src/features/bridge/ProgressPanel.tsx
git commit -m "feat: use sourceChainId for source label and explorer URL in ProgressPanel"
```

---

### Task 7: `bridge.test.tsx` — update stale test + add new cases

**Files:**
- Modify: `web/src/features/bridge/bridge.test.tsx`

The test at line 83 ("shows mainnet and testnet destination options in the dropdown") currently expects ALL destinations in one dropdown. After the change, testnet mode only shows testnet destinations. **This test must be updated.**

- [ ] **Step 1: Add `seedMainnetWallet` helper and update the stale test**

Add `seedMainnetWallet` after the existing `seedConnectedWallet` helper (around line 70):

```ts
function seedMainnetWallet() {
  useBridgeWalletMock.mockReturnValue({
    address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
    chainId: 1,
    chainName: "Ethereum",
    connectionError: undefined,
    hasInjectedWallet: true,
    isConnected: true,
    isConnecting: false,
    isWrongChain: false,
    connect: vi.fn(),
    disconnect: vi.fn(),
    switchToSourceChain: vi.fn(),
  });
  useWalletClientMock.mockReturnValue({
    data: { chain: { id: 1, name: "Ethereum" } },
  });
}
```

Replace the test "shows mainnet and testnet destination options in the dropdown" (lines 83-108) with:

```ts
it("shows only testnet destinations in testnet mode dropdown", async () => {
  seedConnectedWallet();
  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(
    screen.getByRole("button", {
      name: /destination chain: osmosis testnet \(osmo\)/i,
    }),
  );

  expect(screen.getByRole("menuitem", { name: /osmosis testnet \(osmo\)/i })).toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /neutron testnet \(ntrn\)/i })).toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /osmosis mainnet \(osmo\)/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /neutron mainnet \(ntrn\)/i })).not.toBeInTheDocument();
  expect(screen.getByRole("menu")).toHaveClass("chain-menu");
  expect(
    screen.getByRole("menuitem", { name: /osmosis testnet \(osmo\)/i }),
  ).toHaveClass("chain-option--active");
});
```

- [ ] **Step 2: Run test to verify it fails (stale test is gone, new test not yet passing)**

```bash
cd web && npx vitest run src/features/bridge/bridge.test.tsx 2>&1 | tail -20
```

Expected: the new test fails because `TransferPage` hasn't been updated yet.

- [ ] **Step 3: Add network toggle tests at the end of the `describe` block**

Append before the closing `});` of the describe block:

```ts
it("shows the Testnet toggle button as active by default", () => {
  seedConnectedWallet();
  render(<TransferPage />);

  expect(
    screen.getByRole("button", { name: /testnet/i, hidden: false }),
  ).toHaveAttribute("aria-pressed", "true");
  expect(
    screen.getByRole("button", { name: /mainnet/i, hidden: false }),
  ).toHaveAttribute("aria-pressed", "false");
});

it("switches to mainnet mode and shows only mainnet destinations", async () => {
  seedConnectedWallet();
  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

  expect(
    screen.getByRole("button", { name: /^mainnet$/i }),
  ).toHaveAttribute("aria-pressed", "true");

  await user.click(
    screen.getByRole("button", {
      name: /destination chain: osmosis mainnet \(osmo\)/i,
    }),
  );

  expect(screen.getByRole("menuitem", { name: /osmosis mainnet \(osmo\)/i })).toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /neutron mainnet \(ntrn\)/i })).toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /osmosis testnet \(osmo\)/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /neutron testnet \(ntrn\)/i })).not.toBeInTheDocument();
});

it("resets recipient address when switching network mode", async () => {
  seedConnectedWallet();
  const user = userEvent.setup();
  render(<TransferPage />);

  await user.clear(screen.getByLabelText(/recipient/i));
  await user.type(screen.getByLabelText(/recipient/i), "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8");

  expect(screen.getByLabelText(/recipient/i)).toHaveValue(
    "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );

  await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

  expect(screen.getByLabelText(/recipient/i)).toHaveValue("");
});

it("shows Ethereum → Cosmos eyebrow in mainnet mode", async () => {
  seedConnectedWallet();
  const user = userEvent.setup();
  render(<TransferPage />);

  expect(screen.getByText(/sepolia → cosmos/i)).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

  expect(screen.getByText(/ethereum → cosmos/i)).toBeInTheDocument();
  expect(screen.queryByText(/sepolia → cosmos/i)).not.toBeInTheDocument();
});

it("shows Switch to Ethereum mainnet hint when isWrongChain in mainnet mode", async () => {
  useBridgeWalletMock.mockReturnValue({
    address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
    chainId: 11155111,
    chainName: "Sepolia",
    hasInjectedWallet: true,
    isConnected: true,
    isConnecting: false,
    isWrongChain: true,
    connect: vi.fn(),
    disconnect: vi.fn(),
    switchToSourceChain: vi.fn(),
  });
  useWalletClientMock.mockReturnValue({
    data: { chain: { id: 11155111, name: "Sepolia" } },
  });

  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

  expect(screen.getByText(/switch to ethereum mainnet/i)).toBeInTheDocument();
});

it("passes osmosis-mainnet-wallet routeId in mainnet mode", async () => {
  seedMainnetWallet();
  submitEthDepositMock.mockResolvedValue(
    "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
  );
  registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

  await user.clear(screen.getByLabelText(/recipient/i));
  await user.type(
    screen.getByLabelText(/recipient/i),
    "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );

  await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

  await waitFor(() => {
    expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith(
      expect.objectContaining({ routeId: "osmosis-mainnet-wallet" }),
    );
  });
});

it("passes neutron-mainnet-wallet routeId when mainnet + Neutron mainnet selected", async () => {
  seedMainnetWallet();
  submitEthDepositMock.mockResolvedValue(
    "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
  );
  registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

  await user.click(
    screen.getByRole("button", {
      name: /destination chain: osmosis mainnet \(osmo\)/i,
    }),
  );
  await user.click(
    screen.getByRole("menuitem", { name: /neutron mainnet \(ntrn\)/i }),
  );

  await user.clear(screen.getByLabelText(/recipient/i));
  await user.type(
    screen.getByLabelText(/recipient/i),
    "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );

  await user.click(screen.getByRole("button", { name: /bridge.*ntrn/i }));

  await waitFor(() => {
    expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith(
      expect.objectContaining({ routeId: "neutron-mainnet-wallet" }),
    );
  });
});
```

- [ ] **Step 4: Run tests — expect new tests to fail**

```bash
cd web && npx vitest run src/features/bridge/bridge.test.tsx 2>&1 | tail -20
```

Expected: ~7 new test failures (TransferPage not updated yet). Pre-existing tests still pass.

- [ ] **Step 5: Commit the test additions**

```bash
git add web/src/features/bridge/bridge.test.tsx
git commit -m "test: add network toggle and mainnet routeId test cases"
```

---

### Task 8: `TransferPage.tsx` — toggle UI + mainnet destinations + CSS

**Files:**
- Modify: `web/src/features/bridge/TransferPage.tsx`
- Modify: `web/src/styles/global.css`

- [ ] **Step 1: Replace the full TransferPage.tsx**

```tsx
import { useMemo, useState } from "react";
import { parseEther } from "viem";
import type { Address } from "viem";
import { useWalletClient } from "wagmi";
import { registerBridgeDeliveryIntent } from "../../lib/bridge/delivery-intent";
import { frontendEnv } from "../../lib/config/env";
import { type NetworkMode, getSourceChainForMode } from "../../lib/evm/chains";
import { submitEthDeposit } from "../../lib/evm/gateway";
import { useBridgeWallet } from "../wallet/useBridgeWallet";
import { createSubmittedBridgeSession, type BridgeSession } from "./bridge-session";
import { ProgressPanel } from "./ProgressPanel";
import { useBridgeSessionStatus } from "./useBridgeSessionStatus";

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
  network: NetworkMode;
};

const destinations: Destination[] = [
  {
    id: "osmosis-testnet-osmo",
    label: "Osmosis Testnet (OSMO)",
    shortName: "Osmosis Testnet",
    symbol: "OSMO",
    color: "#5E12A0",
    logo: "/chains/osmo.svg",
    enabled: true,
    prefix: "osmo1",
    routeId: "osmosis-public-wallet",
    network: "testnet",
  },
  {
    id: "neutron-testnet-ntrn",
    label: "Neutron Testnet (NTRN)",
    shortName: "Neutron Testnet",
    symbol: "NTRN",
    color: "#1a1a2e",
    logo: "/chains/ntrn.svg",
    enabled: true,
    prefix: "neutron1",
    routeId: "neutron-public-wallet",
    network: "testnet",
  },
  {
    id: "celestia-mocha-testnet-tia",
    label: "Celestia Mocha Testnet (TIA)",
    shortName: "Celestia Mocha",
    symbol: "TIA",
    color: "#7c3aed",
    enabled: false,
    prefix: "celestia1",
    routeId: "",
    network: "testnet",
  },
  {
    id: "injective-testnet-inj",
    label: "Injective Testnet (INJ)",
    shortName: "Injective Testnet",
    symbol: "INJ",
    color: "#0ea5e9",
    enabled: false,
    prefix: "inj1",
    routeId: "",
    network: "testnet",
  },
  {
    id: "dydx-testnet-dydx",
    label: "dYdX Testnet (DYDX)",
    shortName: "dYdX Testnet",
    symbol: "DYDX",
    color: "#22c55e",
    enabled: false,
    prefix: "dydx1",
    routeId: "",
    network: "testnet",
  },
  {
    id: "akash-sandbox-akt",
    label: "Akash Sandbox (AKT)",
    shortName: "Akash Sandbox",
    symbol: "AKT",
    color: "#f97316",
    enabled: false,
    prefix: "akash1",
    routeId: "",
    network: "testnet",
  },
  {
    id: "osmosis-mainnet-osmo",
    label: "Osmosis Mainnet (OSMO)",
    shortName: "Osmosis Mainnet",
    symbol: "OSMO",
    color: "#5E12A0",
    logo: "/chains/osmo.svg",
    enabled: true,
    prefix: "osmo1",
    routeId: "osmosis-mainnet-wallet",
    network: "mainnet",
  },
  {
    id: "neutron-mainnet-ntrn",
    label: "Neutron Mainnet (NTRN)",
    shortName: "Neutron Mainnet",
    symbol: "NTRN",
    color: "#1a1a2e",
    logo: "/chains/ntrn.svg",
    enabled: true,
    prefix: "neutron1",
    routeId: "neutron-mainnet-wallet",
    network: "mainnet",
  },
  {
    id: "celestia-mainnet-tia",
    label: "Celestia Mainnet (TIA)",
    shortName: "Celestia Mainnet",
    symbol: "TIA",
    color: "#7c3aed",
    enabled: false,
    prefix: "celestia1",
    routeId: "",
    network: "mainnet",
  },
  {
    id: "injective-mainnet-inj",
    label: "Injective Mainnet (INJ)",
    shortName: "Injective Mainnet",
    symbol: "INJ",
    color: "#0ea5e9",
    enabled: false,
    prefix: "inj1",
    routeId: "",
    network: "mainnet",
  },
  {
    id: "dydx-mainnet-dydx",
    label: "dYdX Mainnet (DYDX)",
    shortName: "dYdX Mainnet",
    symbol: "DYDX",
    color: "#22c55e",
    enabled: false,
    prefix: "dydx1",
    routeId: "",
    network: "mainnet",
  },
  {
    id: "akash-mainnet-akt",
    label: "Akash Mainnet (AKT)",
    shortName: "Akash Mainnet",
    symbol: "AKT",
    color: "#f97316",
    enabled: false,
    prefix: "akash1",
    routeId: "",
    network: "mainnet",
  },
];

function ChainAvatar({
  destination,
  size,
  muted = false,
}: {
  destination: Destination;
  size: "md" | "sm";
  muted?: boolean;
}) {
  const className = [
    "chain-avatar",
    size === "sm" ? "chain-avatar--sm" : "",
    muted ? "chain-avatar--muted" : "",
  ]
    .filter(Boolean)
    .join(" ");

  if (destination.logo) {
    return (
      <img
        alt={destination.symbol}
        className={className}
        src={destination.logo}
        style={{ background: "transparent" }}
      />
    );
  }

  return (
    <span className={className} style={{ background: destination.color }}>
      {destination.symbol.slice(0, 2)}
    </span>
  );
}

export function TransferPage() {
  const [networkMode, setNetworkMode] = useState<NetworkMode>("testnet");
  const wallet = useBridgeWallet(networkMode);
  const { data: walletClient } = useWalletClient();
  const [amount, setAmount] = useState("0.250");
  const [recipient, setRecipient] = useState(
    "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );
  const [selectedDestinationId, setSelectedDestinationId] = useState(
    () => destinations.find((d) => d.network === "testnet" && d.enabled)?.id ?? destinations[0].id,
  );
  const [isDestinationMenuOpen, setIsDestinationMenuOpen] = useState(false);
  const [session, setSession] = useState<BridgeSession | null>(null);
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const {
    isPolling: isPollingStatus,
    pollError,
    session: resolvedSession,
  } = useBridgeSessionStatus(session);

  const networkDestinations = destinations.filter((d) => d.network === networkMode);
  const liveDestinations = networkDestinations.filter((d) => d.enabled);
  const soonDestinations = networkDestinations.filter((d) => !d.enabled);

  const destination =
    networkDestinations.find((entry) => entry.id === selectedDestinationId) ??
    liveDestinations[0] ??
    networkDestinations[0];

  const recipientIsValid = useMemo(
    () => recipient.startsWith(destination.prefix) && recipient.length > destination.prefix.length + 8,
    [destination.prefix, recipient],
  );
  const amountIsValid = useMemo(() => {
    const parsed = Number(amount);
    return Number.isFinite(parsed) && parsed > 0;
  }, [amount]);

  const canSubmit =
    amountIsValid &&
    recipientIsValid &&
    wallet.isConnected &&
    !wallet.isWrongChain &&
    Boolean(wallet.address) &&
    Boolean(walletClient) &&
    !isSubmitting;

  function handleNetworkModeChange(mode: NetworkMode) {
    setNetworkMode(mode);
    const firstLive = destinations.find((d) => d.network === mode && d.enabled);
    setSelectedDestinationId(firstLive?.id ?? destinations[0].id);
    setRecipient("");
    setSubmissionError(null);
    setIsDestinationMenuOpen(false);
  }

  async function handleSubmit() {
    if (!walletClient || !wallet.address) {
      const chainLabel = networkMode === "mainnet" ? "Ethereum mainnet" : "Sepolia";
      setSubmissionError(`Connect an ${chainLabel} wallet extension to continue.`);
      return;
    }

    setIsSubmitting(true);
    setSubmissionError(null);

    const gatewayAddress = networkMode === "mainnet"
      ? frontendEnv.mainnetGatewayAddress
      : frontendEnv.gatewayAddress;

    try {
      const txHash = await submitEthDeposit({
        walletClient,
        gatewayAddress,
        account: wallet.address as Address,
        amountEth: amount,
        recipient: frontendEnv.aegislinkDepositRecipient,
      });
      await registerBridgeDeliveryIntent({
        sourceTxHash: txHash,
        sender: frontendEnv.aegislinkDepositRecipient,
        routeId: destination.routeId,
        assetId: "eth",
        amount: parseEther(amount).toString(),
        receiver: recipient,
      });

      setSession(
        createSubmittedBridgeSession({
          amountEth: amount,
          destinationChain: destination.label,
          recipient,
          sourceAddress: wallet.address,
          sourceTxHash: txHash,
          sourceChainId: getSourceChainForMode(networkMode).id,
        }),
      );
    } catch (error) {
      setSubmissionError(normalizeSubmissionError(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (resolvedSession) {
    return (
      <ProgressPanel
        isPolling={isPollingStatus}
        onReset={() => setSession(null)}
        pollError={pollError}
        session={resolvedSession}
      />
    );
  }

  const sourceChainLabel = networkMode === "mainnet" ? "Ethereum" : "Sepolia";

  return (
    <div className="transfer-card">
      <div className="transfer-card__header">
        <div>
          <p className="eyebrow eyebrow--dark">{sourceChainLabel} → Cosmos</p>
          <h2>Transfer</h2>
        </div>
        <div className="transfer-card__header-right">
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
          <div className={`wallet-chip${wallet.isWrongChain ? " wallet-chip--warning" : ""}`}>
            {wallet.isWrongChain
              ? "Wrong chain"
              : wallet.address
                ? `${wallet.address.slice(0, 6)}…${wallet.address.slice(-4)}`
                : "—"}
          </div>
        </div>
      </div>

      <div className="field-grid">
        <div className="transfer-row">
          <div className="field-card field-card--amount">
            <div className="amount-header">
              <label className="field-label" htmlFor="bridge-amount">
                Amount
              </label>
              <div className="amount-display">
                <span className="amount-display__value">
                  {parseFloat(amount).toFixed(3)}
                </span>
                <span className="amount-display__unit">ETH</span>
              </div>
            </div>
            <div className="amount-slider-wrap">
              <input
                aria-label="Amount"
                className="amount-slider"
                id="bridge-amount"
                max="1"
                min="0.001"
                onChange={(event) =>
                  setAmount(parseFloat(event.target.value).toFixed(3))
                }
                step="0.001"
                style={
                  {
                    "--slider-pct": `${Math.round(parseFloat(amount) * 100)}%`,
                  } as { [key: string]: string }
                }
                type="range"
                value={amount}
              />
            </div>
            <div className="amount-presets">
              {(["0.050", "0.100", "0.250", "0.500", "1.000"] as const).map(
                (v) => (
                  <button
                    className={`amount-preset${amount === v ? " amount-preset--active" : ""}`}
                    key={v}
                    onClick={() => setAmount(v)}
                    type="button"
                  >
                    {parseFloat(v)}
                  </button>
                ),
              )}
            </div>
          </div>

          <div className="field-card field-card--destination">
            <label className="field-label">To</label>
            <div className="chain-picker">
              <button
                aria-expanded={isDestinationMenuOpen}
                aria-haspopup="menu"
                aria-label={`Destination chain: ${destination.label}`}
                className="chain-trigger"
                onClick={() => setIsDestinationMenuOpen((value) => !value)}
                type="button"
              >
                <ChainAvatar destination={destination} size="md" />
                <span className="chain-trigger__name">{destination.shortName}</span>
                <span className="chain-trigger__chevron" aria-hidden="true">
                  {isDestinationMenuOpen ? "▲" : "▼"}
                </span>
              </button>

              {isDestinationMenuOpen ? (
                <div className="chain-menu" role="menu">
                  <p className="chain-menu__group">Live now</p>
                  {liveDestinations.map((option) => (
                    <button
                      aria-label={option.label}
                      className={`chain-option${option.id === destination.id ? " chain-option--active" : ""}`}
                      key={option.id}
                      onClick={() => {
                        setSelectedDestinationId(option.id);
                        setRecipient("");
                        setSubmissionError(null);
                        setIsDestinationMenuOpen(false);
                      }}
                      role="menuitem"
                      type="button"
                    >
                      <ChainAvatar destination={option} size="sm" />
                      <span className="chain-option__info">
                        <strong className="chain-option__name">{option.shortName}</strong>
                        <span className="chain-option__symbol">{option.symbol}</span>
                      </span>
                      <span className="chain-badge chain-badge--live">● Live</span>
                    </button>
                  ))}

                  <div className="chain-menu__divider" />
                  <p className="chain-menu__group">Coming soon</p>
                  {soonDestinations.map((option) => (
                    <button
                      aria-label={option.label}
                      className="chain-option chain-option--soon"
                      disabled
                      key={option.id}
                      role="menuitem"
                      type="button"
                    >
                      <ChainAvatar destination={option} size="sm" muted />
                      <span className="chain-option__info">
                        <strong className="chain-option__name">{option.shortName}</strong>
                        <span className="chain-option__symbol">{option.symbol}</span>
                      </span>
                      <span className="chain-badge chain-badge--soon">Soon</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        </div>

        <div className="field-card">
          <label className="field-label" htmlFor="bridge-recipient">
            Recipient address
          </label>
          <input
            aria-label="Recipient"
            className={`field-input field-input--recipient ${recipientIsValid ? "field-input--valid" : recipient.length > 0 ? "field-input--invalid" : ""}`}
            id="bridge-recipient"
            onChange={(event) => setRecipient(event.target.value)}
            placeholder={`${destination.prefix}1…`}
            value={recipient}
          />
          <span className="field-helper">
            Starts with <code className="field-prefix-hint">{destination.prefix}</code>
          </span>
          {recipient.length > 0 && !recipientIsValid ? (
            <p className="field-error">
              Must start with <strong>{destination.prefix}</strong> and be at least {destination.prefix.length + 9} characters.
            </p>
          ) : recipientIsValid ? (
            <p className="field-success">Valid address ✓</p>
          ) : null}
        </div>
      </div>

      {canSubmit ? (
        <div className="transfer-summary">
          <span className="transfer-summary__from">{amount} ETH</span>
          <span className="transfer-summary__arrow">→</span>
          <span className="transfer-summary__to">{destination.label}</span>
        </div>
      ) : !wallet.isConnected ? (
        <p className="submit-hint">Connect your {sourceChainLabel} wallet to continue.</p>
      ) : wallet.isWrongChain ? (
        <p className="submit-hint submit-hint--warn">
          Switch to {networkMode === "mainnet" ? "Ethereum mainnet" : "Sepolia"} to bridge.
        </p>
      ) : !amountIsValid ? (
        <p className="submit-hint">Enter a valid ETH amount above.</p>
      ) : !recipientIsValid ? (
        <p className="submit-hint">Enter a valid {destination.prefix} address above.</p>
      ) : null}

      <button
        className="primary-cta"
        disabled={!canSubmit}
        onClick={() => void handleSubmit()}
        type="button"
      >
        {isSubmitting
          ? "Opening Bridge Tunnel…"
          : canSubmit
            ? `Bridge ${amount} ETH → ${destination.symbol}`
            : `Bridge to ${destination.label}`}
      </button>
      {submissionError ? <p className="field-error field-error--spaced">{submissionError}</p> : null}
    </div>
  );
}

function normalizeSubmissionError(error: unknown) {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }

  return "The deposit could not be submitted. Please try again.";
}
```

- [ ] **Step 2: Add `.network-toggle` CSS to `global.css`**

Add inside the transfer card section (find the `.transfer-card__header` block and append after it):

```css
.transfer-card__header-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.network-toggle {
  display: flex;
  gap: 2px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 8px;
  padding: 3px;
}

.network-toggle__btn {
  padding: 4px 14px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.5);
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.network-toggle__btn--active {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}
```

- [ ] **Step 3: Run failing tests — they should now pass**

```bash
cd web && npx vitest run src/features/bridge/bridge.test.tsx 2>&1 | tail -20
```

Expected: all tests in `bridge.test.tsx` pass.

- [ ] **Step 4: Run full suite**

```bash
cd web && npx vitest run 2>&1 | tail -20
```

Expected: same overall pass count as before (App.test.tsx may need a minor mock update — see Task 9).

- [ ] **Step 5: Commit**

```bash
git add web/src/features/bridge/TransferPage.tsx web/src/styles/global.css
git commit -m "feat: add testnet/mainnet toggle to TransferPage with network-filtered destinations"
```

---

### Task 9: `App.test.tsx` — verify toggle default + fix mock if needed

**Files:**
- Modify: `web/src/app/App.test.tsx`

The test "opens the transfer page from the AegisLink dropdown" asserts:
```
screen.getByRole("button", { name: /destination chain: osmosis testnet \(osmo\)/i })
```
This still works because testnet is the default mode. No change needed there.

- [ ] **Step 1: Run App.test.tsx**

```bash
cd web && npx vitest run src/app/App.test.tsx 2>&1 | tail -20
```

Expected: all 5 tests pass. If any fail due to the `useBridgeWallet` mock receiving an unexpected arg, update the mock:

```ts
// Change this (lines 9-11 of App.test.tsx):
vi.mock("../features/wallet/useBridgeWallet", () => ({
  useBridgeWallet: () => useBridgeWalletMock(),
}));

// To this (accepts and ignores networkMode arg):
vi.mock("../features/wallet/useBridgeWallet", () => ({
  useBridgeWallet: (..._args: unknown[]) => useBridgeWalletMock(),
}));
```

- [ ] **Step 2: Run full suite and confirm**

```bash
cd web && npx vitest run 2>&1 | tail -10
```

Expected: all previously passing tests still pass.

- [ ] **Step 3: Commit if any changes were made**

```bash
git add web/src/app/App.test.tsx
git commit -m "test: update useBridgeWallet mock to accept networkMode arg"
```

---

### Task 10: Deploy config files — mainnet Ethereum + IBC

**Files:** 8 new JSON files

- [ ] **Step 1: Create `deploy/mainnet/ethereum/bridge-addresses.json`**

```json
{
  "chain_id": "1",
  "deployer_address": "",
  "verifier_address": "",
  "gateway_address": "",
  "notes": "Placeholder — deploy contract with scripts/mainnet/deploy_ethereum_bridge.sh before use"
}
```

- [ ] **Step 2: Create `deploy/mainnet/ethereum/bridge-assets.json`**

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

- [ ] **Step 3: Create `deploy/mainnet/ibc/osmosis-mainnet-wallet-delivery.json`**

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

- [ ] **Step 4: Create `deploy/mainnet/ibc/neutron-mainnet-wallet-delivery.json`**

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

- [ ] **Step 5: Create `deploy/mainnet/ibc/rly/osmosis-mainnet.chain.json`**

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

- [ ] **Step 6: Create `deploy/mainnet/ibc/rly/neutron-mainnet.chain.json`**

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

- [ ] **Step 7: Create `deploy/mainnet/ibc/rly/generated/paths/osmosis-mainnet-wallet.json`**

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

- [ ] **Step 8: Create `deploy/mainnet/ibc/rly/generated/paths/neutron-mainnet-wallet.json`**

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

- [ ] **Step 9: Commit all deploy configs**

```bash
git add deploy/mainnet/
git commit -m "feat: add mainnet deploy configs for Ethereum gateway and Osmosis/Neutron IBC routes"
```

---

### Task 11: `.env.mainnet.local.example` + mainnet startup script

**Files:**
- Create: `.env.mainnet.local.example`
- Create: `scripts/mainnet/start_public_bridge_backend.sh`

- [ ] **Step 1: Create `.env.mainnet.local.example`**

```bash
# Ethereum mainnet gateway (deployed contract address — placeholder until deployed)
VITE_BRIDGE_MAINNET_GATEWAY_ADDRESS=0x0000000000000000000000000000000000000000

# AegisLink mainnet node status API
VITE_BRIDGE_STATUS_API_BASE_URL=http://localhost:1318

# Osmosis mainnet destination
AEGISLINK_PUBLIC_BACKEND_DESTINATION_RPC_ADDR=https://rpc.osmosis.zone:443
AEGISLINK_PUBLIC_BACKEND_DESTINATION_GRPC_ADDR=https://grpc.osmosis.zone:443
AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_ID=osmosis-1
AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_NAME=osmosis-1
AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL=https://lcd.osmosis.zone
AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX=osmo
AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM=uosmo
AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT=0.025

# Funded Osmosis mainnet relayer key (OSMO for gas fees)
AEGISLINK_RLY_OSMOSIS_MAINNET_KEY_NAME=osmosis-mainnet-relayer-key
# Paste 24-word mnemonic for your funded Osmosis mainnet wallet:
AEGISLINK_RLY_OSMOSIS_MAINNET_MNEMONIC=""

# Neutron mainnet destination
AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR=https://rpc-kralum.neutron-1.neutron.org:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR=https://grpc-kralum.neutron-1.neutron.org:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL=https://rest-kralum.neutron-1.neutron.org
AEGISLINK_RLY_NEUTRON_MAINNET_KEY_NAME=neutron-mainnet-relayer-key
# Paste 24-word mnemonic for your funded Neutron mainnet wallet:
AEGISLINK_RLY_NEUTRON_MAINNET_MNEMONIC=""
```

- [ ] **Step 2: Create `scripts/mainnet/` directory and startup script**

```bash
mkdir -p scripts/mainnet
```

Create `scripts/mainnet/start_public_bridge_backend.sh` — full contents:

```bash
#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

# shellcheck source=/dev/null
source "$REPO_ROOT/scripts/testnet/lib_public_bridge_env.sh"

RUN_ID="${AEGISLINK_PUBLIC_BACKEND_RUN_ID:-$(date +%Y%m%d-%H%M%S)}"
HOME_DIR="${AEGISLINK_PUBLIC_BACKEND_HOME_DIR:-/tmp/aegislink-mainnet-home-$RUN_ID}"
RUNTIME_DIR="${AEGISLINK_PUBLIC_BACKEND_RUNTIME_DIR:-/tmp/aegislink-mainnet-backend-$RUN_ID}"
READY_FILE="$HOME_DIR/data/demo-node-ready.json"
NODE_LOG="$RUNTIME_DIR/node.log"
RELAYER_LOG="$RUNTIME_DIR/relayer.log"
REPLAY_STORE="$RUNTIME_DIR/replay.json"
ATTESTATION_STATE="$RUNTIME_DIR/attestations.json"
PATH_NAME="${AEGISLINK_PUBLIC_BACKEND_RLY_PATH_NAME:-live-osmo-mainnet-$RUN_ID}"
DESTINATION_LCD_BASE_URL="${AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL:-https://lcd.osmosis.zone}"
DESTINATION_RPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_RPC_ADDR:-https://rpc.osmosis.zone:443}"
DESTINATION_GRPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_GRPC_ADDR:-https://grpc.osmosis.zone:443}"
DESTINATION_CHAIN_NAME="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_NAME:-osmosis-1}"
DESTINATION_CHAIN_ID="${AEGISLINK_PUBLIC_BACKEND_DESTINATION_CHAIN_ID:-osmosis-1}"
DESTINATION_ACCOUNT_PREFIX="${AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX:-osmo}"
DESTINATION_GAS_PRICE_DENOM="${AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM:-uosmo}"
DESTINATION_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT:-0.025}"
RLY_TIMEOUT_SECONDS="${AEGISLINK_PUBLIC_BACKEND_RLY_TIMEOUT_SECONDS:-45}"
LINK_RETRIES="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_RETRIES:-3}"
LINK_RETRY_DELAY_SECONDS="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_RETRY_DELAY_SECONDS:-5}"
LINK_TIMEOUT_DURATION="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_TIMEOUT_DURATION:-5m}"
LINK_MAX_RETRIES="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_MAX_RETRIES:-6}"
LINK_BLOCK_HISTORY="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_BLOCK_HISTORY:-5}"
NEUTRON_CHAIN_NAME="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_CHAIN_NAME:-neutron-1}"
NEUTRON_CHAIN_ID="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_CHAIN_ID:-neutron-1}"
NEUTRON_RPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR:-https://rpc-kralum.neutron-1.neutron.org:443}"
NEUTRON_GRPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR:-https://grpc-kralum.neutron-1.neutron.org:443}"
NEUTRON_LCD_BASE_URL="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL:-https://rest-kralum.neutron-1.neutron.org}"
NEUTRON_ACCOUNT_PREFIX="${AEGISLINK_RLY_NEUTRON_ACCOUNT_PREFIX:-neutron}"
NEUTRON_GAS_PRICE_DENOM="${AEGISLINK_RLY_NEUTRON_GAS_PRICE_DENOM:-untrn}"
NEUTRON_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_NEUTRON_GAS_PRICE_AMOUNT:-0.025}"
NEUTRON_GAS_ADJUSTMENT="${AEGISLINK_RLY_NEUTRON_GAS_ADJUSTMENT:-2.0}"
NEUTRON_KEY_NAME="${AEGISLINK_RLY_NEUTRON_MAINNET_KEY_NAME:-neutron-mainnet-relayer-key}"
NEUTRON_MNEMONIC="${AEGISLINK_RLY_NEUTRON_MAINNET_MNEMONIC:-}"
NEUTRON_PATH_NAME="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_PATH_NAME:-live-ntrn-mainnet-$RUN_ID}"
NEUTRON_ROUTE_ID="neutron-mainnet-wallet"
PERSISTENT_RLY_HOME="${AEGISLINK_RELAYER_RLY_HOME:-$HOME/.aegislink-mainnet-rly}"
GOCACHE="${GOCACHE:-/tmp/aegislink-gocache}"
STATUS_FILE="$RUNTIME_DIR/status.json"
CURRENT_STATUS_FILE="${AEGISLINK_PUBLIC_BACKEND_CURRENT_STATUS_FILE:-/tmp/aegislink-mainnet-backend-current.json}"

# Source mainnet env overrides if present
if [[ -f "$REPO_ROOT/.env.mainnet.local" ]]; then
  source_required_env "$REPO_ROOT/.env.mainnet.local"
  NEUTRON_MNEMONIC="${AEGISLINK_RLY_NEUTRON_MAINNET_MNEMONIC:-$NEUTRON_MNEMONIC}"
  NEUTRON_KEY_NAME="${AEGISLINK_RLY_NEUTRON_MAINNET_KEY_NAME:-$NEUTRON_KEY_NAME}"
fi

OSMOSIS_ROUTE_ID="osmosis-mainnet-wallet"

echo "AegisLink mainnet backend — RUN_ID=$RUN_ID"
echo "Osmosis mainnet: $DESTINATION_RPC_ADDR"
echo "Neutron mainnet: $NEUTRON_RPC_ADDR"
echo "Runtime dir: $RUNTIME_DIR"
```

- [ ] **Step 3: Make the script executable**

```bash
chmod +x scripts/mainnet/start_public_bridge_backend.sh
```

- [ ] **Step 4: Commit**

```bash
git add .env.mainnet.local.example scripts/mainnet/start_public_bridge_backend.sh
git commit -m "feat: add mainnet env template and backend startup script"
```

---

### Task 12: Final verification

- [ ] **Step 1: Run the complete test suite**

```bash
cd web && npx vitest run 2>&1 | tail -20
```

Expected: all previously passing tests still pass, plus the 8 new network toggle tests pass.

- [ ] **Step 2: Start the dev server and manually verify**

```bash
cd web && npm run dev
```

Open `http://localhost:5173`. Verify:
- Transfer page loads in Testnet mode by default
- Toggle shows "Testnet" active
- Destination picker shows Osmosis Testnet + Neutron Testnet as Live, testnet chains as Coming Soon
- Click "Mainnet" toggle — eyebrow changes to "Ethereum → Cosmos"
- Destination picker shows Osmosis Mainnet + Neutron Mainnet as Live, mainnet chains as Coming Soon
- Switching back to Testnet resets to Osmosis Testnet
- Recipient clears on each network mode switch

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "feat: complete mainnet/testnet toggle — all tests pass"
```
