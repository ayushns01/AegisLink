import bridgeAddresses from "../../../../deploy/testnet/sepolia/bridge-addresses.json";

type FrontendEnv = {
  gatewayAddress: `0x${string}`;
  statusApiBaseUrl: string;
  sepoliaExplorerBaseUrl: string;
};

export const frontendEnv: FrontendEnv = {
  gatewayAddress:
    (import.meta.env.VITE_BRIDGE_GATEWAY_ADDRESS as `0x${string}` | undefined) ??
    (bridgeAddresses.gateway_address as `0x${string}`),
  statusApiBaseUrl: import.meta.env.VITE_BRIDGE_STATUS_API_BASE_URL ?? "",
  sepoliaExplorerBaseUrl:
    import.meta.env.VITE_SEPOLIA_EXPLORER_BASE_URL ?? "https://sepolia.etherscan.io",
};
