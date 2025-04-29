package txintent

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

type Call interface {
	To() (*common.Address, error)
	Data() ([]byte, error)
	AccessList() (types.AccessList, error)
}

type Result interface {
	FromReceipt(ctx context.Context, rec *types.Receipt, includedIn eth.BlockRef, chainID eth.ChainID) error
	Init() Result
}

type IntentTx[V Call, R Result] struct {
	PlannedTx *txplan.PlannedTx
	Content   plan.Lazy[V]
	Result    plan.Lazy[R]
}

// WithCall creates a txplan.Option that makes the
// tx inputs depend on the given lazy-loaded call value.
func WithCall[V Call](v *plan.Lazy[V]) txplan.Option {
	return func(tx *txplan.PlannedTx) {
		tx.To.DependOn(v)
		tx.To.Fn(func(ctx context.Context) (*common.Address, error) {
			return v.Value().To()
		})
		tx.Data.DependOn(v)
		tx.Data.Fn(func(ctx context.Context) (hexutil.Bytes, error) {
			return v.Value().Data()
		})
		tx.AccessList.DependOn(v)
		tx.AccessList.Fn(func(ctx context.Context) (types.AccessList, error) {
			return v.Value().AccessList()
		})
	}
}

// WithResult creates a txplan.Option that makes the
// given result value depend on the result of the tx.
func WithResult[R Result](v *plan.Lazy[R]) txplan.Option {
	return func(tx *txplan.PlannedTx) {
		v.DependOn(&tx.Included, &tx.IncludedBlock, &tx.ChainID)
		v.Fn(func(ctx context.Context) (R, error) {
			r := (*new(R)).Init().(R)
			err := r.FromReceipt(ctx, tx.Included.Value(), tx.IncludedBlock.Value(), tx.ChainID.Value())
			return r, err
		})
	}
}

func NewIntent[V Call, R Result](opts ...txplan.Option) *IntentTx[V, R] {
	v := new(IntentTx[V, R])
	v.PlannedTx = txplan.NewPlannedTx(
		txplan.Combine(
			txplan.Combine(opts...),
			WithCall(&v.Content),
			WithResult(&v.Result),
		),
	)
	return v
}
