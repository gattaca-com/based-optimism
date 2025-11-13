package engine

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// Generic Option type
type Option[T comparable] struct {
	value T
	set   bool
}

// Sets a value to the option and marks it as set.
func (o *Option[T]) Set(value T) {
	o.value = value
	o.set = true
}

// Checks if the value is set.
func (o *Option[T]) IsSet() bool {
	return o.set
}

// Checks if the value is set and equal to the one passed.
func (o Option[T]) IsEqual(v T) bool {
	return o.IsSet() && o.value == v
}

// Returns an optional with set = false and the default value.
func None[T comparable]() Option[T] {
	return Option[T]{set: false}
}

// Entrypoint to the PreconfHandler.
type PreconfChannels struct {
	EnvCh     chan *eth.SignedEnv
	NewFragCh chan *eth.SignedNewFrag
	SealCh    chan *eth.SignedSeal
	l2BlockCh chan *eth.L2BlockRef
}

func NewPreconfChannels() PreconfChannels {
	return PreconfChannels{
		EnvCh:     make(chan *eth.SignedEnv),
		NewFragCh: make(chan *eth.SignedNewFrag),
		SealCh:    make(chan *eth.SignedSeal),
		l2BlockCh: make(chan *eth.L2BlockRef),
	}
}

func (c *PreconfChannels) SendEnv(e *eth.SignedEnv)      { c.EnvCh <- e }
func (c *PreconfChannels) SendFrag(f *eth.SignedNewFrag) { c.NewFragCh <- f }
func (c *PreconfChannels) SendSeal(s *eth.SignedSeal)    { c.SealCh <- s }
func (c *PreconfChannels) SendL2Block(b *eth.L2BlockRef) { c.l2BlockCh <- b }

type FragIndex struct {
	BlockNumber uint64
	Sequence    uint64
}

func index(f eth.NewFrag) FragIndex {
	return FragIndex{BlockNumber: f.BlockNumber, Sequence: f.Seq}
}

func (fi FragIndex) prev() FragIndex {
	return FragIndex{BlockNumber: fi.BlockNumber, Sequence: fi.Sequence - 1}
}

func (fi FragIndex) next() FragIndex {
	return FragIndex{BlockNumber: fi.BlockNumber, Sequence: fi.Sequence + 1}
}

// In charge of holding the current known preconf state and sending ready
// events to the engine api. The events that are not ready yet will be held
// until they are.
type PreconfState struct {
	// Block number of the last Env sent to the engine api.
	lastEnvSent Option[uint64]
	// Block number + sequence number of the last frag sent to the engine api.
	lastFragSent Option[FragIndex]
	// Block number of the last seal sent to the engine api.
	lastSealSent Option[uint64]
	// Block number of the last safe L2 block known to this state.
	lastL2BlockSent Option[uint64]
	// Contains the latest block for which all frags where sent.
	lastBlockWithAllFrags Option[uint64]
	lastBlockPruned       uint64

	pendingEnvs  map[uint64]eth.SignedEnv
	pendingFrags map[FragIndex]eth.SignedNewFrag
	pendingSeals map[uint64]eth.SignedSeal
	ctx          context.Context
	e            ExecEngine
	m            metrics.Metrics
	log          log.Logger
}

func NewPreconfState(ctx context.Context, e ExecEngine, m metrics.Metrics, log log.Logger) PreconfState {
	return PreconfState{
		pendingEnvs:  make(map[uint64]eth.SignedEnv),
		pendingFrags: make(map[FragIndex]eth.SignedNewFrag),
		pendingSeals: make(map[uint64]eth.SignedSeal),

		lastBlockWithAllFrags: None[uint64](),
		lastEnvSent:           None[uint64](),
		lastFragSent:          None[FragIndex](),
		lastSealSent:          None[uint64](),
		lastL2BlockSent:       None[uint64](),

		lastBlockPruned: 0,
		ctx:             ctx,
		e:               e,
		m:               m,
		log:             log,
	}
}

// Builds the preconf channels and starts a concurrent preconf handler in a separate goroutine.
func StartPreconf(ctx context.Context, e ExecEngine, m metrics.Metrics, log log.Logger) PreconfChannels {
	channels := NewPreconfChannels()
	go preconfHandler(ctx, channels, e, m, log)
	return channels
}

// Checks if the state is new or if the previous block is sealed.
func (s *PreconfState) putEnv(sEnv *eth.SignedEnv) {
	env := sEnv.Env
	if s.lastL2BlockSent.IsEqual(env.Number - 1) {
		s.lastEnvSent.Set(env.Number)
		s.e.Env(s.ctx, sEnv)
		s.prune(env.Number)

		// When an env is sent we should check if we have the first frag of the block and put it.
		nextIndex := FragIndex{BlockNumber: env.Number, Sequence: 0}
		nextFrag, ok := s.pendingFrags[nextIndex]
		if ok {
			delete(s.pendingFrags, nextIndex)
			s.putFrag(&nextFrag)
		}
	} else if env.Number >= s.lastBlockPruned {
		s.pendingEnvs[env.Number] = *sEnv
	}
}

// Checks if the frag is the first of the block and the env is present,
// or if the previous frag is sent.
func (s *PreconfState) putFrag(sFrag *eth.SignedNewFrag) {
	frag := sFrag.Frag
	idx := index(frag)
	isFirst := frag.Seq == 0 && s.lastEnvSent.IsEqual(frag.BlockNumber)
	previousSent := s.lastFragSent.IsEqual(idx.prev())

	s.log.Debug("putFrag check",
		"block", frag.BlockNumber,
		"seq", frag.Seq,
		"isFirst", isFirst,
		"previousSent", previousSent,
		"lastEnvSent", s.lastEnvSent.IsSet(),
		"lastFragSent", s.lastFragSent.IsSet(),
		"pendingFrags", len(s.pendingFrags))

	if isFirst || previousSent {
		s.lastFragSent.Set(idx)
		s.log.Info("Sending frag to engine", "block", frag.BlockNumber, "seq", frag.Seq)
		_, err := s.e.NewFrag(s.ctx, sFrag)
		if err != nil {
			s.log.Error("Failed to send frag to engine", "block", frag.BlockNumber, "seq", frag.Seq, "err", err)
		} else {
			s.log.Info("Successfully sent frag to engine", "block", frag.BlockNumber, "seq", frag.Seq)
		}
		s.m.BasedNewFrag.Inc()

		// When a frag is sent we should check if the next is present or if the seal is present
		if frag.IsLast {
			s.lastBlockWithAllFrags.Set(idx.BlockNumber)
			s.m.BasedLastBlockWithAllFrags.Set(float64(idx.BlockNumber))
			nextSeal, ok := s.pendingSeals[idx.BlockNumber]
			if ok {
				delete(s.pendingSeals, idx.BlockNumber)
				s.putSeal(&nextSeal)
			}
		} else {
			nextFrag, ok := s.pendingFrags[idx.next()]
			if ok {
				delete(s.pendingFrags, idx.next())
				s.putFrag(&nextFrag)
			}
		}
	} else if idx.BlockNumber >= s.lastBlockPruned {
		s.log.Warn("Frag held as pending - conditions not met",
			"block", frag.BlockNumber,
			"seq", frag.Seq,
			"needsEnv", frag.Seq == 0 && !s.lastEnvSent.IsEqual(frag.BlockNumber),
			"needsPrevFrag", frag.Seq > 0 && !s.lastFragSent.IsEqual(idx.prev()))
		s.pendingFrags[idx] = *sFrag
	} else {
		s.log.Warn("Frag discarded - block already pruned", "block", frag.BlockNumber, "seq", frag.Seq, "lastPruned", s.lastBlockPruned)
	}
}

// Checks if the last frag of the block is sent.
func (s *PreconfState) putSeal(sSeal *eth.SignedSeal) {
	seal := sSeal.Seal
	if s.lastBlockWithAllFrags.IsEqual(seal.BlockNumber) {
		s.lastSealSent.Set(seal.BlockNumber)
		s.e.SealFrag(s.ctx, sSeal)
		s.m.BasedSealFrag.Inc()

		// When we put a seal we should check if the env of the next is present.
		nextEnv, ok := s.pendingEnvs[seal.BlockNumber+1]
		if ok {
			delete(s.pendingEnvs, seal.BlockNumber+1)
			s.putEnv(&nextEnv)
		}
	} else if seal.BlockNumber >= s.lastBlockPruned {
		s.pendingSeals[seal.BlockNumber] = *sSeal
	}
}

// Checks if there's envs blocked because of gaps and sends them over.
func (s *PreconfState) putL2Block(block *eth.L2BlockRef) {
	s.lastL2BlockSent.Set(block.Number)
	nextEnv, ok := s.pendingEnvs[block.Number+1]
	if ok {
		delete(s.pendingEnvs, block.Number+1)
		s.putEnv(&nextEnv)
	}

	s.prune(block.Number)
}

// The amount of blocks we don't prune back from the current block.
const PruneSafeWindow = 512

func (s *PreconfState) prune(currentBlock uint64) {
	// We only prune if there's at least a full window of events to prune.
	if currentBlock-s.lastBlockPruned < 2*PruneSafeWindow {
		return
	}

	latestBlock := currentBlock - PruneSafeWindow

	for key := range s.pendingEnvs {
		if key < latestBlock {
			delete(s.pendingEnvs, key)
		}
	}

	for key := range s.pendingSeals {
		if key < latestBlock {
			delete(s.pendingSeals, key)
		}
	}

	for key := range s.pendingFrags {
		if key.BlockNumber < latestBlock {
			delete(s.pendingFrags, key)
		}
	}

	s.lastBlockPruned = latestBlock
}

// Listens for env, frag and seal events and updates the local state.
// If the events are ready, they are sent to the engine api. If not, they are
// saved in the local state as pending until they are.
func preconfHandler(ctx context.Context, c PreconfChannels, e ExecEngine, m metrics.Metrics, log log.Logger) {
	state := NewPreconfState(ctx, e, m, log)
	log.Info("Preconf handler started")

	for {
		select {
		case env := <-c.EnvCh:
			log.Info("Preconf handler received env", "block", env.Env.Number)
			state.putEnv(env)
		case frag := <-c.NewFragCh:
			log.Info("Preconf handler received frag", "block", frag.Frag.BlockNumber, "seq", frag.Frag.Seq)
			state.putFrag(frag)
		case seal := <-c.SealCh:
			log.Info("Preconf handler received seal", "block", seal.Seal.BlockNumber)
			state.putSeal(seal)
		case l2Block := <-c.l2BlockCh:
			log.Info("Preconf handler received L2 block", "block", l2Block.Number)
			state.putL2Block(l2Block)
		case <-ctx.Done():
			log.Warn("Preconf handler context cancelled, exiting")
			return
		}
	}
}
