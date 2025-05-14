package db

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/status"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// GetPreActivationStatus returns the currently tracked status for a chain in pre-activation mode
func (db *ChainsDB) GetPreActivationStatus(id eth.ChainID) (*status.NodeSyncStatus, bool) {
	return db.preActivationStatus.Get(id)
}

// IsInPreActivationMode checks if a chain is currently in pre-activation mode
func (db *ChainsDB) IsInPreActivationMode(id eth.ChainID) bool {
	_, ok := db.preActivationStatus.Get(id)
	return ok
}

// ExitPreActivationMode transitions a chain from pre-activation to normal mode
// This should be called when interop activation is detected
func (db *ChainsDB) ExitPreActivationMode(id eth.ChainID, anchor types.DerivedBlockRefPair) error {
	// Check if we're in pre-activation mode
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		return fmt.Errorf("chain %s is not in pre-activation mode", id)
	}

	db.logger.Info("exiting pre-activation mode", "chain", id, "anchor", anchor)

	// Save the current cross-unsafe value before clearing state
	crossUnsafe := nodeStatus.CrossUnsafe

	// Clear pre-activation state
	db.preActivationStatus.Delete(id)

	// Initialize using the anchor point
	db.initFromAnchor(id, anchor)

	// After initialization, set the cross-unsafe to the previously tracked value
	// This prevents cross-unsafe from being reset during transition
	if err := db.UpdateCrossUnsafe(id, crossUnsafe); err != nil {
		db.logger.Error("Failed to restore cross-unsafe value after exiting pre-activation mode",
			"chain", id, "cross-unsafe", crossUnsafe, "err", err)
	} else {
		db.logger.Info("Successfully preserved cross-unsafe value after exiting pre-activation mode",
			"chain", id, "cross-unsafe", crossUnsafe)
	}

	return nil
}

// InitializePreActivation sets up pre-activation tracking for a chain
// If the chain is already in pre-activation mode, it updates the tracked status with the new block
func (db *ChainsDB) InitializePreActivation(id eth.ChainID, block eth.BlockRef) {
	// Initialize NodeSyncStatus with the block
	blockSeal := types.BlockSealFromRef(block)
	nodeStatus := &status.NodeSyncStatus{
		LocalUnsafe: block,
		LocalSafe:   blockSeal,
		CrossUnsafe: blockSeal,
		CrossSafe:   blockSeal,
		Finalized:   blockSeal,
	}

	// Check if we're already in pre-activation mode
	if db.IsInPreActivationMode(id) {
		db.logger.Debug("chain already in pre-activation mode, updating status")
		db.preActivationStatus.Set(id, nodeStatus)
		return
	}

	db.logger.Info("setting up pre-activation tracking", "chain", id, "block", block)

	// Set initial status
	db.preActivationStatus.Set(id, nodeStatus)

	// Mark as initialized for API consistency
	db.initialized.Set(id, struct{}{})
}

// UpdatePreActivationUnsafe updates the LocalUnsafe head in pre-activation mode
func (db *ChainsDB) UpdatePreActivationUnsafe(id eth.ChainID, block eth.BlockRef) {
	// Update local unsafe head in pre-activation mode

	// Get the current status or initialize if not already in pre-activation mode
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		// Chain not in pre-activation mode, initialize it
		db.InitializePreActivation(id, block)
		return
	}
	// Chain in pre-activation mode

	// Only update if the block is newer
	currentUnsafe := nodeStatus.LocalUnsafe
	if block.Number <= currentUnsafe.Number {
		// Block is not newer than current unsafe, ignore it
		return
	}

	// Update the Unsafe heads
	db.logger.Debug("updating pre-activation unsafe blocks", "chain", id, "current", currentUnsafe, "new", block)
	nodeStatus.LocalUnsafe = block
	nodeStatus.CrossUnsafe = types.BlockSealFromRef(block)
	db.preActivationStatus.Set(id, nodeStatus)
}

// UpdatePreActivationSafe updates the LocalSafe head in pre-activation mode
func (db *ChainsDB) UpdatePreActivationSafe(id eth.ChainID, block eth.BlockRef) {
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		db.InitializePreActivation(id, block)
		return
	}

	// Only update if the block is newer
	blockSeal := types.BlockSealFromRef(block)
	currentSafe := nodeStatus.LocalSafe
	if blockSeal.Number <= currentSafe.Number {
		return
	}

	// Update the Safe heads
	db.logger.Debug("updating pre-activation safe blocks", "chain", id, "current", currentSafe.Number, "new", blockSeal.Number)
	nodeStatus.LocalSafe = blockSeal
	nodeStatus.CrossSafe = blockSeal
	// Also update LocalUnsafe and CrossUnsafe to match the new safe block
	nodeStatus.LocalUnsafe = block
	nodeStatus.CrossUnsafe = blockSeal
	// Also update Finalized to match the new safe block
	nodeStatus.Finalized = blockSeal
	db.preActivationStatus.Set(id, nodeStatus)
}

// UpdatePreActivationFinalized updates the Finalized head in pre-activation mode
func (db *ChainsDB) UpdatePreActivationFinalized(id eth.ChainID, block eth.BlockRef) {
	// Get the current status or initialize if not already in pre-activation mode
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		db.InitializePreActivation(id, block)
		return
	}

	// Only update if the block is newer
	if block.Number <= nodeStatus.Finalized.Number {
		return
	}

	// Update the Finalized head
	db.logger.Debug("updating pre-activation finalized",
		"chain", id,
		"current", nodeStatus.Finalized.Number,
		"new", block.Number)
	nodeStatus.Finalized = types.BlockSealFromRef(block)
	db.preActivationStatus.Set(id, nodeStatus)
}
