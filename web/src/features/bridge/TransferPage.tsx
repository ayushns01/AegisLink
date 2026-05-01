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
  shortName: string;
  symbol: string;
  color: string;
  logo?: string;
  enabled: boolean;
  prefix: string;
  routeId: string;
};

const destinations: Destination[] = [
  {
    id: "osmosis-testnet-osmo",
    label: "Osmosis Testnet (OSMO)",
    shortName: "Osmosis Testnet",
    symbol: "OSMO",
    color: "#5E12A0",
    logo: "/chains/osmo.svg",
    enabled: true,
    prefix: "osmo1",
    routeId: "osmosis-public-wallet",
  },
  {
    id: "neutron-testnet-ntrn",
    label: "Neutron Testnet (NTRN)",
    shortName: "Neutron Testnet",
    symbol: "NTRN",
    color: "#1a1a2e",
    logo: "/chains/ntrn.svg",
    enabled: true,
    prefix: "neutron1",
    routeId: "neutron-public-wallet",
  },
  {
    id: "osmosis-mainnet-osmo",
    label: "Osmosis Mainnet (OSMO)",
    shortName: "Osmosis Mainnet",
    symbol: "OSMO",
    color: "#5E12A0",
    logo: "/chains/osmo.svg",
    enabled: false,
    prefix: "osmo1",
    routeId: "",
  },
  {
    id: "celestia-mainnet-tia",
    label: "Celestia Mainnet (TIA)",
    shortName: "Celestia Mainnet",
    symbol: "TIA",
    color: "#7c3aed",
    enabled: false,
    prefix: "celestia1",
    routeId: "",
  },
  {
    id: "celestia-mocha-testnet-tia",
    label: "Celestia Mocha Testnet (TIA)",
    shortName: "Celestia Mocha",
    symbol: "TIA",
    color: "#7c3aed",
    enabled: false,
    prefix: "celestia1",
    routeId: "",
  },
  {
    id: "injective-mainnet-inj",
    label: "Injective Mainnet (INJ)",
    shortName: "Injective Mainnet",
    symbol: "INJ",
    color: "#0ea5e9",
    enabled: false,
    prefix: "inj1",
    routeId: "",
  },
  {
    id: "injective-testnet-inj",
    label: "Injective Testnet (INJ)",
    shortName: "Injective Testnet",
    symbol: "INJ",
    color: "#0ea5e9",
    enabled: false,
    prefix: "inj1",
    routeId: "",
  },
  {
    id: "dydx-mainnet-dydx",
    label: "dYdX Mainnet (DYDX)",
    shortName: "dYdX Mainnet",
    symbol: "DYDX",
    color: "#22c55e",
    enabled: false,
    prefix: "dydx1",
    routeId: "",
  },
  {
    id: "dydx-testnet-dydx",
    label: "dYdX Testnet (DYDX)",
    shortName: "dYdX Testnet",
    symbol: "DYDX",
    color: "#22c55e",
    enabled: false,
    prefix: "dydx1",
    routeId: "",
  },
  {
    id: "akash-mainnet-akt",
    label: "Akash Mainnet (AKT)",
    shortName: "Akash Mainnet",
    symbol: "AKT",
    color: "#f97316",
    enabled: false,
    prefix: "akash1",
    routeId: "",
  },
  {
    id: "akash-sandbox-akt",
    label: "Akash Sandbox (AKT)",
    shortName: "Akash Sandbox",
    symbol: "AKT",
    color: "#f97316",
    enabled: false,
    prefix: "akash1",
    routeId: "",
  },
];

const liveDestinations = destinations.filter((d) => d.enabled);
const soonDestinations = destinations.filter((d) => !d.enabled);

function ChainAvatar({
  destination,
  size,
  muted = false,
}: {
  destination: Destination;
  size: "md" | "sm";
  muted?: boolean;
}) {
  const className = [
    "chain-avatar",
    size === "sm" ? "chain-avatar--sm" : "",
    muted ? "chain-avatar--muted" : "",
  ]
    .filter(Boolean)
    .join(" ");

  if (destination.logo) {
    return (
      <img
        alt={destination.symbol}
        className={className}
        src={destination.logo}
        style={{ background: "transparent" }}
      />
    );
  }

  return (
    <span className={className} style={{ background: destination.color }}>
      {destination.symbol.slice(0, 2)}
    </span>
  );
}

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
        routeId: destination.routeId,
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
          <p className="eyebrow eyebrow--dark">Sepolia → Cosmos</p>
          <h2>Transfer</h2>
        </div>
        <div className={`wallet-chip${wallet.isWrongChain ? " wallet-chip--warning" : ""}`}>
          {wallet.isWrongChain
            ? "Wrong chain"
            : wallet.address
              ? `${wallet.address.slice(0, 6)}…${wallet.address.slice(-4)}`
              : "—"}
        </div>
      </div>

      <div className="field-grid">
        <div className="transfer-row">
          <div className="field-card field-card--amount">
            <div className="amount-header">
              <label className="field-label" htmlFor="bridge-amount">
                Amount
              </label>
              <div className="amount-display">
                <span className="amount-display__value">
                  {parseFloat(amount).toFixed(3)}
                </span>
                <span className="amount-display__unit">ETH</span>
              </div>
            </div>
            <div className="amount-slider-wrap">
              <input
                aria-label="Amount"
                className="amount-slider"
                id="bridge-amount"
                max="1"
                min="0.001"
                onChange={(event) =>
                  setAmount(parseFloat(event.target.value).toFixed(3))
                }
                step="0.001"
                style={
                  {
                    "--slider-pct": `${Math.round(parseFloat(amount) * 100)}%`,
                  } as { [key: string]: string }
                }
                type="range"
                value={amount}
              />
            </div>
            <div className="amount-presets">
              {(["0.050", "0.100", "0.250", "0.500", "1.000"] as const).map(
                (v) => (
                  <button
                    className={`amount-preset${amount === v ? " amount-preset--active" : ""}`}
                    key={v}
                    onClick={() => setAmount(v)}
                    type="button"
                  >
                    {parseFloat(v)}
                  </button>
                ),
              )}
            </div>
          </div>

          <div className="field-card field-card--destination">
            <label className="field-label">To</label>
            <div className="chain-picker">
              <button
                aria-expanded={isDestinationMenuOpen}
                aria-haspopup="menu"
                aria-label={`Destination chain: ${destination.label}`}
                className="chain-trigger"
                onClick={() => setIsDestinationMenuOpen((value) => !value)}
                type="button"
              >
                <ChainAvatar destination={destination} size="md" />
                <span className="chain-trigger__name">{destination.shortName}</span>
                <span className="chain-trigger__chevron" aria-hidden="true">
                  {isDestinationMenuOpen ? "▲" : "▼"}
                </span>
              </button>

              {isDestinationMenuOpen ? (
                <div className="chain-menu" role="menu">
                  <p className="chain-menu__group">Live now</p>
                  {liveDestinations.map((option) => (
                    <button
                      className={`chain-option${option.id === destination.id ? " chain-option--active" : ""}`}
                      key={option.id}
                      onClick={() => {
                        setSelectedDestinationId(option.id);
                        setRecipient("");
                        setSubmissionError(null);
                        setIsDestinationMenuOpen(false);
                      }}
                      role="menuitem"
                      type="button"
                    >
                      <ChainAvatar destination={option} size="sm" />
                      <span className="chain-option__info">
                        <strong className="chain-option__name">{option.shortName}</strong>
                        <span className="chain-option__symbol">{option.symbol}</span>
                      </span>
                      <span className="chain-badge chain-badge--live">● Live</span>
                    </button>
                  ))}

                  <div className="chain-menu__divider" />
                  <p className="chain-menu__group">Coming soon</p>
                  {soonDestinations.map((option) => (
                    <button
                      className="chain-option chain-option--soon"
                      disabled
                      key={option.id}
                      role="menuitem"
                      type="button"
                    >
                      <ChainAvatar destination={option} size="sm" muted />
                      <span className="chain-option__info">
                        <strong className="chain-option__name">{option.shortName}</strong>
                        <span className="chain-option__symbol">{option.symbol}</span>
                      </span>
                      <span className="chain-badge chain-badge--soon">Soon</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        </div>

        <div className="field-card">
          <label className="field-label" htmlFor="bridge-recipient">
            Recipient address
          </label>
          <input
            aria-label="Recipient"
            className={`field-input field-input--recipient ${recipientIsValid ? "field-input--valid" : recipient.length > 0 ? "field-input--invalid" : ""}`}
            id="bridge-recipient"
            onChange={(event) => setRecipient(event.target.value)}
            placeholder={`${destination.prefix}1…`}
            value={recipient}
          />
          <span className="field-helper">
            Starts with <code className="field-prefix-hint">{destination.prefix}</code>
          </span>
          {recipient.length > 0 && !recipientIsValid ? (
            <p className="field-error">
              Must start with <strong>{destination.prefix}</strong> and be at least {destination.prefix.length + 9} characters.
            </p>
          ) : recipientIsValid ? (
            <p className="field-success">Valid address ✓</p>
          ) : null}
        </div>
      </div>

      {canSubmit ? (
        <div className="transfer-summary">
          <span className="transfer-summary__from">{amount} ETH</span>
          <span className="transfer-summary__arrow">→</span>
          <span className="transfer-summary__to">{destination.label}</span>
        </div>
      ) : !wallet.isConnected ? (
        <p className="submit-hint">Connect your Sepolia wallet to continue.</p>
      ) : wallet.isWrongChain ? (
        <p className="submit-hint submit-hint--warn">Switch to Sepolia to bridge.</p>
      ) : !amountIsValid ? (
        <p className="submit-hint">Enter a valid ETH amount above.</p>
      ) : !recipientIsValid ? (
        <p className="submit-hint">Enter a valid {destination.prefix} address above.</p>
      ) : null}

      <button
        className="primary-cta"
        disabled={!canSubmit}
        onClick={() => void handleSubmit()}
        type="button"
      >
        {isSubmitting
          ? "Opening Bridge Tunnel…"
          : canSubmit
            ? `Bridge ${amount} ETH → ${destination.symbol}`
            : `Bridge to ${destination.label}`}
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
