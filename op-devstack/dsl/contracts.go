package dsl

import (
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Contracts struct {
	t devtest.T
}

func (c *Contracts) require() *require.Assertions {
	return c.t.Require()
}

func (c *Contracts) ERC20(addr common.Address) *ERC20 {
	b, err := script.MakeBindings[ERC20](c.bindTo(addr), nil)
	c.require().NoError(err)
	return b
}

func (c *Contracts) WETH() *WETH {
	b, err := script.MakeBindings[WETH](c.bindTo(constants.WETH), nil)
	c.require().NoError(err)
	return b
}

type BindingConfig struct {
}

type BindingOpt func(cfg *BindingConfig)

func MakeCallBindings[B any](opts ...BindingOpt) *B {
	// TODO reflection like script.MakeBindings to create the functions that instantiate a CallAction
	return new(B)
}

type CallAction[V any] struct {
	addr *common.Address // optionally bind calls to a contract
}

func (a *CallAction[V]) View() V {
	var out V
	// TODO
	return out
}

func (a *CallAction[V]) Plan() txplan.Option {

}

func (a *CallAction[V]) Input() ([]byte, error) {

}

// On the ELNode add a View() function that takes an interface{ Input() ([]byte, error) }

type ERC20 struct {
	BalanceOf func(addr common.Address) eth.ETH
}

func (e *ERC20) Transfer(to common.Address, amount eth.ETH) txplan.Option {
	return nil // TODO abi encode
}

type WETH struct {
	ERC20
}

type L2ToL2CrossDomainMessenger struct {
	SendMessage func(
		chainID eth.ChainID,
		target common.Address,
		message types.Identifier) *CallAction[struct{}]

	RelayMessage func(id types.Identifier, msg []byte) *CallAction[struct{}]
}
