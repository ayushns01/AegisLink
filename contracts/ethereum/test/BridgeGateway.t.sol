// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "../BridgeGateway.sol";
import "../BridgeVerifier.sol";

interface Vm {
    function addr(uint256 privateKey) external returns (address);
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

contract BridgeGatewayTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    BridgeGateway private gateway;
    BridgeVerifier private verifier;
    TestToken private token;

    address private owner;
    address private user;
    address private attester;
    uint256 private attesterKey;

    string private constant ASSET_ID = "eth.usdc";
    string private constant RECIPIENT = "cosmos1recipient";

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
        owner = address(this);
        user = address(0xBEEF);
        attesterKey = 0xA11CE;
        attester = vm.addr(attesterKey);

        verifier = new BridgeVerifier(attester);
        gateway = new BridgeGateway(address(verifier));
        verifier.setGateway(address(gateway));

        token = new TestToken("USDC", "USDC", 6);
        token.mint(user, 1_000_000_000);
        token.mint(address(gateway), 1_000_000_000);

        gateway.setSupportedAsset(address(token), ASSET_ID, true);
    }

    function testDepositEmitsCanonicalEvent() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        uint256 expectedNonce = 1;
        bytes32 expectedDepositId = _depositId(expectedNonce);
        bytes32 expectedMessageId = _depositMessageId(expectedDepositId, address(token), amount, RECIPIENT, expiry);

        vm.startPrank(user);
        token.approve(address(gateway), amount);
        vm.expectEmit(true, true, true, true);
        emit DepositInitiated(
            expectedDepositId, expectedMessageId, expectedNonce, address(token), ASSET_ID, amount, RECIPIENT, expiry
        );
        gateway.deposit(address(token), amount, RECIPIENT, expiry);
        vm.stopPrank();
    }

    function testDepositRejectsEmptyRecipient() public {
        uint64 expiry = uint64(block.timestamp + 1 days);

        vm.startPrank(user);
        token.approve(address(gateway), 1);
        vm.expectRevert(BridgeGateway.InvalidRecipient.selector);
        gateway.deposit(address(token), 1, "", expiry);
        vm.stopPrank();
    }

    function testDepositRejectsExpiredClaim() public {
        uint64 expiry = uint64(block.timestamp);

        vm.startPrank(user);
        token.approve(address(gateway), 1);
        vm.expectRevert(BridgeGateway.ExpiredClaim.selector);
        gateway.deposit(address(token), 1, RECIPIENT, expiry);
        vm.stopPrank();
    }

    function testDepositRejectsUnsupportedAsset() public {
        uint64 expiry = uint64(block.timestamp + 1 days);

        vm.prank(user);
        vm.expectRevert(abi.encodeWithSelector(BridgeGateway.UnsupportedAsset.selector, address(0xCAFE)));
        gateway.deposit(address(0xCAFE), 1, RECIPIENT, expiry);
    }

    function testDepositRejectsFeeOnTransferToken() public {
        FeeOnTransferToken feeToken = new FeeOnTransferToken("FEE", "FEE", 18, 1);
        feeToken.mint(user, 1_000 ether);
        gateway.setSupportedAsset(address(feeToken), "eth.fee", true);

        vm.startPrank(user);
        feeToken.approve(address(gateway), 10 ether);
        vm.expectRevert(abi.encodeWithSelector(BridgeGateway.NonCanonicalToken.selector, address(feeToken)));
        gateway.deposit(address(feeToken), 10 ether, RECIPIENT, uint64(block.timestamp + 1 days));
        vm.stopPrank();
    }

    function testReleaseTransfersCanonicalBalanceAndMarksProofUsed() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, attesterKey);
        address recipient = address(0xCA11);

        uint256 recipientBefore = token.balanceOf(recipient);
        uint256 gatewayBefore = token.balanceOf(address(gateway));

        vm.expectEmit(true, true, true, true);
        emit WithdrawalReleased(
            messageId,
            _releaseId(messageId, address(token), recipient, amount),
            address(token),
            recipient,
            amount,
            expiry
        );
        gateway.release(address(token), payable(recipient), amount, messageId, expiry, attestation);

        if (token.balanceOf(recipient) != recipientBefore + amount) {
            revert("recipient balance mismatch");
        }
        if (token.balanceOf(address(gateway)) != gatewayBefore - amount) {
            revert("gateway balance mismatch");
        }
        if (!verifier.usedProofs(messageId)) {
            revert("proof not marked used");
        }
    }

    function testReleaseRejectsFeeOnTransferToken() public {
        FeeOnTransferToken feeToken = new FeeOnTransferToken("FEE", "FEE", 18, 1);
        feeToken.mint(address(gateway), 1_000 ether);
        gateway.setSupportedAsset(address(feeToken), "eth.fee", true);

        uint256 amount = 10 ether;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageIdForAsset(address(feeToken), 1, amount, expiry);
        bytes memory attestation = _attestationForAsset(address(feeToken), messageId, amount, expiry, attesterKey);

        vm.expectRevert(abi.encodeWithSelector(BridgeGateway.NonCanonicalToken.selector, address(feeToken)));
        gateway.release(address(feeToken), payable(address(0xCA11)), amount, messageId, expiry, attestation);
    }

    function testReleaseRejectsBadAttestation() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, uint256(0xBADDCAFE));

        vm.expectRevert(BridgeVerifier.InvalidAttestation.selector);
        gateway.release(address(token), payable(address(0xCA11)), amount, messageId, expiry, attestation);
    }

    function testPauseRejectsDepositAndRelease() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, attesterKey);

        gateway.pause();

        vm.prank(user);
        vm.expectRevert(BridgeGateway.Paused.selector);
        gateway.deposit(address(token), amount, RECIPIENT, expiry);

        vm.expectRevert(BridgeGateway.Paused.selector);
        gateway.release(address(token), payable(address(0xCA11)), amount, messageId, expiry, attestation);
    }

    function testReleaseRejectsExpiredAttestation() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, attesterKey);

        vm.warp(uint256(expiry) + 1);
        vm.expectRevert(BridgeVerifier.AttestationExpired.selector);
        gateway.release(address(token), payable(address(0xCA11)), amount, messageId, expiry, attestation);
    }

    function testReleaseRejectsReusedProof() public {
        uint256 amount = 25_000_000;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, attesterKey);
        address recipient = address(0xCA11);

        gateway.release(address(token), payable(recipient), amount, messageId, expiry, attestation);

        vm.expectRevert(BridgeVerifier.ProofAlreadyUsed.selector);
        gateway.release(address(token), payable(recipient), amount, messageId, expiry, attestation);
    }

    function testVerifierOwnerCanRepairGatewayAndAttesterBinding() public {
        address repairedGateway = address(0xDEAD);
        address repairedAttester = address(0xB0B);

        verifier.setGateway(repairedGateway);
        verifier.setAttester(repairedAttester);

        if (verifier.gateway() != repairedGateway) {
            revert("gateway not updated");
        }
        if (verifier.attester() != repairedAttester) {
            revert("attester not updated");
        }
    }

    function _depositId(uint256 nonce) internal view returns (bytes32) {
        return keccak256(abi.encode(address(gateway), nonce));
    }

    function _depositMessageId(bytes32 depositId, address asset, uint256 amount, string memory recipient, uint64 expiry)
        internal
        view
        returns (bytes32)
    {
        return keccak256(
            abi.encode(block.chainid, address(gateway), depositId, asset, amount, keccak256(bytes(recipient)), expiry)
        );
    }

    function _releaseMessageId(uint256 nonce, uint256 amount, uint64 expiry) internal view returns (bytes32) {
        return keccak256(
            abi.encode(block.chainid, address(gateway), nonce, address(token), amount, address(uint160(0xCA11)), expiry)
        );
    }

    function _releasePayloadHash(bytes32 messageId, address asset, address recipient, uint256 amount, uint64 expiry)
        internal
        view
        returns (bytes32)
    {
        return keccak256(abi.encode(block.chainid, address(gateway), asset, recipient, amount, messageId, expiry));
    }

    function _releasePayloadHashForAsset(
        address gatewayAddress,
        address asset,
        bytes32 messageId,
        address recipient,
        uint256 amount,
        uint64 expiry
    ) internal view returns (bytes32) {
        return keccak256(abi.encode(block.chainid, gatewayAddress, asset, recipient, amount, messageId, expiry));
    }

    function _releaseId(bytes32 messageId, address asset, address recipient, uint256 amount)
        internal
        view
        returns (bytes32)
    {
        return keccak256(abi.encode(address(gateway), messageId, asset, recipient, amount));
    }

    function _releaseIdForAsset(
        address gatewayAddress,
        bytes32 messageId,
        address asset,
        address recipient,
        uint256 amount
    ) internal pure returns (bytes32) {
        return keccak256(abi.encode(gatewayAddress, messageId, asset, recipient, amount));
    }

    function _releaseMessageIdForAsset(address gatewayAddress, uint256 nonce, uint256 amount, uint64 expiry)
        internal
        view
        returns (bytes32)
    {
        return keccak256(
            abi.encode(block.chainid, address(gateway), nonce, gatewayAddress, amount, address(uint160(0xCA11)), expiry)
        );
    }

    function _attestationForAsset(address asset, bytes32 messageId, uint256 amount, uint64 expiry, uint256 privateKey)
        internal
        returns (bytes memory)
    {
        bytes32 payloadHash =
            _releasePayloadHashForAsset(address(gateway), asset, messageId, address(uint160(0xCA11)), amount, expiry);
        bytes32 digest = keccak256(abi.encode(messageId, payloadHash, expiry));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, digest);
        return abi.encodePacked(r, s, v);
    }

    function _attestation(bytes32 messageId, uint256 amount, uint64 expiry, uint256 privateKey)
        internal
        returns (bytes memory)
    {
        bytes32 payloadHash = _releasePayloadHash(messageId, address(token), address(uint160(0xCA11)), amount, expiry);
        bytes32 digest = keccak256(abi.encode(messageId, payloadHash, expiry));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, digest);
        return abi.encodePacked(r, s, v);
    }
}

contract TestToken {
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

contract FeeOnTransferToken {
    string public name;
    string public symbol;
    uint8 public immutable decimals;
    uint256 public immutable feeBps;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    constructor(string memory name_, string memory symbol_, uint8 decimals_, uint256 feeBps_) {
        name = name_;
        symbol = symbol_;
        decimals = decimals_;
        feeBps = feeBps_;
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
        uint256 fee = (amount * feeBps) / 10_000;
        uint256 received = amount - fee;
        balanceOf[from] -= amount;
        balanceOf[to] += received;
    }
}
