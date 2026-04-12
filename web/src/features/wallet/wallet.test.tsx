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
      hasInjectedWallet: true,
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
      hasInjectedWallet: true,
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

  it("shows an install state when no wallet extension is available", () => {
    useBridgeWalletMock.mockReturnValue({
      hasInjectedWallet: false,
      isConnected: false,
      isConnecting: false,
      isWrongChain: false,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });

    render(<WalletConnectButton />);

    expect(
      screen.getByRole("button", { name: /install wallet extension/i }),
    ).toBeDisabled();
  });
});
