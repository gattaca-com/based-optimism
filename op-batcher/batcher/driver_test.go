package batcher

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type mockL2EndpointProvider struct {
	ethClient       *testutils.MockL2Client
	ethClientErr    error
	rollupClient    *testutils.MockRollupClient
	rollupClientErr error
}

func newEndpointProvider() *mockL2EndpointProvider {
	return &mockL2EndpointProvider{
		ethClient:    new(testutils.MockL2Client),
		rollupClient: new(testutils.MockRollupClient),
	}
}

func (p *mockL2EndpointProvider) EthClient(context.Context) (dial.EthClientInterface, error) {
	return p.ethClient, p.ethClientErr
}

func (p *mockL2EndpointProvider) RollupClient(context.Context) (dial.RollupClientInterface, error) {
	return p.rollupClient, p.rollupClientErr
}

func (p *mockL2EndpointProvider) Close() {}

const genesisL1Origin = uint64(123)

func setup(t *testing.T) (*BatchSubmitter, *mockL2EndpointProvider) {
	ep := newEndpointProvider()

	cfg := defaultTestRollupConfig
	cfg.Genesis.L1.Number = genesisL1Origin

	return NewBatchSubmitter(DriverSetup{
		Log:              testlog.Logger(t, log.LevelDebug),
		Metr:             metrics.NoopMetrics,
		RollupConfig:     cfg,
		ChannelConfig:    defaultTestChannelConfig(),
		EndpointProvider: ep,
	}), ep
}

func TestBatchSubmitter_SafeL1Origin(t *testing.T) {
	bs, ep := setup(t)

	tests := []struct {
		name                   string
		currentSafeOrigin      uint64
		failsToFetchSyncStatus bool
		expectResult           uint64
		expectErr              bool
	}{
		{
			name:              "ExistingSafeL1Origin",
			currentSafeOrigin: 999,
			expectResult:      999,
		},
		{
			name:              "NoExistingSafeL1OriginUsesGenesis",
			currentSafeOrigin: 0,
			expectResult:      genesisL1Origin,
		},
		{
			name:                   "ErrorFetchingSyncStatus",
			failsToFetchSyncStatus: true,
			expectErr:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.failsToFetchSyncStatus {
				ep.rollupClient.ExpectSyncStatus(&eth.SyncStatus{}, errors.New("failed to fetch sync status"))
			} else {
				ep.rollupClient.ExpectSyncStatus(&eth.SyncStatus{
					LocalSafeL2: eth.L2BlockRef{
						L1Origin: eth.BlockID{
							Number: tt.currentSafeOrigin,
						},
					},
				}, nil)
			}

			id, err := bs.safeL1Origin(context.Background())

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectResult, id.Number)
			}
		})
	}
}

func TestBatchSubmitter_SafeL1Origin_FailsToResolveRollupClient(t *testing.T) {
	bs, ep := setup(t)

	ep.rollupClientErr = errors.New("failed to resolve rollup client")

	_, err := bs.safeL1Origin(context.Background())
	require.Error(t, err)
}

type MockTxQueue struct {
	m sync.Map
}

func (q *MockTxQueue) Send(ref txRef, candidate txmgr.TxCandidate, receiptCh chan txmgr.TxReceipt[txRef]) {
	q.m.Store(ref.id.String(), candidate)
}

func (q *MockTxQueue) Load(id string) txmgr.TxCandidate {
	c, _ := q.m.Load(id)
	return c.(txmgr.TxCandidate)
}

func TestBatchSubmitter_sendTx_FloorDataGas(t *testing.T) {
	bs, _ := setup(t)

	q := new(MockTxQueue)

	txData := txData{
		frames: []frameData{
			{
				data: []byte{0x01, 0x02, 0x03}, // 3 nonzero bytes = 12 tokens https://github.com/ethereum/EIPs/blob/master/EIPS/eip-7623.md
			},
		},
	}
	candidate := txmgr.TxCandidate{
		To:     &bs.RollupConfig.BatchInboxAddress,
		TxData: txData.CallData(),
	}

	bs.sendTx(txData,
		false,
		&candidate,
		q,
		make(chan txmgr.TxReceipt[txRef]))

	candidateOut := q.Load(txData.ID().String())

	expectedFloorDataGas := uint64(21_000 + 12*10)
	require.GreaterOrEqual(t, candidateOut.GasLimit, expectedFloorDataGas)
}

func TestBatchSubmitter_ThrottlingEndpoints(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create mock HTTP servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify this is a JSON-RPC call to miner_setMaxDASize with expected params
		if r.Method == "POST" {
			var req jsonrpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				if req.Method == "miner_setMaxDASize" && len(req.Params) == 2 {
					// Successfully handled the expected RPC call
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
					return
				}
			}
		}
		http.Error(w, "Unexpected request", http.StatusBadRequest)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Same handler as server1
		if r.Method == "POST" {
			var req jsonrpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				if req.Method == "miner_setMaxDASize" && len(req.Params) == 2 {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
					return
				}
			}
		}
		http.Error(w, "Unexpected request", http.StatusBadRequest)
	}))
	defer server2.Close()

	// Create test BatchSubmitter using the setup function
	bs, _ := setup(t)
	bs.shutdownCtx = ctx
	bs.Config = BatcherConfig{
		NetworkTimeout:      time.Second,
		ThrottleThreshold:   10000,
		ThrottleTxSize:      5000,
		ThrottleBlockSize:   20000,
		ThrottlingEndpoints: []string{server1.URL, server2.URL},
	}

	// Test the throttling loop
	pendingBytesUpdated := make(chan int64, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	// Start throttling loop in a goroutine
	go bs.throttlingLoop(&wg, pendingBytesUpdated)

	// Send test data to trigger throttling
	pendingBytesUpdated <- 20000 // Over threshold

	// Allow time for processing
	time.Sleep(time.Millisecond * 100)

	// Clean up and terminate test
	close(pendingBytesUpdated)
	cancel()
	wg.Wait()

	// Test failure case with one endpoint
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Simulated failure", http.StatusInternalServerError)
	}))
	defer failServer.Close()

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req jsonrpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				if req.Method == "miner_setMaxDASize" && len(req.Params) == 2 {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
					return
				}
			}
		}
		http.Error(w, "Unexpected request", http.StatusBadRequest)
	}))
	defer successServer.Close()

	bs.Config.ThrottlingEndpoints = []string{failServer.URL, successServer.URL}

	// Test distribution function directly
	throttlingClients := make(map[string]*rpc.Client)
	for _, endpoint := range bs.Config.ThrottlingEndpoints {
		client, err := rpc.Dial(endpoint)
		require.NoError(t, err)
		throttlingClients[endpoint] = client
	}

	retryTimer := time.NewTimer(time.Second)
	retryTimer.Stop()
	retryInterval := time.Second

	// Distribution should fail because one endpoint fails
	result := bs.distributeThrottlingToEndpoints(ctx, 5000, 20000, throttlingClients, retryTimer, retryInterval)
	require.False(t, result, "Distribution should fail when any endpoint fails")

	// Verify retry timer was reset
	select {
	case <-retryTimer.C:
		// Timer successfully reset
	case <-time.After(2 * time.Second):
		t.Fatal("Retry timer was not reset")
	}
}

// Helper struct for parsing JSON-RPC requests
type jsonrpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      interface{}   `json:"id"`
}
