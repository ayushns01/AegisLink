import { useState } from "react";
import { aboutDocs, bridgeStages, type BridgeStage } from "./about-content";

const ABOUT_PARTICLES: Array<{
  top: number;
  size: number;
  delay: number;
  duration: number;
  color: "pink" | "blue" | "gold";
  blur: number;
}> = [
  { top: 42, size: 9,  delay: 0,    duration: 55, color: "pink", blur: 9  },
  { top: 47, size: 6,  delay: 2.2,  duration: 65, color: "pink", blur: 6  },
  { top: 52, size: 11, delay: 0.8,  duration: 50, color: "pink", blur: 11 },
  { top: 38, size: 5,  delay: 4.0,  duration: 60, color: "pink", blur: 5  },
  { top: 57, size: 7,  delay: 6.3,  duration: 45, color: "pink", blur: 7  },
  { top: 44, size: 10, delay: 8.1,  duration: 70, color: "pink", blur: 10 },
  { top: 50, size: 4,  delay: 5.5,  duration: 55, color: "pink", blur: 4  },
  { top: 46, size: 13, delay: 1.4,  duration: 60, color: "blue", blur: 13 },
  { top: 54, size: 7,  delay: 3.6,  duration: 50, color: "blue", blur: 7  },
  { top: 49, size: 5,  delay: 7.2,  duration: 65, color: "blue", blur: 5  },
  { top: 52, size: 9,  delay: 0.5,  duration: 55, color: "blue", blur: 9  },
  { top: 43, size: 6,  delay: 9.0,  duration: 70, color: "blue", blur: 6  },
  { top: 59, size: 4,  delay: 2.8,  duration: 50, color: "blue", blur: 4  },
  { top: 48, size: 8,  delay: 5.9,  duration: 60, color: "blue", blur: 8  },
  { top: 50, size: 12, delay: 1.0,  duration: 50, color: "gold", blur: 12 },
  { top: 44, size: 6,  delay: 3.3,  duration: 65, color: "gold", blur: 6  },
  { top: 57, size: 8,  delay: 6.8,  duration: 55, color: "gold", blur: 8  },
  { top: 48, size: 5,  delay: 9.5,  duration: 45, color: "gold", blur: 5  },
  { top: 52, size: 10, delay: 4.7,  duration: 70, color: "gold", blur: 10 },
  { top: 46, size: 4,  delay: 7.4,  duration: 60, color: "gold", blur: 4  },
  { top: 54, size: 7,  delay: 0.2,  duration: 65, color: "gold", blur: 7  },
  { top: 41, size: 9,  delay: 2.6,  duration: 50, color: "gold", blur: 9  },
];

const COLOR_MAP = {
  pink: "rgba(255, 130, 190, 0.80)",
  blue: "rgba(138, 198, 255, 0.78)",
  gold: "rgba(255, 210, 120, 0.80)",
};

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
        <div className="about-visual" onMouseLeave={clearPreview}>
          <div className="about-visual__scene">

            {/* ── Twinkling background stars ── */}
            <div className="about-visual__stars" aria-hidden="true">
              {Array.from({ length: 32 }).map((_, i) => (
                <span
                  key={i}
                  className="about-visual__star"
                  style={{
                    left: `${(i * 37 + 5) % 100}%`,
                    top: `${(i * 53 + 8) % 100}%`,
                    animationDelay: `${(i * 0.41) % 4}s`,
                    animationDuration: `${3 + (i % 5) * 0.7}s`,
                    width: i % 5 === 0 ? "2.5px" : "1.5px",
                    height: i % 5 === 0 ? "2.5px" : "1.5px",
                    opacity: i % 3 === 0 ? 0.9 : 0.55,
                  }}
                />
              ))}
            </div>

            {/* ── Drifting wormhole particles (Sepolia → Osmosis) ── */}
            <div className="about-visual__particles" aria-hidden="true">
              {ABOUT_PARTICLES.map((p, i) => (
                <span
                  key={i}
                  className="about-particle"
                  style={{
                    top: `${p.top}%`,
                    width: `${p.size}px`,
                    height: `${p.size}px`,
                    background: COLOR_MAP[p.color],
                    boxShadow: `0 0 ${p.blur}px ${p.blur / 2}px ${COLOR_MAP[p.color]}`,
                    animationDelay: `${p.delay}s`,
                    animationDuration: `${p.duration}s`,
                  }}
                />
              ))}
            </div>

            {/* ── Left portal (Sepolia — pink) ── */}
            <div className="about-visual__portal about-visual__portal--left">
              <div className="about-visual__portal-ring about-visual__portal-ring--pink-1" />
              <div className="about-visual__portal-ring about-visual__portal-ring--pink-2" />
              <div className="about-visual__portal-ring about-visual__portal-ring--pink-3" />
            </div>

            {/* ── Right portal (Osmosis — gold) ── */}
            <div className="about-visual__portal about-visual__portal--right">
              <div className="about-visual__portal-ring about-visual__portal-ring--gold-1" />
              <div className="about-visual__portal-ring about-visual__portal-ring--gold-2" />
              <div className="about-visual__portal-ring about-visual__portal-ring--gold-3" />
            </div>

            {/* ── Core node with rotating glow ── */}
            <div className="about-visual__core">
              <div className="about-visual__core-halo" />
              <div className="about-visual__core-glow" />
              <strong>AegisLink</strong>
            </div>

            {/* ── Dual staggered pulse beams ── */}
            <div aria-hidden="true" className={`about-visual__pulse about-visual__pulse--${activeStageId}`} />
            <div aria-hidden="true" className={`about-visual__pulse about-visual__pulse--${activeStageId} about-visual__pulse--delayed`} />

            {/* ── Chain labels ── */}
            <span className="about-visual__chain-label about-visual__chain-label--left">Sepolia</span>
            <span className="about-visual__chain-label about-visual__chain-label--right">Osmosis</span>

            {bridgeStages.map((stage) => {
              const isActive = stage.id === activeStageId;
              return (
                <div
                  className={`about-stage-node about-stage-node--${stage.accent} ${isActive ? "about-stage-node--active" : ""}`}
                  data-stage={stage.id}
                  key={stage.id}
                >
                  <button
                    aria-controls={`about-stage-panel-${stage.id}`}
                    aria-expanded={isActive}
                    aria-pressed={isActive}
                    className={`about-stage about-stage--${stage.accent} ${isActive ? "about-stage--active" : ""}`}
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
