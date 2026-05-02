import type { CSSProperties } from "react";
import type { TransferVisualStage, TransferVisualStageId } from "./transfer-progress";

// ── Particle definitions ──────────────────────────────────────────────────────
type ParticleSegment = "left" | "center" | "right";

type ParticleDef = {
  top: number;
  size: number;
  blur: number;
  baseDelay: number;
  baseDuration: number;
  segment: ParticleSegment;
};

export const PARTICLES: ParticleDef[] = [
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

export const STAGE_CONFIG: Record<TransferVisualStageId, {
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

export const CHECKPOINT_POSITIONS: Record<TransferVisualStageId, string> = {
  sepolia: "11%",
  verify: "32%",
  accounting: "50%",
  handoff: "68%",
  receipt: "89%",
};

type BridgeWormholeSceneProps = {
  activeStageId: TransferVisualStageId;
  stages: TransferVisualStage[];
  onCheckpointClick?: (stageId: TransferVisualStageId) => void;
};

export function BridgeWormholeScene({
  activeStageId,
  stages,
  onCheckpointClick,
}: BridgeWormholeSceneProps) {
  const stageConfig = STAGE_CONFIG[activeStageId];

  const progressSceneClassName = [
    "progress-scene",
    "progress-scene--abyss",
    "progress-scene--ignited",
    `progress-scene--stage-${activeStageId}`,
  ].join(" ");

  return (
    <div aria-label="Bridge tunnel" className={progressSceneClassName}>
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
          {stages.map((stage, index) =>
            onCheckpointClick ? (
              <button
                aria-label={stage.label}
                className={[
                  "progress-route__checkpoint",
                  `progress-route__checkpoint--${stage.state}`,
                  "progress-route__checkpoint--interactive",
                ].join(" ")}
                data-label={stage.label}
                key={stage.id}
                onClick={() => onCheckpointClick(stage.id)}
                style={
                  {
                    "--checkpoint-x": CHECKPOINT_POSITIONS[stage.id],
                    "--checkpoint-index": String(index + 1).padStart(2, "0"),
                  } as CSSProperties
                }
                type="button"
              />
            ) : (
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
            )
          )}
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
  );
}
