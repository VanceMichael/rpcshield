package graph

import (
	"fmt"
	"go/constant"
	"go/types"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

// ValueLattice is a type that represents the possible values of a variable.
// It can take a Top (unknown), ConstantVal/FunctionVal, or Bottom (anything) values.
type ValueLattice interface {
	String() string
	isValueLattice()
}

// TopVal represents the top value, which means nothing is known about the value.
// This is when the value is uninitialized.
type TopVal struct{}

func (t TopVal) String() string  { return "T_val" }
func (t TopVal) isValueLattice() {}

// ConstantVal represents a compile time constant.
type ConstantVal struct{ Value ssa.Const }

func (c ConstantVal) String() string {
	return c.Value.String()
}
func (c ConstantVal) isValueLattice() {}

// FunctionVal represents a closure/function pointer, which is a function plus bindings for its free variables.
type FunctionVal struct {
	fn ssa.Value // only allowed types are *ssa.Function and *ssa.MakeClosure
}

func (f FunctionVal) String() string  { return fmt.Sprintf("Func(%s)", f.fn.String()) }
func (f FunctionVal) isValueLattice() {}

// BottomVal represents the bottom value, which means the value can take all possible values.
type BottomVal struct{}

func (b BottomVal) String() string  { return "B_val" }
func (b BottomVal) isValueLattice() {}

// TypeLattice is a type that represents the possible types of a variable.
// It can take a Top, Concrete, or Bottom type.
type TypeLattice interface {
	String() string
	isTypeLattice()
}

// TopType represents the fact that nothing is known about the type.
type TopType struct{}

func (t TopType) String() string { return "T_type" }
func (t TopType) isTypeLattice() {}

// ConcreteType represents a concrete type.
// However, note that even an interface is a concrete type.
type ConcreteType struct{ T types.Type }

func (c ConcreteType) String() string { return fmt.Sprintf("Type(%s)", c.T.String()) }
func (c ConcreteType) isTypeLattice() {}

// BottomType represents the fact that the type can take all possible values.
type BottomType struct{}

func (b BottomType) String() string { return "B_type" }
func (b BottomType) isTypeLattice() {}

// ValueTypePair represents a pair of value and type.
// The reason for ValueTypePair is that even if the value bottoms out, the type need not.
// Having access to concrete type allows us to perform precise function calls, type assertions, etc.
type ValueTypePair struct {
	// Value is the value of the variable.
	Value ValueLattice
	Type  TypeLattice
}

// Equal checks if two ValueTypePairs are equal.
func (p ValueTypePair) Equal(other ValueTypePair) bool {
	return p.Value == other.Value && p.Type == other.Type
}

// String returns a string representation of the ValueTypePair.
func (p ValueTypePair) String() string {
	buf := strings.Builder{}
	buf.WriteString(fmt.Sprintf("%s: %s", p.Value, p.Type))
	return buf.String()
}

// LatticeElement represents a combined value and type lattice.
type LatticeElement struct {
	// vtPair is a list of value-type pairs.
	// The reason to maintain a list is that we need to handle multiple values and types for type.Tuple.
	// Not having a lattice element for type.Tuple leads to too many checks in the code.
	// This approach keeps code simple.
	vtPair []ValueTypePair
}

// Equal checks if two LatticeElements are equal.
func (le LatticeElement) Equal(other LatticeElement) bool {
	if len(le.vtPair) != len(other.vtPair) {
		return false
	}
	for i := range le.vtPair {
		if !le.vtPair[i].Equal(other.vtPair[i]) {
			return false
		}
	}
	return true
}

func (le LatticeElement) String() string {
	buf := strings.Builder{}
	for _, vtPair := range le.vtPair {
		buf.WriteString(vtPair.String())
		buf.WriteString("\n")
	}
	return buf.String()
}

// meetValue computes the meet for the ValueLattice.
// The meet is the most precise value that is consistent with both a and b.
func meetValue(a, b ValueLattice, logger *zap.SugaredLogger) ValueLattice {
	logger.Debugw("meetValue", zap.String("a", a.String()), zap.String("b", b.String()))
	switch x := a.(type) {
	case TopVal:
		return b
	case BottomVal:
		return BottomVal{}
	case ConstantVal:
		switch y := b.(type) {
		case TopVal:
			return a
		case BottomVal:
			return BottomVal{}
		case ConstantVal:
			if constsEqual(x.Value, y.Value) {
				return a
			}
			return BottomVal{}
		default:
			return BottomVal{}
		}
	case FunctionVal:
		switch y := b.(type) {
		case TopVal:
			return a
		case BottomVal:
			return BottomVal{}
		case ConstantVal:
			return BottomVal{}
		case FunctionVal:
			if x.fn != y.fn {
				return BottomVal{} // Meet of two different functions is Bottom.
			}
			return a
		}
		return BottomVal{}
	}
	panic("unreachable")
}

// meetType computes the meet for the TypeLattice.
// The meet is the most precise type that is consistent with both a and b.
func meetType(a, b TypeLattice, logger *zap.SugaredLogger) TypeLattice {
	logger.Debugw("meetType", zap.String("a", a.String()), zap.String("b", b.String()))
	switch x := a.(type) {
	case TopType:
		return b
	case BottomType:
		return BottomType{}
	case ConcreteType:
		switch y := b.(type) {
		case TopType:
			return a
		case BottomType:
			return BottomType{}
		case ConcreteType:
			if types.Identical(x.T, y.T) {
				return a
			}
			// TODO: Can lower to the least common sub-type i.e., types.Implements(x.T, y.T)
			return BottomType{}
		}
	}
	panic("unreachable")
}

// meet combines two LatticeElements.
// The meet is the most precise value and type that is consistent with both a and b.
func meet(a, b LatticeElement, logger *zap.SugaredLogger) LatticeElement {
	// If we have no value-type pairs, return the other lattice element.
	if len(a.vtPair) == 0 {
		return b
	}
	if len(b.vtPair) == 0 {
		return a
	}
	if len(a.vtPair) != len(b.vtPair) {
		logger.Debugw("meet_length_mismatch", zap.String("a", a.String()), zap.String("b", b.String()))
		return LatticeElement{
			vtPair: []ValueTypePair{
				{
					Value: BottomVal{},
					Type:  BottomType{},
				},
			},
		}
	}
	vtPair := make([]ValueTypePair, len(a.vtPair))
	for i := range a.vtPair {
		vtPair[i] = ValueTypePair{
			Value: meetValue(a.vtPair[i].Value, b.vtPair[i].Value, logger),
			Type:  meetType(a.vtPair[i].Type, b.vtPair[i].Type, logger),
		}
	}
	return LatticeElement{vtPair: vtPair}
}

// createTopLatticeElement creates a lattice element with top value and top type.
// Since tuples are special, we handle them separately.
func createTopLatticeElement(t types.Type) LatticeElement {
	if tupleType, ok := t.(*types.Tuple); ok && tupleType.Len() > 0 {
		return createTopTupleLatticeElement(*tupleType)
	}
	return LatticeElement{vtPair: []ValueTypePair{{Value: TopVal{}, Type: TopType{}}}}
}

// createBottomLatticeElement creates a lattice element with bottom value and bottom type.
func createBottomLatticeElement() LatticeElement {
	return LatticeElement{vtPair: []ValueTypePair{{Value: BottomVal{}, Type: BottomType{}}}}
}

// getSingletonLatticeElement returns the value and type of the singleton lattice element.
func (le LatticeElement) getSingletonLatticeElement() (ValueLattice, TypeLattice) {
	switch len(le.vtPair) {
	case 0:
		return TopVal{}, TopType{}
	case 1:
		return le.vtPair[0].Value, le.vtPair[0].Type
	default:
		return BottomVal{}, BottomType{}
	}
}

// createSingletonLatticeElement creates a lattice element with a single value and type.
func createSingletonLatticeElement(value ValueLattice, typeLattice TypeLattice) LatticeElement {
	return LatticeElement{vtPair: []ValueTypePair{{Value: value, Type: typeLattice}}}
}

// createTypedLatticeBottom creates a lattice element with bottom value and concrete type.
func createTypedLatticeBottom(t types.Type) LatticeElement {
	return LatticeElement{vtPair: []ValueTypePair{{Value: BottomVal{}, Type: ConcreteType{T: t}}}}
}

// createTupleLatticeBottom creates a lattice element with bottom value and tuple type.
func createTupleLatticeBottom(t types.Tuple) LatticeElement {
	le := LatticeElement{vtPair: make([]ValueTypePair, t.Len())}
	for i := 0; i < t.Len(); i++ {
		le.vtPair[i] = ValueTypePair{Value: BottomVal{}, Type: ConcreteType{T: t.At(i).Type()}}
	}
	return le
}

// createTrueConstantVal creates a constant true value.
func createTrueConstantVal() ConstantVal {
	return ConstantVal{Value: *ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool])}
}

// createFalseConstantVal creates a constant false value.
func createFalseConstantVal() ConstantVal {
	return ConstantVal{Value: *ssa.NewConst(constant.MakeBool(false), types.Typ[types.Bool])}
}

// createBoolConstantVal creates a constant boolean value.
func createBoolConstantVal(b bool) ConstantVal {
	return ConstantVal{Value: *ssa.NewConst(constant.MakeBool(b), types.Typ[types.Bool])}
}

// createFloatConstantVal creates a constant float value.
func createFloatConstantVal(f float64) ConstantVal {
	return ConstantVal{Value: *ssa.NewConst(constant.MakeFloat64(f), types.Typ[types.Float64])}
}

// createConstantVal creates a constant value.
func createConstantVal(c ssa.Const) ConstantVal {
	return ConstantVal{Value: c}
}

// createConstantFalseLatticeElement creates a constant false value with boolean type.
func createConstantFalseLatticeElement() LatticeElement {
	return createSingletonLatticeElement(createFalseConstantVal(), ConcreteType{T: types.Typ[types.Bool]})
}

// createConstantTrueLatticeElement creates a constant true value with boolean type.
func createConstantTrueLatticeElement() LatticeElement {
	return createSingletonLatticeElement(createTrueConstantVal(), ConcreteType{T: types.Typ[types.Bool]})
}

// createTopTupleLatticeElement creates a lattice element for a tuple of top values.
func createTopTupleLatticeElement(v types.Tuple) LatticeElement {
	le := LatticeElement{vtPair: make([]ValueTypePair, v.Len())}
	for i := 0; i < v.Len(); i++ {
		le.vtPair[i] = ValueTypePair{Value: TopVal{}, Type: TopType{}}
	}
	return le
}

// constsEqual determines semantic equality between two SSA constants by comparing
// both their static type and their constant values.
func constsEqual(a, b ssa.Const) bool {

	// Cannot use cont.IsNil() because IsNil() checks nillability only for nilable types like here:
	// func (c *Const) IsNil() bool {
	// 	return c.Value == nil && nillable(c.typ)
	// }

	// Nil vs. non-nil are treated as unequal.
	if (a.Value == nil) != (b.Value == nil) {
		return false
	}
	// Mismatched types are treated as unequal.
	if a.Type() != b.Type() {
		return false
	}
	// if both are nil, they are equal and of the same type.
	if (a.Value == nil) && (b.Value == nil) {
		return true
	}

	// Unknown constants are treated as unequal to preserve conservatism.
	if a.Value.Kind() == constant.Unknown || b.Value.Kind() == constant.Unknown {
		return false
	}
	return a.Value == b.Value
}

// constHasNoScalarValue reports whether c carries no go/constant.Value payload. This is true both
// for a genuine nil of a nillable type (ptr/slice/chan/map/func/interface) and for the zero value
// of a non-nillable composite type (struct/array), which ssa.NewConst(nil, T) also represents as
// Value==nil even though c.IsNil() is false for those types. Callers that need "is there a real
// scalar to fold" must check this, not IsNil() -- see constsEqual above, which already uses this
// idiom.
func constHasNoScalarValue(c ssa.Const) bool {
	return c.Value == nil
}
