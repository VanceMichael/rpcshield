package prta_kumo

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
	t.Run("RarestMethodByConcreteType", func(t *testing.T) {
		rtatest.AssertMatchesStdlib(t, New(), rtalib.WithNumWorkers(4))
	})
	t.Run("RarestMethodByInterfaceType", func(t *testing.T) {
		rtatest.AssertMatchesStdlib(t, New(), rtalib.WithNumWorkers(4), rtalib.WithMethodSelectionStrategy(rtalib.RarestMethodByInterfaceType))
	})
	t.Run("RandomMethodStrategy", func(t *testing.T) {
		rtatest.AssertMatchesStdlib(t, New(), rtalib.WithNumWorkers(4), rtalib.WithMethodSelectionStrategy(rtalib.RandomMethodStrategy))
	})
}

func TestResultMethods(t *testing.T) {
	_, entry := rtatest.LoadProgramAndRoots(t)
	result := New().Analyze(entry, true, rtalib.WithNumWorkers(4)).(*Result)

	_ = result.MCBucketSizePercentiles()
	_ = result.MIBucketSizePercentiles()
	_ = result.InterfaceMethodCountBuckets()
}
