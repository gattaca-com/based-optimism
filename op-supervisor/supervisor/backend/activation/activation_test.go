package activation

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// MockDependencySet implements the DependencySet interface for testing
type mockDependencySet struct {
	chainConfigs   map[eth.ChainID]*depset.StaticConfigDependency
	messageExpiry  uint64
	activationTime uint64
}

func (m *mockDependencySet) AddChain(chainID eth.ChainID, activationTime uint64) {
	if m.chainConfigs == nil {
		m.chainConfigs = make(map[eth.ChainID]*depset.StaticConfigDependency)
	}

	chainValue, ok := chainID.Uint64()
	if !ok {
		panic("chain ID too large")
	}

	m.chainConfigs[chainID] = &depset.StaticConfigDependency{
		ChainIndex:     types.ChainIndex(chainValue),
		ActivationTime: activationTime,
		HistoryMinTime: activationTime - 1,
	}
}

func (m *mockDependencySet) Chains() []eth.ChainID {
	chains := make([]eth.ChainID, 0, len(m.chainConfigs))
	for chain := range m.chainConfigs {
		chains = append(chains, chain)
	}
	return chains
}

func (m *mockDependencySet) CanInitiateAt(chain eth.ChainID, timestamp uint64) (bool, error) {
	cfg, ok := m.chainConfigs[chain]
	if !ok {
		return false, nil
	}
	return timestamp > cfg.ActivationTime, nil
}

func (m *mockDependencySet) CanReceiveAt(chain eth.ChainID, timestamp uint64) (bool, error) {
	return m.CanInitiateAt(chain, timestamp)
}

func (m *mockDependencySet) CanExecuteAt(chain eth.ChainID, timestamp uint64) (bool, error) {
	return m.CanInitiateAt(chain, timestamp)
}

func (m *mockDependencySet) MessageExpiryWindow() uint64 {
	return m.messageExpiry
}

func (m *mockDependencySet) ReverseChainLookup(idx types.ChainIndex) (eth.ChainID, error) {
	for chain, cfg := range m.chainConfigs {
		if cfg.ChainIndex == idx {
			return chain, nil
		}
	}
	return eth.ChainID{}, nil
}

func (m *mockDependencySet) ChainIDFromIndex(idx types.ChainIndex) (eth.ChainID, error) {
	return m.ReverseChainLookup(idx)
}

func (m *mockDependencySet) ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error) {
	cfg, ok := m.chainConfigs[id]
	if !ok {
		return 0, nil
	}
	return cfg.ChainIndex, nil
}

func (m *mockDependencySet) HasChain(id eth.ChainID) bool {
	_, ok := m.chainConfigs[id]
	return ok
}

func (m *mockDependencySet) ValidMessageLifespan(timestamp uint64) (bool, error) {
	now := uint64(time.Now().Unix())
	if timestamp > now {
		return false, nil
	}
	age := now - timestamp
	return age <= m.messageExpiry, nil
}

func TestActivationTimestampChecks(t *testing.T) {
	baseTime := uint64(time.Now().Unix() + 60)

	mockDepSet := &mockDependencySet{
		activationTime: baseTime,
		messageExpiry:  3600,
	}
	chainID := eth.ChainID{1}
	mockDepSet.AddChain(chainID, baseTime)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheckFn := NewCheckFn(mockDepSet, logger)

	testCases := map[uint64]bool{
		baseTime - 2: false,
		baseTime - 1: false,
		baseTime:     false,
		baseTime + 1: true,
		baseTime + 2: true,
	}

	for ts, expectedVal := range testCases {
		active := activationCheckFn(chainID, ts)
		require.Equal(t, expectedVal, active,
			"IsActiveForChain at timestamp %d (activation+%d)", ts, int(ts)-int(baseTime))
	}
}

func TestActivationTimestampChecksEdgeCases(t *testing.T) {
	activationTime := uint64(1000000)
	mockDepSet := &mockDependencySet{
		activationTime: activationTime,
		messageExpiry:  3600,
	}

	chainID := eth.ChainID{1}
	mockDepSet.AddChain(chainID, activationTime)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheckFn := NewCheckFn(mockDepSet, logger)

	testCases := []struct {
		name      string
		timestamp uint64
		expected  bool
	}{
		{"Zero timestamp", 0, false},
		{"One before activation", activationTime - 1, false},
		{"At activation", activationTime, false},
		{"One after activation", activationTime + 1, true},
		{"Far future", activationTime + 1000000, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			active := activationCheckFn(chainID, tc.timestamp)

			require.Equal(t, tc.expected, active,
				"IsActiveForChain at timestamp %d", tc.timestamp)
		})
	}

	unknownChain := eth.ChainID{99}
	active := activationCheckFn(unknownChain, activationTime+1)
	require.False(t, active, "Unknown chain should not be active")
}

func TestActivationBlockFiltering(t *testing.T) {
	activationTime := uint64(time.Now().Unix() + 3600)

	mockDepSet := &mockDependencySet{
		activationTime: activationTime,
		messageExpiry:  3600,
	}

	chainID := eth.ChainID{1}
	mockDepSet.AddChain(chainID, activationTime)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheckFn := NewCheckFn(mockDepSet, logger)

	preActivationBlock := eth.BlockRef{
		Time: activationTime - 600,
	}

	postActivationBlock := eth.BlockRef{
		Time: activationTime + 600,
	}

	isActiveForPreActivation := activationCheckFn(chainID, preActivationBlock.Time)
	require.False(t, isActiveForPreActivation, "Chain should not be active at pre-activation time")

	isActiveForPostActivation := activationCheckFn(chainID, postActivationBlock.Time)
	require.True(t, isActiveForPostActivation, "Chain should be active at post-activation time")
}

func TestActivationBoundary(t *testing.T) {
	activationTime := uint64(time.Now().Unix())

	mockDepSet := &mockDependencySet{
		activationTime: activationTime,
		messageExpiry:  3600,
	}

	chainA := eth.ChainID{1}
	chainB := eth.ChainID{2}
	mockDepSet.AddChain(chainA, activationTime)
	mockDepSet.AddChain(chainB, activationTime)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheckFn := NewCheckFn(mockDepSet, logger)

	blockAtActivationA := eth.BlockRef{
		Time: activationTime,
	}

	blockAtActivationB := eth.BlockRef{
		Time: activationTime,
	}

	isActiveA := activationCheckFn(chainA, blockAtActivationA.Time)
	isActiveB := activationCheckFn(chainB, blockAtActivationB.Time)

	require.False(t, isActiveA, "Chain A should not be active at exactly the activation time")
	require.False(t, isActiveB, "Chain B should not be active at exactly the activation time")

	blockJustAfterA := eth.BlockRef{
		Time: activationTime + 1,
	}

	blockJustAfterB := eth.BlockRef{
		Time: activationTime + 1,
	}

	isActiveJustAfterA := activationCheckFn(chainA, blockJustAfterA.Time)
	isActiveJustAfterB := activationCheckFn(chainB, blockJustAfterB.Time)

	require.True(t, isActiveJustAfterA, "Chain A should be active just after the activation time")
	require.True(t, isActiveJustAfterB, "Chain B should be active just after the activation time")

	require.False(t, activationCheckFn(chainA, blockAtActivationA.Time))
	require.False(t, activationCheckFn(chainB, blockAtActivationB.Time))
	require.True(t, activationCheckFn(chainA, blockJustAfterA.Time))
	require.True(t, activationCheckFn(chainB, blockJustAfterB.Time))
}

func TestActivationBoundaryMultipleChainsSameActivationTime(t *testing.T) {
	activationTime := uint64(time.Now().Unix() + 10)

	mockDepSet := &mockDependencySet{
		activationTime: activationTime,
		messageExpiry:  3600,
	}

	chainA := eth.ChainID{1}
	chainB := eth.ChainID{2}
	chainC := eth.ChainID{3}
	mockDepSet.AddChain(chainA, activationTime)
	mockDepSet.AddChain(chainB, activationTime)
	mockDepSet.AddChain(chainC, activationTime)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheckFn := NewCheckFn(mockDepSet, logger)

	beforeActivation := eth.BlockRef{Time: activationTime - 5}
	atActivation := eth.BlockRef{Time: activationTime}
	afterActivation := eth.BlockRef{Time: activationTime + 5}

	require.False(t, activationCheckFn(chainA, beforeActivation.Time))
	require.False(t, activationCheckFn(chainB, beforeActivation.Time))
	require.False(t, activationCheckFn(chainC, beforeActivation.Time))

	require.False(t, activationCheckFn(chainA, atActivation.Time))
	require.False(t, activationCheckFn(chainB, atActivation.Time))
	require.False(t, activationCheckFn(chainC, atActivation.Time))

	require.True(t, activationCheckFn(chainA, afterActivation.Time))
	require.True(t, activationCheckFn(chainB, afterActivation.Time))
	require.True(t, activationCheckFn(chainC, afterActivation.Time))
}

func TestActivationBoundaryMultipleChainsDifferentActivationTimes(t *testing.T) {
	baseTime := uint64(time.Now().Unix())

	mockDepSet := &mockDependencySet{
		activationTime: baseTime,
		messageExpiry:  3600,
	}

	chainA := eth.ChainID{1}
	chainB := eth.ChainID{2}
	chainC := eth.ChainID{3}

	mockDepSet.AddChain(chainA, baseTime)
	mockDepSet.AddChain(chainB, baseTime+10)
	mockDepSet.AddChain(chainC, baseTime+20)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheckFn := NewCheckFn(mockDepSet, logger)

	t1 := eth.BlockRef{Time: baseTime + 5}
	t2 := eth.BlockRef{Time: baseTime + 15}
	t3 := eth.BlockRef{Time: baseTime + 25}

	require.True(t, activationCheckFn(chainA, t1.Time))
	require.False(t, activationCheckFn(chainB, t1.Time))
	require.False(t, activationCheckFn(chainC, t1.Time))

	require.True(t, activationCheckFn(chainA, t2.Time))
	require.True(t, activationCheckFn(chainB, t2.Time))
	require.False(t, activationCheckFn(chainC, t2.Time))

	require.True(t, activationCheckFn(chainA, t3.Time))
	require.True(t, activationCheckFn(chainB, t3.Time))
	require.True(t, activationCheckFn(chainC, t3.Time))
}
