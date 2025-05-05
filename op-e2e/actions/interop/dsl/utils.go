package dsl

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssertAncestorDescentantRelationship(t require.TestingT, chain *Chain, ancestor, descendant eth.BlockID) bool {
	assert.GreaterOrEqual(t, descendant.Number, ancestor.Number, "descendant block has a lower number than ancestor block")

	current := descendant
	result := true
	for current.Number > ancestor.Number && current.Number > 0 {
		// TODO: Update typing of t to use test-level context
		header, err := chain.SequencerEngine.Eth.APIBackend.HeaderByNumber(context.Background(), rpc.BlockNumber(current.Number-1))
		result = result && assert.NoError(t, err)
		current = eth.BlockID{Hash: header.Hash(), Number: header.Number.Uint64()}
	}
	return result && assert.Equal(t, current, ancestor, "descendant block is not a descendant of the ancestor block")
}
