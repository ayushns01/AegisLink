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
    expect(
      screen.getByRole("button", { name: /open aegislink menu/i }),
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

  it("opens the transfer page from the AegisLink dropdown", async () => {
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
      screen.getByRole("heading", { name: /^transfer$/i }).closest(".landing-transfer-card"),
    ).toHaveClass("landing-transfer-card--compact");
    expect(
      screen.getByRole("heading", { name: /^transfer$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: /destination chain: osmosis testnet \(osmo\)/i,
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", {
        name: /connect ethereum to the cosmos ecosystem/i,
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/ethereum to cosmos bridge surface/i),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/sepolia source/i)).not.toBeInTheDocument();
  });

  it("opens a standalone About page from the AegisLink dropdown and updates the stage explainer", async () => {
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
    await user.click(screen.getByRole("menuitem", { name: /about/i }));

    expect(
      screen.getByRole("heading", { name: /how the bridge works/i }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /^transfer$/i })).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /deposit signed/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /system architecture/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/stage inspector/i)).not.toBeInTheDocument();
    expect(screen.getByText(/what's happening now/i)).toBeInTheDocument();
    expect(screen.getByText(/inside aegislink/i)).toBeInTheDocument();
    expect(screen.getByText(/why this matters/i)).toBeInTheDocument();
    expect(
      screen.getByText(/the connected sepolia wallet submits the bridge transaction/i),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /ibc handoff/i }));

    expect(screen.getByRole("button", { name: /ibc handoff/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText(/what's happening now/i)).toBeInTheDocument();
    expect(
      screen.getByText(/aegislink initiates the outbound route toward osmosis/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/route, timeout policy, and packet state are created/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/this is the moment where the system turns verification into cross-chain delivery/i),
    ).toBeInTheDocument();
  });

  it("renders only the transfer wormhole preview when requested", () => {
    window.history.pushState({}, "", "/?preview=wormhole&stage=handoff");
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

    expect(screen.getByRole("heading", { name: /transfer in progress/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /ibc handoff/i })).toBeInTheDocument();
    expect(screen.queryByTestId("progress-energy-packet")).not.toBeInTheDocument();
    expect(screen.queryByTestId("progress-flow-lane")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", {
        name: /connect ethereum to the cosmos ecosystem/i,
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /connect wallet/i }),
    ).not.toBeInTheDocument();
  });
});
