package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/finality"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Finalizer interface {
	FinalizedL1() eth.L1BlockRef
	event.Deriver
}

func SetupFinalizer() {
	var finalizer Finalizer
	if cfg.AltDAEnabled() {
		finalizer = finality.NewAltDAFinalizer(driverCtx, log, cfg, l1, altDA)
	} else {
		finalizer = finality.NewFinalizer(driverCtx, log, cfg, l1)
	}
	sys.Register("finalizer", finalizer)
}
