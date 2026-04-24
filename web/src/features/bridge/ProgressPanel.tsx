import type { CSSProperties } from "react";
import { frontendEnv } from "../../lib/config/env";
import type { BridgeSession } from "./bridge-session";
import { deriveTransferProgressModel, type TransferVisualStageId } from "./transfer-progress";

type ProgressPanelProps = {
  isPolling?: boolean;
  onReset: () => void;
  pollError?: string | null;
  session: BridgeSession;
};

// ── Particle definitions ──────────────────────────────────────────────────────
type ParticleSegment = "left" | "center" | "right";

type ParticleDef = {
  top: number;     // % vertical position in scene
  size: number;    // px
  blur: number;    // px
  baseDelay: number;   // s — spread particles over time
  baseDuration: number; // s — baseline travel time
  segment: ParticleSegment;
};

const PARTICLES: ParticleDef[] = [
  // Left (pink — Sepolia origin)
  { top: 44, size: 9,  blur: 9,  baseDelay: 0,    baseDuration: 55, segment: "left" },
  { top: 48, size: 6,  blur: 6,  baseDelay: 2.4,  baseDuration: 65, segment: "left" },
  { top: 52, size: 11, blur: 11, baseDelay: 0.9,  baseDuration: 50, segment: "left" },
  { top: 40, size: 5,  blur: 5,  baseDelay: 4.2,  baseDuration: 60, segment: "left" },
  { top: 56, size: 7,  blur: 7,  baseDelay: 6.8,  baseDuration: 45, segment: "left" },
  { top: 46, size: 10, blur: 10, baseDelay: 8.5,  baseDuration: 70, segment: "left" },
  { top: 50, size: 4,  blur: 4,  baseDelay: 5.6,  baseDuration: 55, segment: "left" },
  // Center (blue — AegisLink hub)
  { top: 46, size: 12, blur: 12, baseDelay: 1.5,  baseDuration: 60, segment: "center" },
  { top: 54, size: 7,  blur: 7,  baseDelay: 3.8,  baseDuration: 50, segment: "center" },
  { top: 49, size: 5,  blur: 5,  baseDelay: 7.4,  baseDuration: 65, segment: "center" },
  { top: 52, size: 9,  blur: 9,  baseDelay: 0.6,  baseDuration: 55, segment: "center" },
  { top: 43, size: 6,  blur: 6,  baseDelay: 9.2,  baseDuration: 70, segment: "center" },
  { top: 58, size: 4,  blur: 4,  baseDelay: 2.9,  baseDuration: 50, segment: "center" },
  { top: 48, size: 8,  blur: 8,  baseDelay: 6.1,  baseDuration: 60, segment: "center" },
  // Right (gold — Osmosis destination)
  { top: 50, size: 12, blur: 12, baseDelay: 1.1,  baseDuration: 50, segment: "right" },
  { top: 44, size: 6,  blur: 6,  baseDelay: 3.5,  baseDuration: 65, segment: "right" },
  { top: 57, size: 8,  blur: 8,  baseDelay: 7.0,  baseDuration: 55, segment: "right" },
  { top: 48, size: 5,  blur: 5,  baseDelay: 9.8,  baseDuration: 45, segment: "right" },
  { top: 52, size: 10, blur: 10, baseDelay: 4.9,  baseDuration: 70, segment: "right" },
  { top: 46, size: 4,  blur: 4,  baseDelay: 7.6,  baseDuration: 60, segment: "right" },
  { top: 54, size: 7,  blur: 7,  baseDelay: 0.4,  baseDuration: 65, segment: "right" },
  { top: 41, size: 9,  blur: 9,  baseDelay: 2.7,  baseDuration: 50, segment: "right" },
];

const STAGE_CONFIG: Record<TransferVisualStageId, {
  left: { speed: number; opacity: number };
  center: { speed: number; opacity: number };
  right: { speed: number; opacity: number };
}> = {
  sepolia: {
    left:   { speed: 0.5, opacity: 1.0 },
    center: { speed: 1.2, opacity: 0.35 },
    right:  { speed: 1.5, opacity: 0.2  },
  },
  verify: {
    left:   { speed: 0.9, opacity: 0.55 },
    center: { speed: 0.45, opacity: 1.0 },
    right:  { speed: 1.4, opacity: 0.25 },
  },
  accounting: {
    left:   { speed: 1.1, opacity: 0.4  },
    center: { speed: 0.6, opacity: 0.9  },
    right:  { speed: 0.9, opacity: 0.55 },
  },
  handoff: {
    left:   { speed: 1.4, opacity: 0.25 },
    center: { speed: 0.85, opacity: 0.6  },
    right:  { speed: 0.45, opacity: 1.0 },
  },
  receipt: {
    left:   { speed: 0.9, opacity: 0.5  },
    center: { speed: 0.9, opacity: 0.5  },
    right:  { speed: 0.9, opacity: 0.75 },
  },
};

const SEGMENT_COLOR: Record<ParticleSegment, string> = {
  left:   "rgba(255, 130, 190, VAR)",
  center: "rgba(138, 198, 255, VAR)",
  right:  "rgba(255, 210, 120, VAR)",
};

function makeColor(segment: ParticleSegment, opacity: number): string {
  return SEGMENT_COLOR[segment].replace("VAR", String(opacity.toFixed(2)));
}

const CHECKPOINT_POSITIONS: Record<TransferVisualStageId, string> = {
  sepolia: "11%",
  verify: "32%",
  accounting: "50%",
  handoff: "68%",
  receipt: "89%",
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
  const stageConfig = STAGE_CONFIG[currentStageId];

  const progressSceneClassName = [
    "progress-scene",
    "progress-scene--abyss",
    "progress-scene--ignited",
    `progress-scene--stage-${currentStageId}`,
  ].join(" ");

  return (
    <div className="transfer-card transfer-card--progress transfer-card--progress-expanded transfer-card--progress-obsidian transfer-card--progress-contained">
      <div className="progress-shell progress-shell--ignited">
        <div className="progress-shell__top progress-summary-bar">
          <div className="progress-manifest">
            <p className="eyebrow">Bridge Session</p>
            <h2>Transfer in progress</h2>
            <small>Transfer route</small>
            <strong>{session.amountEth} ETH</strong>
            <div className="progress-manifest__route" aria-label="Sepolia to Osmosis route">
              <span>Sepolia</span>
              <i aria-hidden="true" />
              <span>Osmosis</span>
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

        <div aria-label={progress.sceneLabel} className={progressSceneClassName}>
            <div
              className="progress-scene__viewport"
              data-testid="progress-scene-viewport"
            >
            {/* ── Twinkling background stars ── */}
            <div className="progress-scene__stars" aria-hidden="true">
              {Array.from({ length: 26 }).map((_, i) => (
                <span
                  key={i}
                  className="progress-scene__star"
                  style={{
                    left: `${(i * 41 + 7) % 100}%`,
                    top: `${(i * 59 + 11) % 100}%`,
                    animationDelay: `${(i * 0.38) % 4.2}s`,
                    animationDuration: `${2.8 + (i % 5) * 0.6}s`,
                    width: i % 5 === 0 ? "2px" : "1.5px",
                    height: i % 5 === 0 ? "2px" : "1.5px",
                    opacity: i % 3 === 0 ? 0.85 : 0.45,
                  }}
                />
              ))}
            </div>

            {/* ── Stage-reactive wormhole particles ── */}
            <div className="progress-scene__particles" aria-hidden="true">
              {PARTICLES.map((p, i) => {
                const cfg = stageConfig[p.segment];
                const duration = p.baseDuration * cfg.speed;
                const color = makeColor(p.segment, cfg.opacity * 0.85);
                const glowColor = makeColor(p.segment, cfg.opacity * 0.55);
                return (
                  <span
                    key={i}
                    className="ps-particle"
                    style={{
                      top: `${p.top}%`,
                      width: `${p.size}px`,
                      height: `${p.size}px`,
                      background: color,
                      boxShadow: `0 0 ${p.blur}px ${Math.round(p.blur / 2)}px ${glowColor}`,
                      filter: `blur(${p.size > 8 ? 4 : 3}px)`,
                      animationDelay: `${p.baseDelay}s`,
                      animationDuration: `${duration}s`,
                      opacity: cfg.opacity,
                    }}
                  />
                );
              })}
            </div>

            {/* ── Left portal (Sepolia — pink) ── */}
            <div className="progress-scene__portal progress-scene__portal--left" />

            {/* ── Right portal (Osmosis — gold) ── */}
            <div className="progress-scene__portal progress-scene__portal--right" />

            {/* ── Bridge glow bands ── */}
            <div aria-hidden="true" className="progress-scene__bridge-glow" data-testid="progress-bridge-glow">
              <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--portal-left" />
              <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--core" />
              <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--portal-right" />
            </div>

            <div className="progress-route-corridor" aria-label="Bridge route checkpoints">
              {progress.stages.map((stage, index) => (
                <span
                  aria-label={stage.label}
                  className={[
                    "progress-route__checkpoint",
                    `progress-route__checkpoint--${stage.state}`,
                  ].join(" ")}
                  data-label={stage.label}
                  key={stage.id}
                  style={
                    {
                      "--checkpoint-x": CHECKPOINT_POSITIONS[stage.id],
                      "--checkpoint-index": String(index + 1).padStart(2, "0"),
                    } as CSSProperties
                  }
                />
              ))}
            </div>

            {/* ── Core node ── */}
            <div className="progress-scene__core">
              <div aria-hidden="true" className="progress-scene__core-aura" data-testid="progress-core-aura" />
              <div aria-hidden="true" className="progress-scene__core-halo" />
              <div aria-hidden="true" className="progress-scene__core-shell" data-testid="progress-core-shell" />
              <div className="progress-scene__core-wordmark">
                <strong>AegisLink</strong>
              </div>
            </div>


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
