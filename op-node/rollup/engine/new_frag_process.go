package engine

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type NewFragProcessEvent struct {
	SignedNewFrag *eth.SignedNewFrag
}

func (ev NewFragProcessEvent) String() string {
	return "new-frag-process"
}

func (ec *EngineController) onNewFragProcess(ev NewFragProcessEvent) {
	ec.engine.NewFrag(ec.ctx, ev.SignedNewFrag)
	ec.log.Info("new fragment sent", "frag", ev.SignedNewFrag)
}
