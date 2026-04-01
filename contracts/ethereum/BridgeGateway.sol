// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./BridgeVerifier.sol";

interface IERC20Minimal {
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function transfer(address to, uint256 amount) external returns (bool);
}

interface IERC20BalanceOf {
    function balanceOf(address account) external view returns (uint256);
}

contract BridgeGateway {
    error NotOwner();
    error Paused();
    error UnsupportedAsset(address asset);
    error InvalidAmount();
    error InvalidRecipient();
    error ExpiredClaim();
    error NonCanonicalToken(address asset);
    error InvalidVerifier();
    error TransferFailed();

    struct AssetConfig {
        string assetId;
        bool supported;
    }

    address public owner;
    BridgeVerifier public immutable verifier;
    bool public paused;
    uint256 public nextNonce = 1;

    mapping(address => AssetConfig) private supportedAssets;

    event AssetConfigured(address indexed asset, string assetId, bool supported);
    event PausedStateChanged(bool paused);
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

    constructor(address verifier_) {
        if (verifier_ == address(0)) revert InvalidVerifier();
        owner = msg.sender;
        verifier = BridgeVerifier(verifier_);
    }

    modifier onlyOwner() {
        if (msg.sender != owner) revert NotOwner();
        _;
    }

    modifier whenNotPaused() {
        if (paused) revert Paused();
        _;
    }

    function setSupportedAsset(address asset, string calldata assetId, bool supported) external onlyOwner {
        if (asset == address(0)) revert UnsupportedAsset(asset);
        supportedAssets[asset] = AssetConfig({assetId: assetId, supported: supported});
        emit AssetConfigured(asset, assetId, supported);
    }

    function pause() external onlyOwner {
        paused = true;
        emit PausedStateChanged(true);
    }

    function unpause() external onlyOwner {
        paused = false;
        emit PausedStateChanged(false);
    }

    function deposit(address asset, uint256 amount, string calldata recipient, uint64 expiry)
        external
        whenNotPaused
        returns (bytes32 messageId)
    {
        AssetConfig memory config = supportedAssets[asset];
        if (!config.supported) revert UnsupportedAsset(asset);
        if (amount == 0) revert InvalidAmount();
        if (bytes(recipient).length == 0) revert InvalidRecipient();
        if (expiry <= block.timestamp) revert ExpiredClaim();

        uint256 nonce = nextNonce++;
        bytes32 depositId = keccak256(abi.encode(address(this), nonce));
        messageId = keccak256(
            abi.encode(block.chainid, address(this), depositId, asset, amount, keccak256(bytes(recipient)), expiry)
        );

        uint256 balanceBefore = IERC20BalanceOf(asset).balanceOf(address(this));
        _transferIn(asset, msg.sender, amount);
        uint256 balanceAfter = IERC20BalanceOf(asset).balanceOf(address(this));
        if (balanceAfter < balanceBefore || balanceAfter - balanceBefore != amount) revert NonCanonicalToken(asset);

        emit DepositInitiated(depositId, messageId, nonce, asset, config.assetId, amount, recipient, expiry);
    }

    function release(
        address asset,
        address recipient,
        uint256 amount,
        bytes32 messageId,
        uint64 expiry,
        bytes calldata signature
    ) external whenNotPaused returns (bytes32 releaseId) {
        if (!supportedAssets[asset].supported) revert UnsupportedAsset(asset);
        if (amount == 0) revert InvalidAmount();
        if (recipient == address(0)) revert InvalidRecipient();

        uint256 gatewayBalanceBefore = IERC20BalanceOf(asset).balanceOf(address(this));
        uint256 recipientBalanceBefore = IERC20BalanceOf(asset).balanceOf(recipient);
        verifier.verifyAndConsume(
            messageId,
            keccak256(abi.encode(block.chainid, address(this), asset, recipient, amount, messageId, expiry)),
            expiry,
            signature
        );

        releaseId = keccak256(abi.encode(address(this), messageId, asset, recipient, amount));
        _transferOut(asset, recipient, amount);
        _assertCanonicalRelease(asset, recipient, amount, gatewayBalanceBefore, recipientBalanceBefore);

        emit WithdrawalReleased(messageId, releaseId, asset, recipient, amount, expiry);
    }

    function _transferIn(address asset, address from, uint256 amount) internal {
        bool ok = IERC20Minimal(asset).transferFrom(from, address(this), amount);
        if (!ok) revert TransferFailed();
    }

    function _transferOut(address asset, address to, uint256 amount) internal {
        bool ok = IERC20Minimal(asset).transfer(to, amount);
        if (!ok) revert TransferFailed();
    }

    function _assertCanonicalRelease(
        address asset,
        address recipient,
        uint256 amount,
        uint256 gatewayBalanceBefore,
        uint256 recipientBalanceBefore
    ) internal view {
        uint256 gatewayBalanceAfter = IERC20BalanceOf(asset).balanceOf(address(this));
        uint256 recipientBalanceAfter = IERC20BalanceOf(asset).balanceOf(recipient);
        if (
            gatewayBalanceBefore < gatewayBalanceAfter || gatewayBalanceBefore - gatewayBalanceAfter != amount
                || recipientBalanceAfter < recipientBalanceBefore
                || recipientBalanceAfter - recipientBalanceBefore != amount
        ) revert NonCanonicalToken(asset);
    }
}
