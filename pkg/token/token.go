package token

type Type int

const (
	// Meta
	EOF Type = iota
	Comment
	Directive

	// Literals
	Ident
	Number
	String

	// Keywords
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
	Asm // `__asm__`

	// Bx Type System Keywords that are not types themselves
	TypeKeyword // 'type'
	Struct
	Const

	// Bx Type System Keywords that ARE types
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
	StringKeyword // 'string'
	Any

	// Punctuation
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

	// Assignment Operators
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

	// Binary Operators
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

	// Unary & Postfix Operators
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
	"void":     Void,
	"type":     TypeKeyword,
	"struct":   Struct,
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
}

type Token struct {
	Type      Type
	Value     string
	FileIndex int
	Line      int
	Column    int
	Len       int
}
