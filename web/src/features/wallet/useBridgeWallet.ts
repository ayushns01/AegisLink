import { useMemo } from "react";
import {
  useAccount,
  useConnect,
  useDisconnect,
  useSwitchChain,
} from "wagmi";
import { injected } from "wagmi/connectors";
import { sourceChain } from "../../lib/evm/chains";

export type BridgeWalletState = {
  address?: string;
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
  const { connectAsync, isPending } = useConnect();
  const { disconnect } = useDisconnect();
  const { switchChainAsync } = useSwitchChain();

  return useMemo(
    () => ({
      address,
      isConnected,
      isConnecting: isPending,
      isWrongChain: Boolean(isConnected && chain?.id !== sourceChain.id),
      chainName: chain?.name,
      connect: async () => {
        await connectAsync({
          connector: injected(),
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
      disconnect,
      isConnected,
      isPending,
      switchChainAsync,
    ],
  );
}
