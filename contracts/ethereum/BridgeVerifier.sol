// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./IBridgeVerifier.sol";

contract BridgeVerifier is IBridgeVerifier {
    bytes32 private constant EIP712_DOMAIN_TYPEHASH =
        keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)");
    bytes32 private constant BRIDGE_ATTESTATION_TYPEHASH =
        keccak256("BridgeAttestation(bytes32 messageId,bytes32 payloadHash,uint64 expiry)");
    bytes32 private constant NAME_HASH = keccak256("AegisLink Bridge Verifier");
    bytes32 private constant VERSION_HASH = keccak256("1");
    uint256 private constant SECP256K1_HALF_N =
        0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0;

    error InvalidAttestation();
    error AttestationExpired();
    error ProofAlreadyUsed();
    error NotOwner();
    error NotGateway();
    error InvalidGateway();
    error InvalidAttester();

    address public owner;
    address public gateway;
    address public attester;

    mapping(bytes32 => bool) public usedProofs;

    event GatewaySet(address indexed gateway);
    event AttesterSet(address indexed attester);

    constructor(address attester_) {
        if (attester_ == address(0)) revert InvalidAttester();
        owner = msg.sender;
        attester = attester_;
    }

    modifier onlyOwner() {
        if (msg.sender != owner) revert NotOwner();
        _;
    }

    modifier onlyGateway() {
        if (msg.sender != gateway) revert NotGateway();
        _;
    }

    function setGateway(address gateway_) external onlyOwner {
        if (gateway_ == address(0)) revert InvalidGateway();
        gateway = gateway_;
        emit GatewaySet(gateway_);
    }

    function setAttester(address attester_) external onlyOwner {
        if (attester_ == address(0)) revert InvalidAttester();
        attester = attester_;
        emit AttesterSet(attester_);
    }

    function verifyAndConsume(bytes32 messageId, bytes32 payloadHash, uint64 expiry, bytes calldata signature)
        external
        override
        onlyGateway
        returns (address signer)
    {
        if (block.timestamp > expiry) revert AttestationExpired();
        if (usedProofs[messageId]) revert ProofAlreadyUsed();

        bytes32 digest = attestationDigest(messageId, payloadHash, expiry);
        signer = _recover(digest, signature);
        if (signer != attester) revert InvalidAttestation();

        usedProofs[messageId] = true;
    }

    function activeSignerSetVersion() external pure override returns (uint64) {
        return 1;
    }

    function domainSeparator() public view returns (bytes32) {
        return keccak256(
            abi.encode(EIP712_DOMAIN_TYPEHASH, NAME_HASH, VERSION_HASH, block.chainid, address(this))
        );
    }

    function attestationDigest(bytes32 messageId, bytes32 payloadHash, uint64 expiry) public view returns (bytes32) {
        bytes32 structHash = keccak256(abi.encode(BRIDGE_ATTESTATION_TYPEHASH, messageId, payloadHash, expiry));
        return keccak256(abi.encodePacked("\x19\x01", domainSeparator(), structHash));
    }

    function _recover(bytes32 digest, bytes calldata signature) internal pure returns (address signer) {
        if (signature.length != 65) revert InvalidAttestation();

        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := calldataload(signature.offset)
            s := calldataload(add(signature.offset, 32))
            v := byte(0, calldataload(add(signature.offset, 64)))
        }

        if (v < 27) {
            v += 27;
        }
        if (v != 27 && v != 28) revert InvalidAttestation();
        if (uint256(s) > SECP256K1_HALF_N) revert InvalidAttestation();

        signer = ecrecover(digest, v, r, s);
        if (signer == address(0)) revert InvalidAttestation();
    }
}
