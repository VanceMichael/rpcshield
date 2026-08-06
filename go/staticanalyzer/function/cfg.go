package function

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/types/typeutil"
)

// CFGBuilder is a helper struct to build the control flow graph (BuildAndTransformCFG) for a given function declaration.
type CFGBuilder struct {
	decl       ast.Node // Can be *ast.FuncDecl or *ast.FuncLit
	astContext *asthelper.ASTContext

	// funcDecls is a field of ctrlflow.BuildAndTransformCFG (golang.org/x/tools/go/analysis/passes/ctrlflow/ctrlflow.go) that is used for building the BuildAndTransformCFG.
	// It stores information about all function declarations in the package.
	funcDecls map[*types.Func]*declInfo

	// noReturnFuncs simulates the inter-procedural "no return" fact used by ctrlflow.
	// It is scoped per CFGBuilder instance to eliminate data races and prevent stale state
	// from leaking across independent CFG constructions (e.g., parallel tests or successive
	// invocations with different packages).
	noReturnFuncs map[*types.Func]*noReturn
}

// buildCFG constructs the control flow graph (BuildAndTransformCFG) for the given function declaration.
func (c *CFGBuilder) buildCFG() *cfg.CFG {
	// First, find all function declarations in the package.
	c.funcDecls = make(map[*types.Func]*declInfo)
	c.noReturnFuncs = make(map[*types.Func]*noReturn)
	for _, file := range c.astContext.PkgInfo.Pkg.Syntax {
		for _, decl := range file.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				obj := c.astContext.ObjectOf(funcDecl.Name)
				if obj == nil {
					continue
				}
				funcObj, ok := obj.(*types.Func)
				if !ok {
					continue
				}
				c.funcDecls[funcObj] = &declInfo{decl: funcDecl}
			}
		}
	}

	var graph *cfg.CFG
	switch f := c.decl.(type) {
	case *ast.FuncDecl:
		di := &declInfo{decl: f}
		funcObj := c.astContext.ObjectOf(f.Name).(*types.Func)
		c.buildDecl(funcObj, di)
		graph = di.cfg

	case *ast.FuncLit:
		graph = cfg.New(f.Body, c.callMayReturn)
	default:
		panic("unsupported function declaration type")
	}

	return graph
}

// <START> -----------------------------------------------------------------------------
// Below code for BuildAndTransformCFG construction is by and large copied from golang.org/x/tools/go/analysis/passes/ctrlflow/ctrlflow.go
// The parts not applicable to our analysis, such as exporting or importing facts, are commented out and replaced with
// our own logic marked with the comment "Added by us".

// noReturn is a fact indicating that a function does not return.
type noReturn struct{}

type declInfo struct {
	decl     *ast.FuncDecl
	cfg      *cfg.CFG // iff decl.Body != nil
	started  bool     // to break cycles
	noReturn bool
}

// di.cfg may be nil on return.
func (c *CFGBuilder) buildDecl(fn *types.Func, di *declInfo) {
	// buildDecl may call itself recursively for the same function,
	// because cfg.New is passed the callMayReturn method, which
	// builds the CFG of the callee, leading to recursion.
	// The buildDecl call tree thus resembles the static call graph.
	// We mark each node when we start working on it to break cycles.

	if !di.started { // break cycle
		di.started = true

		if isIntrinsicNoReturn(fn) {
			di.noReturn = true
		}
		if di.decl.Body != nil {
			di.cfg = cfg.New(di.decl.Body, c.callMayReturn)
			if !hasReachableReturn(di.cfg) {
				di.noReturn = true
			}
		}

		// Added by us
		if di.noReturn {
			c.noReturnFuncs[fn] = new(noReturn)
		}

		// if di.noReturn {
		// 	c.pass.ExportObjectFact(fn, new(noReturn))
		// }
		//
		// // debugging
		// if false {
		// 	log.Printf("CFG for %s:\n%s (noreturn=%t)\n", fn, di.cfg.Format(c.pass.Fset), di.noReturn)
		// }
	}
}

// callMayReturn reports whether the called function may return.
// It is passed to the BuildAndTransformCFG builder.
func (c *CFGBuilder) callMayReturn(call *ast.CallExpr) (r bool) {
	if id, ok := call.Fun.(*ast.Ident); ok && c.astContext.PkgInfo.Pkg.TypesInfo.Uses[id] == panicBuiltin {
		return false // panic never returns
	}

	// "Some driver implementations (such as those based on Bazel and Blaze) do not
	// currently apply analyzers to packages of the standard library. Therefore, for
	// best results, analyzer authors should not rely on analysis facts being available
	// for standard packages." [1].
	// This means that we cannot rely on the noReturn fact being available for standard
	// library functions.
	// Therefore, here we check if the function is an intrinsic no-return function.
	//
	// [1]: https://pkg.go.dev/golang.org/x/tools/go/analysis
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if fn, ok := c.astContext.PkgInfo.Pkg.TypesInfo.ObjectOf(sel.Sel).(*types.Func); ok && isIntrinsicNoReturn(fn) {
			return false // intrinsic no-return functions
		}
	}

	// Is this a static call? Also includes static functions
	// parameterized by a type. Such functions may or may not
	// return depending on the parameter type, but in some
	// cases the answer is definite. We let ctrlflow figure
	// that out.
	fn := typeutil.StaticCallee(c.astContext.PkgInfo.Pkg.TypesInfo, call)
	if fn == nil {
		return true // callee not statically known; be conservative
	}

	// Function or method declared in this package?
	if di, ok := c.funcDecls[fn]; ok {
		c.buildDecl(fn, di)
		return !di.noReturn
	}

	// Added by us
	return c.noReturnFuncs[fn] == nil

	// // Not declared in this package.
	// // Is there a fact from another package?
	// return !c.pass.ImportObjectFact(fn, new(noReturn))
}

// isIntrinsicNoReturn reports whether a function intrinsically never
// returns because it stops execution of the calling thread.
// It is the base case in the recursion.
func isIntrinsicNoReturn(fn *types.Func) bool {
	// Add functions here as the need arises, but don't allocate memory.
	path, name := fn.Pkg().Path(), fn.Name()
	return path == "syscall" && (name == "Exit" || name == "ExitProcess" || name == "ExitThread") ||
		(path == "runtime" && name == "Goexit") ||
		// Ideally we should not need to list the following functions since they
		// internally call the functions above. However, we will only know that _if_
		// we also analyze the standard library. For some driver implementations like
		// Bazel it would not do so currently. Therefore, we list them here.
		(path == "log" && (name == "Fatal" || name == "Fatalf")) ||
		(path == "os" && (name == "Exit")) ||
		(path == "testing" && (name == "FailNow" || name == "Skip" || name == "Skipf" || name == "SkipNow"))
}

var panicBuiltin = types.Universe.Lookup("panic").(*types.Builtin)

func hasReachableReturn(g *cfg.CFG) bool {
	for _, b := range g.Blocks {
		if b.Live && b.Return() != nil {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------- <END>

// BuildAndTransformCFG first builds the CFG for the given function declaration, and then performs several
// transformations on the CFG normalize and canonicalize it for further analysis.
func BuildAndTransformCFG(a *asthelper.ASTContext, funcDecl ast.Node) *cfg.CFG {
	// Build the CFG for the function.
	cfgBuilder := &CFGBuilder{
		astContext: a,
		decl:       funcDecl,
	}
	graph := cfgBuilder.buildCFG()

	// Perform CFG transformations.

	// Restructure switch statements to have a proper if-else structure.
	restructureSwitchDefault(graph)
	transformTypeSwitchToIfElse(graph)

	for _, block := range graph.Blocks {
		if block.Live {
			// Canonicalize conditionals.
			restructureConditional(a, graph, block)
			// Normalize named return statements.
			normalizeNamedReturn(funcDecl, block)
		}
	}

	// Add an empty "return" block as a successor for:
	// - all returning blocks
	// - while loops (`for <EXPR> {}` or `for {}`)
	numBlocks := len(graph.Blocks)
	blocks := append(graph.Blocks, &cfg.Block{
		Nodes: nil,
		Succs: nil,
		Index: int32(numBlocks),
		Live:  true,
	})
	for i := 0; i < numBlocks; i++ {
		succ := blocks[i].Succs
		if blocks[i].Return() != nil || (len(succ) == 1 && succ[0].Index == blocks[i].Index) {
			blocks[i].Succs = append(blocks[i].Succs, blocks[numBlocks])
		}
	}
	graph.Blocks = blocks

	return graph
}

// restructureConditional canonicalizes the conditionals in the given block. Specifically, it performs the following transformations:
// Canonicalize conditionals:
// - replace `if !cond {T} {F}` with `if cond {F} {T}` (swap successors)
// - replace `if cond1 && cond2 {T} {F}` with `if cond1 {if cond2 {T} else {F}}{F}` (nesting)
// - replace `if cond1 || cond2 {T} {F}` with `if cond1 {T} else {if cond2 {T} else {F}}` (nesting)
//
// Canonicalize nil comparisons:
// It also performs the following useful transformation:
// - replace `if x != nil {T} {F}` with `if x == nil {F} {T}` (swap successors)
// - replace `nil == x {T} {F}` with `if x == nil {T} {F}` (swap comparison order)
func restructureConditional(a *asthelper.ASTContext, graph *cfg.CFG, thisBlock *cfg.Block) {
	// We only restructure non-empty branching blocks.
	if len(thisBlock.Nodes) == 0 || len(thisBlock.Succs) != 2 {
		return
	}
	cond, ok := thisBlock.Nodes[len(thisBlock.Nodes)-1].(ast.Expr)
	if !ok {
		return
	}

	// places a new given node into the last position of this block
	replaceCond := func(node ast.Node) {
		thisBlock.Nodes[len(thisBlock.Nodes)-1] = node
	}

	trueBranch := thisBlock.Succs[0]  // type *cfg.Block
	falseBranch := thisBlock.Succs[1] // type *cfg.Block

	replaceTrueBranch := func(block *cfg.Block) {
		thisBlock.Succs[0] = block
	}
	replaceFalseBranch := func(block *cfg.Block) {
		thisBlock.Succs[1] = block
	}

	swapTrueFalseBranches := func() {
		replaceTrueBranch(falseBranch)
		replaceFalseBranch(trueBranch)
	}

	switch cond := cond.(type) {
	case *ast.ParenExpr:
		// if a parenexpr, strip and restart - this is done with recursion to account for ((((x)))) case
		replaceCond(cond.X)
		restructureConditional(a, graph, thisBlock) // recur within parens
	case *ast.UnaryExpr:
		if cond.Op == token.NOT {
			// swap successors - i.e. swap true and false branches
			swapTrueFalseBranches()
			replaceCond(cond.X)
			restructureConditional(a, graph, thisBlock) // recur within NOT
		}
	case *ast.BinaryExpr:
		// Logical `AND` and logical `OR` actually require the exact same short-circuiting behavior
		// except for whether the true or false branch leads to the short-circuiting. This split
		// is captured by the following switch, and, as can be observed, all other logic is the same
		binShortCircuit := func(replaceWhichBranch bool) {
			replaceCond(cond.X)
			newBlock := &cfg.Block{
				Nodes: []ast.Node{cond.Y},
				Succs: []*cfg.Block{trueBranch, falseBranch},
				Index: int32(len(graph.Blocks)),
				Live:  true,
			}
			if replaceWhichBranch {
				replaceTrueBranch(newBlock)
			} else {
				replaceFalseBranch(newBlock)
			}
			graph.Blocks = append(graph.Blocks, newBlock)
			restructureConditional(a, graph, thisBlock)
			restructureConditional(a, graph, newBlock)
		}

		// Standardize binary expressions to be of the form `expr OP literal` by swapping `x` and `y`, if `x` is a literal.
		// For example, standardizes `nil == v` to the `v == nil` form
		x, y := cond.X, cond.Y
		if a.IsNil(x) {
			newCond := &ast.BinaryExpr{
				// Swap X and Y
				X:     y,
				Y:     x,
				Op:    cond.Op,
				OpPos: cond.OpPos,
			}
			replaceCond(newCond)
			x, y = y, x
		}

		switch cond.Op {
		case token.LAND:
			binShortCircuit(true)
		case token.LOR:
			binShortCircuit(false)

		// The NEQ and EQL cases here rewrite the ASTs to ensure all _nil comparisons_ are
		// standardized to the form of `x == nil` (i.e., "variable" == "literal nil"). For example:
		// (1) x != nil -> x == nil, and swapping the true and false branches in the CFG.

		// Note that we _should not_ directly modify the AST nodes, since they are shared across
		// other nogo analyzers. Instead, whenever a rewrite is needed we create a new AST node
		// and replace the original node pointer with the clone in the block.Nodes slice instead.
		case token.NEQ:
			// Rewrite when operand `y` is a literal `nil`.
			if a.IsNil(y) {
				// Copy the AST Node first.
				newCond := &ast.BinaryExpr{
					X:     x,
					Y:     y,
					Op:    token.EQL, // As discussed, we change the operator to EQL here.
					OpPos: cond.OpPos,
				}
				// Replace the condition, and swap the branches since we modified a NEQ conditional
				// to a EQL one.
				replaceCond(newCond)
				swapTrueFalseBranches()
				break
			}
		default:
			// noop
		}
	}
}

// normalizeNamedReturn normalizes the named return statements in the given block to normal return statements.
// For example, consider the below function:
//
//	func foo() (resp *S, err error) {
//		resp, err = bar()
//		return
//	}
//
// Here the bare `return` is transformed to `return resp, err`.
// We do this by appending the named return values to the bare return statement.
//
// NOTE: This modifies the AST in place. It is fine for the current whole program analysis
// approach, but we would need to modify this behavior if we move to Bazel+nogo system
// in the future, since there different analyzers share the same ASTs.
func normalizeNamedReturn(funcDecl ast.Node, thisBlock *cfg.Block) {
	if len(thisBlock.Nodes) == 0 {
		return
	}

	// Check if the last node is a return statement
	retStmt, ok := thisBlock.Nodes[len(thisBlock.Nodes)-1].(*ast.ReturnStmt)
	if !ok {
		return
	}

	// Check if the return statement is a bare return, indicating that it is a named return.
	if len(retStmt.Results) != 0 {
		return
	}

	// Fetch return values from the function declaration and append them to the return statement
	var funcSigResults *ast.FieldList
	switch f := funcDecl.(type) {
	case *ast.FuncDecl:
		funcSigResults = f.Type.Results
	case *ast.FuncLit:
		funcSigResults = f.Type.Results
	default:
		return
	}

	if funcSigResults != nil && len(funcSigResults.List) > 0 {
		for _, field := range funcSigResults.List {
			for _, retVariable := range field.Names {
				retStmt.Results = append(retStmt.Results, retVariable)
			}
		}
	}
}

// restructureSwitchDefault canonicalizes the default case of a switch statement to a case block
// allowing existing logic for handling "case" blocks to also handle "default" blocks without any special handling.
func restructureSwitchDefault(graph *cfg.CFG) {
	for _, block := range graph.Blocks {
		if block.Kind == cfg.KindSwitchNextCase && len(block.Nodes) > 0 {
			// This is the "default" case of a switch statement. To normalize it as other "case" blocks,
			// we create a new block for the "default" case body and make it the successor of this block.
			// This will allow existing logic for handling "case" blocks to also handle "default" blocks
			// without any special handling.
			newDefaultBodyBlock := &cfg.Block{
				Nodes: block.Nodes,
				Succs: block.Succs,
				Index: int32(len(graph.Blocks)),
				Live:  true,
				Kind:  cfg.KindSwitchCaseBody,
				Stmt:  block.Stmt,
			}
			block.Nodes = []ast.Node{}
			block.Succs = []*cfg.Block{newDefaultBodyBlock}
			graph.Blocks = append(graph.Blocks, newDefaultBodyBlock)

			// If the "default" case only has one successor (i.e., the "default" case body).
			// We add an empty block as the second successor to make it a proper if-else structure.
			emptyBlock := &cfg.Block{
				Nodes: []ast.Node{},
				Succs: []*cfg.Block{},
				Index: int32(len(graph.Blocks)),
				Live:  true,
			}
			graph.Blocks = append(graph.Blocks, emptyBlock)
			block.Succs = append(block.Succs, emptyBlock)
		}
	}
}

// transformTypeSwitchToIfElse transforms the type switch cases to if-else structures by adding binary expressions to the case blocks.
// For example, consider the below function:
//
//	func foo() {
//		switch err.(type) {
//		case *InternalError:
//			...
//		case *OtherError:
//			...
//		}
//	}
//
// Here the type switch is transformed to:
//
//	func foo() {
//			if _, ok := err.(*InternalError); err != nil && ok {
//				...
//			} else if _, ok := err.(*OtherError); err != nil && ok {
//				...
//			}
//		}
func transformTypeSwitchToIfElse(graph *cfg.CFG) {
	// Track blocks that have been processed to avoid reprocessing
	processedBlocks := make(map[int32]bool)

	var switchExpr ast.Expr
	for _, block := range graph.Blocks {
		if !block.Live || processedBlocks[block.Index] {
			continue
		}

		// Look for type assertion patterns that indicate a type switch case
		expr, ok := asTypeSwitchCaseBlock(block)
		if ok && expr != nil {
			switchExpr = expr
		}
		if ok && switchExpr != nil {
			transformTypeSwitchCaseToIfElse(graph, block, switchExpr)
			processedBlocks[block.Index] = true
		}
	}
}

// asTypeSwitchCaseBlock checks if the given block is a type switch case block.
func asTypeSwitchCaseBlock(block *cfg.Block) (ast.Expr, bool) {
	// Switch statement: Check if the last node is an expression statement containing a type assertion
	if len(block.Nodes) > 0 && len(block.Succs) == 2 {
		if exprStmt, ok := block.Nodes[len(block.Nodes)-1].(*ast.ExprStmt); ok {
			if typeAssert, ok := exprStmt.X.(*ast.TypeAssertExpr); ok {
				return typeAssert.X, true
			}
		}
	}

	// Case clause: Check if the kind is of type case
	return nil, block.Kind == cfg.KindSwitchNextCase
}

// transformTypeSwitchCaseToIfElse transforms the type switch case to an if-else structure by adding a binary expression to the case block.
// For example, consider the below switch statement:
//
//	switch err.(type) {
//	case *InternalError:
//		// ...
//	}
//
// Here the type switch case is transformed to:
//
//	if _, ok := err.(*InternalError); err != nil && ok {
//		// ...
//	}
func transformTypeSwitchCaseToIfElse(graph *cfg.CFG, block *cfg.Block, switchExpr ast.Expr) {
	if len(block.Succs) == 0 {
		// No successors to transform
		return
	}

	// Get case clause
	var caseType ast.Expr
	if caseClause, ok := block.Succs[0].Stmt.(*ast.CaseClause); ok {
		// Set the type in the type assertion
		if len(caseClause.List) == 1 {
			caseType = caseClause.List[0]
		} else {
			// This should not happen in a well-formed type switch
			panic("unexpected multiple types in type switch case")
		}
	} else {
		// Not a case clause, cannot transform.
		return
	}

	// Create type assertion with comma ok: _, ok := expr.(Type)
	assignment := &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.Ident{Name: "_"},
			&ast.Ident{Name: "ok"},
		},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.TypeAssertExpr{
			X:    switchExpr,
			Type: caseType,
		}},
	}

	// Create the condition: ok
	okCondition := &ast.Ident{Name: "ok"}

	binaryExpr := &ast.BinaryExpr{
		X: &ast.BinaryExpr{
			X:  switchExpr,
			Op: token.NEQ,
			Y: &ast.Ident{
				Name: "nil",
			},
		},
		Op: token.LAND,
		Y:  okCondition,
	}

	// Add new nodes
	block.Nodes = append(block.Nodes, assignment)
	block.Nodes = append(block.Nodes, binaryExpr)
}
