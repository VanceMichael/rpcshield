package srta_kumo_random

import (
	"os"
	"testing"

	"github.com/uber-research/rpcshield/go/rta/rtatest"
)

func TestMain(m *testing.M) {
	rtatest.SetupGoSDK()
	os.Exit(m.Run())
}

func TestCorrectness(t *testing.T) {
	rtatest.AssertMatchesStdlib(t, New())
}
