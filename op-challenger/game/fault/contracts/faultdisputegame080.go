package contracts

import (
	_ "embed"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
)

//go:embed abis/FaultDisputeGame-0.8.0.json
var faultDisputeGameAbi020 []byte

var resolvedBondAmount = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

var (
	methodGameDuration = "gameDuration"
)

func v080Compatibility(ops *faultDisputeGameOps) {
	moveWithoutParentClaim(ops)
	distributionModeUnsupported(ops)
	l2BlockNumberChallengeUnsupported(ops)
	// Replace GetClaim with a version that also restores the original bond amount even for resolved claims.
	ops.GetClaim = func(contract *batching.BoundContract, idx uint64) GetterContractOp[types.Claim] {
		getClaimOp := NewGetClaimOp(contract, idx)
		fixBondOp := &FixBondOp{getClaimOp}
		op := ChainOps(getClaimOp.Result, getClaimOp, fixBondOp)
		return op
	}
	ops.IsResolved = func(contract *batching.BoundContract, claims ...types.Claim) GetterContractOp[[]bool] {
		args := make([]interface{}, len(claims))
		for i, claim := range claims {
			args[i] = big.NewInt(int64(claim.ContractIndex))
		}
		return NewMultiGetterOp(contract, methodClaim, func(result *batching.CallResult, idx int) (bool, error) {
			claim := decodeClaim(result, uint64(idx))
			return claim.Bond.Cmp(resolvedBondAmount) == 0, nil
		}, args...)
	}
	ops.GetMaxClockDuration = func(contract *batching.BoundContract) GetterContractOp[uint64] {
		return NewSimpleGetterOp(contract, methodGameDuration, func(result *batching.CallResult) (uint64, error) {
			return result.GetUint64(0) / 2, nil
		})
	}
	ops.ResolveClaimTx = func(contract *batching.BoundContract, claimIdx uint64) (*batching.ContractCall, error) {
		return contract.Call(methodResolveClaim, new(big.Int).SetUint64(claimIdx)), nil
	}
}
