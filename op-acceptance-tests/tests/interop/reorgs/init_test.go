package reorgs

import (
	"os"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/presets"
)

// TestMain creates the test-setups against the shared backend
func TestMain(m *testing.M) {
	// Check if -list is among the arguments
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.list=") || strings.HasPrefix(arg, "-list=") {
			// Don't do anything extra — just run and exit
			os.Exit(m.Run())
		}
	}

	// Other setups may be added here, hydrated from the same orchestrator
	presets.DoMain(m, presets.WithSimpleInterop())
}
