package dsl

import "github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"

type L2Network struct {
	common
	net stack.Network
}

func newL2Network(c common, net stack.Network) *L2Network {
	return &L2Network{
		common: c,
		net:    net,
	}
}

func (n *L2Network) NewWallet() *Wallet {
	user := n.net.Faucet().NewUser()
	return newWallet(commonWithLog(n.common, n.common.log.New("id", user.ID(), "address", user.Address())), user)
}
