package jsonmode

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/uber-research/rpcshield/go/callnode"
	"github.com/uber-research/rpcshield/go/cmd/rpcshield/common"
	"github.com/uber-research/rpcshield/go/genai"
	"github.com/uber-research/rpcshield/go/output"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const _htmlReportFilename = "rpcshield_report.html"

// Options is the options for the JSON mode of rpcshield.
type Options struct {
	// InputFile is the JSON file that contains the call chain.
	InputFile string
	// UserName is the username that runs rpcshield.
	UserName string
	// ConduitToken is the conduit token.
	ConduitToken string
	// Workspace is the workspace of the user.
	Workspace string
	// OutputDir is the output directory.
	OutputDir string
	// Debug enables debug mode (intermediate files are not deleted).
	Debug bool
	// NeedsProgramSlice indicates whether the program slice is enabled.
	NeedsProgramSlice bool
	// LogLevel is the log level.
	LogLevel zapcore.Level
	// Language is the language of the code.
	Language string
	// HTMLReport indicates whether to generate an HTML report.
	HTMLReport bool
}

// Run runs the JSON mode of rpcshield.
func Run(opts *Options, aiclient genai.Client) error {
	ctx := context.Background()
	logger, err := common.BuildLogger(opts.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	logger.Infow("Work Space : ", "Workspace ", opts.Workspace)
	if opts.InputFile == "" {
		return errors.New("empty input JSON")
	}
	logger.Infow("Input JSON : ", "JSON name", opts.InputFile)

	roots, err := ParseCallchain(opts.InputFile)
	if err != nil {
		return fmt.Errorf("JSON format not correct %w", err)
	}
	var totalLeafNodes []callnode.Node

	for i := range roots {
		root := &roots[i]
		leafNodes := setParentSection(root)
		for _, leaf := range leafNodes {
			totalLeafNodes = append(totalLeafNodes, leaf)
		}
	}

	if opts.OutputDir == "" {
		tmpDirout, err := os.MkdirTemp("", "out")
		if err != nil {
			log.Fatal(err)
		}
		// TODO: rather than overwriting opts.OutputDir, we should use a separate variable.
		opts.OutputDir = tmpDirout
	}
	genOutDir := filepath.Join(opts.OutputDir, "genout")
	if err := os.MkdirAll(genOutDir, os.ModePerm); err != nil {
		return fmt.Errorf("genout folder creation : %w", err)
	}
	var outtree *callnode.Data
	// Dump the call chain nodes to YAML as input of LLM
	yamlFolderPath, err := os.MkdirTemp("", "yaml")
	if err != nil {
		return fmt.Errorf("LLM yaml input folder creation : %w", err)
	}
	for indx, leaf := range totalLeafNodes {
		err = dumpYAML(&leaf, indx, yamlFolderPath)
		if err != nil {
			return fmt.Errorf("Yaml dump: %w ", err)
		}
	}

	req := genai.AnalyzeCallChainsRequest{
		Workspace:         opts.Workspace,
		YamlFolderPath:    yamlFolderPath,
		OutputDir:         genOutDir,
		Language:          opts.Language,
		NeedsProgramSlice: opts.NeedsProgramSlice,
	}
	totalRes, err := genai.AnalyzeCallChains(ctx, aiclient, req, logger.Desugar())
	if err != nil {
		return fmt.Errorf("GenAI analyze : %w ", err)
	}

	if len(totalLeafNodes) > 0 {
		logger.Debugw("Processing output file at ", "output file", opts.OutputDir)
		outtree, err = processOutput(opts, totalRes, totalLeafNodes, roots)
	}
	if err != nil {
		return fmt.Errorf("process output : %w ", err)
	}

	if opts.HTMLReport {
		err := createReport(outtree, opts.OutputDir, logger.Desugar())
		if err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func createReport(outtree *callnode.Data, ouputdir string, logger *zap.Logger) error {
	logger.Info("Creating report at ", zap.String("output dir", ouputdir))
	if outtree != nil {
		err := generateHTMLReport(*outtree, ouputdir)
		if err != nil {
			return fmt.Errorf("generate report : %w ", err)
		}
	}
	return nil
}

func generateHTMLReport(outtree callnode.Data, ouputdir string) error {
	// Generate HTML report
	htmlReportFilepath := filepath.Join(ouputdir, _htmlReportFilename)
	err := output.GenerateHTMLReport(outtree, htmlReportFilepath)
	if err != nil {
		return fmt.Errorf("HTML generation : %w ", err)
	}
	return nil
}
