package interop

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

func TestWrapETH(gt *testing.T) {
	t := devtest.ParallelT(gt)
	sys := SimpleInterop(t)
	require := t.Require()

	funds := eth.Ether(2)

	user := sys.FunderA.NewFundedEOA(funds)

	wethAddr := constants.WETH
	user.Transfer(wethAddr, eth.Ether(1))

	user.Transact(weth.Transfer(bob, eth.Ether(1234)))
	initialBalance := user.View(weth.BalanceOf(bob))
	user.ExpectRevert(weth.BalanceOf())

	// TODO WETH
	weth, err := chain.Nodes()[0].ContractsRegistry().WETH(wethAddr)
	require.NoError(err)

	initialBalance := weth.BalanceOf(user.Address()).Call(user.Plan())
	weth.SendETH(wethAddr, funds).Send(user.Plan())

	initialBalance, err := weth.BalanceOf(user.Address()).Call(ctx)
	require.NoError(err)

	logger := t.Logger().With("user", user.Address())
	logger.Info("initial balance retrieved", "balance", initialBalance)

	logger.Info("sending ETH to contract", "amount", funds)
	require.NoError(user.SendETH(wethAddr, funds).Send(ctx).Wait())

	balance, err := weth.BalanceOf(user.Address()).Call(ctx)
	require.NoError(err)
	logger.Info("final balance retrieved", "balance", balance)

	require.Equal(initialBalance.Add(funds), balance)
}

func TestFinalizing(gt *testing.T) {
	t := devtest.ParallelT(gt)
	sys := SimpleInterop(t)

	sys.Supervisor.VerifySyncStatus(
	// TODO option to verify L1 and L2 finalized new blocks
	)
	t.Logger().Info("finalized new blocks")
}
