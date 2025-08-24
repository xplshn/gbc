// package ir defines a lower-level representation of our program, that is independent of any specific backend (QBE, LLVM, ... etc)
package ir

import (
	"github.com/xplshn/gbc/pkg/ast"
)

// Op represents an operation code for an instruction.
type Op int

const (
	// Memory Operations
	OpAlloc Op = iota // Allocate stack memory
	OpLoad
	OpStore
	OpBlit // Memory copy

	// Integer Arithmetic/Bitwise Operations
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpRem
	OpAnd
	OpOr
	OpXor
	OpShl
	OpShr

	// Floating-Point Operations
	OpAddF
	OpSubF
	OpMulF
	OpDivF
	OpRemF
	OpNegF

	// Comparison Operations
	OpCEq
	OpCNeq
	OpCLt
	OpCGt
	OpCLe
	OpCGe

	// Type Conversion/Extension Operations
	OpExtSB
	OpExtUB
	OpExtSH
	OpExtUH
	OpExtSW
	OpExtUW
	OpTrunc
	OpCast
	OpFToI
	OpIToF
	OpFToF

	// Control Flow
	OpJmp
	OpJnz
	OpRet

	// Function Call
	OpCall

	// Special
	OpPhi
)

// Type represents a data type in the IR.
type Type int

const (
	TypeNone Type = iota
	TypeB         // byte (1)
	TypeH         // half-word (2)
	TypeW         // word (4)
	TypeL         // long (8)
	TypeS         // single-precision float (4)
	TypeD         // double-precision float (8)
	TypePtr       // pointer (word size)
)

// Value represents an operand for an instruction. It can be a constant,
// a temporary register, a global symbol, or a label
type Value interface {
	isValue()
	String() string
}

// Const represents a constant integer value
type Const struct {
	Value int64
	Typ   Type
}

// FloatConst represents a constant floating-point value
type FloatConst struct {
	Value float64
	Typ   Type
}

// Global represents a global symbol (function or data)
type Global struct {
	Name string
}

// Temporary represents a temporary, virtual register
type Temporary struct {
	Name string
	ID   int
}

// Label represents a basic block label
type Label struct {
	Name string
}

// CastValue is a wrapper to signal an explicit cast in the backend
type CastValue struct {
	Value
	TargetType string
}

// Func represents a function in the IR
type Func struct {
	Name       string
	Params     []*Param
	ReturnType Type
	HasVarargs bool
	Blocks     []*BasicBlock
}

// Param represents a function parameter
type Param struct {
	Name string
	Typ  Type
	Val  Value
}

// BasicBlock represents a sequence of instructions ending with a terminator
type BasicBlock struct {
	Label        *Label
	Instructions []*Instruction
}

// Instruction represents a single operation with its operands
type Instruction struct {
	Op       Op
	Typ      Type    // The type of the operation/result
	Result   Value
	Args     []Value
	ArgTypes []Type  // Used for OpCall
	Align    int     // Used for OpAlloc
}

// Program is the top-level container for the entire IR
type Program struct {
	Globals          []*Data
	Strings          map[string]string // Maps string content to its label
	Funcs            []*Func
	ExtrnFuncs       []string
	ExtrnVars        map[string]bool
	WordSize         int
	BackendTempCount int
}

// Data represents a global data variable
type Data struct {
	Name  string
	Align int
	Items []DataItem
}

// DataItem represents an item within a global data definition
type DataItem struct {
	Typ   Type
	Value Value // Can be Const or Global
	Count int   // For zero-initialization (z)
}

// isValue implementations to satisfy the Value interface
func (c *Const) isValue()      {}
func (f *FloatConst) isValue() {}
func (g *Global) isValue()     {}
func (t *Temporary) isValue()  {}
func (l *Label) isValue()      {}
func (c *CastValue) isValue()  {}

// String representations for Value types.
func (c *Const) String() string      { return "" } // Handled by backend
func (f *FloatConst) String() string { return "" } // Handled by backend
func (g *Global) String() string     { return g.Name }
func (t *Temporary) String() string  { return t.Name }
func (l *Label) String() string      { return l.Name }
func (c *CastValue) String() string  { return c.Value.String() }

// GetType converts an AST type to an IR type
func GetType(typ *ast.BxType, wordSize int) Type {
	if typ == nil || typ.Kind == ast.TYPE_UNTYPED {
		return wordTypeFromSize(wordSize)
	}
	switch typ.Kind {
	case ast.TYPE_VOID:
		return TypeNone
	case ast.TYPE_POINTER, ast.TYPE_ARRAY:
		return TypePtr
	case ast.TYPE_FLOAT:
		switch typ.Name {
		case "float", "float32":
			return TypeS
		case "float64":
			return TypeD
		default:
			return TypeS
		}
	case ast.TYPE_PRIMITIVE:
		switch typ.Name {
		case "int", "uint", "string":
			return wordTypeFromSize(wordSize)
		case "int64", "uint64":
			return TypeL
		case "int32", "uint32":
			return TypeW
		case "int16", "uint16":
			return TypeH
		case "byte", "bool", "int8", "uint8":
			return TypeB
		default:
			return wordTypeFromSize(wordSize)
		}
	case ast.TYPE_STRUCT:
		return wordTypeFromSize(wordSize)
	}
	return wordTypeFromSize(wordSize)
}

func wordTypeFromSize(size int) Type {
	switch size {
	case 8:
		return TypeL
	case 4:
		return TypeW
	case 2:
		return TypeH
	case 1:
		return TypeB
	default:
		// Default to the largest supported integer size if word size is unusual
		return TypeL
	}
}

func SizeOfType(t Type, wordSize int) int64 {
	switch t {
	case TypeB:
		return 1
	case TypeH:
		return 2
	case TypeW:
		return 4
	case TypeL:
		return 8
	case TypeS:
		return 4
	case TypeD:
		return 8
	case TypePtr:
		return int64(wordSize)
	default:
		return int64(wordSize)
	}
}

// GetBackendTempCount returns the current backend temporary count
func (p *Program) GetBackendTempCount() int { return p.BackendTempCount }

// IncBackendTempCount increments and returns the new backend temporary count
func (p *Program) IncBackendTempCount() int {
	p.BackendTempCount++
	return p.BackendTempCount
}

// IsStringLabel checks if a global name corresponds to a string literal
func (p *Program) IsStringLabel(name string) (string, bool) {
	for s, label := range p.Strings {
		if label == name { return s, true }
	}
	return "", false
}

// FindFunc finds a function by name in the program.
func (p *Program) FindFunc(name string) *Func {
	for _, f := range p.Funcs {
		if f.Name == name { return f }
	}
	return nil
}
