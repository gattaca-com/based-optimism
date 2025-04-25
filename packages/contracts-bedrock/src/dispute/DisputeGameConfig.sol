// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { Claim, Duration } from "src/dispute/lib/Types.sol";
import { GameType } from "src/dispute/lib/Types.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { Math } from "@openzeppelin/contracts/utils/math/Math.sol";
import { InvalidClockExtension, ReservedGameType } from "src/dispute/lib/Errors.sol";

/// @title DisputeGameConfig
/// @notice Holds the immutable configuration parameters for a specific type of Dispute Game.
///         This allows multiple games of the same type to share the same configuration deployed once,
///         reducing deployment costs and providing a central point for configuration verification.
contract DisputeGameConfig {
    /// @notice The absolute prestate of the instruction trace.
    Claim public immutable ABSOLUTE_PRESTATE;

    /// @notice The max depth of the game tree.
    uint256 public immutable MAX_GAME_DEPTH;

    /// @notice The max depth of the output bisection portion of the position tree.
    uint256 public immutable SPLIT_DEPTH;

    /// @notice The duration added to a clock when a valid move extends it.
    Duration public immutable CLOCK_EXTENSION;

    /// @notice The maximum duration that may accumulate on a team's chess clock.
    Duration public immutable MAX_CLOCK_DURATION;

    /// @notice An onchain VM that performs single instruction steps.
    IBigStepper public immutable VM;

    /// @notice The game type ID.
    GameType public immutable GAME_TYPE;

    /// @notice The DelayedWETH contract used for bond management.
    IDelayedWETH public immutable WETH;

    /// @notice The AnchorStateRegistry contract.
    address public immutable ANCHOR_STATE_REGISTRY;

    /// @notice The L2 chain ID.
    uint256 public immutable L2_CHAIN_ID;

    /// @notice Constructor to set the immutable configuration parameters.
    /// @param _absolutePrestate The absolute prestate claim.
    /// @param _maxGameDepth The maximum depth of the game tree.
    /// @param _splitDepth The depth at which the game splits between output and execution bisection.
    /// @param _clockExtension The duration to extend the clock on a valid move.
    /// @param _maxClockDuration The maximum duration allowed on a clock.
    /// @param _vm The IBigStepper VM contract address.
    /// @param _gameType The type identifier for this game.
    /// @param _weth The DelayedWETH contract address.
    /// @param _anchorStateRegistry The AnchorStateRegistry contract address.
    /// @param _l2ChainId The L2 chain ID.
    constructor(
        Claim _absolutePrestate,
        uint256 _maxGameDepth,
        uint256 _splitDepth,
        Duration _clockExtension,
        Duration _maxClockDuration,
        IBigStepper _vm,
        GameType _gameType,
        IDelayedWETH _weth,
        address _anchorStateRegistry,
        uint256 _l2ChainId
    ) {
        // Validation checks copied from FaultDisputeGame constructor
        uint256 splitDepthExtension = uint256(_clockExtension.raw()) * 2;
        // Use vm.oracle().challengePeriod() for max game depth extension calculation
        uint256 maxGameDepthExtension;
        // Prevent underflow if challengePeriod is 0
        uint256 challengePeriod = uint256(_vm.oracle().challengePeriod());
        if (challengePeriod > 0) {
            maxGameDepthExtension = uint256(_clockExtension.raw()) + challengePeriod;
        } else {
            maxGameDepthExtension = uint256(_clockExtension.raw());
        }

        uint256 maxClockExtension = Math.max(splitDepthExtension, maxGameDepthExtension);

        // The maximum clock extension must fit into a uint64.
        if (maxClockExtension > type(uint64).max) revert InvalidClockExtension();

        // The maximum clock extension may not be greater than the maximum clock duration.
        if (uint64(maxClockExtension) > _maxClockDuration.raw()) revert InvalidClockExtension();

        // Block type(uint32).max from being used as a game type.
        if (_gameType.raw() == type(uint32).max) revert ReservedGameType();

        // Set immutable state variables
        ABSOLUTE_PRESTATE = _absolutePrestate;
        MAX_GAME_DEPTH = _maxGameDepth;
        SPLIT_DEPTH = _splitDepth;
        CLOCK_EXTENSION = _clockExtension;
        MAX_CLOCK_DURATION = _maxClockDuration;
        VM = _vm;
        GAME_TYPE = _gameType;
        WETH = _weth;
        ANCHOR_STATE_REGISTRY = _anchorStateRegistry;
        L2_CHAIN_ID = _l2ChainId;
    }
}
