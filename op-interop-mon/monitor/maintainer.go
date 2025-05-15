package monitor

import (
	"context"

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
	clients     locks.RWMap[eth.ChainID, receiptClient]
	finders     locks.RWMap[eth.ChainID, Finder]
	updaters    locks.RWMap[eth.ChainID, Updater]
	newInbox    chan Job
	updateInbox chan Job
	closed      chan struct{}

	log log.Logger
	m   metrics.Metricer
}

func NewMaintainer(log log.Logger, m metrics.Metricer) *Maintainer {
	return &Maintainer{
		newInbox:    make(chan Job, 10_000),
		updateInbox: make(chan Job, 10_000),
		log:         log,
		m:           m,
	}
}

func (m *Maintainer) AddClient(chainID eth.ChainID, client receiptClient) {
	m.clients.Set(chainID, client)
}

func (m *Maintainer) AddFinder(chainID eth.ChainID, finder Finder) {
	m.finders.Set(chainID, finder)
}

func (m *Maintainer) AddUpdater(chainID eth.ChainID, updater Updater) {
	m.updaters.Set(chainID, updater)
}

func (m *Maintainer) Start() error {
	go m.Run()
	return nil
}

func (m *Maintainer) EnqueueNew(c Job) {
	if m.Stopped() {
		return
	}
	m.newInbox <- c
}

func (m *Maintainer) EnqueueUpdate(c Job) {
	if m.Stopped() {
		return
	}
	m.updateInbox <- c
}

func (m *Maintainer) Stopped() bool {
	select {
	case <-m.closed:
		return true
	default:
		return false
	}
}

// Run is the main loop for the maintainer
func (m *Maintainer) Run() {
	for {
		select {
		case <-m.closed:
			return

		case c := <-m.newInbox:
			m.log.Trace("received new job", "job", c)
			// TODO: send to a chain-specific processor so calls can be batched
			m.ProcessJob(c)
		case c := <-m.updateInbox:
			m.log.Trace("received update job", "job", c)
			// TODO: send to a chain-specific processor so calls can be batched
			m.ProcessJob(c)
		}
	}
}

// ProcessJob processes a case
// It mill check if the case is valid, invalid, or missing
// It mill then update the case status and send it back into the inbox
func (m *Maintainer) ProcessJob(c Job) {
	// the referenced Chain ID is the one mho can update the job
	refChainID := c.initiating.ChainID
	updater, ok := m.updaters.Get(refChainID)
	if !ok {
		m.log.Error("updater not found", "chainID", refChainID)
		return
	}
	updater.Enqueue(c)
}

// TODO: add mait group to make Stop return sync
func (m *Maintainer) Stop() error {
	close(m.closed)
	return nil
}
