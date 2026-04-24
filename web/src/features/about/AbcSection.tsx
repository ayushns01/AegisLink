import { useState } from "react";
import { ProgressPanel } from "../bridge/ProgressPanel";
import { BridgeSession, BridgeSessionStatus } from "../bridge/bridge-session";

const STAGES: BridgeSessionStatus[] = [
  "sepolia_confirming",
  "aegislink_processing",
  "accounting_something" as BridgeSessionStatus, // accounting isn't explicitly in status but ProgressPanel derives stage from status
  "osmosis_pending",
  "completed",
];

export function AbcSection() {
  const [status, setStatus] = useState<BridgeSessionStatus>("aegislink_processing");

  const mockSession: BridgeSession = {
    amountEth: "1.5",
    destinationChain: "Osmosis Mainnet (OSMO)",
    recipient: "osmo1mockaddress...",
    sourceAddress: "0xMockAddress...",
    sourceTxHash: "0xMockTxHash...",
    status: status,
    createdAt: Date.now(),
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "2rem", width: "100%", alignItems: "center" }}>
      <div style={{ display: "flex", gap: "1rem" }}>
        <button onClick={() => setStatus("sepolia_confirming")}>Sepolia</button>
        <button onClick={() => setStatus("aegislink_processing")}>Verify</button>
        <button onClick={() => setStatus("osmosis_pending")}>Handoff</button>
        <button onClick={() => setStatus("completed")}>Receipt</button>
      </div>
      <div style={{ width: "100%" }}>
        <ProgressPanel session={mockSession} onReset={() => {}} />
      </div>
    </div>
  );
}
