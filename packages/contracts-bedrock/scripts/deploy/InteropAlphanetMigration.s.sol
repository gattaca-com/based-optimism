// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IOPContractsManagerInteropMigrator, IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { Claim, Duration, Proposal, Hash } from "src/dispute/lib/Types.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";

contract InteropAlphanetMigration is Script {
    function run() public {
        IOPContractsManager opcm = 0x0; // TODO
        bytes32 absolutePrestate = hex"0387beeb10e2139751e069ad40f0d1f0fa91b6076fa6f2d5dd488d453a46eec6";
        bool usePermissionlessGame = true;
        Proposal memory startingAnchorRoot = Proposal({ root: Hash.wrap(hex""), l2SequenceNumber: 0 }); // TODO
        address proposer; // TODO
        address challenger; // TODO
        uint64 maxGameDepth = 73;
        uint64 splitDepth = 30;
        uint256 initBond = 0.08 ether;
        Duration clockExtension = Duration.wrap(10800);
        Duration maxClockDuration = Duration.wrap(302400);

        IOPContractsManager.OpChainConfig[] memory opChainConfigs = new IOPContractsManager.OpChainConfig[](2);
        opChainConfigs[0] = IOPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(0x38CFB302cdA19FD376bE2237D220D35C404A36bA),
            proxyAdmin: IProxyAdmin(0x0),
            absolutePrestate: Claim.wrap(absolutePrestate)
        });
        opChainConfigs[0] = IOPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(0xCE1da8571d67d139A8040EBa35BEF8cfd34a0F2f),
            proxyAdmin: IProxyAdmin(0x0),
            absolutePrestate: Claim.wrap(absolutePrestate)
        });

        IOPContractsManagerInteropMigrator.MigrateInput memory inputs = IOPContractsManagerInteropMigrator.MigrateInput({
            usePermissionlessGame: usePermissionlessGame,
            startingAnchorRoot: startingAnchorRoot,
            gameParameters: IOPContractsManagerInteropMigrator.GameParameters({
                proposer: proposer,
                challenger: challenger,
                maxGameDepth: maxGameDepth,
                splitDepth: splitDepth,
                initBond: initBond,
                clockExtension: clockExtension,
                maxClockDuration: maxClockDuration
            }),
            opChainConfigs: opChainConfigs
        });

        vm.broadcast(msg.sender);
        opcm.migrate(inputs);
    }
}
