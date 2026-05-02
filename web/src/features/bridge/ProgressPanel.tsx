import { frontendEnv } from "../../lib/config/env";
import type { BridgeSession } from "./bridge-session";
import { deriveTransferProgressModel, type TransferVisualStageId } from "./transfer-progress";
import { BridgeWormholeScene, DESTINATION_RIGHT_RGB } from "./BridgeWormholeScene";

function resolveDestinationKey(destinationChain: string): string {
  return destinationChain.split(" ")[0].toLowerCase();
}

function resolveDestinationShortName(destinationChain: string): string {
  return destinationChain.split(" ")[0];
}

type ProgressPanelProps = {
  isPolling?: boolean;
  onReset: () => void;
  pollError?: string | null;
  session: BridgeSession;
};

export function ProgressPanel({
  isPolling = false,
  onReset,
  pollError = null,
  session,
}: ProgressPanelProps) {
  const destinationTxUrl = resolveDestinationTxUrl(session);
  const progress = deriveTransferProgressModel(session, isPolling);
  const currentStageId =
    (progress.stages.find((stage) => stage.state === "current")?.id ?? "sepolia") as TransferVisualStageId;
  const destKey = resolveDestinationKey(session.destinationChain);
  const destShortName = resolveDestinationShortName(session.destinationChain);
  const rightRgb = DESTINATION_RIGHT_RGB[destKey] ?? DESTINATION_RIGHT_RGB.osmosis;

  return (
    <div className="transfer-card transfer-card--progress transfer-card--progress-expanded transfer-card--progress-obsidian transfer-card--progress-contained">
      <div className="progress-shell progress-shell--ignited">
        <div className="progress-shell__top progress-summary-bar">
          <div className="progress-manifest">
            <p className="eyebrow">Bridge Session</p>
            <h2>Transfer in progress</h2>
            <small>Transfer route</small>
            <strong>{session.amountEth} ETH</strong>
            <div className="progress-manifest__route" aria-label={`Sepolia to ${destShortName} route`}>
              <span>Sepolia</span>
              <i aria-hidden="true" />
              <span>{destShortName}</span>
            </div>
            <p>{session.recipient}</p>
          </div>

          <div className="progress-live">
            <small>{progress.sceneLabel}</small>
            <h3>{progress.headline}</h3>
            <p>{progress.summary}</p>
            <div className="wallet-chip wallet-chip--progress wallet-chip--progress-live">
              {progress.chipLabel}
            </div>
          </div>
        </div>

        <BridgeWormholeScene
          activeStageId={currentStageId}
          stages={progress.stages}
          rightRgb={rightRgb}
        />

        <div className="progress-proof-grid">
          <div className="progress-proof-card">
            <small>Source transaction</small>
            <a
              className="tx-link"
              href={`${frontendEnv.sepoliaExplorerBaseUrl}/tx/${session.sourceTxHash}`}
              rel="noreferrer"
              target="_blank"
            >
              {shortHash(session.sourceTxHash)}
            </a>
            <span>{session.sourceAddress}</span>
          </div>

          <div className="progress-proof-card">
            <small>Destination receipt</small>
            {session.destinationTxHash ? (
              destinationTxUrl ? (
                <a
                  className="tx-link"
                  href={destinationTxUrl}
                  rel="noreferrer"
                  target="_blank"
                >
                  {shortHash(session.destinationTxHash)}
                </a>
              ) : (
                <strong>{shortHash(session.destinationTxHash)}</strong>
              )
            ) : (
              <strong>Waiting for final destination hash</strong>
            )}
            <span>
              {session.destinationTxHash
                ? "Confirmed by the configured bridge status source."
                : `This appears as soon as the operator tracking endpoint observes the ${destShortName} receipt.`}
            </span>
          </div>
        </div>

        {pollError ? <p className="progress-alert">{pollError}</p> : null}

        <div className="progress-actions">
          <button className="secondary-cta" onClick={onReset} type="button">
            Start New Transfer
          </button>
        </div>
      </div>
    </div>
  );
}

function shortHash(hash: string) {
  return `${hash.slice(0, 10)}...${hash.slice(-8)}`;
}

function resolveDestinationTxUrl(session: BridgeSession) {
  if (!session.destinationTxHash) {
    return undefined;
  }
  if (session.destinationTxUrl) {
    return normalizeDestinationTxUrl(session.destinationTxUrl);
  }
  if (session.destinationChain === "Osmosis Testnet (OSMO)") {
    return `https://www.mintscan.io/osmosis-testnet/tx/${session.destinationTxHash}`;
  }
  if (session.destinationChain === "Osmosis Mainnet (OSMO)") {
    return `https://www.mintscan.io/osmosis/tx/${session.destinationTxHash}`;
  }
  return undefined;
}

function normalizeDestinationTxUrl(url: string) {
  return url
    .replace("https://www.mintscan.io/osmosis-testnet/txs/", "https://www.mintscan.io/osmosis-testnet/tx/")
    .replace("https://www.mintscan.io/osmosis/txs/", "https://www.mintscan.io/osmosis/tx/");
}
