import { useMemo, useState } from "react";
import type { Address } from "viem";
import { useWalletClient } from "wagmi";
import { frontendEnv } from "../../lib/config/env";
import { submitEthDeposit } from "../../lib/evm/gateway";
import { useBridgeWallet } from "../wallet/useBridgeWallet";
import { createSubmittedBridgeSession, type BridgeSession } from "./bridge-session";
import { ProgressPanel } from "./ProgressPanel";

type Destination = {
  id: string;
  name: string;
  helper: string;
  enabled: boolean;
  prefix: string;
};

const destinations: Destination[] = [
  {
    id: "osmosis-testnet",
    name: "Osmosis Testnet",
    helper: "Live route available now",
    enabled: true,
    prefix: "osmo1",
  },
  {
    id: "cosmos-hub",
    name: "Cosmos Hub",
    helper: "Visible destination, route not live yet",
    enabled: false,
    prefix: "cosmos1",
  },
];

export function TransferPage() {
  const wallet = useBridgeWallet();
  const { data: walletClient } = useWalletClient();
  const [amount, setAmount] = useState("0.250");
  const [recipient, setRecipient] = useState(
    "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );
  const [session, setSession] = useState<BridgeSession | null>(null);
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const destination = destinations[0];
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
        recipient,
      });

      setSession(
        createSubmittedBridgeSession({
          amountEth: amount,
          destinationChain: destination.name,
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

  if (session) {
    return <ProgressPanel onReset={() => setSession(null)} session={session} />;
  }

  return (
    <div className="transfer-card">
      <div className="transfer-card__header">
        <div>
          <p className="eyebrow eyebrow--dark">Bridge</p>
          <h2>Transfer</h2>
          <p className="transfer-card__copy">
            Select a destination chain, enter the matching recipient address,
            and bridge ETH from the connected Sepolia wallet.
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
          <span>ETH from connected Sepolia wallet</span>
        </div>

        <div className="field-card">
          <small>Destination chain</small>
          <div className="chain-list">
            <button
              aria-label="Osmosis Testnet"
              aria-pressed="true"
              className="chain-row chain-row--active"
              type="button"
            >
              <div>
                <strong>Osmosis Testnet</strong>
                <span>Live route available now</span>
              </div>
              <em>Enabled</em>
            </button>
            <button
              aria-label="Cosmos Hub"
              className="chain-row"
              disabled
              type="button"
            >
              <div>
                <strong>Cosmos Hub</strong>
                <span>Visible destination, route not live yet</span>
              </div>
              <em>Soon</em>
            </button>
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
          <span>Recipient format adapts to the selected chain.</span>
          {!recipientIsValid ? (
            <p className="field-error">Enter a valid osmo1 recipient.</p>
          ) : null}
        </div>
      </div>

      <button
        className="primary-cta"
        disabled={!canSubmit}
        onClick={() => void handleSubmit()}
        type="button"
      >
        {isSubmitting ? "Submitting Deposit..." : "Bridge to Osmosis"}
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
