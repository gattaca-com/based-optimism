package contracts

import (
	_ "embed"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed abis/FaultDisputeGame-1.1.1.json
var faultDisputeGameAbi111 []byte

func moveWithoutParentClaim(ops *faultDisputeGameOps) {
	ops.AttackTx = func(contract *batching.BoundContract, parent types.Claim, pivot common.Hash) (*batching.ContractCall, error) {
		return contract.Call(methodAttack, big.NewInt(int64(parent.ContractIndex)), pivot), nil
	}
	ops.DefendTx = func(contract *batching.BoundContract, parent types.Claim, pivot common.Hash) (*batching.ContractCall, error) {
		return contract.Call(methodDefend, big.NewInt(int64(parent.ContractIndex)), pivot), nil
	}
}
