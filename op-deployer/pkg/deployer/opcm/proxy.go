package opcm

import (
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type DeployProxyInput struct {
	Owner common.Address
}

type DeployProxyOutput struct {
	Proxy common.Address
}

type DeployProxyScript script.DeployScriptWithOutput[DeployProxyInput, DeployProxyOutput]

// NewDeployProxyScript loads and validates the DeployProxy script contract
func NewDeployProxyScript(host *script.Host) (DeployProxyScript, error) {
	return script.NewDeployScriptWithOutputFromFile[DeployProxyInput, DeployProxyOutput](host, "DeployProxy.s.sol", "DeployProxy")
}
