import { SidebarNav } from "./SidebarNav";
import { TransferPage } from "../../features/bridge/TransferPage";
import { WalletStatus } from "../../features/wallet/WalletStatus";

export function AppShell() {
  return (
    <main className="page page--app">
      <header className="topbar topbar--app">
        <div className="wordmark">AegisLink</div>
        <div className="wallet-pill">
          <WalletStatus />
        </div>
      </header>

      <div className="app-shell">
        <SidebarNav />
        <section className="content-panel">
          <TransferPage />
        </section>
      </div>
    </main>
  );
}
