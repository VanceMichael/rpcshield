package bazel

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/uber-research/rpcshield/go/common/utils"
)

var (
	chmodFunc = os.Chmod
	mkdirFunc = os.Mkdir
)

// processServerlessFix fix the no main pkg issue for serverless targets
// The general flow is to 1) build the target to generate main.go
// and 2) copy the main.go to the synthesized analysis directory
func (br *gopathWrapper) processServerlessFix(runner utils.Runner, target, bazelBinPath, pkgPath, gopathRoot, gopathPkgAndPath string) error {
	targetServiceName := strings.Split(target, ":")[1]
	// check if the target is a serverless targe
	if !strings.HasPrefix(targetServiceName, "serverless-") {
		return nil
	}
	br.Logger.Infow("Fixing main package issue for serverless target since it is automatically generated and not copied by go_path rule ", "target", target)

	destPath := filepath.Join(gopathRoot, pkgPath)
	// build the target to generate main.go
	// TODO: find ways to only run the main.go generation logic via bazel and avoid building the entire target
	_, _, err := utils.ExecCommand(runner, br.BazelPath, append(slices.Clone(_buildCmd), target)...)
	if err != nil {
		return err
	}

	generatedMainPath := filepath.Join(bazelBinPath, pkgPath)
	mainFile := filepath.Join(generatedMainPath, "main.go")
	destMainPath := filepath.Join(destPath, targetServiceName)

	if err := chmodFunc(destPath, 0777); err != nil {
		return fmt.Errorf("failed to make %q writable: %w", destPath, err)
	}

	if err := mkdirFunc(destMainPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory : %q : %w", destMainPath, err)
	}

	// copy main.go to path .../../../service-anal/src/../targetServiceName/
	if _, _, err := utils.ExecCommand(runner, "cp", "-r", mainFile, destMainPath); err != nil {
		return fmt.Errorf("failed to copy generated main from %q to %q: %w", generatedMainPath, destPath, err)
	}

	return nil
}
