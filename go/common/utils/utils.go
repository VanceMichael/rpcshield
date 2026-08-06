package utils

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

// GenericServiceSubDirs lists the generic directory names that are usually part of the service
// directory, and we should hop up to the parent directory to find the service directory.
var GenericServiceSubDirs = map[string]bool{
	"cmd":        true,
	"main":       true,
	"handler":    true,
	"gateway":    true,
	"entity":     true,
	"controller": true,
	"mapper":     true,
	"repository": true,
	"clients":    true,
}

// glueFileRegexCache memoizes glueFileRegex's compiled pattern per codebasePkgPrefix, since
// IsGlueFile is called once per SSA function/object in the whole program and recompiling the
// regex on every call would be wasteful on large services.
var (
	glueFileRegexCacheMu sync.Mutex
	glueFileRegexCache   = map[string]*regexp.Regexp{}
)

// glueFileRegex returns a regex matching files that should be excluded from the analysis because
// they are generated files copied from a glue/.. directory for purely working around the
// limitations of the current codegen. codebasePkgPrefix restricts matches to that GOPATH-style
// src/ subtree; empty matches any src/ subtree.
func glueFileRegex(codebasePkgPrefix string) *regexp.Regexp {
	glueFileRegexCacheMu.Lock()
	defer glueFileRegexCacheMu.Unlock()
	if re, ok := glueFileRegexCache[codebasePkgPrefix]; ok {
		return re
	}
	re := regexp.MustCompile("^.*/src/" + regexp.QuoteMeta(codebasePkgPrefix) + ".*/glue_[^/]+.go$")
	glueFileRegexCache[codebasePkgPrefix] = re
	return re
}

// Runner runs an exec.Cmd. It is structurally identical to example.internal/contrib/execrunner.Runner
// so that implementations of either interface satisfy both without an adapter.
type Runner interface {
	Run(*exec.Cmd) error
}

// OSRunner is the default Runner implementation, which just runs the command via os/exec.
type OSRunner struct{}

// Run executes the given command and waits until it exits.
func (OSRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

// IsGlueFile checks if the file is a glue file. codebasePkgPrefix restricts the check to files
// under that package-path prefix (see config.Config.CodebasePkgPrefix); empty matches any prefix.
func IsGlueFile(fset *token.FileSet, pos token.Pos, codebasePkgPrefix string) bool {
	if fset == nil || pos == token.NoPos {
		return false
	}

	if file := fset.File(pos); file != nil {
		if glueFileRegex(codebasePkgPrefix).MatchString(file.Name()) {
			return true
		}
	}
	return false
}

// IsGlueService checks for the presense of .glue, gateway, handler, controller, and repository
// directories, per the Glue-layer directory-layout conventions.
func IsGlueService(rootDir, target string) bool {
	// Target looks like "//src/example.internal/my/target:target".
	pkgPath := strings.TrimPrefix(target, "//") // Trim the "//" prefix.
	pkgAndTgt := strings.Split(pkgPath, ":")    // Trim the rule suffix.

	// Make sure it is a valid target.
	if len(pkgAndTgt) != 2 {
		return false
	}

	pkgPath = pkgAndTgt[0]
	// Join the root directory with the package path.
	dir := filepath.Join(rootDir, pkgPath)
	// Check for the presence of the directories.
	for _, d := range []string{".glue", "gateway", "handler", "controller", "repository"} {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			return false
		}
	}
	return true
}

// DirectlyImportsPkg checks if the parent package directly imports the child package.
func DirectlyImportsPkg(parent *ssa.Package, child *ssa.Package) (bool, error) {
	if parent == nil || child == nil {
		return false, errors.New("parent/child is nil")
	}

	// Iterate over the imports of the parent package
	for _, importedTypesPkg := range parent.Pkg.Imports() {
		if importedTypesPkg == child.Pkg {
			return true, nil
		}
	}
	return false, nil
}

// GetPackagesDirectlyImportingPkg returns all packages that directly import the child package.
func GetPackagesDirectlyImportingPkg(prog *ssa.Program, child *ssa.Package) (map[*ssa.Package]struct{}, error) {
	if prog == nil || child == nil {
		return nil, errors.New("prog/re is nil")
	}
	result := make(map[*ssa.Package]struct{})

	// Iterate over all packages in the program
	for _, parent := range prog.AllPackages() {
		importsChildren, err := DirectlyImportsPkg(parent, child)
		if err != nil {
			return nil, err
		}
		if importsChildren {
			result[parent] = struct{}{}
		}
	}
	return result, nil
}

// GetPackagesDirectlyImportingPkgsByNameRegex returns all packages that directly import packages matching  the provided package name regex pattern.
func GetPackagesDirectlyImportingPkgsByNameRegex(prog *ssa.Program, re *regexp.Regexp) (map[*ssa.Package]struct{}, error) {
	if prog == nil || re == nil {
		return nil, errors.New("prog/re is nil")
	}

	result := make(map[*ssa.Package]struct{})

	// Iterate over all packages in the program
	for _, parent := range prog.AllPackages() {
		// Check each imported package
		for _, importedTypesPkg := range parent.Pkg.Imports() {
			pkgName := importedTypesPkg.Name()
			if re.MatchString(pkgName) {
				result[parent] = struct{}{}
			}
		}
	}
	return result, nil
}

// GetPackagesDirectlyImportingPkgsByPathRegex returns all packages that directly import packages matching the provided package path regex pattern.
func GetPackagesDirectlyImportingPkgsByPathRegex(prog *ssa.Program, re *regexp.Regexp) (map[*ssa.Package]struct{}, error) {
	if prog == nil || re == nil {
		return nil, errors.New("prog/re is nil")
	}
	result := make(map[*ssa.Package]struct{})
	// Iterate over all packages in the program
	for _, parent := range prog.AllPackages() {
		// Check each imported package
		for _, importedTypesPkg := range parent.Pkg.Imports() {
			pkgPath := importedTypesPkg.Path()
			if re.MatchString(pkgPath) {
				result[parent] = struct{}{}
			}
		}
	}
	return result, nil
}

// MergeStatus merges two fail-ness statuses based on the order provided.
// For example, FailClose > FailUnknownWithError > FailUnknown > FailOpen, when the order is [FailClose: 0, FailUnknownWithError: 1, FailUnknown: 2, FailOpen: 3].
func MergeStatus(s1 constant.Failness, s2 constant.Failness, order map[constant.Failness]int) constant.Failness {
	if s1 == s2 {
		return s1
	}

	// Validate input statuses.
	r1, ok1 := order[s1]
	r2, ok2 := order[s2]
	if !ok1 || !ok2 {
		// This should never happen. Status should always be one of the provided statuses.
		// If this happens, it is a bug in the code.
		panic(fmt.Sprintf("invalid fail-ness status: %s or %s; expected one of: %v", s1, s2, order))
	}

	// Return the higher priority status.
	if r1 < r2 {
		return s1
	}
	return s2
}

// ExecCommand executes a command and returns its stdout or an error.
func ExecCommand(runner Runner, name string, arg ...string) (string, string, error) {
	cmd := exec.Command(name, arg...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := runner.Run(cmd)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute command %q %v: %w\nSTDOUT: %s\nSTDERR: %s", name, arg, err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), nil
}

// GetMaxUsedDram returns the maximum used DRAM (Resident Set Size) in bytes.
func GetMaxUsedDram() int64 {
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err != nil {
		return 0
	}
	return rusage.Maxrss
}
