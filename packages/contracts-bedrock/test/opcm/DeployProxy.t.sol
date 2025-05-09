// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";

import { DeployProxy } from "scripts/deploy/DeployProxy.s.sol";

contract DeployProxy_Test is Test {
    DeployProxy deployProxy;

    // Define default input variables for testing.
    address defaultProxyAdmin = makeAddr("ProxyAdmin");

    function setUp() public {
        deployProxy = new DeployProxy();
    }

    function testFuzz_run_memory_succeeds(DeployProxy.Input memory _input) public {
        vm.assume(_input.owner != address(0));

        // Run the deployment script.
        DeployProxy.Output memory output = deployProxy.run(_input);

        // Assert inputs were properly passed through to the contract initializers.
        vm.prank(_input.owner);
        assertEq(address(output.proxy.admin()), _input.owner, "100");
    }

    function test_run_nullInput_reverts() public {
        DeployProxy.Input memory input;

        input = defaultInput();
        input.owner = address(0);
        vm.expectRevert("DeployProxy: owner not set");
        deployProxy.run(input);
    }

    function defaultInput() internal view returns (DeployProxy.Input memory input_) {
        input_ = DeployProxy.Input({ owner: defaultProxyAdmin });
    }
}
