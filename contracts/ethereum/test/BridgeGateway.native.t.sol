// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "../BridgeGateway.sol";
import "../BridgeVerifier.sol";

interface Vm {
    function addr(uint256 privateKey) external returns (address);
    function deal(address account, uint256 newBalance) external;
    function expectEmit(bool checkTopic1, bool checkTopic2, bool checkTopic3, bool checkData) external;
    function expectRevert() external;
    function expectRevert(bytes4 revertSelector) external;
    function expectRevert(bytes calldata revertData) external;
    function prank(address msgSender) external;
    function startPrank(address msgSender) external;
    function stopPrank() external;
    function sign(uint256 privateKey, bytes32 digest) external returns (uint8 v, bytes32 r, bytes32 s);
    function warp(uint256 newTimestamp) external;
}

contract BridgeGatewayNativeTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
    bytes32 private constant EIP712_DOMAIN_TYPEHASH =
        keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)");
    bytes32 private constant BRIDGE_ATTESTATION_TYPEHASH =
        keccak256("BridgeAttestation(bytes32 messageId,bytes32 payloadHash,uint64 expiry)");

    BridgeGateway private gateway;
    BridgeVerifier private verifier;

    address private user;
    address private attester;
    uint256 private attesterKey;

    string private constant RECIPIENT = "cosmos1native";
    string private constant ETH_ASSET_ID = "eth";

    event DepositInitiated(
        bytes32 indexed depositId,
        bytes32 indexed messageId,
        uint256 indexed nonce,
        address asset,
        string assetId,
        uint256 amount,
        string recipient,
        uint64 expiry
    );

    event WithdrawalReleased(
        bytes32 indexed messageId,
        bytes32 indexed releaseId,
        address indexed asset,
        address recipient,
        uint256 amount,
        uint64 expiry
    );

    function setUp() public {
        user = address(0xBEEF);
        attesterKey = 0xA11CE;
        attester = vm.addr(attesterKey);

        verifier = new BridgeVerifier(attester);
        gateway = new BridgeGateway(address(verifier));
        verifier.setGateway(address(gateway));
    }

    function testDepositEthEmitsCanonicalEventAndIncreasesCustody() public {
        uint256 amount = 2 ether;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 expectedDepositId = _depositId(1);
        bytes32 expectedMessageId = _depositMessageId(expectedDepositId, amount, RECIPIENT, expiry);

        vm.deal(user, amount);
        vm.startPrank(user);
        vm.expectEmit(true, true, true, true);
        emit DepositInitiated(
            expectedDepositId, expectedMessageId, 1, address(0), ETH_ASSET_ID, amount, RECIPIENT, expiry
        );
        gateway.depositETH{value: amount}(RECIPIENT, expiry);
        vm.stopPrank();

        if (address(gateway).balance != amount) {
            revert("gateway ETH custody mismatch");
        }
    }

    function testReleaseEthSendsToRecipientAndRejectsReplay() public {
        uint256 amount = 3 ether;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, attesterKey);
        address payable recipient = payable(address(0xCA11));

        vm.deal(address(gateway), amount);
        uint256 recipientBefore = recipient.balance;

        vm.expectEmit(true, true, true, true);
        emit WithdrawalReleased(messageId, _releaseId(messageId, recipient, amount), address(0), recipient, amount, expiry);
        gateway.release(address(0), recipient, amount, messageId, expiry, attestation);

        if (recipient.balance != recipientBefore + amount) {
            revert("recipient ETH balance mismatch");
        }
        if (address(gateway).balance != 0) {
            revert("gateway ETH custody mismatch");
        }

        vm.expectRevert(BridgeVerifier.ProofAlreadyUsed.selector);
        gateway.release(address(0), recipient, amount, messageId, expiry, attestation);
    }

    function testReleaseEthSupportsPayableRecipientThatForwardsValue() public {
        uint256 amount = 4 ether;
        uint64 expiry = uint64(block.timestamp + 1 days);
        address payable sink = payable(address(0xF00D));
        ForwardingRecipient recipient = new ForwardingRecipient(sink);
        bytes32 messageId = _releaseMessageIdForRecipient(2, amount, address(recipient), expiry);
        bytes memory attestation =
            _attestationForRecipient(messageId, amount, address(recipient), expiry, attesterKey);

        vm.deal(address(gateway), amount);
        uint256 sinkBefore = sink.balance;

        gateway.release(address(0), payable(address(recipient)), amount, messageId, expiry, attestation);

        if (address(gateway).balance != 0) {
            revert("gateway ETH custody mismatch");
        }
        if (address(recipient).balance != 0) {
            revert("forwarding recipient should not retain ETH");
        }
        if (sink.balance != sinkBefore + amount) {
            revert("forwarded ETH mismatch");
        }
    }

    function testPausedGatewayRejectsEthDepositAndRelease() public {
        uint256 amount = 1 ether;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, attesterKey);
        address payable recipient = payable(address(0xCA11));

        gateway.pause();

        vm.deal(user, amount);
        vm.startPrank(user);
        vm.expectRevert(BridgeGateway.Paused.selector);
        gateway.depositETH{value: amount}(RECIPIENT, expiry);
        vm.stopPrank();

        vm.deal(address(gateway), amount);
        vm.expectRevert(BridgeGateway.Paused.selector);
        gateway.release(address(0), recipient, amount, messageId, expiry, attestation);
    }

    function _depositId(uint256 nonce) internal view returns (bytes32) {
        return keccak256(abi.encode(address(gateway), nonce));
    }

    function _depositMessageId(bytes32 depositId, uint256 amount, string memory recipient, uint64 expiry)
        internal
        view
        returns (bytes32)
    {
        return keccak256(
            abi.encode(block.chainid, address(gateway), depositId, address(0), amount, keccak256(bytes(recipient)), expiry)
        );
    }

    function _releaseMessageId(uint256 nonce, uint256 amount, uint64 expiry) internal view returns (bytes32) {
        return keccak256(abi.encode(block.chainid, address(gateway), nonce, address(0), amount, address(0xCA11), expiry));
    }

    function _releasePayloadHash(bytes32 messageId, uint256 amount, uint64 expiry) internal view returns (bytes32) {
        return keccak256(abi.encode(block.chainid, address(gateway), address(0), address(0xCA11), amount, messageId, expiry));
    }

    function _releaseMessageIdForRecipient(
        uint256 nonce,
        uint256 amount,
        address recipient,
        uint64 expiry
    ) internal view returns (bytes32) {
        return keccak256(abi.encode(block.chainid, address(gateway), nonce, address(0), amount, recipient, expiry));
    }

    function _releasePayloadHashForRecipient(
        bytes32 messageId,
        uint256 amount,
        address recipient,
        uint64 expiry
    ) internal view returns (bytes32) {
        return keccak256(abi.encode(block.chainid, address(gateway), address(0), recipient, amount, messageId, expiry));
    }

    function _attestationForRecipient(
        bytes32 messageId,
        uint256 amount,
        address recipient,
        uint64 expiry,
        uint256 privateKey
    ) internal returns (bytes memory) {
        bytes32 payloadHash = _releasePayloadHashForRecipient(messageId, amount, recipient, expiry);
        bytes32 digest = verifier.attestationDigest(messageId, payloadHash, expiry);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, digest);
        return abi.encodePacked(r, s, v);
    }

    function _releaseId(bytes32 messageId, address recipient, uint256 amount) internal view returns (bytes32) {
        return keccak256(abi.encode(address(gateway), messageId, address(0), recipient, amount));
    }

    function _attestation(bytes32 messageId, uint256 amount, uint64 expiry, uint256 privateKey)
        internal
        returns (bytes memory)
    {
        bytes32 payloadHash = _releasePayloadHash(messageId, amount, expiry);
        bytes32 digest = verifier.attestationDigest(messageId, payloadHash, expiry);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, digest);
        return abi.encodePacked(r, s, v);
    }
}

contract ForwardingRecipient {
    address payable public immutable sink;
    uint256 public received;

    constructor(address payable sink_) {
        sink = sink_;
    }

    receive() external payable {
        received += msg.value;
        (bool ok, ) = sink.call{value: msg.value}("");
        require(ok, "forward failed");
    }
}
