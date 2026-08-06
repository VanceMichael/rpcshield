package anonymousfunc

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/packages"
)

// _fakeFuncDeclPrefix is prepended to the fake func decl node we generated for the func lit nodes.
// It contains an illegal character to avoid conflicts with other variables.
const _fakeFuncDeclPrefix = "__anonymousFunction$"

// FuncLitInfo stores information about a func lit that is useful in the main analysis.
type FuncLitInfo struct {
	// FakeFuncDecl is the fake func decl node created for the func lit node so that it can be
	// treated like a regular function declaration during the analysis.
	FakeFuncDecl *ast.FuncDecl
	// FakeFuncObj is the fake object for the fake func decl node.
	FakeFuncObj *types.Func
}

// createFakeFuncDecl creates a fake function declaration (AST node and a type object) for the
// given anonymous function (i.e., func lit node). The fake function declaration returns the
// fake func decl node and the fake func type object.
func createFakeFuncDecl(pkg *packages.Package, funcLit *ast.FuncLit) (*ast.FuncDecl, *types.Func) {
	// The name for the node is named "<prefix>Line:Column" for easier identification.
	pos := pkg.Fset.Position(funcLit.Pos())
	name := _fakeFuncDeclPrefix + strconv.Itoa(pos.Line) + ":" + strconv.Itoa(pos.Column)
	ident := &ast.Ident{
		NamePos: funcLit.Pos(),
		Name:    name,
		Obj:     ast.NewObj(ast.Fun, name),
	}

	funcDecl := &ast.FuncDecl{
		Name: ident,
		Type: &ast.FuncType{
			Params:  funcLit.Type.Params,
			Results: funcLit.Type.Results,
		},
		Body: funcLit.Body,
	}

	// Then, create the fake func type for the fake decl for type resolution.
	// Create fake func signature type from func lit signature.
	// Anonymous functions do not have receiver or type parameters. For more detail: https://go.dev/ref/spec#Function_literals
	sig := pkg.TypesInfo.TypeOf(funcLit).(*types.Signature)
	if sig.Recv() != nil || sig.RecvTypeParams() != nil || sig.TypeParams() != nil {
		panic(fmt.Sprintf("receiver or type parameters of an anonymous function at %s:%d.%d is not nil",
			pos.Filename, pos.Line, pos.Column))
	}

	fakeSig := types.NewSignatureType(nil /* recv */, nil /* recvTypeParams */, nil, /* typeParams */
		sig.Params(), sig.Results(), sig.Variadic())
	fakeFuncType := types.NewFunc(funcLit.Pos(), pkg.Types, ident.Name, fakeSig)

	return funcDecl, fakeFuncType
}

// CollectAnonymousFuncs collects all the func lit nodes within the given function declaration and returns
// a map of func lit nodes to their corresponding FuncLitInfo.
func CollectAnonymousFuncs(pkg *packages.Package, parent ast.Node) (map[*ast.FuncLit]*FuncLitInfo, map[*ast.Ident]*ast.FuncLit) {
	funcLitMap := make(map[*ast.FuncLit]*FuncLitInfo)
	identFuncLitMap := make(map[*ast.Ident]*ast.FuncLit)

	ast.Inspect(parent, func(node ast.Node) bool {
		funcLit, ok := node.(*ast.FuncLit)
		if !ok {
			return true
		}

		fakeDecl, fakeType := createFakeFuncDecl(pkg, funcLit)
		funcLitMap[funcLit] = &FuncLitInfo{
			FakeFuncDecl: fakeDecl,
			FakeFuncObj:  fakeType,
		}
		identFuncLitMap[fakeDecl.Name] = funcLit

		return true
	})
	return funcLitMap, identFuncLitMap
}
