import { sourceChainName } from "../../lib/evm/chains";
import { useBridgeWallet } from "./useBridgeWallet";

export function WalletStatus() {
  const wallet = useBridgeWallet();

  if (!wallet.isConnected) {
    return (
      <div className="wallet-chip">
        {wallet.hasInjectedWallet ? "Wallet disconnected" : "No wallet extension"}
      </div>
    );
  }

  if (wallet.isWrongChain) {
    return (
      <div className="wallet-chip wallet-chip--warning">
        Wrong chain: {wallet.chainName ?? "Unknown"} · switch to {sourceChainName}
      </div>
    );
  }

  return <div className="wallet-chip">Wallet connected · {wallet.chainName}</div>;
}
