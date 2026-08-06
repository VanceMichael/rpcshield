package jsonmode

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/uber-research/rpcshield/go/callnode"
	"github.com/uber-research/rpcshield/go/genai"
)

func markPath(leaf *callnode.Node, totalAnswer []genai.GenAnswer) {
	current := leaf
	isLeaf := true
	answerIndex := 0
	for current != nil {
		if isLeaf {
			current = current.Parent
			isLeaf = false
		} else {
			if answerIndex >= len(totalAnswer) {
				// should be all fail open
				if current.Parent == nil {
					current.TopSection.Status = append(current.TopSection.Status, "fail-open")
					current.TopSection.Explanation = append(current.TopSection.Explanation, "Fail-open status already returned by a previous call.")
				} else {
					current.Status = append(current.Status, "fail-open")
					current.Explanation = append(current.Explanation, "Fail-open status already returned by a previous call.")
				}
			} else {
				if current.Parent == nil {
					current.TopSection.Status = append(current.TopSection.Status, string(totalAnswer[answerIndex].Decision))
					shrinkExplanation := totalAnswer[answerIndex].Reason[5:]
					current.TopSection.Explanation = append(current.TopSection.Explanation, shrinkExplanation)
				} else {
					current.Status = append(current.Status, string(totalAnswer[answerIndex].Decision))
					shrinkExplanation := totalAnswer[answerIndex].Reason[5:]
					current.Explanation = append(current.Explanation, shrinkExplanation)
				}
			}
			current = current.Parent
			answerIndex++
		}
	}
}

func processOutput(opts *Options, totalRes []genai.AnswerPerChain, totalLeafNodes []callnode.Node, roots []callnode.Section) (*callnode.Data, error) {
	// in-place update the tree
	for _, res := range totalRes {
		index := res.ChainIndex
		answers := res.Answers
		markPath(&totalLeafNodes[index], answers)
	}
	if !opts.Debug {
		err := os.RemoveAll(filepath.Join(opts.OutputDir, "genout"))
		if err != nil {
			return nil, fmt.Errorf("Error deleting Debug genout directory: %v", err)
		}
	}
	outtree := annotateTree(roots)
	err := DumpOut(outtree, opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("Dump Output Json fails: %v", err)
	}
	return &outtree, nil
}

func getFailCloseEdge(status []string, explanation []string) int {
	index := -1
	for i, s := range status {
		if s == "fail-close" {
			index = i
		}
	}
	if index == -1 {
		// special case for all fail open
		for i, s := range explanation {
			if s != "Fail-open status already returned by a previous call." {
				index = i
			}
		}
	}
	return index
}

func cleanUpSection(section *callnode.Section) {
	if len(section.Status) != len(section.Explanation) {
		log.Fatal("<< Error : Status and Explanation length mismatch")
	}
	if len(section.Fcalls) > len(section.Status) {
		log.Fatal("<< Error : Missing status or explanation")
	} else if len(section.Fcalls) < len(section.Status) {
		// fmt.Println("Hit Section cleanup: ", section.Function, " : ", section.Status, " : ")
		index := getFailCloseEdge(section.Status, section.Explanation)
		if index == -1 {
			section.Status = []string{"fail-open"}
			section.Explanation = []string{"Fail-open status already returned by a previous call."}
		} else {
			section.Status = []string{section.Status[index]}
			section.Explanation = []string{section.Explanation[index]}
		}
	}
	for i := 0; i < len(section.Fcalls); i++ {
		cleanUp(&section.Fcalls[i])
	}
}

func cleanUp(callee *callnode.Node) {
	if len(callee.Status) != len(callee.Explanation) {
		log.Fatal("<< Error : Status and Explanation length mismatch")
	}
	if len(callee.Fcalls) > len(callee.Status) {
		log.Fatal("<< Error : Missing status or explanation")
	} else if len(callee.Fcalls) < len(callee.Status) {
		// fmt.Println("Hit node cleanup: ", callee.Function, " : ", callee.Status, " : ")
		index := getFailCloseEdge(callee.Status, callee.Explanation)
		if index == -1 {
			callee.Status = []string{"fail-open"}
			callee.Explanation = []string{"Fail-open status already returned by a previous call."}
		} else {
			callee.Status = []string{callee.Status[index]}
			callee.Explanation = []string{callee.Explanation[index]}
		}
	}
	for i := 0; i < len(callee.Fcalls); i++ {
		cleanUp(&callee.Fcalls[i])
	}
}

func annotateTree(roots []callnode.Section) callnode.Data {
	// clean up status and explanation
	for i := range roots {
		cleanUpSection(&roots[i])
	}
	data := callnode.Data{
		Meta:     make(map[string]interface{}),
		Sections: roots,
	}
	return data
}
