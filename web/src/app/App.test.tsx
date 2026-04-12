import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { App } from "./App";
const useBridgeWalletMock = vi.fn();

vi.mock("../features/wallet/useBridgeWallet", () => ({
  useBridgeWallet: () => useBridgeWalletMock(),
}));

describe("App", () => {
  it("renders the premium landing page before wallet connection", () => {
    useBridgeWalletMock.mockReturnValue({
      isConnected: false,
      isConnecting: false,
      isWrongChain: false,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });

    render(<App />);

    expect(
      screen.getByRole("heading", {
        name: /connect ethereum to the cosmos ecosystem/i,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /connect wallet/i }),
    ).toBeInTheDocument();
  });

  it("moves into the connected shell and shows transfer after connect", async () => {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainName: "Sepolia",
      isConnected: true,
      isConnecting: false,
      isWrongChain: false,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });

    render(<App />);

    expect(
      screen.getByRole("heading", { name: /^transfer$/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/osmosis testnet/i)).toBeInTheDocument();
    expect(screen.getByText(/cosmos hub/i)).toBeInTheDocument();
  });
});
