package core

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/async"
	"github.com/ethereum-optimism/optimism/op-node/rollup/confdepth"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sequencing"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"time"
)

type SequencerConfig struct {
	// SequencerConfDepth is the distance to keep from the L1 head as origin when sequencing new L2 blocks.
	// If this distance is too large, the sequencer may:
	// - not adopt a L1 origin within the allowed time (rollup.Config.MaxSequencerDrift)
	// - not adopt a L1 origin that can be included on L1 within the allowed range (rollup.Config.SeqWindowSize)
	// and thus fail to produce a block with anything more than deposits.
	SequencerConfDepth uint64 `json:"sequencer_conf_depth"`

	// SequencerEnabled is true when the driver should sequence new blocks.
	SequencerEnabled bool `json:"sequencer_enabled"`

	// SequencerStopped is false when the driver should sequence new blocks.
	SequencerStopped bool `json:"sequencer_stopped"`

	// SequencerMaxSafeLag is the maximum number of L2 blocks for restricting the distance between L2 safe and unsafe.
	// Disabled if 0.
	SequencerMaxSafeLag uint64 `json:"sequencer_max_safe_lag"`

	// RecoverMode forces the sequencer to select the next L1 Origin exactly, and create an empty block,
	// to be compatible with verifiers forcefully generating the same block while catching up the sequencing window timeout.
	RecoverMode bool `json:"recover_mode"`
}

type L1HeadProvider interface {
	L1Head() eth.L1BlockRef
}

type SequencerStateListener interface {
	SequencerStarted() error
	SequencerStopped() error
}

type Network interface {
	// SignAndPublishL2Payload is called by the driver whenever there is a new payload to publish, synchronously with the driver main loop.
	SignAndPublishL2Payload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error
}

// sequencer timing planning loop

// sequencer has Close func() that we need to handle on shutdown (close conductor/async gossip)

// TODO sequencer instantiation apply config:
//
// 	if s.driverConfig.SequencerEnabled {
//		if s.driverConfig.RecoverMode {
//			log.Warn("sequencer is in recover mode")
//			s.sequencer.SetRecoverMode(true)
//		}
//		if err := s.sequencer.SetMaxSafeLag(s.driverCtx, s.driverConfig.SequencerMaxSafeLag); err != nil {
//			return fmt.Errorf("failed to set sequencer max safe lag: %w", err)
//		}
//		if err := s.sequencer.Init(s.driverCtx, !s.driverConfig.SequencerStopped); err != nil {
//			return fmt.Errorf("persist initial sequencer state: %w", err)
//		}
//	}

func sequencingLoop() {

	sequencerTimer := time.NewTimer(0)
	var sequencerCh <-chan time.Time
	var prevTime time.Time
	// planSequencerAction updates the sequencerTimer with the next action, if any.
	// The sequencerCh is nil (indefinitely blocks on read) if no action needs to be performed,
	// or set to the timer channel if there is an action scheduled.
	planSequencerAction := func() {
		nextAction, ok := s.sequencer.NextAction()
		if !ok {
			if sequencerCh != nil {
				s.log.Info("Sequencer paused until new events")
			}
			sequencerCh = nil
			return
		}
		// avoid unnecessary timer resets
		if nextAction == prevTime {
			return
		}
		prevTime = nextAction
		sequencerCh = sequencerTimer.C
		if len(sequencerCh) > 0 { // empty if not already drained before resetting
			<-sequencerCh
		}
		delta := time.Until(nextAction)
		s.log.Info("Scheduled sequencer action", "delta", delta)
		sequencerTimer.Reset(delta)
	}
}

func SetupSequencer(log log.Logger, sys event.System, driverCfg *SequencerConfig, cfg *rollup.Config, l1Head L1HeadProvider) sequencing.SequencerIface {
	if !driverCfg.SequencerEnabled {
		return sequencing.DisabledSequencer{}
	}
	asyncGossiper := async.NewAsyncGossiper(driverCtx, network, log, metrics)
	attrBuilder := derive.NewFetchingAttributesBuilder(cfg, l1, l2)
	sequencerConfDepth := confdepth.NewConfDepth(driverCfg.SequencerConfDepth, l1Head.L1Head, l1)
	findL1Origin := sequencing.NewL1OriginSelector(driverCtx, log, cfg, sequencerConfDepth)
	sys.Register("origin-selector", findL1Origin)
	sequencer := sequencing.NewSequencer(driverCtx, log, cfg, attrBuilder, findL1Origin,
		sequencerStateListener, sequencerConductor, asyncGossiper, metrics)
	sys.Register("sequencer", sequencer)
	return sequencer
}

func (s *Driver) StartSequencer(ctx context.Context, blockHash common.Hash) error {
	return s.sequencer.Start(ctx, blockHash)
}

func (s *Driver) StopSequencer(ctx context.Context) (common.Hash, error) {
	return s.sequencer.Stop(ctx)
}

func (s *Driver) SequencerActive(ctx context.Context) (bool, error) {
	return s.sequencer.Active(), nil
}

func (s *Driver) OverrideLeader(ctx context.Context) error {
	return s.sequencer.OverrideLeader(ctx)
}

func (s *Driver) ConductorEnabled(ctx context.Context) (bool, error) {
	return s.sequencer.ConductorEnabled(ctx), nil
}

func (s *Driver) SetRecoverMode(ctx context.Context, mode bool) error {
	s.sequencer.SetRecoverMode(mode)
	return nil
}
