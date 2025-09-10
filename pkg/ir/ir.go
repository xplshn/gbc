package ir

import (
	"github.com/xplshn/gbc/pkg/ast"
)

type Op int

const (
	OpAlloc Op = iota
	OpLoad
	OpStore
	OpBlit
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
	OpAddF
	OpSubF
	OpMulF
	OpDivF
	OpRemF
	OpNegF
	OpCEq
	OpCNeq
	OpCLt
	OpCGt
	OpCLe
	OpCGe
	OpExtSB
	OpExtUB
	OpExtSH
	OpExtUH
	OpExtSW
	OpExtUW
	OpTrunc
	OpCast
	OpFToSI
	OpFToUI
	OpSWToF
	OpUWToF
	OpSLToF
	OpULToF
	OpFToF
	OpJmp
	OpJnz
	OpRet
	OpCall
	OpPhi
)

type Type int

const (
	TypeNone Type = iota
	TypeB         // byte (8-bit, ambiguous signedness)
	TypeH         // half-word (16-bit, ambiguous signedness)
	TypeW         // word (32-bit)
	TypeL         // long (64-bit)
	TypeS         // single float (32-bit)
	TypeD         // double float (64-bit)
	TypePtr
	TypeSB // signed byte (8-bit)
	TypeUB // unsigned byte (8-bit)
	TypeSH // signed half-word (16-bit)
	TypeUH // unsigned half-word (16-bit)
)

type Value interface {
	isValue()
	String() string
}

type Const struct{ Value int64 }
type FloatConst struct{ Value float64; Typ Type }
type Global struct{ Name string }
type Temporary struct{ Name string; ID int }
type Label struct{ Name string }
type CastValue struct{ Value; TargetType string }

func (c *Const) isValue()      {}
func (f *FloatConst) isValue() {}
func (g *Global) isValue()     {}
func (t *Temporary) isValue()  {}
func (l *Label) isValue()      {}
func (c *CastValue) isValue()  {}

func (c *Const) String() string      { return "" }
func (f *FloatConst) String() string { return "" }
func (g *Global) String() string     { return g.Name }
func (t *Temporary) String() string  { return t.Name }
func (l *Label) String() string      { return l.Name }
func (c *CastValue) String() string  { return c.Value.String() }

type Func struct {
	Name          string
	Params        []*Param
	AstParams     []*ast.Node
	ReturnType    Type
	AstReturnType *ast.BxType
	HasVarargs    bool
	Blocks        []*BasicBlock
	Node          *ast.Node
}

type Param struct{ Name string; Typ Type; Val Value }

type BasicBlock struct{ Label *Label; Instructions []*Instruction }

type Instruction struct {
	Op          Op
	Typ         Type
	OperandType Type
	Result      Value
	Args        []Value
	ArgTypes    []Type
	Align       int
}

type Program struct {
	Globals          []*Data
	Strings          map[string]string
	Funcs            []*Func
	ExtrnFuncs       []string
	ExtrnVars        map[string]bool
	WordSize         int
	BackendTempCount int
	GlobalSymbols    map[string]*ast.Node
}

type Data struct {
	Name    string
	Align   int
	AstType *ast.BxType
	Items   []DataItem
}

type DataItem struct{ Typ Type; Value Value; Count int }

func GetType(typ *ast.BxType, wordSize int) Type {
	if typ == nil || typ.Kind == ast.TYPE_UNTYPED { return typeFromSize(wordSize, false) }

	switch typ.Kind {
	case ast.TYPE_LITERAL_INT: return typeFromSize(wordSize, false)
	case ast.TYPE_LITERAL_FLOAT: return typeFromSize(wordSize, true)
	case ast.TYPE_VOID: return TypeNone
	case ast.TYPE_POINTER, ast.TYPE_ARRAY, ast.TYPE_STRUCT: return TypePtr
	case ast.TYPE_ENUM: return typeFromSize(wordSize, false)
	case ast.TYPE_FLOAT:
		size := getTypeSizeByName(typ.Name, wordSize)
		return typeFromSize(int(size), true)
	case ast.TYPE_PRIMITIVE:
		switch typ.Name {
		case "int", "uint", "string": return typeFromSize(wordSize, false)
		case "int64", "uint64": return TypeL
		case "int32", "uint32": return TypeW
		case "int16": return TypeSH
		case "uint16": return TypeUH
		case "int8": return TypeSB
		case "uint8": return TypeUB
		case "byte", "bool": return TypeB
		default: return typeFromSize(wordSize, false)
		}
	}
	return typeFromSize(wordSize, false)
}

func typeFromSize(size int, isFloat bool) Type {
	if isFloat {
		switch size {
		case 4: return TypeS
		case 8: return TypeD
		default: return TypeD
		}
	}

	switch size {
	case 8: return TypeL
	case 4: return TypeW
	case 2: return TypeH
	case 1: return TypeB
	default: return TypeL
	}
}

func getTypeSizeByName(typeName string, wordSize int) int64 {
	switch typeName {
	case "int64", "uint64", "float64": return 8
	case "int32", "uint32", "float32": return 4
	case "int16", "uint16": return 2
	case "int8", "uint8", "byte", "bool": return 1
	case "int", "uint", "string", "float": return int64(wordSize)
	}
	return 0
}

type TypeSizeResolver struct{ wordSize int }

func NewTypeSizeResolver(wordSize int) *TypeSizeResolver { return &TypeSizeResolver{wordSize: wordSize} }

func (r *TypeSizeResolver) GetTypeSize(typeName string) int64 { return getTypeSizeByName(typeName, r.wordSize) }

func floatTypeFromSize(size int) Type { return typeFromSize(size, true) }

func SizeOfType(t Type, wordSize int) int64 {
	switch t {
	case TypeB, TypeSB, TypeUB: return 1
	case TypeH, TypeSH, TypeUH: return 2
	case TypeW: return 4
	case TypeL: return 8
	case TypeS: return 4
	case TypeD: return 8
	case TypePtr: return int64(wordSize)
	default:
		return int64(wordSize)
	}
}

func (p *Program) GetBackendTempCount() int { return p.BackendTempCount }
func (p *Program) IncBackendTempCount()     { p.BackendTempCount++ }

func (p *Program) IsStringLabel(name string) (string, bool) {
	for s, label := range p.Strings {
		if label == name { return s, true }
	}
	return "", false
}

func (p *Program) FindFunc(name string) *Func {
	for _, f := range p.Funcs {
		if f.Name == name { return f }
	}
	return nil
}

func (p *Program) FindFuncSymbol(name string) *ast.Node {
	if p.GlobalSymbols != nil {
		if node, ok := p.GlobalSymbols[name]; ok {
			if _, isFunc := node.Data.(ast.FuncDeclNode); isFunc { return node }
		}
	}
	return nil
}
