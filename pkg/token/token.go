package token

// Type represents the type of a lexical token.
type Type int

// The list of all lexical tokens.
const (
	// Meta tokens
	EOF Type = iota
	Comment
	Directive // '// [b]: ...'

	// Literals
	Ident  // main
	Number // 123, 0x7b, 0173
	String // "A saucerful of secrets"

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

	// Bx Type System Keywords
	Void
	TypeKeyword // 'type'
	Struct
	Const
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
	Any           // For untyped, and symbols marked explicitely with any()

	// Punctuation
	LParen   // (
	RParen   // )
	LBrace   // {
	RBrace   // }
	LBracket // [
	RBracket // ]
	Semi     // ;
	Comma    // ,
	Colon    // :
	Question // ?
	Dots     // ...
	Dot      // .

	// --- Operator Groups ---

	// Assignment Operators
	Eq      // =
	Define  // :=
	PlusEq  // += (C-style)
	MinusEq // -= (C-style)
	StarEq  // *= (C-style)
	SlashEq // /= (C-style)
	RemEq   // %= (C-style)
	AndEq   // &= (C-style)
	OrEq    // |= (C-style)
	XorEq   // ^= (C-style)
	ShlEq   // <<= (C-style)
	ShrEq   // >>= (C-style)
	EqPlus  // =+ (B-style)
	EqMinus // =- (B-style)
	EqStar  // =* (B-style)
	EqSlash // =/ (B-style)
	EqRem   // =% (B-style)
	EqAnd   // =& (B-style)
	EqOr    // =| (B-style)
	EqXor   // =^ (B-style)
	EqShl   // =<< (B-style)
	EqShr   // =>> (B-style)

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
	AndAnd // &&
	OrOr   // ||

	// Unary & Postfix Operators
	Not        // !
	Complement // ~
	Inc        // ++
	Dec        // --
)

// KeywordMap maps keyword strings to their corresponding token Type
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

// Token represents a single lexical unit from the source code
type Token struct {
	Type      Type
	Value     string
	FileIndex int
	Line      int
	Column    int
	Len       int
}
