package dsl

import "github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"

// Wallet represents a
type Wallet struct {
	common
	user stack.User
}

func newWallet(c common, user stack.User) *Wallet {
	return &Wallet{
		common: c,
		user:   user,
	}
}
