import { useMemo, useState } from "react";

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
  const [amount, setAmount] = useState("0.250");
  const [recipient, setRecipient] = useState(
    "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );
  const destination = destinations[0];
  const recipientIsValid = useMemo(
    () => recipient.startsWith(destination.prefix) && recipient.length > destination.prefix.length + 8,
    [destination.prefix, recipient],
  );
  const amountIsValid = useMemo(() => {
    const parsed = Number(amount);
    return Number.isFinite(parsed) && parsed > 0;
  }, [amount]);

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
        <div className="wallet-chip">Wallet connected</div>
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
        disabled={!amountIsValid || !recipientIsValid}
        type="button"
      >
        Bridge to Osmosis
      </button>
    </div>
  );
}
