package http

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/ssa"
)

// TestLastVariadicHandler_MalformedShapes exercises the defensive branches of
// lastVariadicHandler for SSA shapes that don't match gin's variadic
// `...HandlerFunc` allocation pattern. All must fail closed (ok == false)
// rather than misreport a handler.
func TestLastVariadicHandler_MalformedShapes(t *testing.T) {
	t.Parallel()

	t.Run("not a slice", func(t *testing.T) {
		t.Parallel()
		got, ok := lastVariadicHandler(&ssa.Const{})
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("slice not backed by an alloc", func(t *testing.T) {
		t.Parallel()
		got, ok := lastVariadicHandler(&ssa.Slice{X: &ssa.Const{}})
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("alloc without indexed stores", func(t *testing.T) {
		t.Parallel()
		// An Alloc with no IndexAddr/Store referrers yields no candidate handler.
		got, ok := lastVariadicHandler(&ssa.Slice{X: &ssa.Alloc{}})
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

// TestIsGinContextType_NonMatching verifies the type guard rejects types that
// are not *gin.Context.
func TestIsGinContextType_NonMatching(t *testing.T) {
	t.Parallel()

	t.Run("not a pointer", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isGinContextType(types.Typ[types.Int]))
	})

	t.Run("pointer to non-named type", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isGinContextType(types.NewPointer(types.Typ[types.Int])))
	})
}

// TestIsGinHandlerFuncSignatureRelaxed_NoContext verifies a signature without a
// *gin.Context parameter is not treated as a gin handler.
func TestIsGinHandlerFuncSignatureRelaxed_NoContext(t *testing.T) {
	t.Parallel()

	params := types.NewTuple(types.NewVar(token.NoPos, nil, "x", types.Typ[types.Int]))
	sig := types.NewSignature(nil /* recv */, params, nil /* results */, false /* variadic */)
	assert.False(t, isGinHandlerFuncSignatureRelaxed(sig))
}
