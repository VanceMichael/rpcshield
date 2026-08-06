package skeletonize

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func checkEquivalent(exptectsourceCode, modifiedCode []byte) bool {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	exptree := parser.Parse(nil, exptectsourceCode)
	exptectRootNode := exptree.RootNode()

	moditree := parser.Parse(nil, modifiedCode)
	modiRootNode := moditree.RootNode()

	// Compare the two trees
	if !compareTrees(modiRootNode, exptectRootNode, modifiedCode, exptectsourceCode, "") {
		fmt.Println("The trees are not equivalent.")
		return false
	}
	return true
}

func TestCase(t *testing.T) {
	//t.Parallel()
	tests := map[string]struct {
		filename string
		funcline int
	}{
		"test0.go":  {"test0_exp.go", 10},
		"test1.go":  {"test1_exp.go", 15},
		"test2.go":  {"test2_exp.go", 15},
		"test3.go":  {"test3_exp.go", 19},
		"test4.go":  {"test4_exp.go", 19},
		"test5.go":  {"test5_exp.go", 19},
		"test6.go":  {"test6_exp.go", 19},
		"test7.go":  {"test7_exp.go", 19},
		"test8.go":  {"test8_exp.go", 19},
		"test9.go":  {"test9_exp.go", 10},
		"test10.go": {"test10_exp.go", 19},
		"test11.go": {"test11_exp.go", 10},
		"test12.go": {"test12_exp.go", 10},
		"test13.go": {"test13_exp.go", 10},
		"test14.go": {"test14_exp.go", 10},
		"test15.go": {"test15_exp.go", 10},
		"test16.go": {"test16_exp.go", 10},
		"test17.go": {"test17_exp.go", 10},
		"test18.go": {"test18_exp.go", 10},
		"test19.go": {"test19_exp.go", 10},
		"test20.go": {"test20_exp.go", 10},
		"test21.go": {"test21_exp.go", 10},
		"test22.go": {"test22_exp.go", 10},
		"test23.go": {"test23_exp.go", 10},
		"test24.go": {"test24_exp.go", 10},
		"test25.go": {"test25_exp.go", 10},
		"test26.go": {"test26_exp.go", 10},
		"test27.go": {"test27_exp.go", 10},
		"test28.go": {"test28_exp.go", 10},
		"test29.go": {"test29_exp.go", 10},
		"test30.go": {"test30_exp.go", 10},
		"test31.go": {"test31_exp.go", 10},
		"test32.go": {"test32_exp.go", 10},
		"test33.go": {"test33_exp.go", 10},
		"test34.go": {"test34_exp.go", 10},
		"test35.go": {"test35_exp.go", 10},
		"test36.go": {"test36_exp.go", 10},
		"test37.go": {"test37_exp.go", 10},
		"test38.go": {"test38_exp.go", 10},
		"test39.go": {"test39_exp.go", 10},
	}

	logger, _ := zap.NewProduction()
	sugaredLogger := logger.Sugar()

	for srcFileName, expectFile := range tests {
		t.Run(srcFileName, func(t *testing.T) {
			srcpath := filepath.Join("testdata", srcFileName)
			expectpath := filepath.Join("testdata", expectFile.filename)
			fileReader, err := os.Open(srcpath)
			require.NoError(t, err)
			modifiedCode, err := ExtractCode(fileReader, "A", expectFile.funcline, "OutBoundCall", sugaredLogger)
			require.NoError(t, err)
			expectCode, err := os.ReadFile(expectpath)
			require.NoError(t, err)
			res := checkEquivalent(expectCode, []byte(modifiedCode))
			require.True(t, res)

		})

	}
}
