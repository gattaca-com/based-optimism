package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// BlockRefWithStatus blocks the driver event loop and captures the syncing status,
// along with an L2 block reference by number consistent with that same status.
// If the event loop is too busy and the context expires, a context error is returned.
func (s *Driver) BlockRefWithStatus(ctx context.Context, num uint64) (eth.L2BlockRef, *eth.SyncStatus, error) {
	resp := s.statusTracker.SyncStatus()
	if resp.FinalizedL2.Number >= num { // If finalized, we are certain it does not reorg, and don't have to lock.
		ref, err := s.L2.L2BlockRefByNumber(ctx, num)
		return ref, resp, err
	}
	wait := make(chan struct{})
	select {
	case s.stateReq <- wait:
		resp := s.statusTracker.SyncStatus()
		ref, err := s.L2.L2BlockRefByNumber(ctx, num)
		<-wait
		return ref, resp, err
	case <-ctx.Done():
		return eth.L2BlockRef{}, nil, ctx.Err()
	}
}

func todo2() {

	s.Emitter.Emit(engine.TryBackupUnsafeReorgEvent{})

	s.Emitter.Emit(engine.TryUpdateEngineEvent{})

}

func SetupEngine(log log.Logger, sys event.System, cfg *rollup.Config) {
	ec := engine.NewEngineController(l2, log, metrics, cfg, syncCfg,
		sys.Register("engine-controller", nil))
	sys.Register("engine", engine.NewEngDeriver(log, driverCtx, cfg, metrics, ec))
}

func SetupEngineReset() {
	sys.Register("engine-reset",
		engine.NewEngineResetDeriver(driverCtx, log, cfg, l1, l2, syncCfg))
}
