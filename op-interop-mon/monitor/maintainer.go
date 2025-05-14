package monitor

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/op-interop-mon/metrics"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// JobUpdater can take cases and update them
type JobUpdater interface {
	UpdateJob(c Job)
}

type receiptClient interface {
	BlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) ([]*types.Receipt, error)
}

type Maintainer struct {
	clients locks.RWMap[eth.ChainID, receiptClient]
	finders locks.RWMap[eth.ChainID, Finder]

	inbox  chan Job
	closed chan struct{}

	log log.Logger
	m   metrics.Metricer
}

func NewMaintainer(log log.Logger, m metrics.Metricer) *Maintainer {
	return &Maintainer{
		inbox: make(chan Job, 10_000),
		log:   log,
		m:     m,
	}
}

func (w *Maintainer) AddClient(chainID eth.ChainID, client receiptClient) {
	w.clients.Set(chainID, client)
}

func (w *Maintainer) AddFinder(chainID eth.ChainID, finder Finder) {
	w.finders.Set(chainID, finder)
}

func (w *Maintainer) Start() error {
	go w.DrainFinders()
	go w.Run()
	return nil
}

// DrainFinders drains the finders into the inbox
// This is a blocking call, and should be run in a separate goroutine
// It will drain the finders every 500ms
func (w *Maintainer) DrainFinders() {
	// forever,
	for {
		// for each finder,
		w.finders.Range(func(chainID eth.ChainID, finder Finder) bool {
			// for each group of messages in the finder's outbox (taken from a single block)
			for cases := range finder.Jobs() {
				// for each message in the group,
				for _, c := range cases {
					// add the case to the inbox
					c.firstSeen = time.Now()
					c.status = []jobStatus{jobStatusUnknown}
					w.inbox <- c
				}
			}
			return true
		})
		// check if the maintainer is closed or waiting for the next drain
		select {
		case <-w.closed:
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// Run is the main loop for the maintainer
func (w *Maintainer) Run() {
	for {
		select {
		case <-w.closed:
			return
		case c := <-w.inbox:
			// TODO: send to a chain-specific processor so calls can be batched
			w.ProcessJob(c)
		}
	}
}

// ProcessJob processes a case
// It will check if the case is valid, invalid, or missing
// It will then update the case status and send it back into the inbox
func (w *Maintainer) ProcessJob(c Job) {
	refChainID := c.initiating.ChainID

	refClient, ok := w.clients.Get(refChainID)
	if !ok {
		w.log.Error("ref client not found", "chainID", refChainID)
		return
	}
	receipts, err := refClient.BlockReceipts(context.Background(),
		rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(c.initiating.BlockNumber)))
	if err != nil {
		w.log.Error("failed to get receipt", "error", err)
		return
	}
	w.log.Info("got receipts", "receipts", receipts)
	w.log.Info("case", "case", c)
	w.log.Info("I am only partially implemented, so I don't do anything intelligent yet")
	w.inbox <- c
}

// TODO: add wait group to make Stop return sync
func (w *Maintainer) Stop() error {
	close(w.closed)
	return nil
}
