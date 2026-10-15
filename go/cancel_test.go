package rpcshield

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/pkgdeps"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	"github.com/uber-research/rpcshield/go/genai"
	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	genaimock "mock/github.com/uber-research/rpcshield/go/genai/genaimock"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

const cancelTestService = "cancel-svc"

func newCancelTestDriver(t *testing.T, aiClient genai.Client, opts DriverOptions) *Driver {
	t.Helper()
	serviceMapper := servicemapping.New([]grail.Service{{Name: cancelTestService, Tier: 2}}, nil, zap.NewNop().Sugar())
	return NewDriver("", nil, nil, serviceMapper, aiClient,
		[]grail.Service{{Name: cancelTestService, Tier: 2}}, nil, zap.NewNop().Sugar(), opts)
}

// sentinelArtifacts writes "trusted" result.json/CSV/HTML content for the
// service into dir. A canceled run must leave them byte-for-byte untouched.
func sentinelArtifacts(t *testing.T, dir string) []string {
	t.Helper()
	names := []string{
		cancelTestService + "_result.json",
		cancelTestService + ".csv",
		cancelTestService + ".html",
	}
	var paths []string
	for i, name := range names {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte("trusted complete artifact "+string(rune('1'+i))), 0644))
		paths = append(paths, p)
	}
	return paths
}

func assertArtifactsUntouched(t *testing.T, paths []string) {
	t.Helper()
	for i, p := range paths {
		got, err := os.ReadFile(p)
		require.NoError(t, err, "trusted artifact must still exist: %s", p)
		require.Equal(t, "trusted complete artifact "+string(rune('1'+i)), string(got),
			"trusted artifact was overwritten: %s", p)
	}
}

func assertNoTempArtifacts(t *testing.T, dir string) {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	require.NoError(t, err)
	require.Empty(t, left, "temporary artifacts left behind: %v", left)
}

// TestRun_PreCanceledRejectsAllTargets: with the context already canceled,
// Run must not accept any target at all (PackageSource never invoked), must
// return the recognizable ErrCanceled wrapping context.Canceled, and must
// preserve the previously generated trusted artifacts.
func TestRun_PreCanceledRejectsAllTargets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sentinels := sentinelArtifacts(t, dir)

	packageSourceCalled := false
	d := newCancelTestDriver(t, nil, DriverOptions{
		OutputDir: dir,
		Targets:   []string{"//src/some/target:binary", "//src/other/target:binary2"},
		PackageSource: func(string) ([]*packages.Package, []*ssa.Package, *ssa.Program, error) {
			packageSourceCalled = true
			t.Error("PackageSource must not be called after cancellation")
			return nil, nil, nil, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := d.Run(ctx, cancelTestService)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCanceled), "expected ErrCanceled, got %v", err)
	require.True(t, errors.Is(err, context.Canceled), "expected wrapped context.Canceled, got %v", err)
	require.Equal(t, Error, result.Status, "a canceled run must never be reported as success")
	require.NotEmpty(t, result.ErrorMsg)
	require.False(t, packageSourceCalled)

	assertArtifactsUntouched(t, sentinels)
	assertNoTempArtifacts(t, dir)
}

// TestRun_CanceledDuringTargetLoading deterministically cancels while target
// loading is in flight: the PackageSource blocks on a gate until ctx is
// canceled. Run must wait for the loading call to return and then surface
// ErrCanceled -- proving in-flight stages converge and no further targets run.
func TestRun_CanceledDuringTargetLoading(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sentinels := sentinelArtifacts(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entered := make(chan struct{})
	d := newCancelTestDriver(t, nil, DriverOptions{
		OutputDir: dir,
		Targets:   []string{"//src/some/target:binary"},
		PackageSource: func(string) ([]*packages.Package, []*ssa.Package, *ssa.Program, error) {
			close(entered)
			<-ctx.Done() // simulate a long target load; return only once canceled
			return nil, nil, nil, ctx.Err()
		},
	})

	runDone := make(chan struct{})
	var result Result
	var err error
	go func() {
		result, err = d.Run(ctx, cancelTestService)
		close(runDone)
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("target loading never started")
	}
	cancel()

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not converge after canceling an in-flight target load")
	}
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCanceled))
	require.True(t, errors.Is(err, context.Canceled))
	require.Equal(t, Error, result.Status)

	assertArtifactsUntouched(t, sentinels)
	assertNoTempArtifacts(t, dir)
}

// TestRunLLMAnalyzer_CanceledMidRequest proves the "LLM request in flight"
// cancellation scenario at the driver/genai seam: the chat completion call
// blocks until the caller cancels ctx, then returns the context error --
// exactly what the openai HTTP client does when its request is aborted.
// runLLMAnalyzer must propagate an error satisfying errors.Is(context.Canceled),
// which analyzeEdge/analyzeCallPaths use to abort the whole run instead of
// recording a FailUnknownWithError edge diagnostic.
func TestRunLLMAnalyzer_CanceledMidRequest(t *testing.T) {
	t.Parallel()

	// Write a tiny caller source file so genai's func-source dump succeeds.
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "f.go")
	src := "package x\n\nfunc F() {}\n"
	require.NoError(t, os.WriteFile(srcPath, []byte(src), 0644))

	ctrl := gomock.NewController(t)
	aiClient := genaimock.NewMockClient(ctrl)
	inFlight := make(chan struct{})
	aiClient.EXPECT().CreateChatCompletion(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			close(inFlight)
			<-ctx.Done() // request in flight until canceled, as the HTTP client does
			return openai.ChatCompletionResponse{}, ctx.Err()
		})

	d := newCancelTestDriver(t, aiClient, DriverOptions{
		Analyzers: map[constant.AnalyzerKind]bool{constant.LLM: true},
	})

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, src, parser.ParseComments)
	require.NoError(t, err)
	caller := f.Decls[0].(*ast.FuncDecl)
	callee := &ast.CallExpr{Fun: &ast.Ident{Name: "g"}}
	astCtx := &asthelper.ASTContext{
		PkgInfo: &pkgdeps.Info{Pkg: &packages.Package{Fset: fset}},
	}
	calleeObj := types.NewFunc(token.NoPos, nil, "g", nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan error, 1)
	go func() {
		_, e := d.runLLMAnalyzer(ctx, caller, callee, calleeObj, astCtx)
		out <- e
	}()

	select {
	case <-inFlight:
	case <-time.After(5 * time.Second):
		t.Fatal("LLM request never became in flight")
	}
	cancel()

	select {
	case err := <-out:
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled),
			"in-flight LLM cancellation must propagate as context.Canceled, got %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("runLLMAnalyzer did not return after canceling the in-flight request")
	}
}

// TestScheduleGoPathCleanup_KeepGoPathAfterRun verifies the preserved-tree
// policy: with KeepGoPathAfterRun the lifted directory must survive cleanup.
func TestScheduleGoPathCleanup_KeepGoPathAfterRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	marker := filepath.Join(dir, "lifted")
	require.NoError(t, os.WriteFile(marker, []byte("gopath"), 0644))

	d := newCancelTestDriver(t, nil, DriverOptions{KeepGoPathAfterRun: true})
	d.scheduleGoPathCleanup(dir)

	_, err := os.Stat(marker)
	require.NoError(t, err, "KeepGoPathAfterRun must preserve the lifted GOPATH tree")
}

// TestRemoveGoPathDetached verifies the actual reclamation primitive (used
// for every non-test, non-keep run, including runs aborted by cancellation):
// the detached rm removes the lifted tree shortly after the run exits.
func TestRemoveGoPathDetached(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b"), []byte("b"), 0644))

	require.NoError(t, removeGoPathDetached(dir))

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("lifted GOPATH %s was not reclaimed", dir)
}
