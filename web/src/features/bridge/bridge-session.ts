export type BridgeSessionStatus =
  | "deposit_submitted"
  | "sepolia_confirming"
  | "aegislink_processing"
  | "osmosis_pending"
  | "completed"
  | "failed";

export type BridgeSession = {
  amountEth: string;
  destinationChain: string;
  recipient: string;
  sourceAddress: string;
  sourceTxHash: string;
  status: BridgeSessionStatus;
  createdAt: number;
  destinationTxHash?: string;
  errorMessage?: string;
};

type CreateBridgeSessionArgs = {
  amountEth: string;
  destinationChain: string;
  recipient: string;
  sourceAddress: string;
  sourceTxHash: string;
  createdAt?: number;
};

export function createSubmittedBridgeSession({
  amountEth,
  destinationChain,
  recipient,
  sourceAddress,
  sourceTxHash,
  createdAt = Date.now(),
}: CreateBridgeSessionArgs): BridgeSession {
  return {
    amountEth,
    destinationChain,
    recipient,
    sourceAddress,
    sourceTxHash,
    status: "deposit_submitted",
    createdAt,
  };
}
