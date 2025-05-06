package activation

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/syncnode"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// ChainInitializer is an interface for initializing a chain database.
type ChainInitializer interface {
	IsInitialized(chainID eth.ChainID) bool
	InitializeWithAnchor(chainID eth.ChainID, anchor types.DerivedBlockRefPair)
}

// ActivationManager is a manager for interop activation. It allows querying if a chain has active interop,
// and detecting and activating interop when a chain is detected to have become active.
type ActivationManager struct {
	depSet depset.DependencySet
	logger log.Logger
}

func NewActivationManager(depSet depset.DependencySet, logger log.Logger) *ActivationManager {
	return &ActivationManager{
		depSet: depSet,
		logger: logger,
	}
}

// IsActiveForChain checks if a chain is active for interop at a given timestamp.
func (am *ActivationManager) IsActiveForChain(chain eth.ChainID, timestamp uint64) bool {
	if timestamp == 0 || am.depSet == nil {
		return false
	}

	canInitiate, err := am.depSet.CanInitiateAt(chain, timestamp)
	if err != nil {
		am.logger.Debug("Error checking interop activation", "chain", chain, "timestamp", timestamp, "err", err)
		return false
	}
	return canInitiate
}

// DetectAndActivateInterop detects and activates interop for a chain.
func (am *ActivationManager) DetectAndActivateInterop(
	ctx context.Context,
	chain eth.ChainID,
	block eth.BlockRef,
	syncSources *locks.RWMap[eth.ChainID, syncnode.SyncSource],
	initializer ChainInitializer,
) error {
	// If the chain is already initialized or interop isn't active, do nothing.
	if initializer.IsInitialized(chain) {
		return nil
	}
	if !am.IsActiveForChain(chain, block.Time) {
		return nil
	}

	// The chain is not initialized, and interop is active, so fetch the anchor point and initialize the chain.
	am.logger.Info("Interop activation detected, fetching anchor point", "chain", chain, "block", block)
	anchor, err := getAnchorPoint(ctx, chain, syncSources)
	if err != nil {
		return fmt.Errorf("failed to get anchor point at interop activation: %w", err)
	}
	am.logger.Info("Initializing with anchor point at interop activation",
		"chain", chain, "derived", anchor.Derived, "source", anchor.Source)
	initializer.InitializeWithAnchor(chain, anchor)
	return nil
}

// getAnchorPoint gets the anchor point for a chain from the sync sources.
func getAnchorPoint(ctx context.Context, chainID eth.ChainID, syncSources *locks.RWMap[eth.ChainID, syncnode.SyncSource]) (types.DerivedBlockRefPair, error) {
	if syncSources == nil {
		return types.DerivedBlockRefPair{}, fmt.Errorf("sync sources not initialized")
	}

	syncSrc, ok := syncSources.Get(chainID)
	if !ok {
		return types.DerivedBlockRefPair{}, fmt.Errorf("no sync source for chain %s", chainID)
	}

	if syncSrc == nil {
		return types.DerivedBlockRefPair{}, fmt.Errorf("sync source is nil for chain %s", chainID)
	}

	return syncSrc.AnchorPoint(ctx)
}
