// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import {Script} from "forge-std/Script.sol";
import {Blueprint} from "src/libraries/Blueprint.sol";
import {Bytes} from "src/libraries/Bytes.sol";
import {IDisputeGame} from "interfaces/dispute/IDisputeGame.sol";
import {IOPContractsManager} from "interfaces/L1/IOPContractsManager.sol";
import {Claim, GameTypes} from "src/dispute/lib/Types.sol";
import {ISuperFaultDisputeGame} from "interfaces/dispute/ISuperFaultDisputeGame.sol";
import {ISuperPermissionedDisputeGame} from "interfaces/dispute/ISuperPermissionedDisputeGame.sol";
import {IDisputeGameFactory} from "interfaces/dispute/IDisputeGameFactory.sol";
import {IOptimismPortal2 as IOptimismPortal} from "interfaces/L1/IOptimismPortal2.sol";
import {console2 as console} from "forge-std/console2.sol";

contract FixDisputeGame is Script {
    function run() public {
        bytes32 newAbsolutePrestate = vm.envOr("ABSOLUTE_PRESTATE", bytes32(0));
        require(newAbsolutePrestate != bytes32(0), "missing ABSOLUTE_PRESTATE envar");
        // This is the same system config proxy for chain A that's used to compute the salt mixer
        address systemConfigProxy = 0x38CFB302cdA19FD376bE2237D220D35C404A36bA;
        IOPContractsManager opcm = IOPContractsManager(0xEB32e20EbDE266A769a5683CC80976f05D9e6e7B);
        IDisputeGameFactory factory = IDisputeGameFactory(0xC3fa613f53E95c3b1CfCf80dE0E9BF6f29a1113f);

        ISuperFaultDisputeGame oldSuperGame = ISuperFaultDisputeGame(address(factory.gameImpls(GameTypes.SUPER_CANNON)));
        ISuperPermissionedDisputeGame oldPermissionedGame =
            ISuperPermissionedDisputeGame(address(factory.gameImpls(GameTypes.SUPER_PERMISSIONED_CANNON)));

        vm.startBroadcast(msg.sender);
        ISuperPermissionedDisputeGame newSuperPDG =
            deploySuperPermissionedGame(opcm, systemConfigProxy, oldPermissionedGame, Claim.wrap(newAbsolutePrestate));

        ISuperFaultDisputeGame newSuperFDG =
            deploySuperDisputeGame(opcm, systemConfigProxy, oldSuperGame, Claim.wrap(newAbsolutePrestate));
        vm.stopBroadcast();

        require(
            newSuperPDG.gameType().raw() == GameTypes.SUPER_PERMISSIONED_CANNON.raw(),
            "invalid super permissioned game impl"
        );
        require(newSuperFDG.gameType().raw() == GameTypes.SUPER_CANNON.raw(), "invalid super cannon game impl");

        bytes memory cd0 = abi.encodeWithSelector(
            IDisputeGameFactory.setImplementation.selector, GameTypes.SUPER_PERMISSIONED_CANNON, newSuperPDG
        );
        bytes memory cd1 =
            abi.encodeWithSelector(IDisputeGameFactory.setImplementation.selector, GameTypes.SUPER_CANNON, newSuperFDG);
        console.log("setImplementation(SUPER_PERMISSIONED_CANNON) calldata: ");
        console.logBytes(cd0);
        console.log("setImplementation(SUPER_CANNON) calldata: ");
        console.logBytes(cd1);

        // testing
        //address l1pao = 0xe934Dc97E347C6aCef74364B50125bb8689c40ff;
        //vm.startPrank(l1pao);
        //factory.setImplementation(GameTypes.SUPER_PERMISSIONED_CANNON, newSuperPDG);
        //factory.setImplementation(GameTypes.SUPER_CANNON, newSuperFDG);
    }

    function deploySuperPermissionedGame(
        IOPContractsManager _opcm,
        address _systemConfigProxy,
        ISuperPermissionedDisputeGame _oldPermissionedGame,
        Claim _newAbsolutePrestate
    ) internal returns (ISuperPermissionedDisputeGame) {
        ISuperPermissionedDisputeGame newSuperPDG = ISuperPermissionedDisputeGame(
            Blueprint.deployFrom(
                _opcm.blueprints().superPermissionedDisputeGame1,
                _opcm.blueprints().superPermissionedDisputeGame2,
                computeSalt(block.timestamp, reusableSaltMixer(_systemConfigProxy), "SuperPermissionedDisputeGame"),
                encodePermissionedSuperFDGConstructor(
                    ISuperFaultDisputeGame.GameConstructorParams({
                        gameType: GameTypes.SUPER_PERMISSIONED_CANNON,
                        absolutePrestate: _newAbsolutePrestate,
                        maxGameDepth: _oldPermissionedGame.maxGameDepth(),
                        splitDepth: _oldPermissionedGame.splitDepth(),
                        clockExtension: _oldPermissionedGame.clockExtension(),
                        maxClockDuration: _oldPermissionedGame.maxClockDuration(),
                        vm: _oldPermissionedGame.vm(),
                        weth: _oldPermissionedGame.weth(),
                        anchorStateRegistry: _oldPermissionedGame.anchorStateRegistry(),
                        l2ChainId: 0
                    }),
                    _oldPermissionedGame.proposer(),
                    _oldPermissionedGame.challenger()
                )
            )
        );
        console.log("permissioned Super DG deployed to %s", address(newSuperPDG));
        return newSuperPDG;
    }

    function deploySuperDisputeGame(
        IOPContractsManager _opcm,
        address _systemConfigProxy,
        ISuperFaultDisputeGame _oldSuperGame,
        Claim _newAbsolutePrestate
    ) internal returns (ISuperFaultDisputeGame) {
        ISuperFaultDisputeGame newSuperFDG = ISuperFaultDisputeGame(
            Blueprint.deployFrom(
                _opcm.blueprints().superPermissionlessDisputeGame1,
                _opcm.blueprints().superPermissionlessDisputeGame2,
                computeSalt(block.timestamp, reusableSaltMixer(_systemConfigProxy), "SuperFaultDisputeGame"),
                encodePermissionlessSuperFDGConstructor(
                    ISuperFaultDisputeGame.GameConstructorParams({
                        gameType: GameTypes.SUPER_CANNON,
                        absolutePrestate: _newAbsolutePrestate,
                        maxGameDepth: _oldSuperGame.maxGameDepth(),
                        splitDepth: _oldSuperGame.splitDepth(),
                        clockExtension: _oldSuperGame.clockExtension(),
                        maxClockDuration: _oldSuperGame.maxClockDuration(),
                        vm: _oldSuperGame.vm(),
                        weth: _oldSuperGame.weth(),
                        anchorStateRegistry: _oldSuperGame.anchorStateRegistry(),
                        l2ChainId: 0
                    })
                )
            )
        );
        console.log("permissionless Super DG deployed to %s", address(newSuperFDG));
        return newSuperFDG;
    }

    // copied from opcm
    function computeSalt(uint256 _l2ChainId, string memory _saltMixer, string memory _contractName)
        internal
        pure
        returns (bytes32)
    {
        return keccak256(abi.encode(_l2ChainId, _saltMixer, _contractName));
    }

    // copied from opcm
    function reusableSaltMixer(address systemConfigProxy) internal pure returns (string memory) {
        return string(bytes.concat(bytes32(uint256(uint160(systemConfigProxy)))));
    }

    // copied from opcm
    function encodePermissionedSuperFDGConstructor(
        ISuperFaultDisputeGame.GameConstructorParams memory _params,
        address _proposer,
        address _challenger
    ) internal view virtual returns (bytes memory) {
        bytes memory dataWithSelector =
            abi.encodeCall(ISuperPermissionedDisputeGame.__constructor__, (_params, _proposer, _challenger));
        return Bytes.slice(dataWithSelector, 4);
    }

    // copied from opcm
    function encodePermissionlessSuperFDGConstructor(ISuperFaultDisputeGame.GameConstructorParams memory _params)
        internal
        view
        virtual
        returns (bytes memory)
    {
        bytes memory dataWithSelector = abi.encodeCall(ISuperFaultDisputeGame.__constructor__, (_params));
        return Bytes.slice(dataWithSelector, 4);
    }
}
