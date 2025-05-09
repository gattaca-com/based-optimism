package opcm

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type DeployMIPSInput struct {
	PreimageOracle common.Address
	MipsVersion    *big.Int
}

type DeployMIPSOutput struct {
	MipsSingleton common.Address
}

type DeployMIPSScript script.DeployScriptWithOutput[DeployMIPSInput, DeployMIPSOutput]

// NewDeployMIPSScript loads and validates the DeployMIPS script contract
func NewDeployMIPSScript(host *script.Host) (DeployMIPSScript, error) {
	return script.NewDeployScriptWithOutputFromFile[DeployMIPSInput, DeployMIPSOutput](host, "DeployMIPS.s.sol", "DeployMIPS")
}
