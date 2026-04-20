import { useState } from "react";
import { aboutDocs, bridgeStages, type BridgeStage } from "./about-content";

function findStage(stageId: BridgeStage["id"]) {
  return bridgeStages.find((stage) => stage.id === stageId) ?? bridgeStages[0];
}

export function AboutSection() {
  const [pinnedStageId, setPinnedStageId] = useState<BridgeStage["id"]>(bridgeStages[0].id);
  const [hoveredStageId, setHoveredStageId] = useState<BridgeStage["id"] | null>(null);
  const activeStageId = hoveredStageId ?? pinnedStageId;

  function activatePreview(stageId: BridgeStage["id"]) {
    setHoveredStageId(stageId);
  }

  function clearPreview() {
    setHoveredStageId(null);
  }

  function pinStage(stageId: BridgeStage["id"]) {
    setPinnedStageId(stageId);
    setHoveredStageId(stageId);
  }

  return (
    <section aria-labelledby="about-bridge-heading" className="about-section">
      <div className="about-section__intro">
        <p className="eyebrow">About AegisLink</p>
        <h2 id="about-bridge-heading">How the bridge works</h2>
        <p className="about-section__copy">
          Explore the path from Sepolia to Osmosis through AegisLink. Each
          stage opens directly from the wormhole scene so the explanation stays
          attached to the part of the bridge you are inspecting.
        </p>
      </div>

      <div className="about-layout">
        <div
          className="about-visual"
          onMouseLeave={clearPreview}
        >
          <div className="about-visual__scene">
            <div className="about-visual__portal about-visual__portal--left" />
            <div className="about-visual__portal about-visual__portal--right" />

            <div className="about-visual__tunnel" aria-hidden="true">
              <svg preserveAspectRatio="none" viewBox="0 0 1000 220">
                <path
                  d="M0,18 C220,46 390,74 500,108 C610,74 780,46 1000,18"
                  fill="none"
                  stroke="rgba(255,120,180,0.42)"
                  strokeWidth="2"
                />
                <path
                  d="M0,202 C220,174 390,146 500,112 C610,146 780,174 1000,202"
                  fill="none"
                  stroke="rgba(255,210,122,0.4)"
                  strokeWidth="2"
                />
                <path
                  d="M0,46 C230,78 392,93 500,109 C608,93 770,78 1000,46"
                  fill="none"
                  stroke="rgba(138,198,255,0.14)"
                  strokeWidth="1.4"
                />
                <path
                  d="M0,174 C230,142 392,127 500,111 C608,127 770,142 1000,174"
                  fill="none"
                  stroke="rgba(138,198,255,0.12)"
                  strokeWidth="1.4"
                />
              </svg>
            </div>

            <div className="about-visual__core">
              <strong>AegisLink</strong>
            </div>
            <div aria-hidden="true" className={`about-visual__pulse about-visual__pulse--${activeStageId}`} />

            <span className="about-visual__chain-label about-visual__chain-label--left">
              Sepolia
            </span>
            <span className="about-visual__chain-label about-visual__chain-label--right">
              Osmosis
            </span>

            {bridgeStages.map((stage) => {
              const isActive = stage.id === activeStageId;

              return (
                <div
                  className={`about-stage-node about-stage-node--${stage.accent} ${
                    isActive ? "about-stage-node--active" : ""
                  }`}
                  data-stage={stage.id}
                  key={stage.id}
                >
                  <button
                    aria-controls={`about-stage-panel-${stage.id}`}
                    aria-expanded={isActive}
                    aria-pressed={isActive}
                    className={`about-stage about-stage--${stage.accent} ${
                      isActive ? "about-stage--active" : ""
                    }`}
                    onBlur={clearPreview}
                    onClick={() => pinStage(stage.id)}
                    onFocus={() => activatePreview(stage.id)}
                    onMouseEnter={() => activatePreview(stage.id)}
                    type="button"
                  >
                    <small>{stage.eyebrow}</small>
                    <strong>{stage.title}</strong>
                    <span>{stage.summary}</span>
                  </button>

                  {isActive ? (
                    <div
                      className={`about-stage-card about-stage-card--${stage.accent}`}
                      id={`about-stage-panel-${stage.id}`}
                    >
                      <div className="about-stage-card__section">
                        <small>{stage.nowTitle}</small>
                        <p>{stage.nowBody}</p>
                      </div>
                      <div className="about-stage-card__section">
                        <small>{stage.systemTitle}</small>
                        <p>{stage.systemBody}</p>
                      </div>
                      <div className="about-stage-card__section">
                        <small>{stage.whyTitle}</small>
                        <p>{stage.whyBody}</p>
                      </div>
                      <div className="about-stage-card__footer">
                        {stage.footerTags.map((tag) => (
                          <span key={tag}>{tag}</span>
                        ))}
                      </div>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        </div>
      </div>

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
