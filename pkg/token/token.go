package token

// Type represents the type of a lexical token.
type Type int

const (
	// Meta
	EOF Type = iota // End of file/input

	// Literals
	Ident  // Identifier, e.g., my_var
	Number // Numeric literal, e.g., 123, 0x1A, 'a'
	String // String literal, e.g., "hello"

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
	Asm

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

	// --- Operator Groups ---
	// Assignment Operators
	Eq      // =
	PlusEq  // +=
	MinusEq // -=
	StarEq  // *=
	SlashEq // /=
	RemEq   // %=
	AndEq   // &=
	OrEq    // |=
	XorEq   // ^=
	ShlEq   // <<=
	ShrEq   // >>=
	EqPlus  // =+
	EqMinus // =-
	EqStar  // =*
	EqSlash // =/
	EqRem   // =%
	EqAnd   // =&
	EqOr    // =|
	EqXor   // =^
	EqShl   // =<<
	EqShr   // =>>

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

	// Unary & Postfix Operators
	Not        // !
	Complement // ~
	Inc        // ++
	Dec        // --
)

// KeywordMap maps keyword strings to their corresponding Type.
var KeywordMap = map[string]Type{
	"auto":    Auto,
	"extrn":   Extrn,
	"if":      If,
	"else":    Else,
	"while":   While,
	"return":  Return,
	"goto":    Goto,
	"switch":  Switch,
	"case":    Case,
	"default": Default,
	"break":   Break,
	"__asm__": Asm,
}

// Token represents a single lexical unit from the source code.
// It contains the type of the token, its value (for literals), and its position.
type Token struct {
	Type      Type
	Value     string // Value for literals (string, number, ident)
	FileIndex int    // Index into the global sourceFiles slice
	Line      int    // Line number where the token starts
	Column    int    // Column number where the token starts
	Len       int    // Length of the token text in the source
}
