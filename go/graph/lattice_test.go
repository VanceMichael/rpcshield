package graph

import (
	"fmt"
	"go/constant"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

func TestMeetValue_NonFunctionCombinations(t *testing.T) {
	logger := zap.NewNop().Sugar()

	// Create test values for non-function ValueLattice types
	topVal := TopVal{}
	bottomVal := BottomVal{}
	constVal1 := createConstantVal(*ssa.NewConst(constant.MakeInt64(42), types.Typ[types.Int]))
	constVal2 := createConstantVal(*ssa.NewConst(constant.MakeInt64(24), types.Typ[types.Int]))
	prog := ssa.NewProgram(nil, 0)
	fn := prog.NewFunction("fn", types.NewSignature(nil, nil, nil, false), "fn")
	functionVal1 := FunctionVal{fn: fn}
	fn2 := prog.NewFunction("fn2", types.NewSignature(nil, nil, nil, false), "fn2")
	functionVal2 := FunctionVal{fn: fn2}
	// Test isValueLattice interface compliance
	for _, v := range []ValueLattice{topVal, bottomVal, constVal1, functionVal1, functionVal2} {
		v.isValueLattice()
		_ = v.String()
	}

	vals := []ValueLattice{topVal, bottomVal, constVal1, functionVal1}
	// Top with any value meets the value
	for _, b := range vals {
		t.Run(fmt.Sprintf("TopVal_meets_%s", b.String()), func(t *testing.T) {
			top := TopVal{}
			fmt.Printf("\n %v with %v", top, b)
			result1 := meetValue(top, b, logger)
			fmt.Printf("\n %v with %v meets %v", top, b, result1)
			assert.Equal(t, b, result1)
			fmt.Printf("\n %v with %v meets", b, top)
			result2 := meetValue(b, top, logger)
			fmt.Printf("\n %v with %v meets %v", b, top, result2)
			assert.Equal(t, b, result2)
		})
	}
	// Bottom with any value meets the bottom
	for _, b := range vals {
		t.Run(fmt.Sprintf("BottomVal_meets_%s", b.String()), func(t *testing.T) {
			result1 := meetValue(bottomVal, b, logger)
			assert.Equal(t, bottomVal, result1)
			result2 := meetValue(b, bottomVal, logger)
			assert.Equal(t, bottomVal, result2)
		})
	}
	// Two same constant values meet the constant value
	t.Run(fmt.Sprintf("ConstantVal_meets_%s", constVal1.String()), func(t *testing.T) {
		result1 := meetValue(constVal1, constVal1, logger)
		assert.Equal(t, constVal1, result1)
		result2 := meetValue(constVal1, constVal1, logger)
		assert.Equal(t, constVal1, result2)
	})
	// Two different constant values meet the bottom
	t.Run(fmt.Sprintf("ConstantVal_meets_%s", constVal2.String()), func(t *testing.T) {
		result1 := meetValue(constVal1, constVal2, logger)
		assert.Equal(t, BottomVal{}, result1)
		result2 := meetValue(constVal2, constVal1, logger)
		assert.Equal(t, BottomVal{}, result2)
	})
	// A constant value meets a function value meets the bottom
	t.Run(fmt.Sprintf("ConstantVal_meets_%s", functionVal1.String()), func(t *testing.T) {
		result1 := meetValue(constVal1, functionVal1, logger)
		assert.Equal(t, BottomVal{}, result1)
		result2 := meetValue(functionVal1, constVal1, logger)
		assert.Equal(t, BottomVal{}, result2)
	})
	// A function value meets a constant value meets the bottom
	t.Run(fmt.Sprintf("FunctionVal_meets_%s", constVal1.String()), func(t *testing.T) {
		result1 := meetValue(functionVal1, constVal1, logger)
		assert.Equal(t, BottomVal{}, result1)
		result2 := meetValue(constVal1, functionVal1, logger)
		assert.Equal(t, BottomVal{}, result2)
	})
	// Two same function values meet the function value
	t.Run(fmt.Sprintf("FunctionVal_meets_%s", functionVal1.String()), func(t *testing.T) {
		result1 := meetValue(functionVal1, functionVal1, logger)
		assert.Equal(t, functionVal1, result1)
		result2 := meetValue(functionVal1, functionVal1, logger)
		assert.Equal(t, functionVal1, result2)
	})
	// Two different function values meet the bottom
	t.Run(fmt.Sprintf("FunctionVal_meets_%s", functionVal2.String()), func(t *testing.T) {
		result1 := meetValue(functionVal1, functionVal2, logger)
		assert.Equal(t, BottomVal{}, result1)
		result2 := meetValue(functionVal2, functionVal1, logger)
		assert.Equal(t, BottomVal{}, result2)
	})
}

func TestMeetType_AllCombinations(t *testing.T) {
	logger := zap.NewNop().Sugar()

	// Create test values for each TypeLattice type
	topType := TopType{}
	bottomType := BottomType{}
	intType := ConcreteType{T: types.Typ[types.Int]}
	stringType := ConcreteType{T: types.Typ[types.String]}
	sameIntType := ConcreteType{T: types.Typ[types.Int]} // Same type as intType

	typesList := []TypeLattice{topType, bottomType, intType, stringType, sameIntType}
	// Test isTypeLattice interface compliance
	for _, t := range typesList {
		t.isTypeLattice()
		_ = t.String()
	}

	// Top with any type meets the type
	for _, b := range typesList {
		t.Run(fmt.Sprintf("TopType_meets_%s", b.String()), func(t *testing.T) {
			result1 := meetType(topType, b, logger)
			assert.Equal(t, b, result1)
			result2 := meetType(b, topType, logger)
			assert.Equal(t, b, result2)
		})
	}
	// Bottom with any type meets the bottom
	for _, b := range typesList {
		t.Run(fmt.Sprintf("BottomType_meets_%s", b.String()), func(t *testing.T) {
			result1 := meetType(bottomType, b, logger)
			assert.Equal(t, bottomType, result1)
			result2 := meetType(b, bottomType, logger)
			assert.Equal(t, bottomType, result2)
		})
	}
	// Two same concrete types meet the concrete type
	t.Run(fmt.Sprintf("ConcreteType_meets_%s", intType.String()), func(t *testing.T) {
		result1 := meetType(intType, intType, logger)
		assert.Equal(t, intType, result1)
		result2 := meetType(intType, intType, logger)
		assert.Equal(t, intType, result2)
	})
	// Two different concrete types meet the bottom
	t.Run(fmt.Sprintf("ConcreteType_meets_%s", stringType.String()), func(t *testing.T) {
		result1 := meetType(intType, stringType, logger)
		assert.Equal(t, BottomType{}, result1)
		result2 := meetType(stringType, intType, logger)
		assert.Equal(t, BottomType{}, result2)
	})

}

func TestMeet_AllCombinations(t *testing.T) {
	logger := zap.NewNop().Sugar()

	// Create test LatticeElements for different scenarios
	emptyElement := LatticeElement{vtPair: []ValueTypePair{}}
	singleElement := LatticeElement{
		vtPair: []ValueTypePair{{
			Value: TopVal{},
			Type:  TopType{},
		}},
	}
	doubleElement := LatticeElement{
		vtPair: []ValueTypePair{
			{Value: TopVal{}, Type: TopType{}},
			{Value: BottomVal{}, Type: BottomType{}},
		},
	}
	anotherSingleElement := createTypedLatticeBottom(types.Typ[types.Int])

	anotherDoubleElement := LatticeElement{
		vtPair: []ValueTypePair{
			{Value: BottomVal{}, Type: BottomType{}},
			{Value: TopVal{}, Type: ConcreteType{T: types.Typ[types.String]}},
		},
	}

	// Expected bottom element for length mismatches
	expectedBottomElement := createBottomLatticeElement()

	// Comprehensive table of all combinations
	tests := []struct {
		name     string
		a        LatticeElement
		b        LatticeElement
		expected LatticeElement
	}{
		// Empty vtPair cases
		{"empty_a_returns_b", emptyElement, singleElement, singleElement},
		{"empty_b_returns_a", singleElement, emptyElement, singleElement},
		{"both_empty", emptyElement, emptyElement, emptyElement},

		// Length mismatch cases
		{"single_vs_double_length_mismatch", singleElement, doubleElement, expectedBottomElement},
		{"double_vs_single_length_mismatch", doubleElement, singleElement, expectedBottomElement},

		// Same length cases
		{"same_length_single_pairs", singleElement, anotherSingleElement, LatticeElement{
			vtPair: []ValueTypePair{{
				Value: BottomVal{},                           // TopVal meet BottomVal = BottomVal
				Type:  ConcreteType{T: types.Typ[types.Int]}, // TopType meet ConcreteType = ConcreteType
			}},
		}},
		{"same_length_double_pairs", doubleElement, anotherDoubleElement, LatticeElement{
			vtPair: []ValueTypePair{
				{Value: BottomVal{}, Type: BottomType{}}, // TopVal meet BottomVal, TopType meet BottomType
				{Value: BottomVal{}, Type: BottomType{}}, // BottomVal meet TopVal, BottomType meet ConcreteType
			},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result1 := meet(tt.a, tt.b, logger)
			assert.Equal(t, tt.expected, result1)
			result2 := meet(tt.b, tt.a, logger)
			assert.Equal(t, tt.expected, result2)
		})
	}
}

// TestCreateTopLatticeElement verifies that createTopLatticeElement correctly creates lattice elements
// with top values and top types for basic types, and properly handles tuple types by delegating to createTopTupleLatticeElement.
func TestCreateTopLatticeElement(t *testing.T) {
	tests := []struct {
		name     string
		input    types.Type
		expected LatticeElement
	}{
		{
			name:  "basic_type_int",
			input: types.Typ[types.Int],
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: TopVal{}, Type: TopType{}}},
			},
		},
		{
			name:  "basic_type_string",
			input: types.Typ[types.String],
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: TopVal{}, Type: TopType{}}},
			},
		},
		{
			name:  "empty_tuple",
			input: types.NewTuple(),
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: TopVal{}, Type: TopType{}}},
			},
		},
		{
			name: "tuple_with_elements",
			input: types.NewTuple(
				types.NewParam(0, nil, "x", types.Typ[types.Int]),
				types.NewParam(0, nil, "y", types.Typ[types.String]),
			),
			expected: LatticeElement{
				vtPair: []ValueTypePair{
					{Value: TopVal{}, Type: TopType{}},
					{Value: TopVal{}, Type: TopType{}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTopLatticeElement(tt.input)
			assert.Equal(t, tt.expected, result, "createTopLatticeElement should create the expected lattice element for type %s", tt.input.String())
		})
	}
}

// TestCreateBottomLatticeElement verifies that createBottomLatticeElement returns a lattice element
// with bottom value and bottom type as expected.
func TestCreateBottomLatticeElement(t *testing.T) {
	expected := LatticeElement{
		vtPair: []ValueTypePair{{Value: BottomVal{}, Type: BottomType{}}},
	}

	result := createBottomLatticeElement()
	assert.Equal(t, expected, result, "createBottomLatticeElement should create a lattice element with bottom value and bottom type")
}

// TestGetSingletonLatticeElement verifies that getSingletonLatticeElement correctly returns values and types
// based on the number of value-type pairs: top values for empty pairs, actual values for single pairs, and bottom values for multiple pairs.
func TestGetSingletonLatticeElement(t *testing.T) {
	tests := []struct {
		name          string
		input         LatticeElement
		expectedValue ValueLattice
		expectedType  TypeLattice
	}{
		{
			name:          "empty_vtPair",
			input:         LatticeElement{vtPair: []ValueTypePair{}},
			expectedValue: TopVal{},
			expectedType:  TopType{},
		},
		{
			name: "single_vtPair",
			input: LatticeElement{
				vtPair: []ValueTypePair{{Value: BottomVal{}, Type: ConcreteType{T: types.Typ[types.Int]}}},
			},
			expectedValue: BottomVal{},
			expectedType:  ConcreteType{T: types.Typ[types.Int]},
		},
		{
			name: "multiple_vtPair",
			input: LatticeElement{
				vtPair: []ValueTypePair{
					{Value: TopVal{}, Type: TopType{}},
					{Value: BottomVal{}, Type: BottomType{}},
				},
			},
			expectedValue: BottomVal{},
			expectedType:  BottomType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, typeLattice := tt.input.getSingletonLatticeElement()
			assert.Equal(t, tt.expectedValue, value, "getSingletonLatticeElement should return the expected value for case %s", tt.name)
			assert.Equal(t, tt.expectedType, typeLattice, "getSingletonLatticeElement should return the expected type for case %s", tt.name)
		})
	}
}

// TestCreateSingletonLatticeElement verifies that createSingletonLatticeElement correctly creates
// lattice elements with the provided value and type pair.
func TestCreateSingletonLatticeElement(t *testing.T) {
	tests := []struct {
		name        string
		value       ValueLattice
		typeLattice TypeLattice
		expected    LatticeElement
	}{
		{
			name:        "top_value_top_type",
			value:       TopVal{},
			typeLattice: TopType{},
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: TopVal{}, Type: TopType{}}},
			},
		},
		{
			name:        "bottom_value_concrete_type",
			value:       BottomVal{},
			typeLattice: ConcreteType{T: types.Typ[types.String]},
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: BottomVal{}, Type: ConcreteType{T: types.Typ[types.String]}}},
			},
		},
		{
			name:        "constant_value_concrete_type",
			value:       createConstantVal(*ssa.NewConst(constant.MakeInt64(42), types.Typ[types.Int])),
			typeLattice: ConcreteType{T: types.Typ[types.Int]},
			expected: LatticeElement{
				vtPair: []ValueTypePair{{
					Value: createConstantVal(*ssa.NewConst(constant.MakeInt64(42), types.Typ[types.Int])),
					Type:  ConcreteType{T: types.Typ[types.Int]},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createSingletonLatticeElement(tt.value, tt.typeLattice)
			assert.Equal(t, tt.expected, result, "createSingletonLatticeElement should create the expected lattice element for %s", tt.name)
		})
	}
}

// TestCreateTypedLatticeBottom verifies that createTypedLatticeBottom creates lattice elements
// with bottom values and concrete types for various input types.
func TestCreateTypedLatticeBottom(t *testing.T) {
	tests := []struct {
		name     string
		input    types.Type
		expected LatticeElement
	}{
		{
			name:  "int_type",
			input: types.Typ[types.Int],
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: BottomVal{}, Type: ConcreteType{T: types.Typ[types.Int]}}},
			},
		},
		{
			name:  "string_type",
			input: types.Typ[types.String],
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: BottomVal{}, Type: ConcreteType{T: types.Typ[types.String]}}},
			},
		},
		{
			name:  "bool_type",
			input: types.Typ[types.Bool],
			expected: LatticeElement{
				vtPair: []ValueTypePair{{Value: BottomVal{}, Type: ConcreteType{T: types.Typ[types.Bool]}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTypedLatticeBottom(tt.input)
			assert.Equal(t, tt.expected, result, "createTypedLatticeBottom should create the expected lattice element for type %s", tt.input.String())
		})
	}
}

func TestCreateTrueConstantVal(t *testing.T) {
	result := createTrueConstantVal()
	expected := ConstantVal{Value: *ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool])}

	assert.Equal(t, expected, result, "createTrueConstantVal should create the expected true ConstantVal")
	assert.Equal(t, "true:bool", result.String(), "createTrueConstantVal should produce the correct string representation")
}

// TestCreateFalseConstantVal verifies that createFalseConstantVal returns a ConstantVal with the correct false constant value.
func TestCreateFalseConstantVal(t *testing.T) {
	result := createFalseConstantVal()
	expected := ConstantVal{Value: *ssa.NewConst(constant.MakeBool(false), types.Typ[types.Bool])}

	assert.Equal(t, expected, result, "createFalseConstantVal should create the expected false ConstantVal")
	assert.Equal(t, "false:bool", result.String(), "createFalseConstantVal should produce the correct string representation")
}

// TestCreateBoolConstantVal verifies that createBoolConstantVal correctly creates ConstantVal objects
// for both true and false boolean values.
func TestCreateBoolConstantVal(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected ConstantVal
	}{
		{
			name:     "true_bool",
			input:    true,
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool])},
		},
		{
			name:     "false_bool",
			input:    false,
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeBool(false), types.Typ[types.Bool])},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createBoolConstantVal(tt.input)
			assert.Equal(t, tt.expected, result, "createBoolConstantVal should create the expected ConstantVal for %t", tt.input)
		})
	}
}

// TestCreateFloatConstantVal verifies that createFloatConstantVal correctly creates ConstantVal objects
// for various float64 values including positive, negative, and zero.
func TestCreateFloatConstantVal(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected ConstantVal
	}{
		{
			name:     "positive_float",
			input:    3.14,
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeFloat64(3.14), types.Typ[types.Float64])},
		},
		{
			name:     "negative_float",
			input:    -2.5,
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeFloat64(-2.5), types.Typ[types.Float64])},
		},
		{
			name:     "zero_float",
			input:    0.0,
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeFloat64(0.0), types.Typ[types.Float64])},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createFloatConstantVal(tt.input)
			assert.Equal(t, tt.expected, result, "createFloatConstantVal should create the expected ConstantVal for %f", tt.input)
		})
	}
}

// TestCreateConstantVal verifies that createConstantVal correctly wraps ssa.Const values
// into ConstantVal objects for various constant types.
func TestCreateConstantVal(t *testing.T) {
	tests := []struct {
		name     string
		input    ssa.Const
		expected ConstantVal
	}{
		{
			name:     "int_constant",
			input:    *ssa.NewConst(constant.MakeInt64(42), types.Typ[types.Int]),
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeInt64(42), types.Typ[types.Int])},
		},
		{
			name:     "string_constant",
			input:    *ssa.NewConst(constant.MakeString("hello"), types.Typ[types.String]),
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeString("hello"), types.Typ[types.String])},
		},
		{
			name:     "bool_constant",
			input:    *ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool]),
			expected: ConstantVal{Value: *ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool])},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createConstantVal(tt.input)
			assert.Equal(t, tt.expected, result, "createConstantVal should wrap the ssa.Const into the expected ConstantVal for %s", tt.name)
		})
	}
}

// TestCreateConstantFalseLatticeElement verifies that createConstantFalseLatticeElement creates
// a lattice element with false constant value and boolean concrete type.
func TestCreateConstantFalseLatticeElement(t *testing.T) {
	result := createConstantFalseLatticeElement()
	expected := LatticeElement{
		vtPair: []ValueTypePair{{
			Value: createFalseConstantVal(),
			Type:  ConcreteType{T: types.Typ[types.Bool]},
		}},
	}

	assert.Equal(t, expected, result, "createConstantFalseLatticeElement should create the expected lattice element with false constant value")
}

// TestCreateConstantTrueLatticeElement verifies that createConstantTrueLatticeElement creates
// a lattice element with true constant value and boolean concrete type.
func TestCreateConstantTrueLatticeElement(t *testing.T) {
	result := createConstantTrueLatticeElement()
	expected := LatticeElement{
		vtPair: []ValueTypePair{{
			Value: createTrueConstantVal(),
			Type:  ConcreteType{T: types.Typ[types.Bool]},
		}},
	}

	assert.Equal(t, expected, result, "createConstantTrueLatticeElement should create the expected lattice element with true constant value")
}

// TestCreateTopTupleLatticeElement verifies that createTopTupleLatticeElement correctly creates
// lattice elements for tuples with the appropriate number of top value-type pairs.
func TestCreateTopTupleLatticeElement(t *testing.T) {
	tests := []struct {
		name     string
		input    types.Tuple
		expected LatticeElement
	}{
		{
			name:  "single_element_tuple",
			input: *types.NewTuple(types.NewParam(0, nil, "x", types.Typ[types.Int])),
			expected: LatticeElement{
				vtPair: []ValueTypePair{
					{Value: TopVal{}, Type: TopType{}},
				},
			},
		},
		{
			name: "two_element_tuple",
			input: *types.NewTuple(
				types.NewParam(0, nil, "x", types.Typ[types.Int]),
				types.NewParam(0, nil, "y", types.Typ[types.String]),
			),
			expected: LatticeElement{
				vtPair: []ValueTypePair{
					{Value: TopVal{}, Type: TopType{}},
					{Value: TopVal{}, Type: TopType{}},
				},
			},
		},
		{
			name: "three_element_tuple",
			input: *types.NewTuple(
				types.NewParam(0, nil, "x", types.Typ[types.Int]),
				types.NewParam(0, nil, "y", types.Typ[types.String]),
				types.NewParam(0, nil, "z", types.Typ[types.Bool]),
			),
			expected: LatticeElement{
				vtPair: []ValueTypePair{
					{Value: TopVal{}, Type: TopType{}},
					{Value: TopVal{}, Type: TopType{}},
					{Value: TopVal{}, Type: TopType{}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTopTupleLatticeElement(tt.input)
			assert.Equal(t, tt.expected, result, "createTopTupleLatticeElement should create the expected lattice element for %s", tt.name)
		})
	}
}

func TestConstsEqual(t *testing.T) {
	ptrToInt := types.NewPointer(types.Typ[types.Int])
	ptrToString := types.NewPointer(types.Typ[types.String])

	mkNil := func(tpe types.Type) ssa.Const { return *ssa.NewConst(nil, tpe) }
	mkInt := func(v int64) ssa.Const { return *ssa.NewConst(constant.MakeInt64(v), types.Typ[types.Int]) }
	mkFloat := func(v float64) ssa.Const { return *ssa.NewConst(constant.MakeFloat64(v), types.Typ[types.Float64]) }
	mkUnknown := func(tpe types.Type) ssa.Const { return *ssa.NewConst(constant.MakeUnknown(), tpe) }

	tests := []struct {
		name string
		a, b ssa.Const
		want bool
	}{
		{
			name: "nil_vs_non_nil",
			a:    mkNil(ptrToInt),
			b:    mkInt(1),
			want: false,
		},
		{
			name: "type_mismatch",
			a:    mkInt(1),
			b:    mkFloat(1.0),
			want: false,
		},
		{
			name: "both_nil_same_type",
			a:    mkNil(ptrToInt),
			b:    mkNil(ptrToInt),
			want: true,
		},
		{
			name: "both_nil_different_types",
			a:    mkNil(ptrToInt),
			b:    mkNil(ptrToString),
			want: false,
		},
		{
			name: "nil_non_nillable_vs_nil_nillable",
			a:    mkNil(types.Typ[types.Int]),
			b:    mkNil(ptrToInt),
			want: false,
		},
		{
			name: "unknown_constant_left",
			a:    mkUnknown(types.Typ[types.Int]),
			b:    mkInt(2),
			want: false,
		},
		{
			name: "unknown_constant_right",
			a:    mkInt(2),
			b:    mkUnknown(types.Typ[types.Int]),
			want: false,
		},
		{
			name: "equal_constants",
			a:    mkInt(5),
			b:    mkInt(5),
			want: true,
		},
		{
			name: "unequal_constants",
			a:    mkInt(5),
			b:    mkInt(6),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := constsEqual(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}
