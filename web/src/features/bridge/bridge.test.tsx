import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { frontendEnv } from "../../lib/config/env";
import { TransferPage } from "./TransferPage";

const useBridgeWalletMock = vi.fn();
const useWalletClientMock = vi.fn();
const submitEthDepositMock = vi.fn();
const registerBridgeDeliveryIntentMock = vi.fn();
const fetchMock = vi.fn();

const originalStatusApiBaseUrl = frontendEnv.statusApiBaseUrl;
const originalAegislinkDepositRecipient = frontendEnv.aegislinkDepositRecipient;

Object.defineProperty(globalThis, "fetch", {
  configurable: true,
  value: fetchMock,
});

vi.mock("../wallet/useBridgeWallet", () => ({
  useBridgeWallet: () => useBridgeWalletMock(),
}));

vi.mock("wagmi", async () => {
  const actual = await vi.importActual<typeof import("wagmi")>("wagmi");

  return {
    ...actual,
    useWalletClient: () => useWalletClientMock(),
  };
});

vi.mock("../../lib/evm/gateway", () => ({
  submitEthDeposit: (...args: unknown[]) => submitEthDepositMock(...args),
}));

vi.mock("../../lib/bridge/delivery-intent", () => ({
  registerBridgeDeliveryIntent: (...args: unknown[]) =>
    registerBridgeDeliveryIntentMock(...args),
}));

afterEach(() => {
  frontendEnv.statusApiBaseUrl = originalStatusApiBaseUrl;
  frontendEnv.aegislinkDepositRecipient = originalAegislinkDepositRecipient;
  fetchMock.mockReset();
  registerBridgeDeliveryIntentMock.mockReset();
});

describe("TransferPage", () => {
  function seedConnectedWallet() {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainId: 11155111,
      chainName: "Sepolia",
      connectionError: undefined,
      hasInjectedWallet: true,
      isConnected: true,
      isConnecting: false,
      isWrongChain: false,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });
    useWalletClientMock.mockReturnValue({
      data: {
        chain: { id: 11155111, name: "Sepolia" },
      },
    });
  }

  it("shows Osmosis enabled and future chains disabled", () => {
    seedConnectedWallet();
    render(<TransferPage />);

    expect(
      screen.getByText(/osmosis testnet \(osmo\)/i),
    ).toHaveClass("destination-trigger__label--active");
    expect(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    ).toBeInTheDocument();
  });

  it("shows mainnet and testnet destination options in the dropdown", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    );

    expect(screen.getByRole("menuitem", { name: /osmosis testnet \(osmo\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /osmosis mainnet \(osmo\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /celestia mainnet \(tia\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /celestia mocha testnet \(tia\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /injective mainnet \(inj\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /injective testnet \(inj\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /dydx mainnet \(dydx\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /dydx testnet \(dydx\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /akash mainnet \(akt\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /akash sandbox \(akt\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menu")).toHaveClass("destination-menu--scrollable");
    expect(
      within(
        screen.getByRole("menuitem", { name: /osmosis testnet \(osmo\)/i }),
      ).getByText(/osmosis testnet \(osmo\)/i),
    ).toHaveClass("destination-option__title--active");
  });

  it("updates the transfer form inputs and validates the osmosis recipient", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    const amountInput = screen.getByLabelText(/amount/i);
    const recipientInput = screen.getByLabelText(/recipient/i);

    await user.clear(amountInput);
    await user.type(amountInput, "0.75");
    await user.clear(recipientInput);
    await user.type(recipientInput, "bad-recipient");

    expect(amountInput).toHaveValue("0.75");
    expect(screen.getByText(/enter a valid osmo1 recipient/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /bridge to osmosis/i }),
    ).toBeDisabled();
  });

  it("submits a Sepolia deposit and shows the bridge session progress", async () => {
    seedConnectedWallet();
    frontendEnv.aegislinkDepositRecipient =
      "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4";
    submitEthDepositMock.mockResolvedValue(
      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    const button = screen.getByRole("button", { name: /bridge to osmosis/i });
    await user.click(button);

    await waitFor(() => {
      expect(submitEthDepositMock).toHaveBeenCalled();
    });

    expect(
      screen.getByRole("heading", { name: /transfer in progress/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/deposit submitted on sepolia/i)).toBeInTheDocument();
    expect(screen.getByText(/0x12345678/i)).toBeInTheDocument();
  });

  it("submits the configured AegisLink bridge wallet as the deposit recipient", async () => {
    seedConnectedWallet();
    frontendEnv.aegislinkDepositRecipient =
      "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4";
    submitEthDepositMock.mockResolvedValue(
      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.clear(screen.getByLabelText(/recipient/i));
    await user.type(
      screen.getByLabelText(/recipient/i),
      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );
    await user.click(screen.getByRole("button", { name: /bridge to osmosis/i }));

    await waitFor(() => {
      expect(submitEthDepositMock).toHaveBeenCalledWith(
        expect.objectContaining({
          recipient: "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4",
        }),
      );
    });
  });

  it("polls a configured bridge status api and shows the final Osmosis tx hash", async () => {
    seedConnectedWallet();
    frontendEnv.statusApiBaseUrl = "https://status.aegislink.test";
    submitEthDepositMock.mockResolvedValue(
      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        sourceTxHash:
          "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        status: "completed",
        destinationTxHash: "5E40ED4BF5B065DA159D66785534EAAEEE376876749DADAF639F6A51524B2F7D",
        destinationTxUrl:
          "https://www.mintscan.io/osmosis-testnet/txs/5E40ED4BF5B065DA159D66785534EAAEEE376876749DADAF639F6A51524B2F7D",
      }),
    });

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /bridge to osmosis/i }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "https://status.aegislink.test/bridge-status?sourceTxHash=0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        expect.objectContaining({ method: "GET" }),
      );
    });

    expect(screen.getByText(/confirmed by the configured bridge status source/i)).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: /5e40ed4bf5/i,
      }),
    ).toHaveAttribute(
      "href",
      "https://www.mintscan.io/osmosis-testnet/txs/5E40ED4BF5B065DA159D66785534EAAEEE376876749DADAF639F6A51524B2F7D",
    );
  });

  it("shows the AegisLink processing stage once Sepolia confirmation is complete", async () => {
    seedConnectedWallet();
    frontendEnv.statusApiBaseUrl = "https://status.aegislink.test";
    submitEthDepositMock.mockResolvedValue(
      "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        sourceTxHash:
          "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
        status: "aegislink_processing",
        messageId: "5355ecdd643688f596694128c127ed62cdfba1bba5d605ef4e9704b5e035382f",
      }),
    });

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /bridge to osmosis/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/aegislink is validating and crediting your bridged eth/i),
      ).toBeInTheDocument();
    });

    expect(screen.getByText(/sepolia confirmed/i)).toBeInTheDocument();
    expect(screen.queryByText(/sepolia confirmation pending/i)).not.toBeInTheDocument();
  });

  it("registers a delivery intent with the local bridge operator after deposit submission", async () => {
    seedConnectedWallet();
    submitEthDepositMock.mockResolvedValue(
      "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.clear(screen.getByLabelText(/recipient/i));
    await user.type(
      screen.getByLabelText(/recipient/i),
      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );
    await user.click(screen.getByRole("button", { name: /bridge to osmosis/i }));

    await waitFor(() => {
      expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith({
        amount: "250000000000000000",
        assetId: "eth",
        receiver: "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
        routeId: "osmosis-public-wallet",
        sender: "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4",
        sourceTxHash: "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
      });
    });
  });
});
