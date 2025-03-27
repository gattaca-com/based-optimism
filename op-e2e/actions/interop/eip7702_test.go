package interop

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/emit"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/wallet"
)

func createAuthorization(t helpers.Testing, key *ecdsa.PrivateKey, chainID *big.Int, codeAddr common.Address) types.SetCodeAuthorization {
	auth := types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: codeAddr,
		Nonce:   0,
	}
	signedAuth, err := types.SignSetCode(key, auth)
	require.NoError(t, err)

	return signedAuth
}

func createSetCodeTx(t helpers.Testing, auth types.SetCodeAuthorization, user *userWithKeys, chainID *big.Int, code []byte, nonce uint64) *types.Transaction {
	tx := types.NewTx(&types.SetCodeTx{
		ChainID:    uint256.MustFromBig(chainID),
		Nonce:      nonce,
		GasTipCap:  uint256.MustFromBig(big.NewInt(params.GWei)),
		GasFeeCap:  uint256.MustFromBig(big.NewInt(2 * params.GWei)),
		Gas:        100000,
		To:         user.address,
		Value:      uint256.NewInt(0),
		Data:       code,
		AccessList: nil,
		AuthList:   []types.SetCodeAuthorization{auth},
	})

	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, signer, user.secret)
	require.NoError(t, err)
	return signedTx
}

// Test7702WalletDeployment tests basic wallet deployment for EIP-7702
func TestWalletDeployment(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)
	aliceA := setupUser(t, is, actors.ChainA, 0)
	aliceB := setupUser(t, is, actors.ChainB, 0)

	// Deploy wallet on Chain A
	authA, err := bind.NewKeyedTransactorWithChainID(aliceA.secret, actors.ChainA.RollupCfg.L2ChainID)
	require.NoError(t, err)
	authA.GasTipCap = big.NewInt(params.GWei)
	authA.GasLimit = 3000000
	walletAddrA, txA, _, err := wallet.DeployWallet(authA, actors.ChainA.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainA, txA, aliceA.address)

	// Deploy wallet on Chain B
	authB, err := bind.NewKeyedTransactorWithChainID(aliceB.secret, actors.ChainB.RollupCfg.L2ChainID)
	require.NoError(t, err)
	authB.GasTipCap = big.NewInt(params.GWei)
	authB.GasLimit = 3000000
	walletAddrB, txB, _, err := wallet.DeployWallet(authB, actors.ChainB.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainB, txB, aliceB.address)

	// Verify contract code exists and matches on both chains
	codeA, err := actors.ChainA.SequencerEngine.EthClient().CodeAt(t.Ctx(), walletAddrA, nil)
	require.NoError(t, err)
	require.NotEmpty(t, codeA, "Contract code should exist at deployed address on Chain A")

	codeB, err := actors.ChainB.SequencerEngine.EthClient().CodeAt(t.Ctx(), walletAddrB, nil)
	require.NoError(t, err)
	require.NotEmpty(t, codeB, "Contract code should exist at deployed address on Chain B")

	require.Equal(t, codeA, codeB, "Contract code should be identical on both chains")
	require.Equal(t, walletAddrA, walletAddrB, "Contract should be deployed at the same address on both chains")

}

// Test7702SetCode demonstrates setting code on an EOA using EIP-7702 and then using the emitter contract to emit a log
func Test7702SetCode(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	alice := setupUser(t, is, actors.ChainA, 0)
	t.Log("Alice's address:", alice.address.Hex())

	chainID := actors.ChainA.RollupCfg.L2ChainID

	// Deploy an emitter contract and get its code
	auth, err := bind.NewKeyedTransactorWithChainID(alice.secret, chainID)
	require.NoError(t, err)
	auth.GasTipCap = big.NewInt(params.GWei)
	auth.GasLimit = 3000000
	emitterAddr, tx, _, err := emit.DeployEmit(auth, actors.ChainA.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainA, tx, alice.address)

	// Create authorization for setting the emitter code
	// auth7702 := createAuthorization(t, alice.secret, chainID, emitterAddr)
	setCodeAuth := types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: emitterAddr,
		Nonce:   1,
	}
	signedAuth, err := types.SignSetCode(alice.secret, setCodeAuth)
	require.NoError(t, err)

	// Create the calldata for emitData function
	emitDataCallData := common.FromHex("0xd836083e") // Function selector for emitData
	emitDataCallData = append(emitDataCallData, common.LeftPadBytes([]byte("test data"), 32)...)

	// Create a set-code transaction that will also execute the emitData function
	setCodeTx := &types.SetCodeTx{
		ChainID:    uint256.MustFromBig(chainID),
		Nonce:      1,
		GasTipCap:  uint256.MustFromBig(big.NewInt(params.GWei)),
		GasFeeCap:  uint256.MustFromBig(big.NewInt(2 * params.GWei)),
		Gas:        100000,
		To:         alice.address,
		Value:      uint256.NewInt(0),
		Data:       emitDataCallData,
		AccessList: nil,
		AuthList:   []types.SetCodeAuthorization{signedAuth},
	}
	signedTx := types.NewTx(setCodeTx)
	signedTx, err = types.SignTx(signedTx, types.LatestSignerForChainID(chainID), alice.secret)
	require.NoError(t, err)

	// Include the transaction and process it
	includeTxOnChain(t, actors, actors.ChainA, signedTx, alice.address)

	// Verify the transaction succeeded and emitted an event
	receipt, err := actors.ChainA.SequencerEngine.EthClient().TransactionReceipt(t.Ctx(), signedTx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "Transaction should succeed")
	require.Len(t, receipt.Logs, 1, "Should have emitted one event") // TODO: Fails here. If alice's account now executes the emitter code then why no log?

	// Log the receipt in detail
	t.Log("Emitter receipt:", receipt)
	t.Log("Emitter receipt gas used:", receipt.GasUsed)
}
