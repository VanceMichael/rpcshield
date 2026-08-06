package utils

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"mock/github.com/uber-research/rpcshield/go/common/utils/utilsmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"go.uber.org/mock/gomock"
	_ "golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

func TestIsGlueService(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create subdirectories that the function checks for
	requiredDirs := []string{".glue", "gateway", "handler", "controller", "repository"}
	for _, dir := range requiredDirs {
		err := os.MkdirAll(filepath.Join(tempDir, "src/example.internal/my/target", dir), 0755)
		require.NoError(t, err, "Failed to create required directory %s", dir)
	}

	// Define a valid target
	target := "//src/example.internal/my/target:target"

	// Call the function and assert the result
	assert.True(t, IsGlueService(tempDir, target), "Expected IsGlueService() to return true for valid target and directories")

	// Test with missing directories
	err := os.RemoveAll(filepath.Join(tempDir, "src/example.internal/my/target", ".glue"))
	require.NoError(t, err, "Failed to remove .glue directory")

	// Call the function again with a missing directory and assert the result
	assert.False(t, IsGlueService(tempDir, target), "Expected IsGlueService() to return false for missing .glue directory")

	// Test with an invalid target
	invalidTarget := "invalid/target"
	assert.False(t, IsGlueService(tempDir, invalidTarget), "Expected IsGlueService() to return false for invalid target format")
}
func TestGetPackagesDirectlyImportingPkgsByNameRegex(t *testing.T) {
	// Create a token.FileSet
	fset := token.NewFileSet()

	// Create the target types.Package (the one to match by name)
	targetTypesPkg := types.NewPackage("github.com/example/target", "targetpkg")

	// Create a parent package that imports the target package
	parentTypesPkg := types.NewPackage("github.com/example/parent", "parentpkg")
	parentTypesPkg.SetImports([]*types.Package{targetTypesPkg})

	// Create a package that imports a different package
	otherTypesPkg := types.NewPackage("github.com/example/other", "otherpkg")
	otherImportedPkg := types.NewPackage("github.com/example/different", "differentpkg")
	otherTypesPkg.SetImports([]*types.Package{otherImportedPkg})

	// Create an ssa.Program
	prog := ssa.NewProgram(fset, ssa.SanityCheckFunctions)

	// Create empty slices for ast.Files and types.Info since we don't have actual source code
	var emptyASTFiles []*ast.File
	var emptyTypesInfo *types.Info

	// Create ssa.Package instances using prog.CreatePackage
	targetSSAPkg := prog.CreatePackage(targetTypesPkg, emptyASTFiles, emptyTypesInfo, false)
	parentSSAPkg := prog.CreatePackage(parentTypesPkg, emptyASTFiles, emptyTypesInfo, false)
	otherSSAPkg := prog.CreatePackage(otherTypesPkg, emptyASTFiles, emptyTypesInfo, false)
	otherImportedSSAPkg := prog.CreatePackage(otherImportedPkg, emptyASTFiles, emptyTypesInfo, false)

	// Build the packages
	targetSSAPkg.Build()
	parentSSAPkg.Build()
	otherSSAPkg.Build()
	otherImportedSSAPkg.Build()

	// Build the program
	prog.Build()

	ok, err := DirectlyImportsPkg(parentSSAPkg, targetSSAPkg)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = DirectlyImportsPkg(targetSSAPkg, parentSSAPkg)
	require.NoError(t, err)
	require.False(t, ok)

	// nil check
	ok, err = DirectlyImportsPkg(parentSSAPkg, nil)
	require.Error(t, err)
	ok, err = DirectlyImportsPkg(nil, targetSSAPkg)
	require.Error(t, err)

	// Test GetPackagesDirectlyImportingPkg
	result, err := GetPackagesDirectlyImportingPkg(prog, targetSSAPkg)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result, parentSSAPkg)
	// Negative test
	// Nil check
	result, err = GetPackagesDirectlyImportingPkg(nil, targetSSAPkg)
	require.Error(t, err)

	// Define the regex pattern to match the target package name
	nameRegex := regexp.MustCompile(`^targetpkg$`)
	pathRegex := regexp.MustCompile(`^github.com/example/target$`)

	// Call the function under test
	result, err = GetPackagesDirectlyImportingPkgsByNameRegex(prog, nameRegex)
	require.NoError(t, err)
	require.NotNil(t, result)
	// check via path
	resultPath, err := GetPackagesDirectlyImportingPkgsByPathRegex(prog, pathRegex)
	require.NoError(t, err)
	require.NotNil(t, resultPath)

	// Check if the parent package is included in the result
	require.Contains(t, result, parentSSAPkg)

	// Negative test: Check that the other package is not included
	require.NotContains(t, result, otherSSAPkg)

	// Corner case: Regex that does not match any package names
	// Name
	nonMatchingRe := regexp.MustCompile(`nonexistentpkg`)
	result, err = GetPackagesDirectlyImportingPkgsByNameRegex(prog, nonMatchingRe)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result)
	// Path
	resultPath, err = GetPackagesDirectlyImportingPkgsByPathRegex(prog, nonMatchingRe)
	require.NoError(t, err)
	require.NotNil(t, resultPath)
	require.Empty(t, resultPath)

	// Corner case: Package with no imports
	noImportsTypesPkg := types.NewPackage("github.com/example/noimports", "noimportspkg")
	noImportsSSAPkg := prog.CreatePackage(noImportsTypesPkg, emptyASTFiles, emptyTypesInfo, false)
	noImportsSSAPkg.Build()
	prog.Build()

	// Test with the new package added
	// Name
	result, err = GetPackagesDirectlyImportingPkgsByNameRegex(prog, nameRegex)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result, parentSSAPkg)
	require.NotContains(t, result, noImportsSSAPkg)
	// Path
	resultPath, err = GetPackagesDirectlyImportingPkgsByPathRegex(prog, pathRegex)
	require.NoError(t, err)
	require.NotNil(t, resultPath)
	require.Contains(t, resultPath, parentSSAPkg)
	require.NotContains(t, resultPath, noImportsSSAPkg)

	// Error case: Nil program
	// Name
	result, err = GetPackagesDirectlyImportingPkgsByNameRegex(nil, nameRegex)
	require.Error(t, err)
	require.Nil(t, result)
	require.EqualError(t, err, "prog/re is nil")
	// Path
	resultPath, err = GetPackagesDirectlyImportingPkgsByPathRegex(nil, pathRegex)
	require.Error(t, err)
	require.Nil(t, resultPath)
	require.EqualError(t, err, "prog/re is nil")

	// Error case: Nil regex
	// Name
	result, err = GetPackagesDirectlyImportingPkgsByNameRegex(prog, nil)
	require.Error(t, err)
	require.Nil(t, result)
	require.EqualError(t, err, "prog/re is nil")
	// Path
	resultPath, err = GetPackagesDirectlyImportingPkgsByPathRegex(prog, nil)
	require.Error(t, err)
	require.Nil(t, resultPath)
	require.EqualError(t, err, "prog/re is nil")
}

// TestIsGlueFile tests the IsGlueFile function for various cases.
func TestIsGlueFile(t *testing.T) {
	// Create a FileSet for generating file positions
	fset := token.NewFileSet()

	// Helper function to create a mock file position in a given FileSet
	createFile := func(fset *token.FileSet, name string) token.Pos {
		if fset == nil {
			return token.NoPos
		}
		file := fset.AddFile(name, fset.Base(), 10)
		return file.Pos(0)
	}

	// Test cases
	tests := []struct {
		desc     string
		fset     *token.FileSet
		pos      token.Pos
		expected bool
	}{
		{
			desc:     "file matching glue pattern",
			fset:     fset,
			pos:      createFile(fset, "/src/example.internal/some/dir/glue_something.go"),
			expected: true,
		},
		{
			desc:     "file not matching glue pattern",
			fset:     fset,
			pos:      createFile(fset, "/src/example.internal/some/dir/otherfile.go"),
			expected: false,
		},
		{
			desc:     "nil FileSet",
			fset:     nil,
			pos:      createFile(nil, "/src/example.internal/some/dir/glue_something.go"),
			expected: false,
		},
		{
			desc:     "token.NoPos position",
			fset:     fset,
			pos:      token.NoPos,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := IsGlueFile(tt.fset, tt.pos, "example.internal")
			assert.Equal(t, tt.expected, result, "unexpected result for %s", tt.desc)
		})
	}
}

func TestMergeStatus(t *testing.T) {
	// Test cases
	tests := []struct {
		name        string
		s1          constant.Failness
		s2          constant.Failness
		order       map[constant.Failness]int
		want        constant.Failness
		expectPanic bool
	}{
		// Tests with OrderMapFailClosePriority
		{"Equal statuses (FailClose priority)", constant.FailClose, constant.FailClose, constant.OrderMapFailClosePriority, constant.FailClose, false},
		{"FailClose > FailOpen (FailClose priority)", constant.FailClose, constant.FailOpen, constant.OrderMapFailClosePriority, constant.FailClose, false},
		{"FailUnknown > FailOpen (FailClose priority)", constant.FailUnknown, constant.FailOpen, constant.OrderMapFailClosePriority, constant.FailUnknown, false},
		{"FailUnknownWithError > FailUnknown (FailClose priority)", constant.FailUnknownWithError, constant.FailUnknown, constant.OrderMapFailClosePriority, constant.FailUnknownWithError, false},
		{"FailClose > FailCloseSuppressed (FailClose priority)", constant.FailClose, constant.FailCloseSuppressed, constant.OrderMapFailClosePriority, constant.FailClose, false},

		// Tests with OrderMapFailOpenPriority
		{"Equal statuses (FailOpen priority)", constant.FailOpen, constant.FailOpen, constant.OrderMapFailOpenPriority, constant.FailOpen, false},
		{"FailOpen > FailClose (FailOpen priority)", constant.FailOpen, constant.FailClose, constant.OrderMapFailOpenPriority, constant.FailOpen, false},
		{"FailUnknown > FailUnknownWithError (FailOpen priority)", constant.FailUnknownWithError, constant.FailUnknown, constant.OrderMapFailOpenPriority, constant.FailUnknown, false},
		{"FailClose > FailUnknownWithError (FailOpen priority)", constant.FailClose, constant.FailUnknownWithError, constant.OrderMapFailOpenPriority, constant.FailUnknownWithError, false},
		{"FailCloseSuppressed > FailOpen (FailOpen priority)", constant.FailOpen, constant.FailCloseSuppressed, constant.OrderMapFailOpenPriority, constant.FailCloseSuppressed, false},

		// Edge case for invalid status
		{"Invalid status", constant.FailClose, constant.Failness("InvalidStatus"), constant.OrderMapFailClosePriority, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("expected panic for input statuses: %v, %v", tt.s1, tt.s2)
					}
				}()
			}

			actual := MergeStatus(tt.s1, tt.s2, tt.order)
			if actual != tt.want && !tt.expectPanic {
				t.Errorf("MergeStatus(%v, %v, order) = %v, want %v", tt.s1, tt.s2, actual, tt.want)
			}
		})
	}
}

func TestExecCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	runner := utilsmock.NewMockRunner(ctrl)

	runner.EXPECT().Run(gomock.Any()).DoAndReturn(func(cmd *exec.Cmd) error {
		_, err := cmd.Stdout.Write([]byte("stdout"))
		require.NoError(t, err)
		_, err = cmd.Stderr.Write([]byte("stderr"))
		require.NoError(t, err)
		return nil
	}).Times(1)

	out, _, err := ExecCommand(runner, "echo", "hello")
	assert.NoError(t, err)
	// "stderr" should not be part of the output.
	assert.Equal(t, "stdout", out)
}

func TestExecCommandFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	runner := utilsmock.NewMockRunner(ctrl)

	runner.EXPECT().Run(gomock.Any()).DoAndReturn(func(cmd *exec.Cmd) error {
		return errors.New("execution error")
	}).Times(1)

	_, _, err := ExecCommand(runner, "echo", "hello")
	assert.Error(t, err)
}

func TestGetMaxUsedDram(t *testing.T) {
	// Call the function - it should not panic and should return a non-negative value
	usage := GetMaxUsedDram()
	assert.GreaterOrEqual(t, usage, int64(0), "GetMaxUsedDram should return a non-negative value")
}
