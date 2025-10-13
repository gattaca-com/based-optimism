package engine

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type SealFragProcessEvent struct {
	SignedSeal *eth.SignedSeal
}

func (ev SealFragProcessEvent) String() string {
	return "seal-frag-process"
}

func (ec *EngineController) onSealFragProcess(ev SealFragProcessEvent) {
	ec.engine.SealFrag(ec.ctx, ev.SignedSeal)
	ec.log.Info("new seal sent", "seal", ev.SignedSeal)
}
