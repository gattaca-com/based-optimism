package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/status"
	"github.com/ethereum/go-ethereum/log"
)

func SetupStatusTracker(log log.Logger, sys event.System, metrics status.Metrics) {
	statusTracker := status.NewStatusTracker(log, metrics)
	sys.Register("status", statusTracker)
}

func SetupL1Tracker(sys event.System, l1 derive.L1Fetcher) *status.L1Tracker {
	l1Tracker := status.NewL1Tracker(l1)
	sys.Register("l1-blocks", l1Tracker)
	return l1Tracker
}
