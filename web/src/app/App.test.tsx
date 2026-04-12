import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "./App";

const useBridgeWalletMock = vi.fn();
const useWalletClientMock = vi.fn();

vi.mock("../features/wallet/useBridgeWallet", () => ({
  useBridgeWallet: () => useBridgeWalletMock(),
}));

vi.mock("wagmi", async () => {
  const actual = await vi.importActual<typeof import("wagmi")>("wagmi");

  return {
    ...actual,
    useWalletClient: () => useWalletClientMock(),
  };
});

describe("App", () => {
  it("renders the premium landing page before wallet connection", () => {
    useBridgeWalletMock.mockReturnValue({
      hasInjectedWallet: true,
      isConnected: false,
      isConnecting: false,
      isWrongChain: false,
      connect: vi.fn(),
      disconnect: vi.fn(),
      switchToSourceChain: vi.fn(),
    });
    useWalletClientMock.mockReturnValue({ data: undefined });

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

  it("keeps the landing page visible after wallet connection", async () => {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainName: "Sepolia",
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

    render(<App />);

    expect(
      screen.getByRole("heading", {
        name: /connect ethereum to the cosmos ecosystem/i,
      }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /^transfer$/i })).not.toBeInTheDocument();
  });

  it("opens the transfer card from the AegisLink dropdown", async () => {
    useBridgeWalletMock.mockReturnValue({
      address: "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
      chainName: "Sepolia",
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

    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: /open aegislink menu/i }));
    await user.click(screen.getByRole("menuitem", { name: /transfer/i }));

    expect(
      screen.getByRole("heading", { name: /^transfer$/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/osmosis testnet/i)).toBeInTheDocument();
  });
});
