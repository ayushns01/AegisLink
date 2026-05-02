import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useBridgeWallet } from "./useBridgeWallet";

const useAccountMock = vi.fn();
const useConnectMock = vi.fn();
const useDisconnectMock = vi.fn();
const useSwitchChainMock = vi.fn();

vi.mock("wagmi", () => ({
  useAccount: () => useAccountMock(),
  useConnect: () => useConnectMock(),
  useDisconnect: () => useDisconnectMock(),
  useSwitchChain: () => useSwitchChainMock(),
}));

describe("useBridgeWallet", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    delete (window as Window & { ethereum?: unknown }).ethereum;
  });

  it('marks Sepolia as the wrong chain in "mainnet" mode', () => {
    useAccountMock.mockReturnValue({
      address: "0x123",
      chain: {
        id: 11155111,
        name: "Sepolia",
      },
      isConnected: true,
    });
    useConnectMock.mockReturnValue({
      connectAsync: vi.fn(),
      connectors: [],
      error: null,
      isPending: false,
    });
    useDisconnectMock.mockReturnValue({
      disconnect: vi.fn(),
    });
    useSwitchChainMock.mockReturnValue({
      switchChainAsync: vi.fn(),
    });

    const { result } = renderHook(() => useBridgeWallet("mainnet"));

    expect(result.current.isWrongChain).toBe(true);
  });

  it("connects through the configured injected connector", async () => {
    const configuredConnector = {
      id: "injected",
      name: "MetaMask",
      type: "injected",
    };
    const connectAsync = vi.fn().mockResolvedValue(undefined);

    useAccountMock.mockReturnValue({
      address: undefined,
      chain: undefined,
      isConnected: false,
    });
    useConnectMock.mockReturnValue({
      connectAsync,
      connectors: [configuredConnector],
      error: null,
      isPending: false,
    });
    useDisconnectMock.mockReturnValue({
      disconnect: vi.fn(),
    });
    useSwitchChainMock.mockReturnValue({
      switchChainAsync: vi.fn(),
    });

    const { result } = renderHook(() => useBridgeWallet());

    await result.current.connect();

    expect(connectAsync).toHaveBeenCalledWith({
      chainId: 11155111,
      connector: configuredConnector,
    });
  });

  it('uses Ethereum mainnet for connect() in "mainnet" mode', async () => {
    const configuredConnector = {
      id: "injected",
      name: "MetaMask",
      type: "injected",
    };
    const connectAsync = vi.fn().mockResolvedValue(undefined);

    useAccountMock.mockReturnValue({
      address: undefined,
      chain: undefined,
      isConnected: false,
    });
    useConnectMock.mockReturnValue({
      connectAsync,
      connectors: [configuredConnector],
      error: null,
      isPending: false,
    });
    useDisconnectMock.mockReturnValue({
      disconnect: vi.fn(),
    });
    useSwitchChainMock.mockReturnValue({
      switchChainAsync: vi.fn(),
    });

    const { result } = renderHook(() => useBridgeWallet("mainnet"));

    await result.current.connect();

    expect(connectAsync).toHaveBeenCalledWith({
      chainId: 1,
      connector: configuredConnector,
    });
  });

  it('uses Ethereum mainnet for switchToSourceChain() in "mainnet" mode', async () => {
    const switchChainAsync = vi.fn().mockResolvedValue(undefined);

    useAccountMock.mockReturnValue({
      address: "0x123",
      chain: {
        id: 11155111,
        name: "Sepolia",
      },
      isConnected: true,
    });
    useConnectMock.mockReturnValue({
      connectAsync: vi.fn(),
      connectors: [],
      error: null,
      isPending: false,
    });
    useDisconnectMock.mockReturnValue({
      disconnect: vi.fn(),
    });
    useSwitchChainMock.mockReturnValue({
      switchChainAsync,
    });

    const { result } = renderHook(() => useBridgeWallet("mainnet"));

    await result.current.switchToSourceChain();

    expect(switchChainAsync).toHaveBeenCalledWith({
      chainId: 1,
    });
  });

  it("reports the extension as unavailable when no injected connector or provider exists", () => {
    useAccountMock.mockReturnValue({
      address: undefined,
      chain: undefined,
      isConnected: false,
    });
    useConnectMock.mockReturnValue({
      connectAsync: vi.fn(),
      connectors: [],
      error: null,
      isPending: false,
    });
    useDisconnectMock.mockReturnValue({
      disconnect: vi.fn(),
    });
    useSwitchChainMock.mockReturnValue({
      switchChainAsync: vi.fn(),
    });

    const { result } = renderHook(() => useBridgeWallet());

    expect(result.current.hasInjectedWallet).toBe(false);
  });
});
