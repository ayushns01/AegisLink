import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WalletConnectButton } from "./WalletConnectButton";
import { WalletStatus } from "./WalletStatus";

const useBridgeWalletMock = vi.fn();

vi.mock("./useBridgeWallet", () => ({
  useBridgeWallet: () => useBridgeWalletMock(),
}));

describe("wallet components", () => {
  it("shows a connect button when disconnected", () => {
    useBridgeWalletMock.mockReturnValue({
      isConnected: false,
      isConnecting: false,
      isWrongChain: false,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });

    render(<WalletConnectButton />);

    expect(
      screen.getByRole("button", { name: /connect wallet/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/wallet disconnected/i)).not.toBeInTheDocument();
  });

  it("shows wrong-chain state when connected to a non-sepolia network", () => {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainName: "Ethereum",
      isConnected: true,
      isConnecting: false,
      isWrongChain: true,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });

    render(<WalletStatus />);

    expect(screen.getByText(/wrong chain/i)).toBeInTheDocument();
    expect(screen.getByText(/switch to sepolia/i)).toBeInTheDocument();
  });
});
