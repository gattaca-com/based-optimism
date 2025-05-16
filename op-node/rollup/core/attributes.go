package core

import "github.com/ethereum-optimism/optimism/op-node/rollup/attributes"

func SetupAttributesHandler() {
	sys.Register("attributes-handler",
		attributes.NewAttributesHandler(log, cfg, driverCtx, l2))
}
