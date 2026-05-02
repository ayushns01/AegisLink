import { useMemo, useState } from "react";
import { type TransferVisualStage, type TransferVisualStageId } from "../bridge/transfer-progress";
import { BridgeWormholeScene } from "../bridge/BridgeWormholeScene";
import { aboutDocs, bridgeStages, type BridgeStage } from "./about-content";

const VISUAL_STAGE_ORDER: TransferVisualStageId[] = ["sepolia", "verify", "accounting", "handoff", "receipt"];

function buildVisualStages(
  activeId: TransferVisualStageId,
  stageByVisualId: Record<TransferVisualStageId, BridgeStage>,
): TransferVisualStage[] {
  const activeIndex = VISUAL_STAGE_ORDER.indexOf(activeId);
  return VISUAL_STAGE_ORDER.map((id, i) => ({
    id,
    label: stageByVisualId[id]?.title ?? id,
    state: i < activeIndex ? "completed" : i === activeIndex ? "current" : "upcoming",
  }));
}

export function AboutSection() {
  const [hoveredStageId, setHoveredStageId] = useState<TransferVisualStageId | null>(null);

  const stageByVisualId = useMemo(() => {
    const map = {} as Record<TransferVisualStageId, BridgeStage>;
    for (const stage of bridgeStages) {
      map[stage.visualId] = stage;
    }
    return map;
  }, []);

  const activeVisualId = hoveredStageId ?? "sepolia";
  const hoveredStage = hoveredStageId ? stageByVisualId[hoveredStageId] : null;
  const visualStages: TransferVisualStage[] = buildVisualStages(activeVisualId, stageByVisualId);

  return (
    <section aria-labelledby="about-bridge-heading" className="about-section">
      <div className="about-section__intro">
        <p className="eyebrow">About AegisLink</p>
        <h2 id="about-bridge-heading">How the bridge works</h2>
        <p className="about-section__copy">
          Hover any stage checkpoint in the wormhole to explore what happens at that step.
        </p>
      </div>

      {/* Wormhole + inline hover panel */}
      <div
        className="about-wormhole"
        onMouseLeave={() => setHoveredStageId(null)}
      >
        <BridgeWormholeScene
          activeStageId={activeVisualId}
          stages={visualStages}
          onCheckpointClick={setHoveredStageId}
          onCheckpointEnter={setHoveredStageId}
        />

        {/* Hover panel — lives inside the container so mouseleave fires only when leaving the whole block */}
        <div className="about-hover-panel">
          {hoveredStage ? (
            <>
              <div className="about-hover-panel__head">
                <p className={`eyebrow about-hover-panel__eyebrow about-hover-panel__eyebrow--${hoveredStage.accent}`}>
                  {hoveredStage.eyebrow}
                </p>
                <h3 className="about-hover-panel__title">{hoveredStage.title}</h3>
                <p className="about-hover-panel__summary">{hoveredStage.summary}</p>
              </div>
              <div className="about-hover-panel__sections">
                <div className="about-hover-panel__section">
                  <small>{hoveredStage.nowTitle}</small>
                  <p>{hoveredStage.nowBody}</p>
                </div>
                <div className="about-hover-panel__section">
                  <small>{hoveredStage.systemTitle}</small>
                  <p>{hoveredStage.systemBody}</p>
                </div>
                <div className="about-hover-panel__section">
                  <small>{hoveredStage.whyTitle}</small>
                  <p>{hoveredStage.whyBody}</p>
                </div>
              </div>
              <div className="about-hover-panel__tags">
                {hoveredStage.footerTags.map((tag) => (
                  <span key={tag} className="about-hover-panel__tag">{tag}</span>
                ))}
              </div>
            </>
          ) : (
            <p className="about-hover-panel__prompt">← Hover a stage checkpoint to explore</p>
          )}
        </div>
      </div>

      {/* Docs grid */}
      <div className="about-docs">
        <div className="about-docs__intro">
          <p className="eyebrow">Documentation</p>
          <h3>Proof surfaces behind the bridge</h3>
          <p>
            These are the key docs behind the live path, from architecture and
            security to demo flow and operator bootstrapping.
          </p>
        </div>
        <div className="about-docs__grid">
          {aboutDocs.map((doc) => (
            <a
              className="about-doc"
              href={doc.href}
              key={doc.title}
              rel="noreferrer"
              target="_blank"
            >
              <strong>{doc.title}</strong>
              <p>{doc.summary}</p>
              <span>Open documentation</span>
            </a>
          ))}
        </div>
      </div>
    </section>
  );
}
