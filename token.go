package main

type TokenType int

const (
	// Meta
	TOK_EOF TokenType = iota // End of file/input

	// Literals
	TOK_IDENT  // Identifier, e.g., my_var
	TOK_NUMBER // Numeric literal, e.g., 123, 0x1A, 'a'
	TOK_STRING // String literal, e.g., "hello"

	// Keywords
	TOK_AUTO
	TOK_EXTRN
	TOK_IF
	TOK_ELSE
	TOK_WHILE
	TOK_RETURN
	TOK_GOTO
	TOK_SWITCH
	TOK_CASE
	TOK_DEFAULT
	TOK_BREAK
	TOK___ASM__

	// Punctuation
	TOK_LPAREN   // (
	TOK_RPAREN   // )
	TOK_LBRACE   // {
	TOK_RBRACE   // }
	TOK_LBRACKET // [
	TOK_RBRACKET // ]
	TOK_SEMI     // ;
	TOK_COMMA    // ,
	TOK_COLON    // :
	TOK_QUESTION // ?
	TOK_DOTS     // ...

	// --- Operator Groups ---
	// Assignment Operators
	TOK_EQ       // =
	TOK_PLUS_EQ  // +=
	TOK_MINUS_EQ // -=
	TOK_STAR_EQ  // *=
	TOK_SLASH_EQ // /=
	TOK_REM_EQ   // %=
	TOK_AND_EQ   // &=
	TOK_OR_EQ    // |=
	TOK_XOR_EQ   // ^=
	TOK_SHL_EQ   // <<=
	TOK_SHR_EQ   // >>=
	TOK_EQ_PLUS  // =+
	TOK_EQ_MINUS // =-
	TOK_EQ_STAR  // =*
	TOK_EQ_SLASH // =/
	TOK_EQ_REM   // =%
	TOK_EQ_AND   // =&
	TOK_EQ_OR    // =|
	TOK_EQ_XOR   // =^
	TOK_EQ_SHL   // =<<
	TOK_EQ_SHR   // =>>

	// Binary Operators
	TOK_PLUS
	TOK_MINUS
	TOK_STAR
	TOK_SLASH
	TOK_REM
	TOK_AND
	TOK_OR
	TOK_XOR
	TOK_SHL
	TOK_SHR
	TOK_EQEQ
	TOK_NEQ
	TOK_LT
	TOK_GT
	TOK_GTE
	TOK_LTE

	// Unary & Postfix Operators
	TOK_NOT        // !
	TOK_COMPLEMENT // ~
	TOK_INC        // ++
	TOK_DEC        // --
)

// keywordMap maps keyword strings to their corresponding TokenType
var keywordMap = map[string]TokenType{
	"auto":    TOK_AUTO,
	"extrn":   TOK_EXTRN,
	"if":      TOK_IF,
	"else":    TOK_ELSE,
	"while":   TOK_WHILE,
	"return":  TOK_RETURN,
	"goto":    TOK_GOTO,
	"switch":  TOK_SWITCH,
	"case":    TOK_CASE,
	"default": TOK_DEFAULT,
	"break":   TOK_BREAK,
	"__asm__": TOK___ASM__,
}

// Token represents a single lexical unit from the source code
// It contains the type of the token, its value (for literals), and its position
type Token struct {
	Type      TokenType
	Value     string // Value for literals (string, number, ident)
	FileIndex int    // Index into the global sourceFiles slice
	Line      int    // Line number where the token starts
	Column    int    // Column number where the token starts
	Len       int    // Length of the token text in the source
}
