import type { CSSProperties } from "react";
import { frontendEnv } from "../../lib/config/env";
import type { BridgeSession } from "./bridge-session";
import { deriveTransferProgressModel } from "./transfer-progress";

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
    progress.stages.find((stage) => stage.state === "current")?.id ?? "sepolia";
  const progressSceneClassName = [
    "progress-scene",
    "progress-scene--abyss",
    "progress-scene--ignited",
    `progress-scene--stage-${currentStageId}`,
  ].join(" ");
  const containedSceneStyle = {
    "--progress-contained-scene-scale": "0.92",
    "--progress-contained-core-top": "127px",
    "--progress-contained-bridge-top": "116px",
    "--progress-contained-bridge-height": "118px",
    "--progress-contained-core-wordmark-scale": "0.8",
  } as CSSProperties;

  return (
    <div className="transfer-card transfer-card--progress transfer-card--progress-expanded transfer-card--progress-obsidian transfer-card--progress-contained">
      <div className="progress-shell progress-shell--ignited">
        <div className="progress-shell__top">
          <div className="progress-manifest">
            <p className="eyebrow">Bridge Session</p>
            <h2>Transfer in progress</h2>
            <small>Transfer manifest</small>
            <strong>{session.amountEth} ETH</strong>
            <span>{session.destinationChain}</span>
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

        <div aria-label={progress.sceneLabel} className={progressSceneClassName}>
          <div
            className="progress-scene__viewport"
            data-testid="progress-scene-viewport"
            style={containedSceneStyle}
          >
            <div className="progress-scene__portal progress-scene__portal--left" />
            <div className="progress-scene__portal progress-scene__portal--right" />

            <div aria-hidden="true" className="progress-scene__bridge-glow" data-testid="progress-bridge-glow">
              <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--portal-left" />
              <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--core" />
              <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--portal-right" />
            </div>

            <div className="progress-scene__core">
              <div aria-hidden="true" className="progress-scene__core-aura" data-testid="progress-core-aura" />
              <div aria-hidden="true" className="progress-scene__core-shell" data-testid="progress-core-shell" />
              <div className="progress-scene__core-wordmark">
                <strong>AegisLink</strong>
              </div>
            </div>

            <span className="progress-scene__chain-label progress-scene__chain-label--left">
              Sepolia
            </span>
            <span className="progress-scene__chain-label progress-scene__chain-label--right">
              Osmosis
            </span>
          </div>
        </div>

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
                : "This appears as soon as the operator tracking endpoint observes the Osmosis receipt."}
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
