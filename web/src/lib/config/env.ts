import bridgeAddresses from "../../../../deploy/testnet/sepolia/bridge-addresses.json";

type FrontendEnv = {
  gatewayAddress: `0x${string}`;
  mainnetGatewayAddress: `0x${string}`;
  aegislinkDepositRecipient: string;
  statusApiBaseUrl: string;
  sepoliaExplorerBaseUrl: string;
  ethereumExplorerBaseUrl: string;
};

const defaultAegislinkDepositRecipient =
  "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4";

export const unconfiguredMainnetGatewayAddress =
  "0x0000000000000000000000000000000000000000" as `0x${string}`;

export const frontendEnv: FrontendEnv = {
  gatewayAddress:
    (import.meta.env.VITE_BRIDGE_GATEWAY_ADDRESS as `0x${string}` | undefined) ??
    (bridgeAddresses.gateway_address as `0x${string}`),
  mainnetGatewayAddress:
    (import.meta.env.VITE_BRIDGE_MAINNET_GATEWAY_ADDRESS as `0x${string}` | undefined) ??
    unconfiguredMainnetGatewayAddress,
  aegislinkDepositRecipient:
    import.meta.env.VITE_BRIDGE_AEGISLINK_DEPOSIT_RECIPIENT ??
    defaultAegislinkDepositRecipient,
  statusApiBaseUrl: import.meta.env.VITE_BRIDGE_STATUS_API_BASE_URL ?? "",
  sepoliaExplorerBaseUrl:
    import.meta.env.VITE_SEPOLIA_EXPLORER_BASE_URL ?? "https://sepolia.etherscan.io",
  ethereumExplorerBaseUrl:
    import.meta.env.VITE_ETHEREUM_EXPLORER_BASE_URL ?? "https://etherscan.io",
};

export const isMainnetGatewayConfigured = () =>
  frontendEnv.mainnetGatewayAddress !== unconfiguredMainnetGatewayAddress;
