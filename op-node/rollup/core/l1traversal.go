package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
)

// TODO: if not managed, then do ProvideL1Traversal automatically

func todo() {

	// TODO: on engine is syncing event -> backoff traversal

	if s.Engine.IsEngineSyncing() {
		// The pipeline cannot move forwards if doing EL sync.
		s.Log.Debug("Rollup driver is backing off because execution engine is syncing.",
			"unsafe_head", s.Engine.UnsafeL2Head())
		s.Emitter.Emit(ResetStepBackoffEvent{})
		return
	}
}

//case rollup.L1TemporaryErrorEvent:
//s.Log.Warn("L1 temporary error", "err", x.Err)
//s.Emitter.Emit(StepReqEvent{})

func (s *SyncDeriver) onResetEvent(x rollup.ResetEvent) {
	if s.ManagedMode {
		s.Log.Warn("Encountered reset in Managed Mode, waiting for op-supervisor", "err", x.Err)
		// ManagedMode will pick up the ResetEvent
		return
	}
	// If the system corrupts, e.g. due to a reorg, simply reset it
	s.Log.Warn("Deriver system is resetting", "err", x.Err)
	s.Emitter.Emit(StepReqEvent{})
	s.Emitter.Emit(engine.ResetEngineRequestEvent{})
}

func (s *SyncDeriver) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {

	case rollup.EngineTemporaryErrorEvent:
		s.Log.Warn("Engine temporary error", "err", x.Err)
		// Make sure that for any temporarily failed attributes we retry processing.
		// This will be triggered by a step. After appropriate backoff.
		s.Emitter.Emit(StepReqEvent{})
	case engine.EngineResetConfirmedEvent:
		s.onEngineConfirmedReset(x)
	case derive.DeriverIdleEvent:
		// Once derivation is idle the system is healthy
		// and we can wait for new inputs. No backoff necessary.
		s.Emitter.Emit(ResetStepBackoffEvent{})
	case derive.DeriverMoreEvent:
		// If there is more data to process,
		// continue derivation quickly
		s.Emitter.Emit(StepReqEvent{ResetBackoff: true})
	case derive.ProvideL1Traversal:
		s.Emitter.Emit(StepReqEvent{})
	default:
		return false
	}
	return true
}
