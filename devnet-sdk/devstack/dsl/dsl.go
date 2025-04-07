package dsl

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
)

const defaultTimeout = 30 * time.Second

// common provides a set of common values and methods inherited by all DSL structs.
// These should be kept very minimal.
// No public methods or fields should be exposed.
type common struct {
	// Ctx is the context for test execution.
	ctx context.Context
	// log is the component-specific logger instance.
	log log.Logger
	// T is a minimal test interface for panic-checks / assertions.
	t devtest.T
	// Require is a helper around the above T, ready to assert against.
	require *require.Assertions
}

// commonWithLog copies the specified common, replacing the log instance.
// Not an instance method on common to avoid it being inherited to every component that uses common.
func commonWithLog(c common, log log.Logger) common {
	return common{
		ctx:     c.ctx,
		log:     log,
		t:       c.t,
		require: c.require,
	}
}

type System struct {
	common
	log log.Logger
	sys stack.System
}

func (s *System) Supervisor(id stack.SupervisorID) *Supervisor {
	super := s.sys.Supervisor(id)
	return newSupervisor(commonWithLog(s.common, s.log.New("id", id)), super)
}

func (s *System) ClusteredL2Networks(predicate func(cluster stack.Cluster) bool) []*L2Network {
	for _, clusterID := range s.sys.Clusters() {
		cluster := s.sys.Cluster(clusterID)
		if !predicate(cluster) {
			continue
		}
		chainIDs := cluster.DependencySet().Chains()
		l2Networks := make([]*L2Network, len(chainIDs))
		for i, chainID := range chainIDs {
			l2NetworkID := s.sys.L2NetworkID(chainID)
			l2Networks[i] = s.L2Network(l2NetworkID)
		}
		return l2Networks
	}
	s.require.Fail("No suitable cluster found")
	return nil
}

func (s *System) L2Network(id stack.L2NetworkID) *L2Network {
	network := s.sys.L2Network(id)
	return newL2Network(s.common, network)
}

func Hydrate(t devtest.T, sys stack.System) *System {
	return &System{
		common: common{
			ctx:     t.Ctx(),
			log:     t.Logger(),
			t:       t,
			require: t.Require(),
		},
		log: t.Logger(),
		sys: sys,
	}
}

func applyOpts[Config any](defaultConfig Config, opts ...func(config *Config)) Config {
	for _, opt := range opts {
		opt(&defaultConfig)
	}
	return defaultConfig
}
