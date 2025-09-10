package token

type Type int

const (
	EOF Type = iota
	Comment
	Directive
	Ident
	Number
	FloatNumber
	String
	Auto
	Extrn
	If
	Else
	While
	Return
	Goto
	Switch
	Case
	Default
	Break
	Continue
	Asm
	Nil
	Null
	TypeKeyword
	Struct
	Enum
	Const
	Void
	Bool
	Byte
	Int
	Uint
	Int8
	Uint8
	Int16
	Uint16
	Int32
	Uint32
	Int64
	Uint64
	Float
	Float32
	Float64
	StringKeyword
	Any
	TypeOf
	LParen
	RParen
	LBrace
	RBrace
	LBracket
	RBracket
	Semi
	Comma
	Colon
	Question
	Dots
	Dot
	Eq
	Define
	PlusEq
	MinusEq
	StarEq
	SlashEq
	RemEq
	AndEq
	OrEq
	XorEq
	ShlEq
	ShrEq
	EqPlus
	EqMinus
	EqStar
	EqSlash
	EqRem
	EqAnd
	EqOr
	EqXor
	EqShl
	EqShr
	Plus
	Minus
	Star
	Slash
	Rem
	And
	Or
	Xor
	Shl
	Shr
	EqEq
	Neq
	Lt
	Gt
	Gte
	Lte
	AndAnd
	OrOr
	Not
	Complement
	Inc
	Dec
)

var KeywordMap = map[string]Type{
	"auto":     Auto,
	"if":       If,
	"else":     Else,
	"while":    While,
	"return":   Return,
	"goto":     Goto,
	"switch":   Switch,
	"case":     Case,
	"default":  Default,
	"extrn":    Extrn,
	"__asm__":  Asm,
	"break":    Break,
	"continue": Continue,
	"nil":      Nil,
	"null":     Null,
	"void":     Void,
	"type":     TypeKeyword,
	"struct":   Struct,
	"enum":     Enum,
	"const":    Const,
	"bool":     Bool,
	"byte":     Byte,
	"int":      Int,
	"uint":     Uint,
	"int8":     Int8,
	"uint8":    Uint8,
	"int16":    Int16,
	"uint16":   Uint16,
	"int32":    Int32,
	"uint32":   Uint32,
	"int64":    Int64,
	"uint64":   Uint64,
	"float":    Float,
	"float32":  Float32,
	"float64":  Float64,
	"string":   StringKeyword,
	"any":      Any,
	"typeof":   TypeOf,
}

// Reverse mapping from Type to the keyword string
var TypeStrings = make(map[Type]string)

func init() {
	for str, typ := range KeywordMap {
		TypeStrings[typ] = str
	}
}

type Token struct {
	Type      Type
	Value     string
	FileIndex int
	Line      int
	Column    int
	Len       int
}
