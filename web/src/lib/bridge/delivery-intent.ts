import { frontendEnv } from "../config/env";

export type RegisterBridgeDeliveryIntentInput = {
  sourceTxHash: string;
  sender: string;
  routeId: string;
  assetId: string;
  amount: string;
  receiver: string;
};

export async function registerBridgeDeliveryIntent(
  payload: RegisterBridgeDeliveryIntentInput,
): Promise<void> {
  const baseUrl = frontendEnv.statusApiBaseUrl.trim();
  if (!baseUrl) {
    throw new Error("Bridge status API base URL is not configured.");
  }

  const response = await fetch(`${baseUrl.replace(/\/+$/, "")}/delivery-intents`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });

  if (response.ok) {
    return;
  }

  let message = `Delivery intent registration failed with status ${response.status}`;
  try {
    const responseBody = (await response.json()) as { error?: string };
    if (responseBody.error?.trim()) {
      message = responseBody.error;
    }
  } catch {
    // Ignore JSON decoding issues and keep the generic failure message.
  }
  throw new Error(message);
}
