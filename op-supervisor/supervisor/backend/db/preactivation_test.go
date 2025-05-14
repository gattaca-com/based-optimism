package db

import (
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// NoopMetrics for the logs DB
type NoopMetrics struct{}

func (m *NoopMetrics) RecordDBEntryCount(kind string, count int64) {}
func (m *NoopMetrics) RecordDBSearchEntriesRead(count int64)       {}

// NoopChainMetrics for the derivation DB
type NoopChainMetrics struct{}

func (m *NoopChainMetrics) RecordDBDerivedEntryCount(count int64) {}

// setupChainsDB creates a ChainsDB instance with real database files in a temp directory
func setupChainsDB(t *testing.T) (*ChainsDB, eth.ChainID) {
	logger := testlog.Logger(t, log.LvlDebug)
	dbDir := t.TempDir()

	chainID := eth.ChainID{1}

	// Create a dependency set with the test chain
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			chainID: {
				ChainIndex:     0,
				ActivationTime: 0,
				HistoryMinTime: 0,
			},
		},
	)
	require.NoError(t, err)

	// Create a new ChainsDB
	db := NewChainsDB(logger, depSet, nil)

	// Set up the logs database
	logDB, err := logs.NewFromFile(logger, &NoopMetrics{}, chainID, filepath.Join(dbDir, "logs.db"), false)
	require.NoError(t, err)
	db.AddLogDB(chainID, logDB)

	// Set up the local derivation database
	localDB, err := fromda.NewFromFile(logger, &NoopChainMetrics{}, filepath.Join(dbDir, "local.db"))
	require.NoError(t, err)
	db.AddLocalDerivationDB(chainID, localDB)

	// Set up the cross derivation database
	crossDB, err := fromda.NewFromFile(logger, &NoopChainMetrics{}, filepath.Join(dbDir, "cross.db"))
	require.NoError(t, err)
	db.AddCrossDerivationDB(chainID, crossDB)

	// Set up cross tracker
	db.AddCrossUnsafeTracker(chainID)

	return db, chainID
}

func TestPreActivationMode(t *testing.T) {
	// Set up the test database
	db, chainID := setupChainsDB(t)

	// Make sure logDB is available
	_, ok := db.logDBs.Get(chainID)
	require.True(t, ok, "LogDB should be available")

	// Clean up at the end of the test
	t.Cleanup(func() {
		if logdb, ok := db.logDBs.Get(chainID); ok {
			_ = logdb.Close()
		}
	})

	// Create test block references
	blockRef := eth.BlockRef{
		Hash:       common.Hash{1, 2, 3},
		Number:     100,
		Time:       200,
		ParentHash: common.Hash{0, 0, 0},
	}

	// Initialize in pre-activation mode
	db.InitializePreActivation(chainID, blockRef)

	// Verify pre-activation state
	require.True(t, db.IsInPreActivationMode(chainID))
	require.True(t, db.IsInitialized(chainID))

	// Check that we can get the pre-activation status
	_, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)

	// Verify the head can be accessed from the NodeSyncStatus
	status, ok := db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, blockRef, status.LocalUnsafe)

	// Check query methods in pre-activation mode
	// LocalUnsafe should return the head block
	localUnsafe, err := db.LocalUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, localUnsafe.Hash)
	require.Equal(t, blockRef.Number, localUnsafe.Number)
	require.Equal(t, blockRef.Time, localUnsafe.Timestamp)

	// CrossUnsafe should return the head block
	crossUnsafe, err := db.CrossUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, crossUnsafe.Hash)
	require.Equal(t, blockRef.Number, crossUnsafe.Number)
	require.Equal(t, blockRef.Time, crossUnsafe.Timestamp)

	// LocalSafe should return a self-derived pair with the head block
	localSafe, err := db.LocalSafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, localSafe.Derived.Hash)
	require.Equal(t, blockRef.Number, localSafe.Derived.Number)
	require.Equal(t, blockRef.Time, localSafe.Derived.Timestamp)
	require.Equal(t, blockRef.Hash, localSafe.Source.Hash)
	require.Equal(t, blockRef.Number, localSafe.Source.Number)
	require.Equal(t, blockRef.Time, localSafe.Source.Timestamp)

	// CrossSafe should return a self-derived pair with the head block
	crossSafe, err := db.CrossSafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, crossSafe.Derived.Hash)
	require.Equal(t, blockRef.Number, crossSafe.Derived.Number)
	require.Equal(t, blockRef.Time, crossSafe.Derived.Timestamp)
	require.Equal(t, blockRef.Hash, crossSafe.Source.Hash)
	require.Equal(t, blockRef.Number, crossSafe.Source.Number)
	require.Equal(t, blockRef.Time, crossSafe.Source.Timestamp)

	// Update the head in pre-activation mode by initializing with a new block ref
	newBlockRef := eth.BlockRef{
		Hash:       common.Hash{3, 2, 1},
		Number:     101,
		Time:       201,
		ParentHash: blockRef.Hash,
	}
	db.InitializePreActivation(chainID, newBlockRef)

	// Verify the head was updated
	nodeStatus, ok := db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, newBlockRef, nodeStatus.LocalUnsafe)

	// Verify LocalUnsafe returns the updated head
	localUnsafe, err = db.LocalUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, newBlockRef.Hash, localUnsafe.Hash)
	require.Equal(t, newBlockRef.Number, localUnsafe.Number)
	require.Equal(t, newBlockRef.Time, localUnsafe.Timestamp)

	// Verify all other references are updated too
	crossUnsafe, err = db.CrossUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, newBlockRef.Hash, crossUnsafe.Hash)

	localSafe, err = db.LocalSafe(chainID)
	require.NoError(t, err)
	require.Equal(t, newBlockRef.Hash, localSafe.Derived.Hash)

	crossSafe, err = db.CrossSafe(chainID)
	require.NoError(t, err)
	require.Equal(t, newBlockRef.Hash, crossSafe.Derived.Hash)
}

func TestPreActivationUpdateMethods(t *testing.T) {
	// Set up the test database
	db, chainID := setupChainsDB(t)

	// Clean up at the end of the test
	t.Cleanup(func() {
		if logdb, ok := db.logDBs.Get(chainID); ok {
			_ = logdb.Close()
		}
	})

	// Create initial block references
	initialBlock := eth.BlockRef{
		Hash:       common.Hash{1, 2, 3},
		Number:     100,
		Time:       200,
		ParentHash: common.Hash{0, 0, 0},
	}

	// Initialize in pre-activation mode
	db.InitializePreActivation(chainID, initialBlock)

	// Ensure it's initialized correctly
	require.True(t, db.IsInPreActivationMode(chainID))
	require.True(t, db.IsInitialized(chainID))

	// Test 1: Update with a newer LocalUnsafe block
	newerUnsafeBlock := eth.BlockRef{
		Hash:       common.Hash{2, 3, 4},
		Number:     101,
		Time:       210,
		ParentHash: initialBlock.Hash,
	}
	db.UpdatePreActivationUnsafe(chainID, newerUnsafeBlock)

	// Verify the status was updated
	status, ok := db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, newerUnsafeBlock, status.LocalUnsafe, "LocalUnsafe should be updated to the newer block")
	require.Equal(t, newerUnsafeBlock.Hash, status.CrossUnsafe.Hash, "CrossUnsafe should be updated with the newer block")

	// LocalSafe should still have the initial block value
	require.Equal(t, initialBlock.Hash, status.LocalSafe.Hash, "LocalSafe should not be changed by LocalUnsafe update")
	require.Equal(t, initialBlock.Hash, status.CrossSafe.Hash, "CrossSafe should not be changed by LocalUnsafe update")

	// Test 2: Update with an older LocalUnsafe block (should be ignored)
	olderUnsafeBlock := eth.BlockRef{
		Hash:       common.Hash{0, 1, 2},
		Number:     99,
		Time:       190,
		ParentHash: common.Hash{},
	}
	db.UpdatePreActivationUnsafe(chainID, olderUnsafeBlock)

	// Verify status remains unchanged
	status, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, newerUnsafeBlock, status.LocalUnsafe, "LocalUnsafe should not be changed to an older block")
	require.Equal(t, newerUnsafeBlock.Hash, status.CrossUnsafe.Hash, "CrossUnsafe should not be changed to an older block")

	// Test 3: Update with a newer LocalSafe block
	// Create a newer safe block reference
	newerSafeBlockRef := eth.BlockRef{
		Hash:       common.Hash{4, 5, 6},
		Number:     102,
		Time:       220,
		ParentHash: common.Hash{}, // We don't have this in the test
	}
	newerSafeBlockSeal := types.BlockSealFromRef(newerSafeBlockRef)
	db.UpdatePreActivationSafe(chainID, newerSafeBlockRef)

	// Verify the status was updated
	status, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)

	// BlockRef has already been created above

	// Both LocalUnsafe and LocalSafe should be updated
	require.Equal(t, newerSafeBlockRef, status.LocalUnsafe, "LocalUnsafe should be updated from newer LocalSafe")
	require.Equal(t, newerSafeBlockSeal.Hash, status.LocalSafe.Hash, "LocalSafe should be updated to the newer block")
	require.Equal(t, newerSafeBlockSeal.Hash, status.CrossUnsafe.Hash, "CrossUnsafe should be updated with the newer block")
	require.Equal(t, newerSafeBlockSeal.Hash, status.CrossSafe.Hash, "CrossSafe should be updated with the newer block")
	require.Equal(t, newerSafeBlockSeal.Hash, status.Finalized.Hash, "Finalized should be updated with the newer block")

	// Test 4: Update with an older LocalSafe block (should be ignored)
	olderSafeBlockRef := eth.BlockRef{
		Hash:       common.Hash{2, 2, 2},
		Number:     101,
		Time:       210,
		ParentHash: common.Hash{}, // We don't have this in the test
	}
	db.UpdatePreActivationSafe(chainID, olderSafeBlockRef)

	// Verify status remains unchanged for LocalSafe
	status, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, newerSafeBlockRef.Hash, status.LocalSafe.Hash, "LocalSafe should not be changed to an older block")
	require.Equal(t, newerSafeBlockRef.Hash, status.CrossSafe.Hash, "CrossSafe should not be changed to an older block")

	// Test 5: Update with a new LocalUnsafe that's newer than LocalSafe
	evenNewerUnsafeBlock := eth.BlockRef{
		Hash:       common.Hash{7, 8, 9},
		Number:     103,
		Time:       230,
		ParentHash: newerSafeBlockRef.Hash,
	}
	db.UpdatePreActivationUnsafe(chainID, evenNewerUnsafeBlock)

	// Verify the status was updated
	status, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, evenNewerUnsafeBlock, status.LocalUnsafe, "LocalUnsafe should be updated to the newer block")
	require.Equal(t, evenNewerUnsafeBlock.Hash, status.CrossUnsafe.Hash, "CrossUnsafe should be updated with the newer block")

	// But LocalSafe should remain unchanged
	require.Equal(t, newerSafeBlockSeal.Hash, status.LocalSafe.Hash, "LocalSafe should not be changed by a newer LocalUnsafe")
	require.Equal(t, newerSafeBlockSeal.Hash, status.CrossSafe.Hash, "CrossSafe should not be changed by a newer LocalUnsafe")
}

func TestPreActivationAutoInitialization(t *testing.T) {
	// Set up the test database
	db, chainID := setupChainsDB(t)

	// Clean up at the end of the test
	t.Cleanup(func() {
		if logdb, ok := db.logDBs.Get(chainID); ok {
			_ = logdb.Close()
		}
	})

	// Chain shouldn't be in pre-activation mode yet
	require.False(t, db.IsInPreActivationMode(chainID), "Chain shouldn't be in pre-activation mode initially")

	// Test that calling UpdatePreActivationUnsafe automatically initializes pre-activation mode
	firstBlock := eth.BlockRef{
		Hash:       common.Hash{1, 2, 3},
		Number:     100,
		Time:       200,
		ParentHash: common.Hash{0, 0, 0},
	}

	// Update should initialize pre-activation mode
	db.UpdatePreActivationUnsafe(chainID, firstBlock)

	// Now it should be in pre-activation mode
	require.True(t, db.IsInPreActivationMode(chainID), "Chain should be in pre-activation mode after UpdatePreActivationUnsafe")
	require.True(t, db.IsInitialized(chainID), "Chain should be initialized after UpdatePreActivationUnsafe")

	// Verify that the status was correctly initialized
	status, ok := db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, firstBlock, status.LocalUnsafe, "LocalUnsafe should be set to the first block")

	// Set up a new chain for testing LocalSafe auto-initialization
	chainID2 := eth.ChainID{2}

	// Add required DBs for the new chain
	logDB, err := logs.NewFromFile(db.logger, &NoopMetrics{}, chainID2, filepath.Join(t.TempDir(), "logs2.db"), false)
	require.NoError(t, err)
	db.AddLogDB(chainID2, logDB)

	localDB, err := fromda.NewFromFile(db.logger, &NoopChainMetrics{}, filepath.Join(t.TempDir(), "local2.db"))
	require.NoError(t, err)
	db.AddLocalDerivationDB(chainID2, localDB)

	crossDB, err := fromda.NewFromFile(db.logger, &NoopChainMetrics{}, filepath.Join(t.TempDir(), "cross2.db"))
	require.NoError(t, err)
	db.AddCrossDerivationDB(chainID2, crossDB)

	db.AddCrossUnsafeTracker(chainID2)

	// Chain shouldn't be in pre-activation mode yet
	require.False(t, db.IsInPreActivationMode(chainID2), "Chain 2 shouldn't be in pre-activation mode initially")

	// Test that calling UpdatePreActivationSafe automatically initializes pre-activation mode
	safeBlockRef := eth.BlockRef{
		Hash:       common.Hash{4, 5, 6},
		Number:     102,
		Time:       220,
		ParentHash: common.Hash{}, // We don't have this in the test
	}
	safeBlockSeal := types.BlockSealFromRef(safeBlockRef)

	// Update should initialize pre-activation mode
	db.UpdatePreActivationSafe(chainID2, safeBlockRef)

	// Now it should be in pre-activation mode
	require.True(t, db.IsInPreActivationMode(chainID2), "Chain 2 should be in pre-activation mode after UpdatePreActivationSafe")
	require.True(t, db.IsInitialized(chainID2), "Chain 2 should be initialized after UpdatePreActivationSafe")

	// Verify that the status was correctly initialized
	status2, ok := db.GetPreActivationStatus(chainID2)
	require.True(t, ok)
	require.Equal(t, safeBlockSeal.Number, status2.LocalUnsafe.Number, "LocalUnsafe should be set to the safe block")
	require.Equal(t, safeBlockSeal.Hash, status2.LocalSafe.Hash, "LocalSafe should be set to the safe block")
}

func TestPreActivationFinalized(t *testing.T) {
	// Set up the test database
	db, chainID := setupChainsDB(t)

	// Clean up at the end of the test
	t.Cleanup(func() {
		if logdb, ok := db.logDBs.Get(chainID); ok {
			_ = logdb.Close()
		}
	})

	// Create initial block
	initialBlock := eth.BlockRef{
		Hash:       common.Hash{1, 2, 3},
		Number:     100,
		Time:       200,
		ParentHash: common.Hash{0, 0, 0},
	}

	// Initialize in pre-activation mode
	db.InitializePreActivation(chainID, initialBlock)

	// Ensure it's initialized correctly
	require.True(t, db.IsInPreActivationMode(chainID))
	require.True(t, db.IsInitialized(chainID))

	// Verify initial finalized status (initially should match the initialBlock)
	status, ok := db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, initialBlock.Hash, status.Finalized.Hash)
	require.Equal(t, initialBlock.Number, status.Finalized.Number)

	// Create a newer finalized block
	newerFinalizedRef := eth.BlockRef{
		Hash:       common.Hash{4, 5, 6},
		Number:     105,
		Time:       250,
		ParentHash: common.Hash{}, // We don't have this in the test
	}
	newerFinalized := types.BlockSealFromRef(newerFinalizedRef)

	// Update the finalized block
	db.UpdatePreActivationFinalized(chainID, newerFinalizedRef)

	// Verify the status was updated
	status, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, newerFinalized.Hash, status.Finalized.Hash)
	require.Equal(t, newerFinalized.Number, status.Finalized.Number)

	// But LocalUnsafe and LocalSafe should still have the initial values
	require.Equal(t, initialBlock.Hash, status.LocalUnsafe.Hash)
	require.Equal(t, initialBlock.Hash, status.LocalSafe.Hash)

	// Test with older finalized block (should be ignored)
	olderFinalizedRef := eth.BlockRef{
		Hash:       common.Hash{7, 8, 9},
		Number:     99,
		Time:       190,
		ParentHash: common.Hash{}, // We don't have this in the test
	}

	db.UpdatePreActivationFinalized(chainID, olderFinalizedRef)

	// Verify the status was not updated (still has newerFinalized)
	status, ok = db.GetPreActivationStatus(chainID)
	require.True(t, ok)
	require.Equal(t, newerFinalized.Hash, status.Finalized.Hash)
	require.Equal(t, newerFinalized.Number, status.Finalized.Number)

	// Test initialization with finalized block
	chainID2 := eth.ChainID{2}

	// Add required DBs for the new chain
	logDB, err := logs.NewFromFile(db.logger, &NoopMetrics{}, chainID2, filepath.Join(t.TempDir(), "logs2.db"), false)
	require.NoError(t, err)
	db.AddLogDB(chainID2, logDB)

	localDB, err := fromda.NewFromFile(db.logger, &NoopChainMetrics{}, filepath.Join(t.TempDir(), "local2.db"))
	require.NoError(t, err)
	db.AddLocalDerivationDB(chainID2, localDB)

	crossDB, err := fromda.NewFromFile(db.logger, &NoopChainMetrics{}, filepath.Join(t.TempDir(), "cross2.db"))
	require.NoError(t, err)
	db.AddCrossDerivationDB(chainID2, crossDB)

	db.AddCrossUnsafeTracker(chainID2)

	// Chain shouldn't be in pre-activation mode yet
	require.False(t, db.IsInPreActivationMode(chainID2))

	// Initialize with finalized block
	finalizedBlockRef := eth.BlockRef{
		Hash:       common.Hash{10, 11, 12},
		Number:     200,
		Time:       300,
		ParentHash: common.Hash{}, // We don't have this in the test
	}
	finalizedBlock := types.BlockSealFromRef(finalizedBlockRef)

	db.UpdatePreActivationFinalized(chainID2, finalizedBlockRef)

	// Chain should now be in pre-activation mode
	require.True(t, db.IsInPreActivationMode(chainID2))
	require.True(t, db.IsInitialized(chainID2))

	// Verify status was set correctly
	status2, ok := db.GetPreActivationStatus(chainID2)
	require.True(t, ok)
	require.Equal(t, finalizedBlock.Hash, status2.Finalized.Hash)
	require.Equal(t, finalizedBlock.Hash, status2.LocalSafe.Hash)
	require.Equal(t, finalizedBlock.Number, status2.LocalUnsafe.Number)
}
