import { WalletConnectButton } from "../../features/wallet/WalletConnectButton";

export function LandingPage() {
  return (
    <main className="page page--landing">
      <header className="topbar">
        <div className="wordmark">AegisLink</div>
        <WalletConnectButton />
      </header>

      <section className="hero">
        <p className="eyebrow">Ethereum to Cosmos bridge surface</p>
        <h1>Connect Ethereum to the Cosmos ecosystem.</h1>
        <p className="hero-copy">
          AegisLink gives users one clear bridge action, one premium entry
          point, and one place to track a transfer from Sepolia through final
          destination delivery.
        </p>
        <div className="hero-meta">
          <span>Sepolia source</span>
          <span>Cosmos destinations</span>
          <span>Transaction visibility</span>
        </div>
      </section>
    </main>
  );
}
