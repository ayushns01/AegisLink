import type { BridgeSession } from "./bridge-session";

export type TransferVisualStageId =
  | "sepolia"
  | "verify"
  | "accounting"
  | "handoff"
  | "receipt";

export type TransferVisualStage = {
  id: TransferVisualStageId;
  label: string;
  state: "completed" | "current" | "upcoming";
};

type TransferProgressModel = {
  chipLabel: string;
  sceneLabel: string;
  headline: string;
  summary: string;
  stages: TransferVisualStage[];
};

const stageDefinitions: Array<{ id: TransferVisualStageId; label: string }> = [
  { id: "sepolia", label: "Sepolia Secured" },
  { id: "verify", label: "Proof Verified" },
  { id: "accounting", label: "Ledger Synced" },
  { id: "handoff", label: "IBC Relayed" },
  { id: "receipt", label: "Osmosis Minted" },
];

export function deriveTransferProgressModel(
  session: BridgeSession,
  isPolling: boolean,
): TransferProgressModel {
  const activeStageId = resolveActiveStageId(session);
  const activeStage = stageDefinitions.find((stage) => stage.id === activeStageId) ?? stageDefinitions[0];

  return {
    chipLabel: progressChipLabel(session.status, isPolling),
    sceneLabel: sceneLabelForStatus(session.status),
    headline: activeStage.label,
    summary: progressSummaryForStatus(session.status),
    stages: stageDefinitions.map((stage) => ({
      ...stage,
      state: stageState(stage.id, activeStageId),
    })),
  };
}

function stageState(
  stageId: TransferVisualStageId,
  activeStageId: TransferVisualStageId,
): TransferVisualStage["state"] {
  const stageIndex = stageDefinitions.findIndex((stage) => stage.id === stageId);
  const activeIndex = stageDefinitions.findIndex((stage) => stage.id === activeStageId);

  if (stageIndex < activeIndex) {
    return "completed";
  }

  if (stageId === activeStageId) {
    return "current";
  }

  return "upcoming";
}

function resolveActiveStageId(session: BridgeSession): TransferVisualStageId {
  switch (session.status) {
    case "completed":
      return "receipt";
    case "osmosis_pending":
      return "handoff";
    case "aegislink_processing":
      return "verify";
    case "failed":
      return inferFailedStage(session);
    case "sepolia_confirming":
    case "deposit_submitted":
    default:
      return "sepolia";
  }
}

function inferFailedStage(session: BridgeSession): TransferVisualStageId {
  const error = session.errorMessage?.toLowerCase() ?? "";

  if (session.destinationTxHash) {
    return "receipt";
  }

  if (error.includes("osmosis") || error.includes("ibc") || error.includes("packet")) {
    return "handoff";
  }

  if (error.includes("account") || error.includes("credit")) {
    return "accounting";
  }

  return "verify";
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
    return "IBC delivery live";
  }
  if (status === "sepolia_confirming" || status === "deposit_submitted") {
    return isPolling ? "Confirming on Sepolia" : "Awaiting confirmation";
  }
  if (isPolling) {
    return "Tracking live progress";
  }
  return "Awaiting bridge progress";
}

function sceneLabelForStatus(status: BridgeSession["status"]) {
  switch (status) {
    case "completed":
      return "Bridge tunnel resolved";
    case "failed":
      return "Bridge tunnel paused";
    default:
      return "Bridge tunnel";
  }
}

function progressSummaryForStatus(status: BridgeSession["status"]) {
  switch (status) {
    case "completed":
      return "Osmosis receipt resolved";
    case "failed":
      return "Bridge flow paused because this session needs attention";
    case "osmosis_pending":
      return "IBC delivery to Osmosis is underway";
    case "aegislink_processing":
      return "AegisLink is verifying bridge policy";
    case "sepolia_confirming":
    case "deposit_submitted":
    default:
      return "Sepolia deposit confirmed pending finalization";
  }
}
