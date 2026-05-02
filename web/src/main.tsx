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
