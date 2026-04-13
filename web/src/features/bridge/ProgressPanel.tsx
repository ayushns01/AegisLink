import { frontendEnv } from "../../lib/config/env";
import type { BridgeSession } from "./bridge-session";

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
  const milestones = [
    { label: "Deposit submitted on Sepolia", done: true },
    {
      label: "Sepolia confirmed",
      done: session.status !== "deposit_submitted",
    },
    {
      label: "AegisLink processing",
      done:
        session.status === "osmosis_pending" ||
        session.status === "completed" ||
        session.status === "failed",
    },
    {
      label: "Osmosis delivery",
      done: session.status === "completed",
    },
  ];
  const progressLabel = progressChipLabel(session.status, isPolling);
  const progressHeadline = progressHeadlineForStatus(session.status);
  const progressSummary = progressSummaryForStatus(session.status);

  return (
    <div className="transfer-card transfer-card--progress">
      <div className="transfer-card__header">
        <div>
          <p className="eyebrow eyebrow--dark">Bridge Session</p>
          <h2>Transfer in progress</h2>
          <p className="transfer-card__copy">
            {progressSummary}
          </p>
        </div>
        <div className="wallet-chip">{progressLabel}</div>
      </div>

      <div className="progress-card progress-card--hero">
        <small>Current stage</small>
        <strong>{progressHeadline}</strong>
        <span>{progressSummary}</span>
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
          <small>Osmosis receipt</small>
          {session.destinationTxHash ? (
            session.destinationTxUrl ? (
              <a
                className="tx-link"
                href={session.destinationTxUrl}
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
                        : index === 2
                          ? "Waiting for AegisLink to finish bridge processing"
                          : "Waiting for downstream delivery"}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {pollError ? <p className="progress-alert">{pollError}</p> : null}

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

function progressChipLabel(status: BridgeSession["status"], isPolling: boolean) {
  if (status === "completed") {
    return "Delivered to Osmosis";
  }
  if (status === "failed") {
    return "Needs attention";
  }
  if (status === "aegislink_processing") {
    return "AegisLink processing";
  }
  if (status === "osmosis_pending") {
    return "Awaiting Osmosis delivery";
  }
  if (status === "sepolia_confirming" || status === "deposit_submitted") {
    return isPolling ? "Confirming on Sepolia" : "Awaiting confirmation";
  }
  if (isPolling) {
    return "Tracking live progress";
  }
  return "Awaiting bridge progress";
}

function progressHeadlineForStatus(status: BridgeSession["status"]) {
  switch (status) {
    case "completed":
      return "Delivered to Osmosis";
    case "failed":
      return "Bridge flow needs attention";
    case "osmosis_pending":
      return "AegisLink has sent your transfer toward Osmosis";
    case "aegislink_processing":
      return "AegisLink is validating and crediting your bridged ETH";
    case "sepolia_confirming":
    case "deposit_submitted":
    default:
      return "Waiting for Sepolia confirmation";
  }
}

function progressSummaryForStatus(status: BridgeSession["status"]) {
  switch (status) {
    case "completed":
      return "The transfer completed and the final Osmosis receipt is available below.";
    case "failed":
      return "The bridge observed a problem while processing the transfer. Review the error details below.";
    case "osmosis_pending":
      return "Sepolia is confirmed and AegisLink has handed the transfer off toward Osmosis.";
    case "aegislink_processing":
      return "Sepolia is confirmed. AegisLink is now processing the bridged balance before Osmosis delivery.";
    case "sepolia_confirming":
    case "deposit_submitted":
    default:
      return "Your deposit has been submitted. The bridge is waiting for Sepolia confirmation before continuing.";
  }
}
