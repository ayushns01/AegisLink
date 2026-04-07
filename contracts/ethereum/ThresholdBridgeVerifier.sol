// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./IBridgeVerifier.sol";

contract ThresholdBridgeVerifier is IBridgeVerifier {
    error InvalidAttestation();
    error AttestationExpired();
    error ProofAlreadyUsed();
    error NotOwner();
    error NotGateway();
    error InvalidGateway();
    error InvalidSigner();
    error InvalidSignerSet();
    error InvalidSignerSetVersion();
    error DuplicateSigner();
    error InsufficientThreshold();

    address public owner;
    address public gateway;
    uint64 public activeSignerSetVersion = 1;

    mapping(bytes32 => bool) public usedProofs;
    mapping(uint64 => uint32) private signerSetThresholds;
    mapping(uint64 => mapping(address => bool)) private signerSetMembers;

    event GatewaySet(address indexed gateway);
    event SignerSetRotated(uint64 indexed version, uint32 threshold);

    constructor(address[] memory signers_, uint32 threshold_) {
        owner = msg.sender;
        _storeSignerSet(activeSignerSetVersion, signers_, threshold_);
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

    function rotateSignerSet(address[] calldata signers_, uint32 threshold_) external onlyOwner returns (uint64 version) {
        version = activeSignerSetVersion + 1;
        _storeSignerSet(version, signers_, threshold_);
        activeSignerSetVersion = version;
        emit SignerSetRotated(version, threshold_);
    }

    function verifyAndConsume(bytes32 messageId, bytes32 payloadHash, uint64 expiry, bytes calldata proof)
        external
        override
        onlyGateway
        returns (address signer)
    {
        if (block.timestamp > expiry) revert AttestationExpired();
        if (usedProofs[messageId]) revert ProofAlreadyUsed();

        (uint64 signerSetVersion, bytes[] memory signatures) = abi.decode(proof, (uint64, bytes[]));
        if (signerSetVersion != activeSignerSetVersion) revert InvalidSignerSetVersion();

        uint32 threshold = signerSetThresholds[signerSetVersion];
        if (threshold == 0) revert InvalidSignerSetVersion();

        bytes32 digest = keccak256(abi.encode(messageId, payloadHash, expiry, signerSetVersion));
        address[] memory recoveredSigners = new address[](signatures.length);

        uint256 uniqueSigners;
        for (uint256 i = 0; i < signatures.length; i++) {
            address recovered = _recover(digest, signatures[i]);
            if (!signerSetMembers[signerSetVersion][recovered]) revert InvalidAttestation();

            for (uint256 j = 0; j < uniqueSigners; j++) {
                if (recoveredSigners[j] == recovered) revert DuplicateSigner();
            }

            recoveredSigners[uniqueSigners] = recovered;
            if (uniqueSigners == 0) {
                signer = recovered;
            }
            uniqueSigners++;
        }

        if (uniqueSigners < threshold) revert InsufficientThreshold();
        usedProofs[messageId] = true;
    }

    function activeSignerThreshold() external view returns (uint32) {
        return signerSetThresholds[activeSignerSetVersion];
    }

    function isSignerInSet(uint64 version, address signer) external view returns (bool) {
        return signerSetMembers[version][signer];
    }

    function _storeSignerSet(uint64 version, address[] memory signers_, uint32 threshold_) internal {
        if (signers_.length == 0 || threshold_ == 0 || threshold_ > signers_.length) revert InvalidSignerSet();

        for (uint256 i = 0; i < signers_.length; i++) {
            address signer = signers_[i];
            if (signer == address(0)) revert InvalidSigner();
            for (uint256 j = 0; j < i; j++) {
                if (signers_[j] == signer) revert DuplicateSigner();
            }
            signerSetMembers[version][signer] = true;
        }

        signerSetThresholds[version] = threshold_;
    }

    function _recover(bytes32 digest, bytes memory signature) internal pure returns (address signer) {
        if (signature.length != 65) revert InvalidAttestation();

        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }

        if (v < 27) {
            v += 27;
        }
        if (v != 27 && v != 28) revert InvalidAttestation();

        signer = ecrecover(digest, v, r, s);
        if (signer == address(0)) revert InvalidAttestation();
    }
}
