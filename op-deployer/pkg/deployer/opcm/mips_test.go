package opcm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
)

func TestNewDeployMIPSScript(t *testing.T) {
	t.Run("should not fail with current version of DeployMIPS contract", func(t *testing.T) {
		// First we grab a test host
		host1 := createTestHost(t)

		// Then we load the script
		//
		// This would raise an error if the Go types didn't match the ABI
		deployScript, err := NewDeployMIPSScript(host1)
		require.NoError(t, err)

		// Then we deploy
		mipsVersion := int64(standard.MIPSVersion)
		output, err := deployScript.Run(DeployMIPSInput{
			PreimageOracle: common.Address{'P'},
			MipsVersion:    big.NewInt(mipsVersion),
		})

		// And do some simple asserts
		require.NoError(t, err)
		require.NotNil(t, output)
		require.NotEqual(t, common.Address{}, output.MipsSingleton, "MIPS singleton address should not be zero")
	})
}
