import { sourceChainName } from "../../lib/evm/chains";
import { useBridgeWallet } from "./useBridgeWallet";

export function WalletConnectButton() {
  const wallet = useBridgeWallet();

  if (!wallet.hasInjectedWallet) {
    return (
      <button className="connect-button connect-button--warning" disabled type="button">
        Install Wallet Extension
      </button>
    );
  }

  if (wallet.isConnected && wallet.isWrongChain) {
    return (
      <button
        className="connect-button connect-button--warning"
        onClick={() => void wallet.switchToSourceChain()}
        type="button"
      >
        Switch to {sourceChainName}
      </button>
    );
  }

  if (wallet.isConnected && wallet.address) {
    return (
      <button className="connect-button" onClick={wallet.disconnect} type="button">
        {shortAddress(wallet.address)}
      </button>
    );
  }

  return (
    <button className="connect-button" onClick={() => void wallet.connect()} type="button">
      {wallet.isConnecting ? "Connecting..." : "Connect Wallet"}
    </button>
  );
}

function shortAddress(address: string) {
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}
