package bazel

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/uber-research/rpcshield/go/common/utils"
	"github.com/uber-research/rpcshield/go/config"
	"go.uber.org/zap"
)

var goPathRegex = regexp.MustCompile(`def.bzl.*\"go_path\"`)

// Note the --remote_download_all here since it
// (1) forces bazel to fetch files that are not needed by downstream actions, which are required by our analysis;
// (2) Mitigate CI (Odin)'s unreliable storage where it forces bazel to rebuild if remote (including on-disk cache) download failed.
var _buildCmd = []string{"build", "--nocheck_visibility", "--remote_download_all", "--config", "remote_cache_buildbarn", "--modify_execution_info", "GoPath=+no-remote"}

// checkCommandAvailable checks if a command is available in the system.
func checkCommandAvailable(runner utils.Runner, name string) error {
	cmd := exec.Command("which", name)
	if err := runner.Run(cmd); err != nil {
		return fmt.Errorf("%s is not available", name)
	}
	return nil
}

// NewGoPathWrapper returns a new GoPathWrapper instance.
func NewGoPathWrapper(workspace string, serviceName string, outputDir string, logger *zap.SugaredLogger, renamer FSRenamer, cfg config.Config) GoPathWrapper {
	// Create a no-op logger if not provided to avoid nil panics.
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return &gopathWrapper{
		Workspace:     workspace,
		ServiceName:   serviceName,
		OutputDir:     outputDir,
		BazelPath:     filepath.Join(workspace, "bin", "bazel"),
		BuildozerPath: filepath.Join(workspace, "bin", "buildozer"),
		Logger:        logger,
		renamer:       renamer,
		Config:        cfg,
	}
}

type gopathWrapper struct {
	ServiceName   string
	Workspace     string
	BazelPath     string
	BuildozerPath string
	OutputDir     string
	Logger        *zap.SugaredLogger
	renamer       FSRenamer
	Config        config.Config
}

// LiftGoPath takes the given target and runs bazel to lift the target, as well as its transitive
// dependencies to form a GOPATH-like structure. The returned string is the path to the absolute
// lifted directory that is readily available for running analysis and a set of C/C++ library dependencies of the target.
func (br *gopathWrapper) LiftGoPath(runner utils.Runner, target string) (string, []string, error) {
	// Check if bazel and buildozer are available.
	if err := checkCommandAvailable(runner, br.BazelPath); err != nil {
		return "", nil, err
	}

	if err := checkCommandAvailable(runner, br.BuildozerPath); err != nil {
		return "", nil, err
	}

	// Target looks like "//src/example.internal/my/target:target", to construct the file path
	// for the corresponding BUILD.bazel file, we need to:
	pkgPath := strings.TrimPrefix(target, "//") // Trim the "//" prefix.
	pkgAndTgt := strings.Split(pkgPath, ":")    // Trim the rule suffix.

	// Make sure it is a valid target.
	if len(pkgAndTgt) != 2 {
		return "", nil, fmt.Errorf("invalid target: %s", target)
	}

	pkgPath = pkgAndTgt[0]
	targetName := pkgAndTgt[1]
	goPathTgtName := targetName + "-anal"
	tgtWithPkg := "//" + pkgPath + ":__pkg__"
	buildFile := filepath.Join(pkgPath, "BUILD.bazel")
	// Read the content of the BUILD.bazel file.
	data, err := os.ReadFile(buildFile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read BUILD.bazel file: %v", buildFile)
	}
	// Write back the content of the BUILD.bazel file in defer.
	defer func() {
		if err := os.WriteFile(buildFile, data, 0666); err != nil {
			fmt.Println("failed to write back the content of the BUILD.bazel file")
		}
	}()

	// If there is already @io_bazel_rules_go//go:def.bzl go_path rule, we don't need to add it again.
	// Regex to match def.bzl.*"go_path" the BUILD.bazel file.
	if !goPathRegex.Match(data) {
		// Ensure go_path is loaded
		_, _, err = utils.ExecCommand(runner, br.BuildozerPath, "new_load @io_bazel_rules_go//go:def.bzl go_path", tgtWithPkg)
		if err != nil {
			return "", nil, err
		}
	}

	// Add go_path rule
	_, _, err = utils.ExecCommand(runner, br.BuildozerPath, fmt.Sprintf("new go_path %s-anal", targetName), tgtWithPkg)
	// If the rule already exists, ignore the error.
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return "", nil, err
	}

	// Exit code 3 from buildozer [1] means that no changes are made, which is not an error and we can
	// safely continue.
	// [1]: https://github.com/bazelbuild/buildtools/blob/main/buildozer/README.md#error-code
	var exitErr *exec.ExitError

	gopathPkgAndPath := "//" + pkgPath + ":" + goPathTgtName
	// Set testonly and include_data attributes
	_, _, err = utils.ExecCommand(runner, br.BuildozerPath, "set testonly True", gopathPkgAndPath)
	if err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 3) {
		return "", nil, err
	}
	_, _, err = utils.ExecCommand(runner, br.BuildozerPath, "set include_data False", gopathPkgAndPath)
	if err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 3) {
		return "", nil, err
	}

	// Set deps attribute.
	_, _, err = utils.ExecCommand(runner, br.BuildozerPath, fmt.Sprintf("add deps :%s", targetName), gopathPkgAndPath)
	if err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 3) {
		return "", nil, err
	}

	// Run Bazel to build the target-anal.
	stdout, stderr, err := utils.ExecCommand(runner, br.BazelPath, append(slices.Clone(_buildCmd), gopathPkgAndPath)...)
	if err != nil {
		return "", nil, err
	}
	buildLogPath := filepath.Join(br.OutputDir, fmt.Sprintf("%s_bazel_build.log", br.ServiceName))
	br.Logger.Infow("Writing bazel build log", "path", buildLogPath)
	if err := os.WriteFile(buildLogPath, []byte(fmt.Sprintf("STDOUT: %s\n\nSTDERR:%s", stdout, stderr)), 0666); err != nil {
		return "", nil, fmt.Errorf("failed to write bazel build log to %q: %w", buildLogPath, err)
	}

	// Get the path to the root of the `bazel-bin` directory.
	bazelBinPath, _, err := utils.ExecCommand(runner, br.BazelPath, "info", "bazel-bin")
	if err != nil {
		return "", nil, err
	}
	bazelBinPath = strings.TrimSpace(bazelBinPath)

	// Construct the GOPATH-root directory from target and bazel-bin path.
	gopathRoot := filepath.Join(bazelBinPath, pkgPath, goPathTgtName)

	err = br.processServerlessFix(runner, target, bazelBinPath, pkgPath, gopathRoot, gopathPkgAndPath)
	if err != nil {
		return "", nil, err
	}

	if err := br.patchIncorrectMoneyIntegrationGogoprotoDir(gopathRoot); err != nil {
		return "", nil, fmt.Errorf("patch missing money integration test file: %w", err)
	}

	if err := br.renamer.Rename(gopathRoot, gopathRoot); err != nil {
		return "", nil, err
	}

	// Extract CGO flags, if any, by analyzing the C/C++ dependencies ("cdeps") of the target.
	cdeps, err := br.processCGo(runner, target, gopathRoot)
	if err != nil {
		return "", nil, err
	}

	// Fix import comment of the package.
	if err := br.fixImportComment(runner, gopathRoot); err != nil {
		return "", nil, err
	}

	return gopathRoot, cdeps, nil
}

// patchIncorrectMoneyIntegrationGogoprotoDir patches the incorrectly-generated
// <Config.MoneyIntegrationGogoprotoPath>/uber_test folder.
//
// This proto package has a "_test" in its package name and was automatically and mistakenly
// renamed to `uber` (dropping the `_test` suffix) by the bazel/proto compiler since it somehow
// thinks it is a test package (due to the suffix). This causes issues in loading the packages
// as the downstream dependencies rely on the package being `uber_test`. Therefore, this function
// renames the folder from `uber` to `uber_test` upon seeing it.
//
// TODO: remove this once bazel codegen fixes this issue.
func (br *gopathWrapper) patchIncorrectMoneyIntegrationGogoprotoDir(gopathRoot string) error {
	if br.Config.MoneyIntegrationGogoprotoPath == "" {
		return nil
	}

	extensionsDir := filepath.Join(gopathRoot, br.Config.MoneyIntegrationGogoprotoPath)
	uberDir := filepath.Join(extensionsDir, "uber")

	uberTestPB := filepath.Join(uberDir, "uber_test.pb.go")
	if _, err := os.Stat(uberTestPB); errors.Is(err, fs.ErrNotExist) {
		// It is OK if this file does not exist - it simply means the target does not depend on
		// money integration test files - and hence we do not need to patch it.
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %q: %w", uberTestPB, err)
	}

	// By default, the folder is read-only, relax it in order to rename it.
	if err := os.Chmod(extensionsDir, os.ModePerm); err != nil {
		return fmt.Errorf("making %q writable: %w", extensionsDir, err)
	}
	if err := os.Chmod(uberDir, os.ModePerm); err != nil {
		return fmt.Errorf("making %q writable: %w", uberDir, err)
	}

	targetDir := filepath.Join(extensionsDir, "uber_test")
	if err := os.Rename(uberDir, targetDir); err != nil {
		return fmt.Errorf("rename %q to %q: %w", uberDir, targetDir, err)
	}

	return nil
}
