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
            <p className="eyebrow">Ethereum → Cosmos Bridge</p>
            <h1>Move ETH across chains, in one step.</h1>
            <p className="hero-copy">
              Connect your Sepolia wallet, pick a Cosmos destination, and bridge.
              AegisLink handles verification, IBC routing, and live delivery tracking end-to-end.
            </p>

            <div className="hero-destinations">
              <span className="hero-destination hero-destination--live">
                <i className="hero-destination__dot" />
                Osmosis Testnet
              </span>
              <span className="hero-destination hero-destination--live">
                <i className="hero-destination__dot" />
                Neutron Testnet
              </span>
              <span className="hero-destination hero-destination--soon">
                Celestia · dYdX · Injective · soon
              </span>
            </div>

            <div className="hero-actions">
              {wallet.isConnected ? (
                <button
                  className="primary-cta primary-cta--hero"
                  onClick={handleSelectTransfer}
                  type="button"
                >
                  Start bridging
                </button>
              ) : (
                <button
                  className="primary-cta primary-cta--hero"
                  onClick={handleSelectTransfer}
                  type="button"
                >
                  Connect wallet &amp; bridge
                </button>
              )}
              <button
                className="ghost-cta"
                onClick={handleSelectAbout}
                type="button"
              >
                How it works
              </button>
            </div>

            <div className="hero-meta">
              <span>Sepolia source</span>
              <span>IBC delivery</span>
              <span>Live status tracking</span>
              <span>No custodian</span>
            </div>
          </>
        )}
      </section>
    </main>
  );
}
