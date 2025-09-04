package testutils

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type MockRuntimeConfig struct {
	P2PSeqAddress     common.Address
	P2PGatewayAddress common.Address
}

func (m *MockRuntimeConfig) P2PSequencerAddress() common.Address {
	return m.P2PSeqAddress
}

func (m *MockRuntimeConfig) GatewayForBlock(ctx context.Context, blockNumber uint64) (common.Address, error) {
	return m.P2PGatewayAddress, nil
}

func (m *MockRuntimeConfig) FetchNextNGateways(ctx context.Context, n uint64, maxRetries uint64) error {
	return nil
}

func (m *MockRuntimeConfig) UnsafeAllowOldPayloads(timestamp hexutil.Uint64) bool {
	return false
}
