package genai

import (
	"context"
	_ "embed" // embed is used to embed the prompt message in the binary
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/input"
	"github.com/uber-research/rpcshield/go/output"
	"github.com/uber-research/rpcshield/go/skeletonize"
	"go.uber.org/zap"
)

const (
	// Host is the default host for the GenAI service. A plain public OpenAI-compatible endpoint;
	// override via config.Config.GenAIBaseURL for a company-internal gateway.
	Host = "https://api.openai.com"
	// BaseURL is the default base URL for the GenAI API service.
	BaseURL        = Host + "/v1"
	_serviceName   = "genai-api"
	_callerService = "rpcshield"
)

var (
	// Model is the LLM model used for chat completion requests. Defaults to a public OpenAI
	// model; override via config.Config.GenAIModel for a company-internal model route.
	Model = "gpt-4o"
	// OrgID is the organization ID sent with chat completion requests. Empty by default
	// (appropriate for a plain OpenAI-compatible endpoint); override via config.Config.GenAIOrgID.
	OrgID = ""
)

var (
	//go:embed promptskeleton_go.txt
	promptMessageGo string
	//go:embed promptskeleton_java.txt
	promptMessageJava string

	// languageToPrompt is a map of language to prompt message
	languageToPrompt = map[string]string{
		constant.LanguageGo:   promptMessageGo,
		constant.LanguageJava: promptMessageJava,
	}
)

type (

	// GenAnswer is the struct for the answer from GenAI
	GenAnswer struct {
		CallerName string
		CalleeName string
		Decision   constant.Failness
		Reason     string
	}
	// AnswerPerChain is the struct for collecting answers per chain
	AnswerPerChain struct {
		ChainIndex int
		Answers    []GenAnswer
	}

	// AnalyzeCallChainsRequest is the struct for the request to analyze call chains
	AnalyzeCallChainsRequest struct {
		Workspace         string
		YamlFolderPath    string
		OutputDir         string
		Language          string
		NeedsProgramSlice bool
	}

	// AnalyzeEdgeRequest is the struct for the request to analyze an edge
	AnalyzeEdgeRequest struct {
		Workspace string
		Language  string
		Caller    *input.LLMFuncInput
		Callee    string
	}

	// AnalyzeEdgeResponse is the struct for the response to analyze an edge
	AnalyzeEdgeResponse struct {
		Status   constant.Failness
		Response string
	}
)

// Client is the subset of an OpenAI-compatible chat-completion client that rpcshield's GenAI
// analyzer needs. Any client satisfying this (e.g. the plain client returned by New, or Uber's
// internal Michelangelo genai-api client) can be plugged in.
type Client interface {
	CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// New returns a Client pointed at the given OpenAI-compatible baseURL, authenticating with token.
// This is the plug-in point for callers: point baseURL/token at any OpenAI-compatible endpoint,
// and override the Model/OrgID package vars for that endpoint's own model routes and org ID.
func New(baseURL string, token string) Client {
	cfg := openai.DefaultConfig(token)
	cfg.HTTPClient = &http.Client{
		Transport: &transport{
			underlyingTransport: http.DefaultTransport,
			serviceName:         _serviceName,
			callerName:          _callerService,
		}}
	cfg.BaseURL = baseURL
	cfg.OrgID = OrgID

	return openai.NewClientWithConfig(cfg)
}

type transport struct {
	underlyingTransport http.RoundTripper
	serviceName         string
	callerName          string
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("RPC-Service", t.serviceName)
	req.Header.Add("RPC-Caller", t.callerName)
	return t.underlyingTransport.RoundTrip(req)
}

// LLMAnalyzer uses in Go driver integration and returns AnalyzerOutput for the given pair input
func LLMAnalyzer(ctx context.Context, aiClient Client, workspace string, llmInputPair *input.LLMAnalyzerInput, logger *zap.SugaredLogger) (*output.AnalyzerOutput, error) {
	logger.Infof("Working on LLMAnalyzer for caller %s and callee %s", llmInputPair.Caller, llmInputPair.Callee)
	res, err := AnalyzeEdge(
		ctx,
		aiClient,
		&AnalyzeEdgeRequest{
			Workspace: workspace,
			Language:  constant.LanguageGo, // Invoked from Go driver, so language is always Go
			Caller:    llmInputPair.Caller,
			Callee:    llmInputPair.Callee,
		},
		logger,
	)
	if err != nil {
		return nil, err
	}
	return &output.AnalyzerOutput{Status: res.Status, DiagnosticMsg: res.Response, Source: constant.LLM, OriginalSource: constant.LLM}, nil
}

// AnalyzeEdge returns the response from GenAI for a given edge
func AnalyzeEdge(ctx context.Context, aiClient Client, req *AnalyzeEdgeRequest, logger *zap.SugaredLogger) (*AnalyzeEdgeResponse, error) {
	language := req.Language
	promptMessage, err := PromptForLanguage(language)
	if err != nil {
		return nil, err
	}
	workspace := req.Workspace
	callerFuncName := req.Caller.FuncName
	calleeFuncName := req.Callee
	srcFilePath := req.Caller.FilePath
	question := "Is the caller function " + callerFuncName + " and the callee function " + calleeFuncName +
		" a fail-close Caller-Callee relationship ? \n"
	detailer := "Step-by-step reason about the code. In the end provide a final answer which is one of " +
		"Yes: they are fail-close Caller-Callee relationship because of a non-nil error from " +
		calleeFuncName + " or NO: they are fail-open Caller-Callee relationship. \n  " +
		"Ultimately, provide a structured response as either <result> fail-open </result> or <result> fail-close </result> \n"
	funcSource, err := dumpFuncSource(filepath.Join(workspace, srcFilePath), req.Caller.StartLine, req.Caller.EndLine)
	logger.Debugw("Dumping function source", zap.String("source", funcSource))
	if err != nil {
		return nil, fmt.Errorf("dump function source: %w", err)
	}
	longQuestion := promptMessage + question + detailer + funcSource + "\n"
	responseString, err := AskGenAI(ctx, aiClient, longQuestion)
	if err != nil {
		return nil, err
	}

	aiAnswer := getFinalConclusion(responseString, logger)

	logger.Debugw("\n**** User question: \n ", zap.String("question", question+detailer+funcSource))
	logger.Debugw("\n***** Response from GenAI: \n", zap.String("answer", responseString))

	return &AnalyzeEdgeResponse{Status: aiAnswer, Response: responseString}, nil
}

// PromptForLanguage returns the prompt message for the given language
func PromptForLanguage(lang string) (string, error) {
	promptMessage := languageToPrompt[lang]
	if promptMessage == "" {
		return "", fmt.Errorf("prompt message not found for language %s", lang)
	}
	return promptMessage, nil
}

// AnalyzeCallChains returns the response from GenAI for the call chain
func AnalyzeCallChains(ctx context.Context, aiClient Client, req AnalyzeCallChainsRequest, logger *zap.Logger) ([]AnswerPerChain, error) {
	yamlFolderPath := req.YamlFolderPath
	outputdir := req.OutputDir
	workspace := req.Workspace
	lang := req.Language
	if lang == "" {
		lang = constant.LanguageGo
	}
	promptMessage, err := PromptForLanguage(lang)
	if err != nil {
		return nil, err
	}
	logger.Info("Working on folder ", zap.String("yaml folder", yamlFolderPath))
	if _, err := os.Stat(yamlFolderPath); err != nil {
		return nil, fmt.Errorf("get yamlFolderPath: %w", err)
	}

	if _, err := os.Stat(outputdir); err != nil {
		return nil, fmt.Errorf("get outputdir: %w", err)
	}

	files, err := os.ReadDir(yamlFolderPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %v", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no yaml files found in %s ", yamlFolderPath)
	}
	var totalRes []AnswerPerChain

	for _, yamlfile := range files {
		if !yamlfile.IsDir() && strings.HasSuffix(yamlfile.Name(), ".yaml") {
			// get index of yaml file
			fileName := yamlfile.Name()
			fileName = fileName[:strings.Index(yamlfile.Name(), ".yaml")]
			index, err := strconv.Atoi(fileName[strings.LastIndex(fileName, ".")+1:])
			answerOneYaml := AnswerPerChain{ChainIndex: index}

			outputfile := strings.Replace(yamlfile.Name(), ".yaml", ".out.log", -1)
			logger.Debug("Output file name : ", zap.String("output file", outputfile))
			outputfile = filepath.Join(outputdir, outputfile)
			// if file already exists, skip processing
			if _, err := os.Stat(outputfile); err == nil {
				os.Remove(outputfile)
			}
			llmCallPath, err := readYamlFile(filepath.Join(yamlFolderPath, yamlfile.Name()))
			if err != nil {
				return nil, fmt.Errorf("get LLM chain: %w", err)
			}

			leafFunction := true
			aiAnswer := constant.FailUnknown
			lastFunction := ""

			for ii, funcCall := range llmCallPath.FunctionsChain {
				if !leafFunction {
					funcName := funcCall.FuncName
					question := "Is the caller function " + funcName + " and the callee function " + lastFunction +
						" a fail-close Caller-Callee relationship ? \n"
					detailer := "Step-by-step reason about the code. In the end provide a final answer which is one of " +
						"Yes: they are fail-close Caller-Callee relationship because of a non-nil error from " +
						lastFunction + " or NO: they are fail-open Caller-Callee relationship. \n  " +
						"Ultimately, provide a structured response as either <result> fail-open </result> or <result> fail-close </result> \n"
					var funcSource string
					if req.NeedsProgramSlice {
						srcpath := filepath.Join(workspace, funcCall.FilePath)
						fileReader, err := os.Open(srcpath)
						if err != nil {
							return nil, fmt.Errorf("open file: %w", err)
						}
						funcSource, err = skeletonize.ExtractCode(fileReader, funcName, funcCall.StartLine, lastFunction, logger.Sugar())
						if err != nil {
							return nil, fmt.Errorf("program slicing: %w", err)
						}
					} else {
						funcSource, err = dumpFuncSource(filepath.Join(workspace, funcCall.FilePath), funcCall.StartLine, funcCall.EndLine)
					}

					funcSource, err := dumpFuncSource(filepath.Join(workspace, funcCall.FilePath), funcCall.StartLine, funcCall.EndLine)
					logger.Debug("Dumping function source", zap.String("source", funcSource))
					if err != nil {
						return nil, fmt.Errorf("dump function source: %w", err)
					}
					longQuestion := promptMessage + question + detailer + funcSource + "\n"
					responseString, err := AskGenAI(ctx, aiClient, longQuestion)
					if err != nil {
						return nil, err
					}

					aiAnswer = getFinalConclusion(responseString, logger.Sugar())

					logger.Debug("\n**** User question: \n ", zap.String("question", question+detailer+funcSource))
					logger.Debug("\n***** Response from GenAI: \n", zap.String("answer", responseString))

					answerPair := GenAnswer{CallerName: funcName, CalleeName: lastFunction, Decision: aiAnswer, Reason: responseString}
					answerOneYaml.Answers = append(answerOneYaml.Answers, answerPair)
					// carry down the func name
					lastFunction = funcCall.FuncName
				} else {
					leafFunction = false
					lastFunction = funcCall.FuncName
				}
				if aiAnswer == constant.FailOpen {
					// stop if we hit fail-open
					totalRes = append(totalRes, answerOneYaml)
					break
				}
				if ii == len(llmCallPath.FunctionsChain)-1 {
					// last function
					totalRes = append(totalRes, answerOneYaml)
				}
			} // end of the single call chain
		} // end of one yaml file
	}
	return totalRes, nil
}

// AskGenAI asks a question to GenAI and returns the response
func AskGenAI(ctx context.Context, aiClient Client, question string) (string, error) {
	res, err := aiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       Model,
			MaxTokens:   4096,
			Temperature: 0.3,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a helpful assistant for understanding error return flows.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: question,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("get genai response: %w", err)
	}
	return res.Choices[0].Message.Content, nil
}

func getFinalConclusion(aiResponse string, logger *zap.SugaredLogger) constant.Failness {
	responseLines := strings.Split(aiResponse, "\n")
	lineno := 0
	finalAnswer := constant.FailUnknown
	for _, line := range responseLines {
		if strings.Contains(strings.ToLower(line), "<result>") {
			answersplit := responseLines[lineno:]
			allAnswer := strings.Join(answersplit, "")
			if strings.Contains(allAnswer, "fail-open") {
				finalAnswer = constant.FailOpen
				break
			}
			if strings.Contains(allAnswer, "fail-close") {
				finalAnswer = constant.FailClose
				break
			}
		}
		lineno++
	}
	if lineno == len(responseLines) {
		logger.Info("No final answer found in the response")
		finalAnswer = constant.FailUnknown
	}
	return finalAnswer
}
