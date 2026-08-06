package graph

import (
	"errors"
	"go/ast"
	"go/token"
	"sort"

	"golang.org/x/tools/go/ssa"
)

func getViaValForExpr(ssaFunc *ssa.Function, callExpr *ast.CallExpr, fset *token.FileSet, N int) (ssa.CallInstruction, error) {
	value, _ := ssaFunc.ValueForExpr(callExpr)
	if value == nil {
		return nil, errors.New("getViaValForExpr value is nil")
	}

	// Since the SSA value corresponds to the call expression, we can find the instruction that defines it.
	// The value should be an instruction, so we can type assert it.
	instr, ok := value.(ssa.Instruction)
	if !ok {
		return nil, errors.New("getViaValForExpr value is not an ssa.Instruction")
	}

	// Now, check if the instruction is a call instruction.
	ci, ok := instr.(ssa.CallInstruction)
	if !ok {
		return nil, errors.New("getViaValForExpr value is not an ssa.CallInstruction")
	}
	return ci, nil
}

func getClosestCallInstruction(f *ssa.Function, callExpr *ast.CallExpr, fset *token.FileSet, N int) (ssa.CallInstruction, error) {
	if f == nil || callExpr == nil || fset == nil {
		return nil, errors.New("getClosestCallInstruction: Invalid input")
	}
	// first try getViaValForExpr.
	ci, err := getViaValForExpr(f, callExpr, fset, N)
	if err == nil {
		return ci, nil
	}
	// Get the position of the AST call expression (both line and column)
	astPos := fset.Position(callExpr.Pos())

	// Find the top N closest instructions based on line and column proximity
	closestInstructions := findTopNClosestInstructions(f.Blocks, astPos, fset, N)

	// For each of the top N instructions, find the nearest ssa.Call instruction in the same block
	for _, instr := range closestInstructions {
		callInstr, err := findNearestCallInBlock(instr.Block(), fset)
		if err == nil {
			return callInstr, nil // Return the first ssa.Call instruction found
		}
	}

	// No ssa.Call instruction found
	return nil, errors.New("getTopNClosestInstructions: No ssa.Call instruction found")
}

// Find the top N SSA instructions closest to the given line and column (sorted by proximity)
func findTopNClosestInstructions(blocks []*ssa.BasicBlock, targetPos token.Position, fset *token.FileSet, N int) []ssa.Instruction {
	var allInstructions []instructionWithPos

	// Loop through all SSA blocks and instructions
	for _, block := range blocks {
		for _, instr := range block.Instrs {
			if instr.Pos() != token.NoPos {
				allInstructions = append(allInstructions, instructionWithPos{
					instr: instr,
					pos:   fset.Position(instr.Pos()),
				})
			}
		}
	}

	// Sort the instructions based on proximity to the target position (line first, then column)
	sort.Slice(allInstructions, func(i, j int) bool {
		lineDiffI := abs(allInstructions[i].pos.Line - targetPos.Line)
		lineDiffJ := abs(allInstructions[j].pos.Line - targetPos.Line)
		if lineDiffI != lineDiffJ {
			return lineDiffI < lineDiffJ
		}
		columnDiffI := abs(allInstructions[i].pos.Column - targetPos.Column)
		columnDiffJ := abs(allInstructions[j].pos.Column - targetPos.Column)
		return columnDiffI < columnDiffJ
	})

	// Return the top N closest instructions
	closestInstructions := []ssa.Instruction{}
	for i := 0; i < len(allInstructions) && i < N; i++ {
		closestInstructions = append(closestInstructions, allInstructions[i].instr)
	}

	return closestInstructions
}

// Find the nearest ssa.Call instruction in the block of a given instruction
func findNearestCallInBlock(block *ssa.BasicBlock, fset *token.FileSet) (ssa.CallInstruction, error) {
	for _, instr := range block.Instrs {
		// Check if the instruction is a Call
		if ci, ok := instr.(ssa.CallInstruction); ok {
			return ci, nil // Return the first ssa.Call instruction found
		}
	}
	// No ssa.Call instruction found
	return nil, errors.New("findNearestCallInBlock: No ssa.Call instruction found")

}

// Struct to store instructions along with their positions
type instructionWithPos struct {
	instr ssa.Instruction
	pos   token.Position
}

// Utility function to get absolute difference between two numbers
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
