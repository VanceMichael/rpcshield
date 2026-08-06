package output

import (
	"bytes"
	_ "embed" // Using `embed` to embed the HTML template file
	"fmt"
	"html"
	"html/template"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/uber-research/rpcshield/go/callnode"
)

//go:embed htmltemplate.txt
var _htmlTemplate string

const (
	_red     = "#FF0000"
	_green   = "#008000"
	_gray    = "#808080"
	_svgFont = "helvetica"

	_callChainArrowStyle = "[style=dashed, fillcolor=\"" + _gray + "\", color=\"" + _gray + "\"]"
	_failCloseArrowStyle = "[color=\"" + _red + "\", penwidth=5.0]"
	_failOpenArrowStyle  = "[color=\"" + _green + "\", penwidth=5.0]"
	_arrowASCII          = "&#45;&gt;"
	_svgFontTag          = "[fontname = \"" + _svgFont + "\"];"
)

var _visitedMap = make(map[string]int)
var _funcInfoMap = make(map[string]*callnode.FuncBlock)
var _edgeInfoMap = make(map[string]*edgeInfo)
var _edgeElementRegex = regexp.MustCompile(`(?s)(<g id="[^"]+" class="edge resultEdge">\s*<title>)([^<]*)(</title>)`)
var _nodeElementRegex = regexp.MustCompile(`(?s)(<g id="[^"]+" class="node">\s*<title>)([^<]*)(</title>)`)

type edgeInfo struct {
	parent      string
	child       string
	status      []string
	explanation []string
}

func dotGraphPrefix() string {
	return fmt.Sprintf("digraph G {\n"+
		"graph %s\n"+
		"node %s\n"+
		"edge %s\n", _svgFontTag, _svgFontTag, _svgFontTag)
}

func dotGraphSuffix() string {
	return "}\n"
}

// buildDotGraph builds a DOT graph from the input JSON data.
func buildDotGraph(input callnode.Data) []string {
	var dotGraphBySection []string

	for _, section := range input.Sections {
		var dotGraph bytes.Buffer
		dotGraph.WriteString(dotGraphPrefix())

		if _, ok := _visitedMap[section.Function]; !ok {
			_visitedMap[section.Function] = 0
		}
		_visitedMap[section.Function] = _visitedMap[section.Function] + 1
		topLevelCaller := callnode.Node{
			FuncBlock:   section.FuncBlock,
			Status:      section.Status,
			Explanation: section.Explanation,
			Fcalls:      section.Fcalls,
		}
		addNodes(&dotGraph, topLevelCaller)

		dotGraph.WriteString(dotGraphSuffix())
		dotGraphBySection = append(dotGraphBySection, dotGraph.String())
	}

	return dotGraphBySection
}

// addNodes recursively adds nodes to the DOT graph.
func addNodes(buf *bytes.Buffer, caller callnode.Node) {
	callerStr := fmt.Sprintf("%s_%d", caller.Function, _visitedMap[caller.Function])
	callerLabel := fmt.Sprintf("\"%s\" [label=\"%s\"]", callerStr, shortenFuncName(caller.Function))

	if _funcInfoMap[callerStr] == nil {
		_funcInfoMap[callerStr] = &caller.FuncBlock
	}

	for i, callee := range caller.Fcalls {
		if _, ok := _visitedMap[callee.Function]; !ok {
			_visitedMap[callee.Function] = 0
		}
		_visitedMap[callee.Function] = _visitedMap[callee.Function] + 1

		calleeStr := fmt.Sprintf("%s_%d", callee.Function, _visitedMap[callee.Function])
		calleeLabel := fmt.Sprintf("\"%s\" [label=\"%s\"]", calleeStr, shortenFuncName(callee.Function))
		_funcInfoMap[calleeStr] = &callee.FuncBlock

		// Check if leaf node. If yes, draw it as a parallelogram
		leafNodeShape := ""
		if len(callee.Fcalls) == 0 {
			leafNodeShape = fmt.Sprintf(" \"%s\" [shape=parallelogram, style=filled, fillcolor=\"lightgray\"]", calleeStr)
		}

		// Create caller-callee nodes and a forward arrow showing "caller" calling "callee"
		buf.WriteString(fmt.Sprintf("\"%s\" -> \"%s\" %s %s %s;%s\n", callerStr, calleeStr, _callChainArrowStyle, callerLabel, calleeLabel, leafNodeShape))

		// Determine arrows for showing fail-open and -close between caller and callee
		edgeID := calleeStr + _arrowASCII + callerStr
		edgeIDClass := fmt.Sprintf("[id=\"%s\" class=\"resultEdge\"]", edgeID)
		arrowStyle := _failOpenArrowStyle
		if len(caller.Fcalls) == len(caller.Status) {
			_edgeInfoMap[callerStr+_arrowASCII+calleeStr] = &edgeInfo{caller.Function, callee.Function, []string{caller.Status[i]}, []string{caller.Explanation[i]}}
			if caller.Status[i] == "fail-close" {
				arrowStyle = _failCloseArrowStyle
			}
		} else {
			_edgeInfoMap[callerStr+_arrowASCII+calleeStr] = &edgeInfo{caller.Function, callee.Function, caller.Status, caller.Explanation}
			for _, s := range caller.Status {
				if s == "fail-close" {
					arrowStyle = _failCloseArrowStyle
					break
				}
			}
		}
		// Create callee-caller nodes again, to now draw a backward arrow showing the flow of error propagation
		buf.WriteString(fmt.Sprintf("\"%s\" -> \"%s\" %s %s %s %s;%s\n", calleeStr, callerStr, edgeIDClass, arrowStyle, calleeLabel, callerLabel, leafNodeShape))

		addNodes(buf, callee)
	}
}

func shortenFuncName(fullName string) string {
	f := strings.Split(fullName, ".")
	return f[len(f)-1]
}

// buildSVG builds an SVG representation of the DOT graph.
func buildSVG(dotGraph string) string {
	cmd := exec.Command("dot", "-Tsvg")
	cmd.Stdin = bytes.NewReader([]byte(dotGraph))
	svgOutput, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	rawSVG := string(svgOutput)
	enhancedSVG := postProcessSVG(rawSVG)

	return enhancedSVG
}

// postProcessSVG processes the SVG content to refine the information displayed for nodes and edges. Specifically,
// - For nodes, it updates the tooltip to display function name, file path, and start line number.
// - For edges, it updates the tooltip to display caller, callee, and status. It also adds a hidden explanation and status
// section that is displayed when the user clicks on the edge.
func postProcessSVG(svg string) string {
	// Process the node block
	processNodeBlock := func(node string) string {
		matches := _nodeElementRegex.FindStringSubmatch(node)
		titleNode := ""
		if len(matches) > 2 {
			titleNode = matches[2] // Original title content
		}
		if v, ok := _funcInfoMap[titleNode]; ok {
			newTitle := fmt.Sprintf("Function name: %s\n\nFile path: %s\n\nStart line: %d", v.Function, v.File, v.StartLine)
			return _nodeElementRegex.ReplaceAllString(node, fmt.Sprintf("${1}%s${3}", newTitle))
		}
		return node
	}

	// Process the edge block
	processEdgeBlock := func(node string) string {
		matches := _edgeElementRegex.FindStringSubmatch(node)
		titleEdge := ""
		if len(matches) > 2 {
			titleEdge = matches[2] // Original title content
		}
		if len(titleEdge) > 0 {
			// Insert the status and explanation after the </title> tag
			arr := strings.Split(titleEdge, _arrowASCII)
			key := arr[1] + _arrowASCII + arr[0]
			if v, ok := _edgeInfoMap[key]; ok {
				// Format explanation string to be pretty-printed in HTML
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("<span class=\"label\">Caller:</span> %s <br>", v.parent))
				sb.WriteString(fmt.Sprintf("<span class=\"label\">Callee:</span> %s <br>", v.child))
				sb.WriteString(fmt.Sprintf("<span class=\"label\">Status:</span> %s <br><br>", strings.Join(v.status, ", ")))

				if len(v.explanation) == 1 {
					sb.WriteString(fmt.Sprintf("<span class=\"label\">Explanation:</span><br><br> %s", v.explanation[0]))
				} else {
					for i, e := range v.explanation {
						sb.WriteString(fmt.Sprintf("<br><span class=\"label\">Path %d: Status: %s, Explanation:</span><br><br> %s <br><hr><br>", (i + 1), v.status[i], e))
					}
				}

				explanation := fmt.Sprintf("<explanation style=\"display: none;\">%s</explanation>", escapeForHTML(sb.String()))

				newTitle := fmt.Sprintf("Caller: %s\nCallee: %s\nStatus: %s", strings.Split(arr[0], "_")[0], strings.Split(arr[1], "_")[0], v.status)
				node = _edgeElementRegex.ReplaceAllString(node, fmt.Sprintf("${1}%s${3}", newTitle))

				edgeBlock := node + "\n" + explanation + "\n"

				return edgeBlock
			}
			return node
		}
		return node
	}

	// Apply replacement function to each node and edge block
	updatedSVG := _nodeElementRegex.ReplaceAllStringFunc(svg, processNodeBlock)
	updatedSVG = _edgeElementRegex.ReplaceAllStringFunc(updatedSVG, processEdgeBlock)

	// Remove the title tag for the graph
	updatedSVG = strings.Replace(updatedSVG, "<title>G</title>", "", 1)

	return updatedSVG
}

// escapeForHTML encodes backslashes and specific control characters as HTML entities.
func escapeForHTML(input string) string {
	input = html.EscapeString(input)

	// Replace backslashes first to avoid double escaping
	replacer := strings.NewReplacer(
		"\\", "&#92;",
		"\n", "&#92;n",
		"\t", "&#92;t",
		"\u003c", "&#92;u003c", // Unicode less-than sign '<'
		"\u003e", "&#92;u003e", // Unicode greater-than sign '>'
	)
	return replacer.Replace(input)
}

type arrow int8

const (
	_callArrow arrow = iota
	_failCloseArrow
	_failOpenArrow
)

func helpArrowSVG(a arrow) string {
	var dotGraph bytes.Buffer
	dotGraph.WriteString(dotGraphPrefix())
	switch a {
	case _callArrow:
		dotGraph.WriteString(fmt.Sprintf("caller -> callee %s", _callChainArrowStyle))
	case _failCloseArrow:
		dotGraph.WriteString(fmt.Sprintf("caller -> callee %s [dir=\"back\"]", _failCloseArrowStyle))
	case _failOpenArrow:
		dotGraph.WriteString(fmt.Sprintf("caller -> callee %s [dir=\"back\"]", _failOpenArrowStyle))
	}
	dotGraph.WriteString(dotGraphSuffix())
	return buildSVG(dotGraph.String())
}

// GenerateHTMLReport is the main driver for creating the HTML output report.
func GenerateHTMLReport(input callnode.Data, outputHTMLFilePath string) error {
	dotGraphs := buildDotGraph(input)
	var htmlSVGs []template.HTML
	for _, g := range dotGraphs {
		svg := buildSVG(g)
		htmlSVGs = append(htmlSVGs, template.HTML(svg))
	}

	t, err := template.New("report").Parse(_htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %v", err)
	}

	data := struct {
		SVGs           []template.HTML
		ArrowCall      template.HTML
		ArrowFailClose template.HTML
		ArrowFailOpen  template.HTML
	}{
		SVGs:           htmlSVGs,
		ArrowCall:      template.HTML(helpArrowSVG(_callArrow)),
		ArrowFailClose: template.HTML(helpArrowSVG(_failCloseArrow)),
		ArrowFailOpen:  template.HTML(helpArrowSVG(_failOpenArrow)),
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, data)
	if err != nil {
		return fmt.Errorf("failed to execute HTML template: %v", err)
	}

	// Write the HTML content to a file
	if err = os.WriteFile(outputHTMLFilePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write HTML content to file: %v", err)
	}
	return nil
}
