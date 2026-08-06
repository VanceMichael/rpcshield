package graph

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"

	"go.uber.org/zap/zapcore"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// Helper function to calculate median
func median(data []int) float64 {
	n := len(data)
	sort.Ints(data)
	if n%2 == 0 {
		return float64(data[n/2-1]+data[n/2]) / 2
	}
	return float64(data[n/2])
}

type graphStats struct {
	numNodes int
	numEdges int
	inAvg    float64
	inMax    int
	outAvg   float64
	outMax   int
}

// Analyze the call graph
func (g *builderImpl) logGraphStats(cg *callgraph.Graph, maxNodes int, level zapcore.Level) graphStats {
	numNodes := len(cg.Nodes)
	numEdges := 0
	incomingEdges := make([]int, numNodes)
	outgoingEdges := make([]int, numNodes)

	nodeIndex := make(map[*callgraph.Node]int)
	i := 0
	for _, node := range cg.Nodes {
		nodeIndex[node] = i
		i++
	}

	for _, node := range cg.Nodes {
		idx := nodeIndex[node]
		outgoingEdges[idx] = len(node.Out)
		numEdges += len(node.Out)
		for _, edge := range node.Out {
			calleeIdx := nodeIndex[edge.Callee]
			incomingEdges[calleeIdx]++
		}
	}

	g.logger.Logf(level, "Number of Nodes: %d", numNodes)
	g.logger.Logf(level, "Number of Edges: %d", numEdges)

	inSum, outSum := 0, 0
	inMax, outMax := 0, 0

	for _, in := range incomingEdges {
		inSum += in
		if in > inMax {
			inMax = in
		}
	}
	for _, out := range outgoingEdges {
		outSum += out
		if out > outMax {
			outMax = out
		}
	}

	inAvg := float64(inSum) / float64(numNodes)
	outAvg := float64(outSum) / float64(numNodes)

	g.logger.Logf(level, "Average In-Coming Edges: %.2f", inAvg)
	g.logger.Logf(level, "Median In-Coming Edges: %.2f", median(incomingEdges))
	g.logger.Logf(level, "Max In-Coming Edges: %d", inMax)
	g.logger.Logf(level, "Average Out-Going Edges: %.2f", outAvg)
	g.logger.Logf(level, "Median Out-Going Edges: %.2f", median(outgoingEdges))
	g.logger.Logf(level, "Max Out-Going Edges: %d", outMax)

	g.dumpTopNodes(cg, maxNodes)
	gs := graphStats{
		numNodes: numNodes,
		numEdges: numEdges,
		inAvg:    inAvg,
		inMax:    inMax,
		outAvg:   outAvg,
		outMax:   outMax,
	}
	return gs
}

// get the callsite instruction with maximum number of outgoing edges
func getMaxOutgoingEdges(myNode *callgraph.Node) (int, string) {
	// Make a map of callsite to count of outgoing edges
	edges := make(map[ssa.CallInstruction]int)
	maxEdgeCallSite := ssa.CallInstruction(nil)
	for _, edge := range myNode.Out {
		edges[edge.Site]++
		if maxEdgeCallSite == nil || edges[edge.Site] > edges[maxEdgeCallSite] {
			maxEdgeCallSite = edge.Site
		}
	}
	return edges[maxEdgeCallSite], getCodeSyntax(maxEdgeCallSite)
}

// get code syntax of the maxEdgeCallSite
func getCodeSyntax(maxEdgeCallSite ssa.CallInstruction) string {
	if maxEdgeCallSite == nil {
		return "maxEdgeCallSite nil"
	}
	// Print the instruction in full detail
	return maxEdgeCallSite.String()
}

func astToString(syntax ast.Node, fset *token.FileSet) string {
	var buf bytes.Buffer
	err := format.Node(&buf, fset, syntax)
	if err != nil {
		return "<error formatting source code>"
	}
	return buf.String()
}

func (g *builderImpl) printTopNodes(myNodes []*callgraph.Node, maxNodes int) {
	// Get top numNodes nodes by OutDegree
	found := 0

	// Function to get the function body
	getFunctionBody := func(function *ssa.Function) string {
		if syntax := function.Syntax(); syntax != nil {
			return astToString(syntax, function.Prog.Fset)
		}
		return "<no source available>"
	}
	for _, nd := range myNodes {
		function := nd.Func
		if function == nil || function.Prog == nil {
			continue
		}
		if function.Pkg == nil || function.Pkg.Pkg == nil {
			continue
		}
		// Want to see only internal code.
		if !strings.HasPrefix(function.Pkg.Pkg.Path(), g.config.CodebasePkgPrefix) {
			continue
		}
		// Ignore glue package.
		if strings.HasPrefix(function.Pkg.Pkg.Path(), g.config.CodebasePkgPrefix+"/glue") {
			continue
		}

		if found >= maxNodes {
			break
		}
		found++
		sig := function.Signature.String()
		pos := function.Pos()
		posn := function.Prog.Fset.Position(pos)
		body := getFunctionBody(function)

		g.logger.Debugf("Degree: %d %d", len(nd.In), len(nd.Out))
		g.logger.Debugf("Signature: %s", sig)
		g.logger.Debugf("Location: %s:%d", posn.Filename, posn.Line)
		count, loc := getMaxOutgoingEdges(nd)
		g.logger.Debugf("Max Outgoing Edge %d Callsite: %s", count, loc)
		g.logger.Debugf("Body:\n%s\n", body)
		g.logger.Debugf("------")
	}
}

func (g *builderImpl) dumpTopNodes(cg *callgraph.Graph, maxNodes int) {
	numNodes := len(cg.Nodes)
	graphNodes := make([]*callgraph.Node, 0, numNodes)
	for _, node := range cg.Nodes {
		if node == nil {
			continue
		}
		if node.Func == nil {
			continue
		}
		if node.Out == nil {
			continue
		}
		if node.In == nil {
			continue
		}
		graphNodes = append(graphNodes, node)
	}

	g.logger.Debugf("Top %d nodes by OutDegree:\n", maxNodes)
	// Sort nodes by OutDegree in descending order
	sort.Slice(graphNodes, func(i, j int) bool {
		return len(graphNodes[i].Out) > len(graphNodes[j].Out)
	})

	// Print top N nodes by OutDegree
	g.printTopNodes(graphNodes, maxNodes)

	g.logger.Debugf("Top %d nodes by InDegree:\n", maxNodes)
	// Print top N nodes by InDegree
	sort.Slice(graphNodes, func(i, j int) bool {
		return len(graphNodes[i].In) > len(graphNodes[j].In)
	})
	g.printTopNodes(graphNodes, maxNodes)
}

func (g *builderImpl) dumpSSAFunction(fn *ssa.Function) {
	var builder strings.Builder
	fn.WriteTo(&builder)
	g.logger.Debugf("SSAFunctions:\n%s\n", builder.String())
}

func (g *builderImpl) dumpSSAFunctionsWithRegexp(cg *callgraph.Graph) {
	// Read the SSA_DUMP_REGEXP env var
	dump := os.Getenv("SSA_DUMP_REGEXP")
	if dump == "" {
		return
	}
	regexp, err := regexp.Compile(dump)
	if err != nil {
		g.logger.Errorf("Error compiling regexp: %v", err)
		return
	}
	// If any node's function name matches the regexp, dump the function
	for _, node := range cg.Nodes {
		if node == nil {
			continue
		}
		if node.Func == nil {
			continue
		}
		if regexp.MatchString(node.Func.String()) {
			g.dumpSSAFunction(node.Func)
		}
	}
}
