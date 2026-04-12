import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TransferPage } from "./TransferPage";

const useBridgeWalletMock = vi.fn();
const useWalletClientMock = vi.fn();
const submitEthDepositMock = vi.fn();

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

    expect(screen.getByRole("button", { name: /osmosis testnet/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: /cosmos hub/i })).toBeDisabled();
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
    submitEthDepositMock.mockResolvedValue(
      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    );

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
});
