import { useMemo, useState } from "react";
import { parseEther } from "viem";
import type { Address } from "viem";
import { useWalletClient } from "wagmi";
import { registerBridgeDeliveryIntent } from "../../lib/bridge/delivery-intent";
import { frontendEnv } from "../../lib/config/env";
import { submitEthDeposit } from "../../lib/evm/gateway";
import { useBridgeWallet } from "../wallet/useBridgeWallet";
import { createSubmittedBridgeSession, type BridgeSession } from "./bridge-session";
import { ProgressPanel } from "./ProgressPanel";
import { useBridgeSessionStatus } from "./useBridgeSessionStatus";

type Destination = {
  id: string;
  label: string;
  symbol: string;
  helper: string;
  enabled: boolean;
  prefix: string;
};

const destinations: Destination[] = [
  {
    id: "osmosis-testnet-osmo",
    label: "Osmosis Testnet (OSMO)",
    symbol: "OSMO",
    helper: "Live route available now",
    enabled: true,
    prefix: "osmo1",
  },
  {
    id: "osmosis-mainnet-osmo",
    label: "Osmosis Mainnet (OSMO)",
    symbol: "OSMO",
    helper: "Coming soon",
    enabled: false,
    prefix: "osmo1",
  },
  {
    id: "celestia-mainnet-tia",
    label: "Celestia Mainnet (TIA)",
    symbol: "TIA",
    helper: "Coming soon",
    enabled: false,
    prefix: "celestia1",
  },
  {
    id: "celestia-mocha-testnet-tia",
    label: "Celestia Mocha Testnet (TIA)",
    symbol: "TIA",
    helper: "Coming soon",
    enabled: false,
    prefix: "celestia1",
  },
  {
    id: "injective-mainnet-inj",
    label: "Injective Mainnet (INJ)",
    symbol: "INJ",
    helper: "Coming soon",
    enabled: false,
    prefix: "inj1",
  },
  {
    id: "injective-testnet-inj",
    label: "Injective Testnet (INJ)",
    symbol: "INJ",
    helper: "Coming soon",
    enabled: false,
    prefix: "inj1",
  },
  {
    id: "dydx-mainnet-dydx",
    label: "dYdX Mainnet (DYDX)",
    symbol: "DYDX",
    helper: "Coming soon",
    enabled: false,
    prefix: "dydx1",
  },
  {
    id: "dydx-testnet-dydx",
    label: "dYdX Testnet (DYDX)",
    symbol: "DYDX",
    helper: "Coming soon",
    enabled: false,
    prefix: "dydx1",
  },
  {
    id: "akash-mainnet-akt",
    label: "Akash Mainnet (AKT)",
    symbol: "AKT",
    helper: "Coming soon",
    enabled: false,
    prefix: "akash1",
  },
  {
    id: "akash-sandbox-akt",
    label: "Akash Sandbox (AKT)",
    symbol: "AKT",
    helper: "Coming soon",
    enabled: false,
    prefix: "akash1",
  },
];

export function TransferPage() {
  const wallet = useBridgeWallet();
  const { data: walletClient } = useWalletClient();
  const [amount, setAmount] = useState("0.250");
  const [recipient, setRecipient] = useState(
    "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );
  const [selectedDestinationId, setSelectedDestinationId] = useState(
    () => destinations.find((destination) => destination.enabled)?.id ?? destinations[0].id,
  );
  const [isDestinationMenuOpen, setIsDestinationMenuOpen] = useState(false);
  const [session, setSession] = useState<BridgeSession | null>(null);
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const {
    isPolling: isPollingStatus,
    pollError,
    session: resolvedSession,
  } = useBridgeSessionStatus(session);
  const destination =
    destinations.find((entry) => entry.id === selectedDestinationId) ?? destinations[0];
  const recipientIsValid = useMemo(
    () => recipient.startsWith(destination.prefix) && recipient.length > destination.prefix.length + 8,
    [destination.prefix, recipient],
  );
  const amountIsValid = useMemo(() => {
    const parsed = Number(amount);
    return Number.isFinite(parsed) && parsed > 0;
  }, [amount]);

  const canSubmit =
    amountIsValid &&
    recipientIsValid &&
    wallet.isConnected &&
    !wallet.isWrongChain &&
    Boolean(wallet.address) &&
    Boolean(walletClient) &&
    !isSubmitting;

  async function handleSubmit() {
    if (!walletClient || !wallet.address) {
      setSubmissionError("Connect a Sepolia wallet extension to continue.");
      return;
    }

    setIsSubmitting(true);
    setSubmissionError(null);

    try {
      const txHash = await submitEthDeposit({
        walletClient,
        gatewayAddress: frontendEnv.gatewayAddress,
        account: wallet.address as Address,
        amountEth: amount,
        recipient: frontendEnv.aegislinkDepositRecipient,
      });
      await registerBridgeDeliveryIntent({
        sourceTxHash: txHash,
        sender: frontendEnv.aegislinkDepositRecipient,
        routeId: "osmosis-public-wallet",
        assetId: "eth",
        amount: parseEther(amount).toString(),
        receiver: recipient,
      });

      setSession(
        createSubmittedBridgeSession({
          amountEth: amount,
          destinationChain: destination.label,
          recipient,
          sourceAddress: wallet.address,
          sourceTxHash: txHash,
        }),
      );
    } catch (error) {
      setSubmissionError(normalizeSubmissionError(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (resolvedSession) {
    return (
      <ProgressPanel
        isPolling={isPollingStatus}
        onReset={() => setSession(null)}
        pollError={pollError}
        session={resolvedSession}
      />
    );
  }

  return (
    <div className="transfer-card">
      <div className="transfer-card__header">
        <div>
          <p className="eyebrow eyebrow--dark">Bridge</p>
          <h2>Transfer</h2>
          <p className="transfer-card__copy">
            Select a supported Cosmos destination, enter the matching recipient
            address, and bridge ETH from the connected Sepolia wallet.
          </p>
        </div>
        <div className="wallet-chip">
          {wallet.isWrongChain
            ? "Wrong chain"
            : wallet.address
              ? `${wallet.address.slice(0, 6)}...${wallet.address.slice(-4)}`
              : "Wallet connected"}
        </div>
      </div>

      <div className="field-grid">
        <div className="field-card field-card--amount">
          <label className="field-label" htmlFor="bridge-amount">
            Amount
          </label>
          <div className="field-input-wrap field-input-wrap--amount">
            <input
              aria-label="Amount"
              className="field-input field-input--amount"
              id="bridge-amount"
              inputMode="decimal"
              onChange={(event) => setAmount(event.target.value)}
              value={amount}
            />
            <span className="field-suffix">ETH</span>
          </div>
          <span className="field-helper">ETH from connected Sepolia wallet</span>
        </div>

        <div className="field-card">
          <small>Destination chain</small>
          <div className="destination-picker">
            <button
              aria-expanded={isDestinationMenuOpen}
              aria-haspopup="menu"
              aria-label={`Destination chain: ${destination.label}`}
              className="destination-trigger"
              onClick={() => setIsDestinationMenuOpen((value) => !value)}
              type="button"
            >
              <span className="destination-trigger__label destination-trigger__label--active">
                {destination.label}
              </span>
              <em>{destination.symbol}</em>
            </button>

            {isDestinationMenuOpen ? (
              <div className="destination-menu destination-menu--scrollable" role="menu">
                {destinations.map((option) => (
                  <button
                    className={
                      option.id === destination.id
                        ? "destination-option destination-option--active"
                        : "destination-option"
                    }
                    disabled={!option.enabled}
                    key={option.id}
                    onClick={() => {
                      if (!option.enabled) {
                        return;
                      }

                      setSelectedDestinationId(option.id);
                      setRecipient("");
                      setSubmissionError(null);
                      setIsDestinationMenuOpen(false);
                    }}
                    role="menuitem"
                    type="button"
                  >
                    <div>
                      <strong
                        className={
                          option.id === destination.id
                            ? "destination-option__title destination-option__title--active"
                            : "destination-option__title"
                        }
                      >
                        {option.label}
                      </strong>
                      <span>{option.helper}</span>
                    </div>
                    <em>{option.enabled ? "Live" : "Soon"}</em>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        </div>

        <div className="field-card">
          <label className="field-label" htmlFor="bridge-recipient">
            Recipient
          </label>
          <input
            aria-label="Recipient"
            className="field-input field-input--recipient"
            id="bridge-recipient"
            onChange={(event) => setRecipient(event.target.value)}
            value={recipient}
          />
          <span className="field-helper">
            Recipient must match the selected chain prefix. Current route support
            is live for Osmosis only.
          </span>
          {!recipientIsValid ? (
            <p className="field-error">
              Enter a valid {destination.prefix} recipient.
            </p>
          ) : null}
        </div>
      </div>

      <button
        className="primary-cta"
        disabled={!canSubmit}
        onClick={() => void handleSubmit()}
        type="button"
      >
        {isSubmitting ? "Opening Bridge Tunnel..." : "Bridge to Osmosis"}
      </button>
      {submissionError ? <p className="field-error field-error--spaced">{submissionError}</p> : null}
    </div>
  );
}

function normalizeSubmissionError(error: unknown) {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }

  return "The deposit could not be submitted. Please try again.";
}
