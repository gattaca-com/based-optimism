package opcm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNewDeployProxyScript(t *testing.T) {
	t.Run("should not fail with current version of DeployProxy contract", func(t *testing.T) {
		// First we grab a test host
		host1 := createTestHost(t)

		// Then we load the script
		//
		// This would raise an error if the Go types didn't match the ABI
		deployProxy, err := NewDeployProxyScript(host1)
		require.NoError(t, err)

		// Then we deploy
		output, err := deployProxy.Run(DeployProxyInput{
			Owner: common.Address{'O'},
		})

		// And do some simple asserts
		require.NoError(t, err)
		require.NotNil(t, output)
		require.NotEqual(t, output.Proxy, common.Address{})
	})
}
