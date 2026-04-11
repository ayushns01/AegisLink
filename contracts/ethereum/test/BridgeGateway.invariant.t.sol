// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "../BridgeGateway.sol";
import "../BridgeVerifier.sol";

interface Vm {
    function addr(uint256 privateKey) external returns (address);
    function expectRevert() external;
    function expectRevert(bytes4 revertSelector) external;
    function expectRevert(bytes calldata revertData) external;
    function startPrank(address msgSender) external;
    function stopPrank() external;
    function sign(uint256 privateKey, bytes32 digest) external returns (uint8 v, bytes32 r, bytes32 s);
}

contract BridgeGatewayInvariantTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    BridgeGateway private gateway;
    BridgeVerifier private verifier;
    TestToken private token;

    address private depositor;
    address private attester;

    uint256 private constant ATTESTER_KEY = 0xA11CE;
    string private constant ASSET_ID = "eth.usdc";
    string private constant RECIPIENT = "cosmos1invariant";
    address private constant RELEASE_RECIPIENT = address(uint160(0xCA11));

    function setUp() public {
        depositor = address(0xBEEF);
        attester = vm.addr(ATTESTER_KEY);

        verifier = new BridgeVerifier(attester);
        gateway = new BridgeGateway(address(verifier));
        verifier.setGateway(address(gateway));

        token = new TestToken("USDC", "USDC", 6);
        token.mint(depositor, 1_000_000_000);
        token.mint(address(gateway), 1_000_000_000);
        gateway.setSupportedAsset(address(token), ASSET_ID, true);
    }

    function testFuzzGatewayBalanceTracksNetFlow(uint256 seed, uint8 steps) public {
        steps = _boundUint8(steps, 1, 20);

        uint256 initialBalance = token.balanceOf(address(gateway));
        uint256 totalDeposited;
        uint256 totalReleased;
        uint256 releaseNonce = 1;

        for (uint256 i = 0; i < steps; ++i) {
            seed = uint256(keccak256(abi.encode(seed, i)));
            if ((seed & 1) == 0) {
                uint256 depositorBalance = token.balanceOf(depositor);
                if (depositorBalance == 0) continue;

                uint256 amount = _bound(uint256(keccak256(abi.encode(seed, "deposit"))), 1, depositorBalance);
                _deposit(amount);
                totalDeposited += amount;
            } else {
                uint256 gatewayBalance = token.balanceOf(address(gateway));
                if (gatewayBalance == 0) continue;

                uint256 amount = _bound(uint256(keccak256(abi.encode(seed, "release"))), 1, gatewayBalance);
                _release(amount, releaseNonce);
                totalReleased += amount;
                releaseNonce++;
            }
        }

        uint256 expectedBalance = initialBalance + totalDeposited - totalReleased;
        if (token.balanceOf(address(gateway)) != expectedBalance) {
            revert("gateway balance drift");
        }
    }

    function testReleaseRejectsAmountsAboveAvailableLiquidity() public {
        uint256 amount = token.balanceOf(address(gateway)) + 1;
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(1, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, ATTESTER_KEY);

        vm.expectRevert();
        gateway.release(address(token), payable(RELEASE_RECIPIENT), amount, messageId, expiry, attestation);
    }

    function _deposit(uint256 amount) internal {
        vm.startPrank(depositor);
        token.approve(address(gateway), amount);
        gateway.deposit(address(token), amount, RECIPIENT, uint64(block.timestamp + 1 days));
        vm.stopPrank();
    }

    function _release(uint256 amount, uint256 nonce) internal {
        uint64 expiry = uint64(block.timestamp + 1 days);
        bytes32 messageId = _releaseMessageId(nonce, amount, expiry);
        bytes memory attestation = _attestation(messageId, amount, expiry, ATTESTER_KEY);
        gateway.release(address(token), payable(RELEASE_RECIPIENT), amount, messageId, expiry, attestation);
    }

    function _releaseMessageId(uint256 nonce, uint256 amount, uint64 expiry) internal view returns (bytes32) {
        return keccak256(
            abi.encode(block.chainid, address(gateway), nonce, address(token), amount, RELEASE_RECIPIENT, expiry)
        );
    }

    function _releasePayloadHash(bytes32 messageId, uint256 amount, uint64 expiry) internal view returns (bytes32) {
        return keccak256(
            abi.encode(block.chainid, address(gateway), address(token), RELEASE_RECIPIENT, amount, messageId, expiry)
        );
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

    function _bound(uint256 value, uint256 min, uint256 max) internal pure returns (uint256) {
        if (max <= min) return min;
        return min + (value % (max - min + 1));
    }

    function _boundUint8(uint8 value, uint8 min, uint8 max) internal pure returns (uint8) {
        if (max <= min) return min;
        return uint8(min + (value % (max - min + 1)));
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
