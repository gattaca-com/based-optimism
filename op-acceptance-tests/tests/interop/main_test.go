package interop

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/presets"
)

var SimpleInterop presets.TestSetup[*presets.SimpleInterop]

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.NewSimpleInterop(&SimpleInterop))
}
