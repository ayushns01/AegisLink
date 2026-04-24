import type { BridgeSession, BridgeSessionStatus } from "./bridge-session";
import { ProgressPanel } from "./ProgressPanel";

type PreviewStage = "sepolia" | "verify" | "handoff" | "receipt" | "failed";

const previewStatusByStage: Record<PreviewStage, BridgeSessionStatus> = {
  sepolia: "sepolia_confirming",
  verify: "aegislink_processing",
  handoff: "osmosis_pending",
  receipt: "completed",
  failed: "failed",
};

const previewDestinationHash =
  "F9D2C088D7B3B4E3412BF0E991DB22E4D4C4FDD88A1126A07A867DD725264001";

export function TransferWormholePreview() {
  const stage = readPreviewStage();
  const session = makePreviewSession(stage);

  return (
    <main className="wormhole-preview-page" aria-label="Transfer wormhole preview">
      <ProgressPanel
        isPolling={stage !== "receipt" && stage !== "failed"}
        onReset={() => undefined}
        pollError={stage === "failed" ? "Previewing the paused bridge state." : null}
        session={session}
      />
    </main>
  );
}

function makePreviewSession(stage: PreviewStage): BridgeSession {
  const status = previewStatusByStage[stage];

  return {
    amountEth: "0.001",
    createdAt: Date.now() - 90_000,
    destinationChain: "Osmosis Testnet (OSMO)",
    destinationTxHash: stage === "receipt" ? previewDestinationHash : undefined,
    destinationTxUrl:
      stage === "receipt"
        ? `https://www.mintscan.io/osmosis-testnet/tx/${previewDestinationHash}`
        : undefined,
    errorMessage:
      stage === "failed"
        ? "IBC delivery paused while the operator checks packet state."
        : undefined,
    recipient: "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    sourceAddress: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
    sourceTxHash:
      "0x175b6e991ee0aef82bc06ec2fde6d2ba4464ff68aeb65ea3da730ba9f100157f",
    status,
  };
}

function readPreviewStage(): PreviewStage {
  if (typeof window === "undefined") {
    return "handoff";
  }

  const requestedStage = new URLSearchParams(window.location.search).get("stage");

  if (
    requestedStage === "sepolia" ||
    requestedStage === "verify" ||
    requestedStage === "handoff" ||
    requestedStage === "receipt" ||
    requestedStage === "failed"
  ) {
    return requestedStage;
  }

  return "handoff";
}
