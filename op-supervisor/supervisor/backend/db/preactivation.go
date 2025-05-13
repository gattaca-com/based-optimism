package db

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/status"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
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
	if !db.IsInPreActivationMode(id) {
		return fmt.Errorf("chain %s is not in pre-activation mode", id)
	}

	db.logger.Info("exiting pre-activation mode", "chain", id, "anchor", anchor)

	// Clear pre-activation state
	db.preActivationStatus.Delete(id)

	// Reset initialization state
	db.initialized.Delete(id)

	// Initialize using the anchor point
	db.initFromAnchor(id, anchor)

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

// UpdatePreActivationLocalUnsafe updates the LocalUnsafe head in pre-activation mode
// If the new block number is higher than the current LocalUnsafe, this also updates CrossUnsafe
// If the chain is not already in pre-activation mode, it will be initialized
func (db *ChainsDB) UpdatePreActivationLocalUnsafe(id eth.ChainID, block eth.BlockRef) {
	// Get the current status or initialize if not already in pre-activation mode
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		db.logger.Debug("chain not in pre-activation mode yet, initializing", "chain", id)
		db.InitializePreActivation(id, block)
		return
	}

	// Only update if the new block is newer
	if block.Number <= nodeStatus.LocalUnsafe.Number {
		db.logger.Debug("ignoring older or equal LocalUnsafe block in pre-activation mode",
			"chain", id,
			"current", nodeStatus.LocalUnsafe.Number,
			"new", block.Number)
		return
	}

	// Update the LocalUnsafe head
	db.logger.Debug("updating pre-activation LocalUnsafe", "chain", id, "block", block)
	nodeStatus.LocalUnsafe = block

	// Also update CrossUnsafe with the same block
	blockSeal := types.BlockSealFromRef(block)
	nodeStatus.CrossUnsafe = blockSeal

	// Save the updated status
	db.preActivationStatus.Set(id, nodeStatus)
}

// UpdatePreActivationLocalSafe updates the LocalSafe head in pre-activation mode
// If the new block number is higher than the current LocalSafe, this also updates CrossSafe and Finalized
// If the new block number is higher than the current LocalUnsafe, it also updates LocalUnsafe and CrossUnsafe
// If the chain is not already in pre-activation mode, it will be initialized
func (db *ChainsDB) UpdatePreActivationLocalSafe(id eth.ChainID, sourceBlockSeal types.BlockSeal, derivedBlockSeal types.BlockSeal) {
	// Get the current status or initialize if not already in pre-activation mode
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		db.logger.Debug("chain not in pre-activation mode yet, initializing", "chain", id)
		// Create a new status with the derived block as both unsafe and safe
		// Create a BlockRef from the BlockSeal for LocalUnsafe
		derivedBlockRef := eth.BlockRef{
			Hash:       derivedBlockSeal.Hash,
			Number:     derivedBlockSeal.Number,
			Time:       derivedBlockSeal.Timestamp,
			ParentHash: common.Hash{}, // We don't have parent hash in BlockSeal
		}

		nodeStatus = &status.NodeSyncStatus{
			LocalUnsafe: derivedBlockRef,
			LocalSafe:   derivedBlockSeal,
			CrossUnsafe: derivedBlockSeal,
			CrossSafe:   derivedBlockSeal,
			Finalized:   derivedBlockSeal,
		}
		db.preActivationStatus.Set(id, nodeStatus)
		db.initialized.Set(id, struct{}{})
		return
	}

	// Only update LocalSafe if the new block is newer
	currentLocalSafe := nodeStatus.LocalSafe
	if derivedBlockSeal.Number > currentLocalSafe.Number {
		db.logger.Debug("updating pre-activation LocalSafe and CrossSafe",
			"chain", id,
			"current", currentLocalSafe.Number,
			"new", derivedBlockSeal.Number)

		// Update LocalSafe, CrossSafe and Finalized with the new derived block
		nodeStatus.LocalSafe = derivedBlockSeal
		nodeStatus.CrossSafe = derivedBlockSeal
		nodeStatus.Finalized = derivedBlockSeal
	}

	// Also update LocalUnsafe and CrossUnsafe if the derived block is newer
	if derivedBlockSeal.Number > nodeStatus.LocalUnsafe.Number {
		db.logger.Debug("updating pre-activation LocalUnsafe from LocalSafe",
			"chain", id,
			"current", nodeStatus.LocalUnsafe.Number,
			"new", derivedBlockSeal.Number)

		// Create a BlockRef from the BlockSeal for LocalUnsafe
		derivedBlockRef := eth.BlockRef{
			Hash:       derivedBlockSeal.Hash,
			Number:     derivedBlockSeal.Number,
			Time:       derivedBlockSeal.Timestamp,
			ParentHash: common.Hash{}, // We don't have parent hash in BlockSeal
		}

		// Update LocalUnsafe and CrossUnsafe as well
		nodeStatus.LocalUnsafe = derivedBlockRef
		nodeStatus.CrossUnsafe = derivedBlockSeal
	}

	// Save the updated status
	db.preActivationStatus.Set(id, nodeStatus)
}

// UpdatePreActivationFinalized updates the Finalized head in pre-activation mode
// If the new block number is higher than the current Finalized block, this updates the Finalized field
// If the chain is not already in pre-activation mode, it will be initialized
// This method only updates the Finalized field, not other fields like LocalSafe or CrossSafe
func (db *ChainsDB) UpdatePreActivationFinalized(id eth.ChainID, blockSeal types.BlockSeal) {
	// Get the current status or initialize if not already in pre-activation mode
	nodeStatus, ok := db.preActivationStatus.Get(id)
	if !ok {
		blockRef := eth.BlockRef{
			Hash:       blockSeal.Hash,
			Number:     blockSeal.Number,
			Time:       blockSeal.Timestamp,
			ParentHash: common.Hash{},
		}

		nodeStatus = &status.NodeSyncStatus{
			LocalUnsafe: blockRef,
			LocalSafe:   blockSeal,
			CrossUnsafe: blockSeal,
			CrossSafe:   blockSeal,
			Finalized:   blockSeal,
		}
		db.preActivationStatus.Set(id, nodeStatus)
		db.initialized.Set(id, struct{}{})
		return
	}

	// Only update Finalized if the new block is newer
	currentFinalized := nodeStatus.Finalized
	if blockSeal.Number > currentFinalized.Number {
		db.logger.Debug("updating pre-activation Finalized",
			"chain", id,
			"current", currentFinalized.Number,
			"new", blockSeal.Number)

		// Update only Finalized with the new block
		nodeStatus.Finalized = blockSeal

		// Save the updated status
		db.preActivationStatus.Set(id, nodeStatus)
	} else {
		db.logger.Debug("ignoring older or equal Finalized block in pre-activation mode",
			"chain", id,
			"current", currentFinalized.Number,
			"new", blockSeal.Number)
	}
}
