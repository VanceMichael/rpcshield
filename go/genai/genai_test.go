package genai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	genaimock "mock/github.com/uber-research/rpcshield/go/genai/genaimock"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/input"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

const _rootTestsDirPath = "../testdata"

func TestNew(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"choices":[{"message":{"content":" blah blah blah blah \n <result> fail-open </result>"}}]}`))
		require.NoError(t, err)
	}))
	client := New(ts.URL, "xxxx")
	require.NotEmpty(t, client)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{})
	require.NoError(t, err)
	require.Equal(t, resp.Choices[0].Message.Content, " blah blah blah blah \n <result> fail-open </result>")
}

func TestAnalyzeCallChains_Wrong_Path(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	genaiClient := genaimock.NewMockClient(ctrl)

	workspace := t.TempDir()
	req := AnalyzeCallChainsRequest{
		Workspace:         workspace,
		YamlFolderPath:    "testdata/xx",
		OutputDir:         "testdata/xx",
		NeedsProgramSlice: false,
	}
	res1, err1 := AnalyzeCallChains(context.Background(), genaiClient, req, zap.NewNop())
	_ = res1
	require.Error(t, err1)
}

func TestAnalyzeCallChains_No_Yaml(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	genaiClient := genaimock.NewMockClient(ctrl)
	req := AnalyzeCallChainsRequest{
		Workspace:         t.TempDir(),
		YamlFolderPath:    "testdata/",
		OutputDir:         "testdata/",
		NeedsProgramSlice: false,
	}
	res1, err1 := AnalyzeCallChains(context.Background(), genaiClient, req, zap.NewNop())
	require.Nil(t, res1)
	require.NoError(t, err1)
}

func TestAnalyzeCallChains(t *testing.T) {
	t.Parallel()
	workspace := _rootTestsDirPath
	localTestFolder, _ := filepath.Abs("testdata")

	ctrl := gomock.NewController(t)
	genaiClient := genaimock.NewMockClient(ctrl)
	genaiClient.EXPECT().CreateChatCompletion(gomock.Any(), gomock.Any()).Return(openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: " blah blah blah blah \n " +
						"<result> fail-open </result>",
				},
			},
		},
	}, nil)

	req := AnalyzeCallChainsRequest{
		Workspace:         workspace,
		YamlFolderPath:    filepath.Join(localTestFolder, "yaml/"),
		OutputDir:         filepath.Join(localTestFolder, "yaml/"),
		NeedsProgramSlice: true,
	}
	res, err := AnalyzeCallChains(context.Background(), genaiClient, req, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestLLMAnalyzer(t *testing.T) {
	t.Parallel()

	workspace := _rootTestsDirPath
	ctrl := gomock.NewController(t)
	genaiClient := genaimock.NewMockClient(ctrl)
	genaiClient.EXPECT().CreateChatCompletion(gomock.Any(), gomock.Any()).Return(openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: " blah blah blah blah \n " +
						"<result> fail-open </result>",
				},
			},
		},
	}, nil)

	caller := input.NewLLMFuncInput("calculatefareshandler.handlerImpl.CalculateEatsOrder", "/src/cmdtests/example.internal/calculatefareshandler/handler.go", 131, 139)
	llmInputPair := input.NewLLMAnalyzerInput(caller, "handlerutil.ExecuteHandler", 5)
	res, err := LLMAnalyzer(context.Background(), genaiClient, workspace, llmInputPair, zap.NewNop().Sugar())
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestAnalyzeEdge_UnknownLanguage(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	genaiClient := genaimock.NewMockClient(ctrl)

	caller := input.NewLLMFuncInput("calculatefareshandler.handlerImpl.CalculateEatsOrder", "/src/cmdtests/example.internal/calculatefareshandler/handler.go", 131, 139)
	req := &AnalyzeEdgeRequest{
		Workspace: _rootTestsDirPath,
		Language:  "rust",
		Caller:    caller,
		Callee:    "handlerutil.ExecuteHandler",
	}
	res, err := AnalyzeEdge(context.Background(), genaiClient, req, zap.NewNop().Sugar())
	require.Error(t, err)
	require.Nil(t, res)
}
