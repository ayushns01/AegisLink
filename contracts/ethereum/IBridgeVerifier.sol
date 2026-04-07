// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

interface IBridgeVerifier {
    function verifyAndConsume(bytes32 messageId, bytes32 payloadHash, uint64 expiry, bytes calldata proof)
        external
        returns (address signer);

    function activeSignerSetVersion() external view returns (uint64);
}
