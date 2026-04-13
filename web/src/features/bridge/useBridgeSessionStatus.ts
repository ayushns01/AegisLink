import { useEffect, useState } from "react";
import { frontendEnv } from "../../lib/config/env";
import { fetchBridgeStatus } from "../../lib/status/bridge-status";
import type { BridgeSession } from "./bridge-session";

const POLL_INTERVAL_MS = 4_000;
const RETRY_INTERVAL_MS = 6_000;

type BridgeSessionStatusState = {
  isPolling: boolean;
  pollError: string | null;
  session: BridgeSession | null;
};

export function useBridgeSessionStatus(
  session: BridgeSession | null,
): BridgeSessionStatusState {
  const [resolvedSession, setResolvedSession] = useState<BridgeSession | null>(session);
  const [isPolling, setIsPolling] = useState(false);
  const [pollError, setPollError] = useState<string | null>(null);

  useEffect(() => {
    setResolvedSession(session);
    setPollError(null);
  }, [session?.sourceTxHash]);

  useEffect(() => {
    if (!session || !frontendEnv.statusApiBaseUrl.trim()) {
      setIsPolling(false);
      return;
    }

    let cancelled = false;
    let retryHandle: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | null = null;

    const schedule = (delayMs: number) => {
      retryHandle = setTimeout(() => {
        void poll();
      }, delayMs);
    };

    const poll = async () => {
      controller?.abort();
      controller = new AbortController();
      setIsPolling(true);

      try {
        const status = await fetchBridgeStatus(session.sourceTxHash, controller.signal);
        if (cancelled || !status) {
          return;
        }

        setResolvedSession((current) =>
          current == null
            ? current
            : {
                ...current,
                status: status.status,
                destinationTxHash: status.destinationTxHash ?? current.destinationTxHash,
                destinationTxUrl: status.destinationTxUrl ?? current.destinationTxUrl,
                errorMessage: status.errorMessage ?? current.errorMessage,
              },
        );
        setPollError(null);

        if (status.status === "completed" || status.status === "failed") {
          setIsPolling(false);
          return;
        }

        schedule(POLL_INTERVAL_MS);
      } catch (error) {
        if (cancelled || isAbortError(error)) {
          return;
        }
        setPollError(normalizeStatusError(error));
        schedule(RETRY_INTERVAL_MS);
      }
    };

    void poll();

    return () => {
      cancelled = true;
      controller?.abort();
      if (retryHandle) {
        clearTimeout(retryHandle);
      }
    };
  }, [session?.sourceTxHash]);

  return {
    isPolling,
    pollError,
    session: resolvedSession,
  };
}

function isAbortError(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

function normalizeStatusError(error: unknown) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return "Unable to load live bridge status right now.";
}
