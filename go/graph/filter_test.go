package graph

import (
	"fmt"
	"go/types"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/mapper"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// loadAndInitBuilder loads a GOPATH-lifted fixture (via a "<pathSlice>-anal" go_path target
// rooted at the "rpcshield" Bazel workspace directory, mirroring rpcshieldtest/rta/rtatest's
// loading convention) and returns a populated *builderImpl for it. Used by this file's own
// tests and by cffenricher_test.go's table-driven tests, each of which loads its own small
// standalone fixture rather than the shared rpcshielddemo one.
func loadAndInitBuilder(t *testing.T, pathSlice []string, loadPath string) (*builderImpl, []*ssa.Package, *ssa.Program) {
	_, allPkgs, err := loader.LoadWithGoPath("rpcshield", pathSlice, loadPath)
	require.NoError(t, err)

	// Build SSA representation.
	prog, allSSAPkgs := ssautil.AllPackages(allPkgs, ssa.SanityCheckFunctions|ssa.GlobalDebug|ssa.InstantiateGenerics)
	prog.Build()
	l := zap.NewNop().Sugar()

	gBase := NewBuilder(l, allPkgs, prog, allSSAPkgs, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, DefaultSerialGraphConfig())
	g := gBase.(*builderImpl)

	g.populateNeededTypes(prog)
	return g, allSSAPkgs, prog
}

func methodCallHelper(t *testing.T, pathSlice []string, loadPath string, fn func(*builderImpl, ssa.CallInstruction) bool) {
	g, allSSAPkgs, _ := loadAndInitBuilder(t, pathSlice, loadPath)

	count := 0
	// Iterate over all functions in the main package.
	for _, pkg := range allSSAPkgs {
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		if pkg.Pkg.Path() == loadPath {
			for _, member := range pkg.Members {
				fmt.Println(member)
				if member, ok := member.(*ssa.Function); ok {
					isSuccessFunction := strings.HasPrefix(member.Name(), "success")
					isFailureFunction := strings.HasPrefix(member.Name(), "failure")
					require.False(t, isSuccessFunction && isFailureFunction, "Expected false for isSuccessFunction or isFailureFunction:"+member.Name())
					if !(isSuccessFunction || isFailureFunction) {
						fmt.Printf("skipping %s: isSuccessFunction = %v, isFailureFunction = %v\n", member.Name(), isSuccessFunction, isFailureFunction)
						continue
					}
					fmt.Println(member.Name())
					// Iterate over all basic blocks in the function.
					for _, block := range member.Blocks {
						// Iterate over all instructions in the basic block.
						for _, instr := range block.Instrs {
							if callIns, ok := instr.(ssa.CallInstruction); ok {
								count++

								v := fn(g, callIns)
								if isSuccessFunction {
									require.True(t, v, "Expected true for:"+member.Name()+" "+callIns.String())
								}
								if isFailureFunction {
									require.False(t, v, "Expected false for:"+member.Name()+" "+callIns.String())
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestErrorMethodCalls(t *testing.T) {
	t.Parallel()

	f := func(g *builderImpl, instr ssa.CallInstruction) bool {
		return g.isErrorMethodCall(instr)
	}
	methodCallHelper(t, []string{"exampleapps", "errormethodcalls", "errormethodcalls-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/errormethodcalls", f)
}

func TestDeferMethodCalls(t *testing.T) {
	t.Parallel()

	f := func(g *builderImpl, instr ssa.CallInstruction) bool {
		return g.isDeferCall(instr)
	}
	methodCallHelper(t, []string{"exampleapps", "defercalls", "defercalls-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/defercalls", f)
}

func TestLockerCalls(t *testing.T) {
	t.Parallel()

	f := func(g *builderImpl, instr ssa.CallInstruction) bool {
		return g.invokesLocker(instr)
	}
	methodCallHelper(t, []string{"exampleapps", "lockercalls", "lockercalls-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/lockercalls", f)
}

func TestContextCalls(t *testing.T) {
	t.Parallel()

	f := func(g *builderImpl, instr ssa.CallInstruction) bool {
		return g.invokesContext(instr)
	}
	methodCallHelper(t, []string{"exampleapps", "contextcalls", "contextcalls-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/contextcalls", f)
}

func TestContextAndErrorInStruct(t *testing.T) {
	t.Parallel()

	// Load program and builder for the example app
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/argumentpassing"
	_, _, _ = loadAndInitBuilder(t, []string{"exampleapps", "argumentpassing", "argumentpassing-anal"}, loadPath)

	resolveCallee := func(g *builderImpl, call ssa.CallInstruction) *ssa.Function {
		c := call.Common()
		if c == nil {
			return nil
		}
		// Direct calls where Value is a function or a closure
		if dc := c.StaticCallee(); dc != nil {
			if dc.Pkg != nil && dc.Pkg.Pkg != nil && dc.Pkg.Pkg.Path() == loadPath {
				return dc
			}
			return nil
		}
		if c.IsInvoke() && c.Value != nil {
			methodName := c.Method.Name()
			fmt.Println("methodName", methodName)
			for f, ok := range ssautil.AllFunctions(g.ssaProg) {
				if !ok || f == nil || f.Signature == nil || f.Name() != methodName {
					continue
				}
				if f.Pkg == nil || f.Pkg.Pkg == nil || f.Pkg.Pkg.Path() != loadPath {
					continue
				}
				recv := f.Signature.Recv()
				if recv == nil {
					continue
				}
				if types.AssignableTo(recv.Type(), c.Value.Type()) {
					fmt.Println("found callee", f.String())
					return f
				}
			}
		}
		return nil
	}

	f := func(g *builderImpl, instr ssa.CallInstruction) bool {
		callee := resolveCallee(g, instr)
		if callee == nil {
			return false
		}
		return g.sendsCtxtAndReturnErr(instr, callee)
	}
	methodCallHelper(t, []string{"exampleapps", "argumentpassing", "argumentpassing-anal"}, loadPath, f)
}

func TestGetBlockListPackages(t *testing.T) {
	t.Parallel()

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	allPkgs, prog, allSSAPkgs := testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs

	l := zap.NewNop().Sugar()

	gBase := NewBuilder(l, allPkgs, prog, allSSAPkgs, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, DefaultSerialGraphConfig())
	g := gBase.(*builderImpl)

	blockedPkgs, err := g.getBlockListPackages(prog.AllPackages())
	require.NoError(t, err)
	count := 0
	// Check that sync is not blocked.
	for _, pkg := range prog.AllPackages() {
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		switch pkg.Pkg.Path() {
		case "sync":
			count++
			require.False(t, blockedPkgs[pkg], "Expected false for sync")
		case "context":
			count++
			require.True(t, blockedPkgs[pkg], "Expected true for context")
		case "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler":
			count++
			require.False(t, blockedPkgs[pkg], "Expected false for rpcshielddemo/handler")
		case "go.uber.org/yarpc":
			// Substitute for the monorepo-internal-only "example.internal/go/envfx.git" blocklist
			// entry the original monorepo test checked (not present in this OSS fixture's
			// dependency tree) -- go.uber.org/yarpc is a real dependency of rpcshielddemo (via its
			// generated YARPC server/client code) and is blocked by the same pkgfilter.go list.
			count++
			require.True(t, blockedPkgs[pkg], "Expected true for go.uber.org/yarpc")
		}
	}
	require.Equal(t, 4, count)
}

func TestIsImpossibleInvoke(t *testing.T) {
	t.Parallel()

	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/invokecalls"
	g, _, prog := loadAndInitBuilder(t, []string{"exampleapps", "invokecalls", "invokecalls-anal"}, loadPath)
	var func1 *ssa.Function
	var func2 *ssa.Function
	var caller ssa.CallInstruction

	count := 0
	// Iterate over all functions in the main package.
	for f, ok := range ssautil.AllFunctions(prog) {
		if !ok || f == nil || f.Pkg == nil || f.Pkg.Pkg == nil {
			continue
		}
		if f.Pkg.Pkg.Path() == loadPath {
			switch f.Name() {
			case "Func1":
				func1 = f
			case "Func2":
				func2 = f
			case "caller":
				for _, block := range f.Blocks {
					for _, instr := range block.Instrs {
						if callIns, ok := instr.(ssa.CallInstruction); ok {
							count++
							caller = callIns
						}
					}
				}
			}
		}
	}

	require.NotNil(t, func1)
	require.NotNil(t, func2)
	require.NotNil(t, caller)
	require.False(t, g.isImpossibleInvoke(nil, func1))
	require.False(t, g.isImpossibleInvoke(caller, nil))
	// Test that calling Func1 from caller is not impossible.
	require.False(t, g.isImpossibleInvoke(caller, func1))
	// Test that calling Func2 from caller is impossible.
	require.True(t, g.isImpossibleInvoke(caller, func2))
}
