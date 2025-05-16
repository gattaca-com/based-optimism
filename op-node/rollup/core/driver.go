package core

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
)

type Core struct {
	emitter event.Emitter
}

func (c *Core) AttachEmitter(em event.Emitter) {
	c.emitter = em
}

var _ event.AttachEmitter = (*Core)(nil)

type Drain interface {
	Drain() error
	Await() <-chan struct{}
}

type mainLoop struct {
	drain Drain
}

func (s *mainLoop) eventLoop() {
	for {
		switch {
		case <-s.drain.Await():
			if err := s.drain.Drain(); err != nil {
				if s.driverCtx.Err() != nil {
					return
				} else {
					s.log.Error("unexpected error from event-draining", "err", err)
					s.Emitter.Emit(rollup.CriticalErrorEvent{Err: fmt.Errorf("unexpected error: %w", err)})
				}
			}
		case <-s.driverCtx.Done():
			return
		}
	}
}

func (s *mainLoop) Close() error {
	s.driverCancel()
	s.wg.Wait()
	return nil
}
