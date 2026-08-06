package skeletonize

import (
	"context"
	"fmt"
	"io"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

func parseFile(reader io.Reader) (*sitter.Node, []byte, error) {
	if reader == nil {
		return nil, nil, io.ErrUnexpectedEOF
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("read file %w", err)
	}
	// Create a new parser
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	// Parse the Go code
	parseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tree, err := parser.ParseCtx(parseCtx, nil, data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse file %w", err)
	}
	rootNode := tree.RootNode()

	return rootNode, data, nil
}

// Compare two trees and print differences if they are not equivalent
func compareTrees(node1, node2 *sitter.Node, sourceCode1, sourceCode2 []byte, indent string) bool {
	if node1.Type() != node2.Type() {
		fmt.Printf("%sDifference found:\n", indent)
		fmt.Printf("%sNode 1: %s [%s]\n", indent, node1.Type(), string(sourceCode1[node1.StartByte():node1.EndByte()]))
		fmt.Printf("%sNode 2: %s [%s]\n", indent, node2.Type(), string(sourceCode2[node2.StartByte():node2.EndByte()]))
		return false
	}

	if node1.ChildCount() != node2.ChildCount() {
		fmt.Printf("%sDifference found in child count:\n", indent)
		fmt.Printf("%sNode 1: %s [%s] has %d children\n", indent, node1.Type(), string(sourceCode1[node1.StartByte():node1.EndByte()]), node1.ChildCount())
		fmt.Printf("%sNode 2: %s [%s] has %d children\n", indent, node2.Type(), string(sourceCode2[node2.StartByte():node2.EndByte()]), node2.ChildCount())
		return false
	}

	for i := 0; i < int(node1.ChildCount()); i++ {
		child1 := node1.Child(i)
		child2 := node2.Child(i)
		if !compareTrees(child1, child2, sourceCode1, sourceCode2, indent+"  ") {
			return false
		}
	}

	return true
}

func printNode(node *sitter.Node, depth int, srccode []byte) {
	if node == nil {
		return
	}
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	fmt.Printf("%s%s: '%s'\n", indent, node.Type(), node.Content(srccode))
	for i := 0; i < int(node.ChildCount()); i++ {
		printNode(node.Child(i), depth+1, srccode)
	}
}
