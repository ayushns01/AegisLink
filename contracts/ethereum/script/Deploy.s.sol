// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "../BridgeGateway.sol";
import "../BridgeVerifier.sol";

interface Vm {
    function envAddress(string calldata name) external returns (address);
    function startBroadcast() external;
    function stopBroadcast() external;
}

contract Deploy {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function run() external returns (BridgeVerifier verifier, BridgeGateway gateway) {
        address attester = vm.envAddress("AEGISLINK_ATTESTER");
        vm.startBroadcast();
        verifier = new BridgeVerifier(attester);
        gateway = new BridgeGateway(address(verifier));
        verifier.setGateway(address(gateway));
        vm.stopBroadcast();
    }
}
