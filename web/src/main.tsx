import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider, createConfig, http } from "wagmi";
import { injected } from "wagmi/connectors";
import { App } from "./app/App";
import { sourceChain } from "./lib/evm/chains";
import "./styles/tokens.css";
import "./styles/global.css";

const queryClient = new QueryClient();
const config = createConfig({
  chains: [sourceChain],
  connectors: [injected()],
  transports: {
    [sourceChain.id]: http(),
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
