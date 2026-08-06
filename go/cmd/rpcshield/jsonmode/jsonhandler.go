package jsonmode

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"

	"github.com/uber-research/rpcshield/go/callnode"
	"gopkg.in/yaml.v3"
)

// ParseCallchain parses the JSON file and returns the call chain
func ParseCallchain(jfile string) ([]callnode.Section, error) {
	// simple checker rule:
	// 1. check if the funcCall startLine <= endLine
	// 2. check if the call site is within the parent function region
	// 3. check if the root node has all the required fields
	jsonData, err := os.ReadFile(jfile)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into the struct
	var data callnode.Data

	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, err
	}

	// Convert to tree structure and record leaf nodes
	sid := 0
	for i := range data.Sections {
		err := checkTreeValidity(&data.Sections[i])
		if err != nil {
			return nil, errors.New("Section " + strconv.Itoa(sid) + " is invalid :: " + err.Error())
		}
		sid++
	}
	return data.Sections, nil
}

// DumpOut write out the solution to the JSON
func DumpOut(data callnode.Data, outdir string) error {
	// Marshal the JSON data into the struct
	log.Println("Dumping final tree... at", outdir)
	outdata, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("json marshal error: %v", err)
	}
	// Write to a JSON file
	outputfile := outdir + "/rpcshield_output.json"
	err = os.WriteFile(outputfile, outdata, 0644)
	if err != nil {
		log.Fatalf("error writing json file: %v", err)
		return err
	}
	return nil
}

func checkTreeValidity(section *callnode.Section) error {
	rootCaller := callnode.Node{
		FuncBlock: callnode.FuncBlock{
			Function:  section.Function,
			File:      section.File,
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
		},
	}
	rooterr := checkFcallValidity(rootCaller, true, math.MinInt32, math.MaxInt32)

	if rooterr != nil {
		return rooterr
	}
	lb := section.FuncBlock.StartLine
	rb := section.FuncBlock.EndLine
	for i := range section.Fcalls {
		err := checkFcallValidity(section.Fcalls[i], false, lb, rb)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkFcallValidity(fcall callnode.Node, isRoot bool, lb int, rb int) error {
	if !isRoot {
		if fcall.FuncBlock.Function == "" || fcall.FuncBlock.File == "" ||
			fcall.FuncBlock.StartLine == 0 || fcall.FuncBlock.EndLine == 0 || fcall.Callsite == 0 {
			return errors.New("Non-root node missing field {function, file, startLine, endLine, callsite} at Function " + fcall.FuncBlock.Function)
		}
	} else {
		if fcall.FuncBlock.Function == "" || fcall.FuncBlock.File == "" ||
			fcall.FuncBlock.StartLine == 0 || fcall.FuncBlock.EndLine == 0 {
			return errors.New("Root node missing field {function, file, startLine, endLine} at Function " + fcall.FuncBlock.Function)
		}
	}
	if fcall.FuncBlock.StartLine > fcall.FuncBlock.EndLine {
		return errors.New("Function StartLine > EndLine at Function " + fcall.FuncBlock.Function)
	}
	if fcall.Callsite < lb || fcall.Callsite > rb {
		return errors.New("Function Callsite out of parent function region at Function " + fcall.FuncBlock.Function)
	}
	return nil
}

func setParentSection(section *callnode.Section) []callnode.Node {
	rootCaller := callnode.Node{
		FuncBlock: callnode.FuncBlock{
			Function:  section.FuncBlock.Function,
			File:      section.FuncBlock.File,
			StartLine: section.FuncBlock.StartLine,
			EndLine:   section.FuncBlock.EndLine,
		},
		TopSection: section,
	}
	var leafNodes []callnode.Node
	for i := 0; i < len(section.Fcalls); i++ {
		setParentCaller(&section.Fcalls[i], &rootCaller, &leafNodes)
	}
	return leafNodes
}

func setParentCaller(callee *callnode.Node, parent *callnode.Node, leafNodes *[]callnode.Node) {
	callee.Parent = parent
	if len(callee.Fcalls) == 0 {
		*leafNodes = append(*leafNodes, *callee)
	} else {
		for i := 0; i < len(callee.Fcalls); i++ {
			setParentCaller(&callee.Fcalls[i], callee, leafNodes)
		}
	}
}

type funcNode struct {
	FilePath  string `yaml:"filepath"`
	FuncName  string `yaml:"funcName"`
	StartLine int    `yaml:"startLine"`
	EndLine   int    `yaml:"endLine"`
}
type callPath struct {
	FunctionsChain []funcNode `yaml:"FunctionsChain"`
}

func dumpYAML(leaf *callnode.Node, pathID int, yamldir string) error {
	funcNodeList := make([]funcNode, 0)
	// maintain the order of the functions
	current := leaf
	for current != nil {
		funcNodeList = append(funcNodeList, funcNode{
			FilePath:  current.FuncBlock.File,
			FuncName:  current.FuncBlock.Function,
			StartLine: current.FuncBlock.StartLine,
			EndLine:   current.FuncBlock.EndLine,
		})
		current = current.Parent
	}
	c := callPath{FunctionsChain: funcNodeList}

	// Marshal the slice to YAML
	data, err := yaml.Marshal(&c)
	if err != nil {

		return fmt.Errorf("<< Error yaml marshal error: %v", err)
	}

	// Write to a YAML file
	err = os.WriteFile(yamldir+"/callpath."+strconv.Itoa(pathID)+".yaml", data, 0644)
	if err != nil {
		return fmt.Errorf("<< Error writing YAML file: %v", err)
	}

	return nil
}
