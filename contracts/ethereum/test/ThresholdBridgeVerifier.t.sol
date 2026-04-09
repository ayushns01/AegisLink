// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "../BridgeGateway.sol";
import "../ThresholdBridgeVerifier.sol";

interface Vm {
    function addr(uint256 privateKey) external returns (address);
    function expectRevert() external;
    function expectRevert(bytes4 revertSelector) external;
    function prank(address msgSender) external;
    function startPrank(address msgSender) external;
    function stopPrank() external;
    function sign(uint256 privateKey, bytes32 digest) external returns (uint8 v, bytes32 r, bytes32 s);
}

contract ThresholdBridgeVerifierTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
    bytes32 private constant EIP712_DOMAIN_TYPEHASH =
        keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)");
    bytes32 private constant THRESHOLD_ATTESTATION_TYPEHASH =
        keccak256("ThresholdBridgeAttestation(bytes32 messageId,bytes32 payloadHash,uint64 expiry,uint64 signerSetVersion)");
    uint256 private constant SECP256K1N =
        0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141;

    ThresholdBridgeVerifier private verifier;
    BridgeGateway private gateway;
    ThresholdTestToken private token;

    uint256 private signerOneKey;
    uint256 private signerTwoKey;
    uint256 private signerThreeKey;
    uint256 private rotatedSignerKey;

    address private signerOne;
    address private signerTwo;
    address private signerThree;
    address private rotatedSigner;

    address private user;

    string private constant ASSET_ID = "eth.usdc";

    function setUp() public {
        signerOneKey = 0xA11CE;
        signerTwoKey = 0xB0B;
        signerThreeKey = 0xCAFE;
        rotatedSignerKey = 0xD00D;

        signerOne = vm.addr(signerOneKey);
        signerTwo = vm.addr(signerTwoKey);
        signerThree = vm.addr(signerThreeKey);
        rotatedSigner = vm.addr(rotatedSignerKey);

        verifier = new ThresholdBridgeVerifier(_initialSigners(), 2);
        gateway = new BridgeGateway(address(verifier));
        verifier.setGateway(address(gateway));

        token = new ThresholdTestToken("USDC", "USDC", 6);
        token.mint(address(gateway), 1_000_000_000);
        gateway.setSupportedAsset(address(token), ASSET_ID, true);

        user = address(0xBEEF);
    }

    function testThresholdReleaseSucceedsAtThreshold() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address recipient = address(0xCA11);
        bytes32 messageId = _releaseMessageId(address(gateway), address(token), recipient, 1, amount, expiry);

        gateway.release(address(token), recipient, amount, messageId, expiry, _proof(messageId, recipient, amount, expiry, _firstThresholdKeys()));

        if (!verifier.usedProofs(messageId)) {
            revert("proof not consumed");
        }
        if (token.balanceOf(recipient) != amount) {
            revert("recipient balance mismatch");
        }
    }

    function testThresholdReleaseRejectsInsufficientSignatures() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address recipient = address(0xCA11);
        bytes32 messageId = _releaseMessageId(address(gateway), address(token), recipient, 2, amount, expiry);
        bytes memory proof = _proof(messageId, recipient, amount, expiry, _singleSignerKeys());

        vm.expectRevert(ThresholdBridgeVerifier.InsufficientThreshold.selector);
        gateway.release(address(token), recipient, amount, messageId, expiry, proof);
    }

    function testThresholdReleaseRejectsDuplicateSigner() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address recipient = address(0xCA11);
        bytes32 messageId = _releaseMessageId(address(gateway), address(token), recipient, 3, amount, expiry);
        bytes memory proof = _proof(messageId, recipient, amount, expiry, _duplicateSignerKeys());

        vm.expectRevert(ThresholdBridgeVerifier.DuplicateSigner.selector);
        gateway.release(address(token), recipient, amount, messageId, expiry, proof);
    }

    function testThresholdReleaseSupportsSignerRotation() public {
        verifier.rotateSignerSet(_rotatedSigners(), 2);

        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address recipient = address(0xCA11);
        bytes32 oldMessageId = _releaseMessageId(address(gateway), address(token), recipient, 4, amount, expiry);
        bytes32 newMessageId = _releaseMessageId(address(gateway), address(token), recipient, 5, amount, expiry);
        bytes memory oldProof = _proofForVersion(oldMessageId, recipient, amount, expiry, 1, _firstThresholdKeys());
        bytes memory newProof = _proof(newMessageId, recipient, amount, expiry, _rotatedThresholdKeys());

        vm.expectRevert(ThresholdBridgeVerifier.InvalidSignerSetVersion.selector);
        gateway.release(address(token), recipient, amount, oldMessageId, expiry, oldProof);

        gateway.release(address(token), recipient, amount, newMessageId, expiry, newProof);
    }

    function testThresholdTypedDataDigestMatchesEIP712Formula() public view {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address recipient = address(0xCA11);
        bytes32 messageId = _releaseMessageId(address(gateway), address(token), recipient, 6, amount, expiry);
        uint64 signerSetVersion = verifier.activeSignerSetVersion();
        bytes32 payloadHash = gateway.releasePayloadHash(address(token), recipient, amount, messageId, expiry);
        bytes32 helperDigest = verifier.attestationDigest(messageId, payloadHash, expiry, signerSetVersion);
        bytes32 manualDigest =
            _thresholdVerifierTypedDigestManually(address(verifier), messageId, payloadHash, expiry, signerSetVersion);

        if (helperDigest != manualDigest) {
            revert("threshold typed digest mismatch");
        }
    }

    function testThresholdReleaseRejectsNonLowSSignature() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address recipient = address(0xCA11);
        bytes32 messageId = _releaseMessageId(address(gateway), address(token), recipient, 7, amount, expiry);

        uint256[] memory keys = _firstThresholdKeys();
        bytes memory proof = _proof(messageId, recipient, amount, expiry, keys);
        (uint64 signerSetVersion, bytes[] memory signatures) = abi.decode(proof, (uint64, bytes[]));
        signatures[0] = _malleateToHighS(signatures[0]);

        vm.expectRevert(ThresholdBridgeVerifier.InvalidAttestation.selector);
        gateway.release(address(token), recipient, amount, messageId, expiry, abi.encode(signerSetVersion, signatures));
    }

    function _proof(bytes32 messageId, address recipient, uint256 amount, uint64 expiry, uint256[] memory signerKeys)
        internal
        returns (bytes memory)
    {
        return _proofForVersion(messageId, recipient, amount, expiry, verifier.activeSignerSetVersion(), signerKeys);
    }

    function _proofForVersion(
        bytes32 messageId,
        address recipient,
        uint256 amount,
        uint64 expiry,
        uint64 signerSetVersion,
        uint256[] memory signerKeys
    ) internal returns (bytes memory) {
        bytes32 digest = verifier.attestationDigest(
            messageId,
            gateway.releasePayloadHash(address(token), recipient, amount, messageId, expiry),
            expiry,
            signerSetVersion
        );

        bytes[] memory signatures = new bytes[](signerKeys.length);
        for (uint256 i = 0; i < signerKeys.length; i++) {
            (uint8 v, bytes32 r, bytes32 s) = vm.sign(signerKeys[i], digest);
            signatures[i] = abi.encodePacked(r, s, v);
        }

        return abi.encode(signerSetVersion, signatures);
    }

    function _thresholdVerifierTypedDigestManually(
        address verifierAddress,
        bytes32 messageId,
        bytes32 payloadHash,
        uint64 expiry,
        uint64 signerSetVersion
    ) internal view returns (bytes32) {
        bytes32 domainSeparator = keccak256(
            abi.encode(
                EIP712_DOMAIN_TYPEHASH,
                keccak256(bytes("AegisLink Threshold Bridge Verifier")),
                keccak256(bytes("1")),
                block.chainid,
                verifierAddress
            )
        );
        bytes32 structHash = keccak256(
            abi.encode(THRESHOLD_ATTESTATION_TYPEHASH, messageId, payloadHash, expiry, signerSetVersion)
        );
        return keccak256(abi.encodePacked("\x19\x01", domainSeparator, structHash));
    }

    function _malleateToHighS(bytes memory signature) internal pure returns (bytes memory mutated) {
        if (signature.length != 65) revert("bad signature length");

        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }

        uint256 highS = SECP256K1N - uint256(s);
        uint8 flippedV = v == 27 ? 28 : 27;
        mutated = abi.encodePacked(r, bytes32(highS), flippedV);
    }

    function _releaseMessageId(
        address gatewayAddress,
        address asset,
        address recipient,
        uint256 nonce,
        uint256 amount,
        uint64 expiry
    ) internal view returns (bytes32) {
        return keccak256(abi.encode(block.chainid, gatewayAddress, nonce, asset, amount, recipient, expiry));
    }

    function _initialSigners() internal view returns (address[] memory signers) {
        signers = new address[](3);
        signers[0] = signerOne;
        signers[1] = signerTwo;
        signers[2] = signerThree;
    }

    function _rotatedSigners() internal view returns (address[] memory signers) {
        signers = new address[](3);
        signers[0] = signerTwo;
        signers[1] = signerThree;
        signers[2] = rotatedSigner;
    }

    function _firstThresholdKeys() internal view returns (uint256[] memory keys) {
        keys = new uint256[](2);
        keys[0] = signerOneKey;
        keys[1] = signerTwoKey;
    }

    function _singleSignerKeys() internal view returns (uint256[] memory keys) {
        keys = new uint256[](1);
        keys[0] = signerOneKey;
    }

    function _duplicateSignerKeys() internal view returns (uint256[] memory keys) {
        keys = new uint256[](2);
        keys[0] = signerOneKey;
        keys[1] = signerOneKey;
    }

    function _rotatedThresholdKeys() internal view returns (uint256[] memory keys) {
        keys = new uint256[](2);
        keys[0] = signerTwoKey;
        keys[1] = rotatedSignerKey;
    }
}

contract ThresholdTestToken {
    string public name;
    string public symbol;
    uint8 public immutable decimals;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    constructor(string memory name_, string memory symbol_, uint8 decimals_) {
        name = name_;
        symbol = symbol_;
        decimals = decimals_;
    }

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        uint256 currentAllowance = allowance[from][msg.sender];
        require(currentAllowance >= amount, "allowance");
        allowance[from][msg.sender] = currentAllowance - amount;
        _transfer(from, to, amount);
        return true;
    }

    function _transfer(address from, address to, uint256 amount) internal {
        require(balanceOf[from] >= amount, "balance");
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
    }
}
