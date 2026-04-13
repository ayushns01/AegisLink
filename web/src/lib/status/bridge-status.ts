import { frontendEnv } from "../config/env";
import type { BridgeSessionStatus } from "../../features/bridge/bridge-session";

export type BridgeStatusResponse = {
  sourceTxHash: string;
  status: BridgeSessionStatus;
  destinationTxHash?: string;
  destinationTxUrl?: string;
  errorMessage?: string;
};

export async function fetchBridgeStatus(
  sourceTxHash: string,
  signal?: AbortSignal,
): Promise<BridgeStatusResponse | null> {
  const baseUrl = frontendEnv.statusApiBaseUrl.trim();
  if (!baseUrl) {
    return null;
  }

  const response = await fetch(
    `${baseUrl.replace(/\/+$/, "")}/bridge-status?sourceTxHash=${encodeURIComponent(sourceTxHash)}`,
    {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
      signal,
    },
  );

  if (!response.ok) {
    let message = `Bridge status request failed with ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (typeof payload.error === "string" && payload.error.trim()) {
        message = payload.error;
      }
    } catch {
      // Preserve the default message when the response is not JSON.
    }
    throw new Error(message);
  }

  return (await response.json()) as BridgeStatusResponse;
}
