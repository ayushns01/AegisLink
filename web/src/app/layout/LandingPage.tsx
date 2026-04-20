import { useState } from "react";
import { AboutSection } from "../../features/about/AboutSection";
import { TransferPage } from "../../features/bridge/TransferPage";
import { WalletConnectButton } from "../../features/wallet/WalletConnectButton";
import { useBridgeWallet } from "../../features/wallet/useBridgeWallet";

export function LandingPage() {
  const wallet = useBridgeWallet();
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [activeView, setActiveView] = useState<"hero" | "transfer" | "about">("hero");

  function handleSelectTransfer() {
    setActiveView("transfer");
    setIsMenuOpen(false);
  }

  function handleSelectAbout() {
    setActiveView("about");
    setIsMenuOpen(false);
  }

  return (
    <main className="page page--landing">
      <header className="topbar">
        <div className="brand-control">
          <div className="brand-menu">
            <button
              aria-expanded={isMenuOpen}
              aria-haspopup="menu"
              aria-label="Open AegisLink menu"
              className="wordmark-button"
              onClick={() => setIsMenuOpen((value) => !value)}
              type="button"
            >
              <span className="wordmark">AegisLink</span>
            </button>

            {isMenuOpen ? (
              <div className="brand-menu__dropdown" role="menu">
                <button
                  className="brand-menu__item"
                  onClick={handleSelectTransfer}
                  role="menuitem"
                  type="button"
                >
                  Transfer
                </button>
                <button
                  className="brand-menu__item"
                  onClick={handleSelectAbout}
                  role="menuitem"
                  type="button"
                >
                  About
                </button>
              </div>
            ) : null}
          </div>
        </div>
        <WalletConnectButton />
      </header>

      <section
        className={
          activeView === "transfer"
            ? "hero hero--with-card"
            : activeView === "about"
              ? "hero hero--with-card hero--with-about-page"
            : "hero"
        }
      >
        {activeView === "transfer" ? (
          <div className="landing-transfer-card landing-transfer-card--compact">
            <TransferPage />
          </div>
        ) : activeView === "about" ? (
          <div className="about-page-shell">
            <AboutSection />
          </div>
        ) : (
          <>
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
          </>
        )}
      </section>
    </main>
  );
}
