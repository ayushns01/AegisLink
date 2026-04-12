import type { Address, Hex, WalletClient } from "viem";
import { parseEther } from "viem";

const bridgeGatewayAbi = [
  {
    inputs: [
      { internalType: "string", name: "recipient", type: "string" },
      { internalType: "uint64", name: "expiry", type: "uint64" },
    ],
    name: "depositETH",
    outputs: [{ internalType: "bytes32", name: "messageId", type: "bytes32" }],
    stateMutability: "payable",
    type: "function",
  },
] as const;

type SubmitEthDepositArgs = {
  walletClient: WalletClient;
  gatewayAddress: Address;
  account: Address;
  amountEth: string;
  recipient: string;
  now?: () => number;
};

export async function submitEthDeposit({
  walletClient,
  gatewayAddress,
  account,
  amountEth,
  recipient,
  now = () => Date.now(),
}: SubmitEthDepositArgs): Promise<Hex> {
  const expiry = BigInt(Math.floor(now() / 1000) + 60 * 60);

  return walletClient.writeContract({
    abi: bridgeGatewayAbi,
    account,
    address: gatewayAddress,
    functionName: "depositETH",
    args: [recipient, expiry],
    value: parseEther(amountEth),
    chain: walletClient.chain,
  });
}
