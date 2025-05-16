package core

import "github.com/ethereum-optimism/optimism/op-node/rollup/engine"

// On L1UnsafeEvent
// and wake-up any derivation work
//case status.L1UnsafeEvent:
////a new L1 head may mean we have the data to not get an EOF again.
//s.Emitter.Emit(StepReqEvent{})

// PendingSafeRequestEvent -> derivation

// Poke
func poker() {

	// Since we don't force attributes to be processed at this point,
	// we cannot safely directly trigger the derivation, as that may generate new attributes that
	// conflict with what attributes have not been applied yet.
	// Instead, we request the engine to repeat where its pending-safe head is at.
	// Upon the pending-safe signal the attributes deriver can then ask the pipeline
	// to generate new attributes, if no attributes are known already.
	s.Emitter.Emit(engine.PendingSafeRequestEvent{})
}
