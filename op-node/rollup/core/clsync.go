package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/clsync"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum/go-ethereum/log"
)

func SetupCLSync(log log.Logger, sys event.System, cfg *rollup.Config, metrics clsync.Metrics) {
	clSync := clsync.NewCLSync(log, cfg, metrics) // alt-sync still uses cl-sync state to determine what to sync to
	sys.Register("cl-sync", clSync)
}
