import { frontendEnv } from "../../lib/config/env";
import type { BridgeSession } from "./bridge-session";

type ProgressPanelProps = {
  onReset: () => void;
  session: BridgeSession;
};

export function ProgressPanel({ onReset, session }: ProgressPanelProps) {
  const milestones = [
    { label: "Deposit submitted on Sepolia", done: true },
    {
      label: "Sepolia confirmation pending",
      done: session.status !== "deposit_submitted",
    },
    {
      label: "AegisLink processing pending",
      done:
        session.status === "osmosis_pending" || session.status === "completed",
    },
    {
      label: "Osmosis delivery pending",
      done: session.status === "completed",
    },
  ];

  return (
    <div className="transfer-card transfer-card--progress">
      <div className="transfer-card__header">
        <div>
          <p className="eyebrow eyebrow--dark">Bridge Session</p>
          <h2>Transfer in progress</h2>
          <p className="transfer-card__copy">
            Your deposit has been submitted. AegisLink will keep processing the
            transfer until the Osmosis receipt is available.
          </p>
        </div>
        <div className="wallet-chip">Awaiting bridge progress</div>
      </div>

      <div className="progress-layout">
        <div className="progress-card">
          <small>Submitted amount</small>
          <strong>{session.amountEth} ETH</strong>
          <span>
            {session.destinationChain} recipient: {session.recipient}
          </span>
        </div>

        <div className="progress-card">
          <small>Sepolia transaction</small>
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

        <div className="progress-card">
          <small>Status timeline</small>
          <div className="progress-steps">
            {milestones.map((milestone, index) => (
              <div className="progress-step" key={milestone.label}>
                <div
                  className={
                    milestone.done
                      ? "progress-step__dot progress-step__dot--done"
                      : "progress-step__dot"
                  }
                />
                <div>
                  <strong>{milestone.label}</strong>
                  <span>
                    {milestone.done
                      ? "Completed or advanced"
                      : index === 1
                        ? "Waiting for block confirmations"
                        : "Waiting for downstream processing"}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="progress-actions">
        <button className="secondary-cta" onClick={onReset} type="button">
          Start New Transfer
        </button>
      </div>
    </div>
  );
}

function shortHash(hash: string) {
  return `${hash.slice(0, 10)}...${hash.slice(-8)}`;
}
