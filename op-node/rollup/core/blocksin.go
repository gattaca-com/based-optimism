package core

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/clsync"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// need a payload block receiver
// if CL sync -> fire CL sync event
// if EL sync -> fire engine new payload event (remove engine new Payload)

// case p2p.ReceivedBlockEvent:
// s.onIncomingP2PBlock(x.Envelope)

func (s *SyncDeriver) onIncomingP2PBlock(envelope *eth.ExecutionPayloadEnvelope) {
	// If we are doing CL sync or done with engine syncing, fallback to the unsafe payload queue & CL P2P sync.
	if s.SyncCfg.SyncMode == sync.CLSync || !s.Engine.IsEngineSyncing() {
		s.Log.Info("Optimistically queueing unsafe L2 execution payload", "id", envelope.ExecutionPayload.ID())
		s.Emitter.Emit(clsync.ReceivedUnsafePayloadEvent{Envelope: envelope})
		s.Emitter.Emit(StepReqEvent{})
	} else if s.SyncCfg.SyncMode == sync.ELSync {
		ref, err := derive.PayloadToBlockRef(s.Config, envelope.ExecutionPayload)
		if err != nil {
			s.Log.Info("Failed to turn execution payload into a block ref", "id", envelope.ExecutionPayload.ID(), "err", err)
			return
		}
		if ref.Number <= s.Engine.UnsafeL2Head().Number {
			return
		}
		s.Log.Info("Optimistically inserting unsafe L2 execution payload to drive EL sync", "id", envelope.ExecutionPayload.ID())
		if err := s.Engine.InsertUnsafePayload(s.Ctx, envelope, ref); err != nil {
			s.Log.Warn("Failed to insert unsafe payload for EL sync", "id", envelope.ExecutionPayload.ID(), "err", err)
		}
	}
}
