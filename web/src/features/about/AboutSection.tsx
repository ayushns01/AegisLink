import { useEffect, useMemo, useState } from "react";
import { type TransferVisualStage, type TransferVisualStageId } from "../bridge/transfer-progress";
import { BridgeWormholeScene } from "../bridge/BridgeWormholeScene";
import { aboutDocs, bridgeStages, type BridgeStage } from "./about-content";

const VISUAL_STAGE_LABELS: Record<TransferVisualStageId, string> = {
  sepolia:    "Sepolia Secured",
  verify:     "Proof Verified",
  accounting: "Ledger Synced",
  handoff:    "IBC Relayed",
  receipt:    "Osmosis Minted",
};

const VISUAL_STAGE_ORDER: TransferVisualStageId[] = ["sepolia", "verify", "accounting", "handoff", "receipt"];

function buildVisualStages(activeId: TransferVisualStageId): TransferVisualStage[] {
  const activeIndex = VISUAL_STAGE_ORDER.indexOf(activeId);
  return VISUAL_STAGE_ORDER.map((id, i) => ({
    id,
    label: VISUAL_STAGE_LABELS[id],
    state: i < activeIndex ? "completed" : i === activeIndex ? "current" : "upcoming",
  }));
}

export function AboutSection() {
  const [activeVisualId, setActiveVisualId] = useState<TransferVisualStageId>("sepolia");
  const [popupStage, setPopupStage] = useState<BridgeStage | null>(bridgeStages[0]);

  // Map visualId → BridgeStage
  const stageByVisualId = useMemo(() => {
    const map = {} as Record<TransferVisualStageId, BridgeStage>;
    for (const stage of bridgeStages) {
      map[stage.visualId] = stage;
    }
    return map;
  }, []);

  function handleCheckpointClick(stageId: TransferVisualStageId) {
    setActiveVisualId(stageId);
    setPopupStage(stageByVisualId[stageId]);
  }

  const visualStages: TransferVisualStage[] = buildVisualStages(activeVisualId);

  return (
    <section aria-labelledby="about-bridge-heading" className="about-section">
      <div className="about-section__intro">
        <p className="eyebrow">About AegisLink</p>
        <h2 id="about-bridge-heading">How the bridge works</h2>
        <p className="about-section__copy">
          Click any stage checkpoint in the wormhole to explore what happens at that step.
        </p>
      </div>

      {/* Wormhole scene */}
      <div className="about-wormhole">
        <BridgeWormholeScene
          activeStageId={activeVisualId}
          stages={visualStages}
          onCheckpointClick={handleCheckpointClick}
        />

        {/* Stage pill tabs below the scene */}
        <div className="about-stage-tabs">
          {bridgeStages.map((stage) => (
            <button
              key={stage.id}
              aria-pressed={stage.visualId === activeVisualId}
              className={`about-stage-tab${stage.visualId === activeVisualId ? " about-stage-tab--active" : ""}`}
              onClick={() => handleCheckpointClick(stage.visualId)}
              type="button"
            >
              <span className="about-stage-tab__num">{stage.eyebrow.slice(0, 2)}</span>
              {stage.title}
            </button>
          ))}
        </div>
      </div>

      {/* Stage popup modal */}
      {popupStage && (
        <StageModal
          stage={popupStage}
          onClose={() => setPopupStage(null)}
        />
      )}

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

function StageModal({ stage, onClose }: { stage: BridgeStage; onClose: () => void }) {
  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [onClose]);

  return (
    <>
      <div aria-hidden="true" className="stage-modal-backdrop" onClick={onClose} />
      <div
        aria-labelledby="stage-modal-title"
        className={`stage-modal stage-modal--${stage.accent}`}
        role="dialog"
      >
        <button
          aria-label="Close stage detail"
          className="stage-modal__close"
          onClick={onClose}
          type="button"
        >
          ✕
        </button>
        <p className="eyebrow stage-modal__eyebrow">{stage.eyebrow}</p>
        <h3 className="stage-modal__title" id="stage-modal-title">{stage.title}</h3>
        <p className="stage-modal__summary">{stage.summary}</p>
        <div className="stage-modal__sections">
          <div className="stage-modal__section">
            <small>{stage.nowTitle}</small>
            <p>{stage.nowBody}</p>
          </div>
          <div className="stage-modal__section">
            <small>{stage.systemTitle}</small>
            <p>{stage.systemBody}</p>
          </div>
          <div className="stage-modal__section">
            <small>{stage.whyTitle}</small>
            <p>{stage.whyBody}</p>
          </div>
        </div>
        <div className="stage-modal__tags">
          {stage.footerTags.map((tag) => (
            <span key={tag} className="stage-modal__tag">{tag}</span>
          ))}
        </div>
      </div>
    </>
  );
}
