package genai

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type funcNode struct {
	FilePath  string `yaml:"filepath"`
	FuncName  string `yaml:"funcName"`
	StartLine int    `yaml:"startLine"`
	EndLine   int    `yaml:"endLine"`
}
type callPath struct {
	FunctionsChain []funcNode `yaml:"FunctionsChain"`
}

func readYamlFile(path string) (*callPath, error) {
	llmCallPath := &callPath{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file: %v", err)
	}
	err = yaml.Unmarshal(data, llmCallPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal yaml: %v", err)
	}
	return llmCallPath, nil
}

func dumpFuncSource(filePath string, startLine, endLine int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(file)
	currentLine := 1

	for scanner.Scan() {
		if currentLine >= startLine && currentLine <= endLine {
			result.WriteString(scanner.Text() + "\n")
		}
		if currentLine > endLine {
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return result.String(), nil
}
