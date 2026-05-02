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
const originalMainnetGatewayAddress = frontendEnv.mainnetGatewayAddress;

Object.defineProperty(globalThis, "fetch", {
  configurable: true,
  value: fetchMock,
});

vi.mock("../wallet/useBridgeWallet", () => ({
  useBridgeWallet: (...args: unknown[]) => useBridgeWalletMock(...args),
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
  frontendEnv.mainnetGatewayAddress = originalMainnetGatewayAddress;
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

  function seedMainnetWallet() {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainId: 1,
      chainName: "Ethereum",
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
        chain: { id: 1, name: "Ethereum" },
      },
    });
    frontendEnv.mainnetGatewayAddress = "0x1111111111111111111111111111111111111111";
  }

  it("shows Osmosis enabled and future chains disabled", () => {
    seedConnectedWallet();
    render(<TransferPage />);

    const triggerButton = screen.getByRole("button", {
      name: /destination chain: osmosis testnet \(osmo\)/i,
    });
    expect(triggerButton).toBeInTheDocument();
    expect(within(triggerButton).getByText("Osmosis Testnet")).toBeInTheDocument();
  });

  it("shows only testnet destinations in testnet mode dropdown", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    );

    expect(screen.getByRole("menuitem", { name: /osmosis testnet \(osmo\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /neutron testnet \(ntrn\)/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /osmosis mainnet \(osmo\)/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /neutron mainnet \(ntrn\)/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /celestia mocha testnet \(tia\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /injective testnet \(inj\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /dydx testnet \(dydx\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /akash sandbox \(akt\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menu")).toHaveClass("chain-menu");
    expect(
      screen.getByRole("menuitem", { name: /osmosis testnet \(osmo\)/i }),
    ).toHaveClass("chain-option--active");
  });

  it("updates the transfer form inputs and validates the osmosis recipient", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    const recipientInput = screen.getByLabelText(/recipient/i);
    await user.clear(recipientInput);
    await user.type(recipientInput, "bad-recipient");

    expect(screen.getByText(/must start with/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /bridge.*osmo/i }),
    ).toBeDisabled();
  });

  it("submits a Sepolia deposit and shows the bridge session progress", async () => {
    seedConnectedWallet();
    frontendEnv.statusApiBaseUrl = "";
    frontendEnv.aegislinkDepositRecipient =
      "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4";
    submitEthDepositMock.mockResolvedValue(
      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    const button = screen.getByRole("button", { name: /bridge.*osmo/i });
    await user.click(button);

    await waitFor(() => {
      expect(submitEthDepositMock).toHaveBeenCalled();
    });

    expect(
      screen.getByRole("heading", { name: /transfer in progress/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /transfer in progress/i }).closest(".transfer-card")).toHaveClass(
      "transfer-card--progress-expanded",
    );
    expect(screen.getByRole("heading", { name: /transfer in progress/i }).closest(".transfer-card")).toHaveClass(
      "transfer-card--progress-obsidian",
    );
    expect(screen.getByRole("heading", { name: /transfer in progress/i }).closest(".transfer-card")).toHaveClass(
      "transfer-card--progress-contained",
    );
    expect(screen.getByText(/transfer route/i)).toBeInTheDocument();
    expect(screen.getByText(/bridge tunnel/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/bridge tunnel/i)).toHaveClass("progress-scene--ignited");
    expect(screen.getByLabelText(/bridge tunnel/i)).toHaveClass("progress-scene--abyss");
    const viewport = screen.getByTestId("progress-scene-viewport");
    expect(viewport).toBeInTheDocument();
    expect(screen.getByTestId("progress-bridge-glow")).toBeInTheDocument();
    expect(screen.queryByTestId("progress-flow-lane")).not.toBeInTheDocument();
    expect(screen.queryByTestId("progress-energy-packet")).not.toBeInTheDocument();
    expect(screen.getByTestId("progress-core-aura")).toBeInTheDocument();
    expect(screen.getByTestId("progress-core-shell")).toBeInTheDocument();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelector(".progress-scene__particles"),
    ).toBeNull();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelector(".progress-scene__tunnel svg"),
    ).toBeNull();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelector(".progress-scene__stream"),
    ).toBeNull();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelector(".progress-scene__bridge-glow--core"),
    ).not.toBeNull();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelector(".progress-scene__bridge-glow--portal-left"),
    ).not.toBeNull();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelector(".progress-scene__bridge-glow--portal-right"),
    ).not.toBeNull();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelectorAll(".progress-route__checkpoint"),
    ).toHaveLength(5);
    expect(
      screen.getByRole("heading", { name: /sepolia confirmed/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelectorAll(".progress-stage"),
    ).toHaveLength(0);
    expect(screen.queryByText(/verifier checks/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/bridge accounting/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/ibc handoff/i)).not.toBeInTheDocument();
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
    await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

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

    await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

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
      "https://www.mintscan.io/osmosis-testnet/tx/5E40ED4BF5B065DA159D66785534EAAEEE376876749DADAF639F6A51524B2F7D",
    );
  });

  it("builds a Mintscan link from the Osmosis tx hash when the backend does not provide one", async () => {
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
        destinationTxHash: "4EC091441766C154ABE9C9D4DE0D8F2CA89AB9D7B8C72F3CBFF9C2FCCFC6C1A9",
      }),
    });

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

    await waitFor(() => {
      expect(
        screen.getByRole("link", {
          name: /4ec0914417/i,
        }),
      ).toHaveAttribute(
        "href",
        "https://www.mintscan.io/osmosis-testnet/tx/4EC091441766C154ABE9C9D4DE0D8F2CA89AB9D7B8C72F3CBFF9C2FCCFC6C1A9",
      );
    });
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

    await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/aegislink is verifying bridge policy/i),
      ).toBeInTheDocument();
    });

    expect(
      screen.getByRole("heading", { name: /verifier checks/i }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("progress-flow-lane")).not.toBeInTheDocument();
    expect(screen.queryByTestId("progress-energy-packet")).not.toBeInTheDocument();
    expect(
      screen.getByLabelText(/bridge tunnel/i).querySelectorAll(".progress-stage"),
    ).toHaveLength(0);
    expect(screen.queryByText(/bridge accounting/i)).not.toBeInTheDocument();
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
    await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

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

  it("shows Neutron testnet as a live enabled destination", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    );

    const neutronItem = screen.getByRole("menuitem", {
      name: /neutron testnet \(ntrn\)/i,
    });
    expect(neutronItem).toBeInTheDocument();
    expect(neutronItem).not.toBeDisabled();
    expect(within(neutronItem).getByText(/live/i)).toBeInTheDocument();
  });

  it("switches to Neutron testnet and validates neutron1 recipient prefix", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    );
    await user.click(
      screen.getByRole("menuitem", { name: /neutron testnet \(ntrn\)/i }),
    );

    const recipientInput = screen.getByLabelText(/recipient/i);
    await user.clear(recipientInput);
    await user.type(recipientInput, "osmo1shouldfailneutronprefix");

    expect(
      screen.getByText(/enter a valid neutron1/i),
    ).toBeInTheDocument();

    await user.clear(recipientInput);
    await user.type(
      recipientInput,
      "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );

    expect(
      screen.queryByText(/enter a valid neutron1/i),
    ).not.toBeInTheDocument();
  });

  it("registers delivery intent with neutron-public-wallet routeId when Neutron is selected", async () => {
    seedConnectedWallet();
    submitEthDepositMock.mockResolvedValue(
      "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    // Switch to Neutron.
    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    );
    await user.click(
      screen.getByRole("menuitem", { name: /neutron testnet \(ntrn\)/i }),
    );

    // Enter a valid neutron1 recipient.
    await user.clear(screen.getByLabelText(/recipient/i));
    await user.type(
      screen.getByLabelText(/recipient/i),
      "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );

    await user.click(
      screen.getByRole("button", { name: /bridge.*ntrn/i }),
    );

    await waitFor(() => {
      expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith(
        expect.objectContaining({
          routeId: "neutron-public-wallet",
          receiver: "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
        }),
      );
    });
  });

  it("shows the Testnet toggle button as active by default", () => {
    seedConnectedWallet();
    render(<TransferPage />);

    expect(screen.getByRole("button", { name: /^testnet$/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: /^mainnet$/i })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("switches to mainnet mode and shows only mainnet destinations", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

    expect(screen.getByRole("button", { name: /^mainnet$/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis mainnet \(osmo\)/i,
      }),
    );

    expect(screen.getByRole("menuitem", { name: /osmosis mainnet \(osmo\)/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /neutron mainnet \(ntrn\)/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /osmosis testnet \(osmo\)/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /neutron testnet \(ntrn\)/i })).not.toBeInTheDocument();
  });

  it("resets recipient address when switching network mode", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    await user.clear(screen.getByLabelText(/recipient/i));
    await user.type(
      screen.getByLabelText(/recipient/i),
      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );

    expect(screen.getByLabelText(/recipient/i)).toHaveValue(
      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );

    await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

    expect(screen.getByLabelText(/recipient/i)).toHaveValue("");
  });

  it("shows Ethereum to Cosmos eyebrow in mainnet mode", async () => {
    seedConnectedWallet();
    const user = userEvent.setup();
    render(<TransferPage />);

    expect(screen.getByText(/sepolia → cosmos/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

    expect(screen.getByText(/ethereum → cosmos/i)).toBeInTheDocument();
    expect(screen.queryByText(/sepolia → cosmos/i)).not.toBeInTheDocument();
  });

  it("shows Switch to Ethereum mainnet hint when isWrongChain in mainnet mode", async () => {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainId: 11155111,
      chainName: "Sepolia",
      hasInjectedWallet: true,
      isConnected: true,
      isConnecting: false,
      isWrongChain: true,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });
    useWalletClientMock.mockReturnValue({
      data: {
        chain: { id: 11155111, name: "Sepolia" },
      },
    });

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

    expect(screen.getByText(/switch to ethereum mainnet/i)).toBeInTheDocument();
  });

  it("passes osmosis-mainnet-wallet routeId in mainnet mode", async () => {
    seedMainnetWallet();
    submitEthDepositMock.mockResolvedValue(
      "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /^mainnet$/i }));

    await user.clear(screen.getByLabelText(/recipient/i));
    await user.type(
      screen.getByLabelText(/recipient/i),
      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );

    await user.click(screen.getByRole("button", { name: /bridge.*osmo/i }));

    await waitFor(() => {
      expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith(
        expect.objectContaining({ routeId: "osmosis-mainnet-wallet" }),
      );
    });
  });

  it("passes neutron-mainnet-wallet routeId when mainnet and Neutron mainnet selected", async () => {
    seedMainnetWallet();
    submitEthDepositMock.mockResolvedValue(
      "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
    );
    registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<TransferPage />);

    await user.click(screen.getByRole("button", { name: /^mainnet$/i }));
    await user.click(
      screen.getByRole("button", {
        name: /destination chain: osmosis mainnet \(osmo\)/i,
      }),
    );
    await user.click(
      screen.getByRole("menuitem", { name: /neutron mainnet \(ntrn\)/i }),
    );

    await user.clear(screen.getByLabelText(/recipient/i));
    await user.type(
      screen.getByLabelText(/recipient/i),
      "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
    );

    await user.click(screen.getByRole("button", { name: /bridge.*ntrn/i }));

    await waitFor(() => {
      expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith(
        expect.objectContaining({ routeId: "neutron-mainnet-wallet" }),
      );
    });
  });
});
