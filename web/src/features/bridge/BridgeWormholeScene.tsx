import type { CSSProperties } from "react";
import type { TransferVisualStage, TransferVisualStageId } from "./transfer-progress";

export const CHECKPOINT_POSITIONS: Record<TransferVisualStageId, string> = {
  sepolia: "11%",
  verify: "32%",
  accounting: "50%",
  handoff: "68%",
  receipt: "89%",
};

// "r, g, b" string keyed by first word of destinationChain (lowercased)
export const DESTINATION_RIGHT_RGB: Record<string, string> = {
  osmosis: "255, 210, 120",
  neutron: "90, 140, 255",
};
const DEFAULT_RIGHT_RGB = DESTINATION_RIGHT_RGB.osmosis;

type BridgeWormholeSceneProps = {
  activeStageId: TransferVisualStageId;
  stages: TransferVisualStage[];
  onCheckpointClick?: (stageId: TransferVisualStageId) => void;
  onCheckpointEnter?: (stageId: TransferVisualStageId) => void;
  onPortalEnter?: (stageId: TransferVisualStageId) => void;
  /** "r, g, b" string for the destination (right) portal colour */
  rightRgb?: string;
};

export function BridgeWormholeScene({
  activeStageId,
  stages,
  onCheckpointClick,
  onCheckpointEnter,
  onPortalEnter,
  rightRgb = DEFAULT_RIGHT_RGB,
}: BridgeWormholeSceneProps) {
  const progressSceneClassName = [
    "progress-scene",
    "progress-scene--abyss",
    "progress-scene--ignited",
    `progress-scene--stage-${activeStageId}`,
  ].join(" ");

  return (
    <div
      aria-label="Bridge tunnel"
      className={progressSceneClassName}
      style={{ "--right-rgb": rightRgb } as { [key: string]: string }}
    >
      <div
        className="progress-scene__viewport"
        data-testid="progress-scene-viewport"
      >
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

        <div
          className={`progress-scene__portal progress-scene__portal--left${onPortalEnter ? " progress-scene__portal--interactive" : ""}`}
          onClick={onPortalEnter ? () => onPortalEnter("sepolia") : undefined}
          onMouseEnter={onPortalEnter ? () => onPortalEnter("sepolia") : undefined}
        />

        <div
          className={`progress-scene__portal progress-scene__portal--right${onPortalEnter ? " progress-scene__portal--interactive" : ""}`}
          onClick={onPortalEnter ? () => onPortalEnter("receipt") : undefined}
          onMouseEnter={onPortalEnter ? () => onPortalEnter("receipt") : undefined}
        />

        <div aria-hidden="true" className="progress-scene__bridge-glow" data-testid="progress-bridge-glow">
          <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--portal-left" />
          <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--core" />
          <div className="progress-scene__bridge-glow-band progress-scene__bridge-glow--portal-right" />
        </div>

        <div className="progress-route-corridor" aria-label="Bridge route checkpoints">
          {stages.map((stage, index) => {
            const isInteractive = Boolean(onCheckpointClick || onCheckpointEnter);
            const sharedStyle = {
              "--checkpoint-x": CHECKPOINT_POSITIONS[stage.id],
              "--checkpoint-index": String(index + 1).padStart(2, "0"),
            } as CSSProperties;

            if (isInteractive) {
              return (
                <button
                  aria-label={stage.label}
                  aria-pressed={stage.state === "current"}
                  className={[
                    "progress-route__checkpoint",
                    `progress-route__checkpoint--${stage.state}`,
                    "progress-route__checkpoint--interactive",
                  ].join(" ")}
                  data-label={stage.label}
                  key={stage.id}
                  onClick={onCheckpointClick ? () => onCheckpointClick(stage.id) : undefined}
                  onMouseEnter={onCheckpointEnter ? () => onCheckpointEnter(stage.id) : undefined}
                  style={sharedStyle}
                  type="button"
                />
              );
            }

            return (
              <span
                aria-label={stage.label}
                className={[
                  "progress-route__checkpoint",
                  `progress-route__checkpoint--${stage.state}`,
                ].join(" ")}
                data-label={stage.label}
                key={stage.id}
                style={sharedStyle}
              />
            );
          })}
        </div>

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
