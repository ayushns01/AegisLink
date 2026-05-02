import { sepolia, mainnet } from "wagmi/chains";

export type NetworkMode = "testnet" | "mainnet";

export const sourceChain = sepolia; // kept for backward compat
export const TESTNET_SOURCE_CHAIN = sepolia; // chainId 11155111
export const MAINNET_SOURCE_CHAIN = mainnet; // chainId 1

export function getSourceChainForMode(mode: NetworkMode) {
  return mode === "mainnet" ? mainnet : sepolia;
}
