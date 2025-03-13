package contracts

import (
	_ "embed"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed abis/FaultDisputeGame-0.18.1.json
var faultDisputeGameAbi0180 []byte

func l2BlockNumberChallengeUnsupported(ops *faultDisputeGameOps) {
	ops.GetL2BlockNumberChallenged = func(contract *batching.BoundContract) GetterContractOp[bool] {
		return NewStaticOp(false)
	}
	ops.GetL2BlockNumberChallenger = func(contract *batching.BoundContract) GetterContractOp[common.Address] {
		return NewStaticOp(common.Address{})
	}
	ops.ChallengeL2BlockNumberTx = func(_ *batching.BoundContract, _ *types.InvalidL2BlockNumberChallenge) (*batching.ContractCall, error) {
		return nil, ErrChallengeL2BlockNotSupported
	}
}
