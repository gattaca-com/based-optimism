package engine

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type EnvProcessEvent struct {
	SignedEnv *eth.SignedEnv
}

func (ev EnvProcessEvent) String() string {
	return "env-frag-process"
}

func (ec *EngineController) onEnvProcess(ev EnvProcessEvent) {
	ec.engine.Env(ec.ctx, ev.SignedEnv)
	ec.log.Info("new env sent", "env", ev.SignedEnv)
}
