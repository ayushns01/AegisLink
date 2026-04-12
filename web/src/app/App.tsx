import { useBridgeWallet } from "../features/wallet/useBridgeWallet";
import { AppShell } from "./layout/AppShell";
import { LandingPage } from "./layout/LandingPage";

export function App() {
  const wallet = useBridgeWallet();

  if (!wallet.isConnected) {
    return <LandingPage />;
  }

  return <AppShell />;
}
