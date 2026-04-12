import { useMemo } from "react";
import {
  useAccount,
  useConnect,
  useDisconnect,
  useSwitchChain,
} from "wagmi";
import { sourceChain } from "../../lib/evm/chains";

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

export function useBridgeWallet(): BridgeWalletState {
  const { address, chain, isConnected } = useAccount();
  const { connectAsync, connectors, error, isPending } = useConnect();
  const { disconnect } = useDisconnect();
  const { switchChainAsync } = useSwitchChain();
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
      switchChainAsync,
    ],
  );
}
