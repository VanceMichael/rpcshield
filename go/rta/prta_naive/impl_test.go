package prta_naive

import (
	"os"
	"testing"

	rtalib "github.com/uber-research/rpcshield/go/rta"
	"github.com/uber-research/rpcshield/go/rta/rtatest"
)

func TestMain(m *testing.M) {
	rtatest.SetupGoSDK()
	os.Exit(m.Run())
}

func TestCorrectness(t *testing.T) {
	rtatest.AssertMatchesStdlib(t, New(), rtalib.WithNumWorkers(4))
}
