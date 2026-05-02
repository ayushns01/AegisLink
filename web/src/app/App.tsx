import { LandingPage } from "./layout/LandingPage";
import { TransferWormholePreview } from "../features/bridge/TransferWormholePreview";

export function App() {
  if (shouldRenderWormholePreview()) {
    return <TransferWormholePreview />;
  }

  return (
    <>
      <div className="bg-moon" aria-hidden="true" />
      <LandingPage />
    </>
  );
}

function shouldRenderWormholePreview() {
  if (typeof window === "undefined") {
    return false;
  }

  const params = new URLSearchParams(window.location.search);

  return params.get("preview") === "wormhole" || window.location.pathname === "/wormhole-preview";
}
