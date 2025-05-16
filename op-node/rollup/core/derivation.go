package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/confdepth"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum/go-ethereum/log"
)

type VerifierConfig struct {
	// VerifierConfDepth is the distance to keep from the L1 head when reading L1 data for L2 derivation.
	VerifierConfDepth uint64 `json:"verifier_conf_depth"`
}

func SetupDerivation(log log.Logger, sys event.System, driverCfg *VerifierConfig, cfg *rollup.Config) {
	verifConfDepth := confdepth.NewConfDepth(driverCfg.VerifierConfDepth, statusTracker.L1Head, l1)
	derivationPipeline := derive.NewDerivationPipeline(log, cfg, verifConfDepth,
		l1Blobs, altDA, l2, metrics, managedMode)
	sys.Register("pipeline",
		derive.NewPipelineDeriver(driverCtx, derivationPipeline))
}
