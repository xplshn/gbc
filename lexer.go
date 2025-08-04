package main

import (
	"strconv"
	"strings"
	"unicode"
)

// Lexer holds the state required for tokenizing a source string.
type Lexer struct {
	source    []rune // Source code as a slice of runes for correct Unicode handling.
	fileIndex int    // The index of the current file in the global sourceFiles slice.
	pos       int    // Current position in the source (index into the rune slice).
	line      int    // Current line number, for error reporting.
	column    int    // Current column number, for error reporting.
	peeked    Token  // A peeked token, for lookahead.
	hasPeeked bool   // Flag indicating if a token has been peeked.
}

// NewLexer creates and initializes a new Lexer instance for a given source
func NewLexer(source []rune, fileIndex int) *Lexer {
	return &Lexer{
		source:    source,
		fileIndex: fileIndex,
		line:      1,
		column:    1,
	}
}

// Next consumes and returns the next token from the source
func (l *Lexer) Next() Token {
	if l.hasPeeked {
		l.hasPeeked = false
		return l.peeked
	}
	return l.getToken()
}

// Peek returns the next token without consuming it
func (l *Lexer) Peek() Token {
	if !l.hasPeeked {
		l.peeked = l.getToken()
		l.hasPeeked = true
	}
	return l.peeked
}

// Character handling
func (l *Lexer) peek() rune {
	if l.pos >= len(l.source) {
		return 0
	}
	return l.source[l.pos]
}

func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.source) {
		return 0
	}
	return l.source[l.pos+1]
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.source) {
		return 0
	}
	r := l.source[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return r
}

// Token creation
func (l *Lexer) makeToken(tokType TokenType, value string, startCol, length int) Token {
	return Token{
		Type:      tokType,
		Value:     value,
		FileIndex: l.fileIndex,
		Line:      l.line,
		Column:    startCol,
		Len:       length,
	}
}

func (l *Lexer) makeSimpleToken(tokType TokenType, length int) Token {
	startCol := l.column
	// Capture the line number at the start of the token
	line := l.line
	for i := 0; i < length; i++ {
		l.advance()
	}
	tok := l.makeToken(tokType, "", startCol, length)
	tok.Line = line // Ensure the token's line is the starting line
	return tok
}

// Main Lexing Logic
func (l *Lexer) getToken() Token {
	l.skipWhitespace()
	startCol := l.column
	startLine := l.line

	c := l.peek()
	if c == 0 {
		return l.makeToken(TOK_EOF, "", startCol, 0)
	}

	if unicode.IsLetter(c) || c == '_' {
		return l.identifierOrKeyword()
	}
	if unicode.IsDigit(c) {
		return l.numberLiteral()
	}

	// For single-character tokens, we can create a helper
	makeCharToken := func(typ TokenType) Token {
		tok := l.makeSimpleToken(typ, 1)
		tok.Line = startLine
		tok.Column = startCol
		return tok
	}

	switch c {
	case '"':
		return l.stringLiteral()
	case '\'':
		return l.charLiteral()
	case '(':
		return makeCharToken(TOK_LPAREN)
	case ')':
		return makeCharToken(TOK_RPAREN)
	case '{':
		return makeCharToken(TOK_LBRACE)
	case '}':
		return makeCharToken(TOK_RBRACE)
	case '[':
		return makeCharToken(TOK_LBRACKET)
	case ']':
		return makeCharToken(TOK_RBRACKET)
	case ';':
		return makeCharToken(TOK_SEMI)
	case ',':
		return makeCharToken(TOK_COMMA)
	case ':':
		return makeCharToken(TOK_COLON)
	case '?':
		return makeCharToken(TOK_QUESTION)
	case '~':
		return makeCharToken(TOK_COMPLEMENT)
	case '.':
		if l.peekNext() == '.' && l.pos+2 < len(l.source) && l.source[l.pos+2] == '.' {
			return l.makeSimpleToken(TOK_DOTS, 3)
		}
	case '+':
		if l.peekNext() == '+' {
			return l.makeSimpleToken(TOK_INC, 2)
		}
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '+=' used")
			return l.makeSimpleToken(TOK_PLUS_EQ, 2)
		}
		return makeCharToken(TOK_PLUS)
	case '-':
		if l.peekNext() == '-' {
			return l.makeSimpleToken(TOK_DEC, 2)
		}
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '-=' used")
			return l.makeSimpleToken(TOK_MINUS_EQ, 2)
		}
		return makeCharToken(TOK_MINUS)
	case '*':
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '*=' used")
			return l.makeSimpleToken(TOK_STAR_EQ, 2)
		}
		return makeCharToken(TOK_STAR)
	case '/':
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '/=' used")
			return l.makeSimpleToken(TOK_SLASH_EQ, 2)
		}
		return makeCharToken(TOK_SLASH)
	case '%':
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '%%=' used")
			return l.makeSimpleToken(TOK_REM_EQ, 2)
		}
		return makeCharToken(TOK_REM)
	case '&':
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '&=' used")
			return l.makeSimpleToken(TOK_AND_EQ, 2)
		}
		return makeCharToken(TOK_AND)
	case '|':
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '|=' used")
			return l.makeSimpleToken(TOK_OR_EQ, 2)
		}
		return makeCharToken(TOK_OR)
	case '^':
		if l.peekNext() == '=' && IsFeatureEnabled(FEAT_C_OPS) {
			Warning(WARN_C_OPS, l.currentToken(), "C-style operator '^=' used")
			return l.makeSimpleToken(TOK_XOR_EQ, 2)
		}
		return makeCharToken(TOK_XOR)
	case '!':
		if l.peekNext() == '=' {
			return l.makeSimpleToken(TOK_NEQ, 2)
		}
		return makeCharToken(TOK_NOT)
	case '<':
		if l.peekNext() == '<' {
			if l.pos+2 < len(l.source) && l.source[l.pos+2] == '=' && IsFeatureEnabled(FEAT_C_OPS) {
				Warning(WARN_C_OPS, l.currentToken(), "C-style operator '<<=' used")
				return l.makeSimpleToken(TOK_SHL_EQ, 3)
			}
			return l.makeSimpleToken(TOK_SHL, 2)
		}
		if l.peekNext() == '=' {
			return l.makeSimpleToken(TOK_LTE, 2)
		}
		return makeCharToken(TOK_LT)
	case '>':
		if l.peekNext() == '>' {
			if l.pos+2 < len(l.source) && l.source[l.pos+2] == '=' && IsFeatureEnabled(FEAT_C_OPS) {
				Warning(WARN_C_OPS, l.currentToken(), "C-style operator '>>=' used")
				return l.makeSimpleToken(TOK_SHR_EQ, 3)
			}
			return l.makeSimpleToken(TOK_SHR, 2)
		}
		if l.peekNext() == '=' {
			return l.makeSimpleToken(TOK_GTE, 2)
		}
		return makeCharToken(TOK_GT)
	case '=':
		next := l.peekNext()
		if next == '=' {
			return l.makeSimpleToken(TOK_EQEQ, 2)
		}
		if IsFeatureEnabled(FEAT_B_OPS) {
			if next == '<' && l.pos+2 < len(l.source) && l.source[l.pos+2] == '<' {
				Warning(WARN_B_OPS, l.currentToken(), "B-style operator '=<<' used")
				return l.makeSimpleToken(TOK_EQ_SHL, 3)
			}
			if next == '>' && l.pos+2 < len(l.source) && l.source[l.pos+2] == '>' {
				Warning(WARN_B_OPS, l.currentToken(), "B-style operator '=>>' used")
				return l.makeSimpleToken(TOK_EQ_SHR, 3)
			}
			if strings.ContainsRune("+-*/%&|^", next) {
				Warning(WARN_B_OPS, l.currentToken(), "B-style assignment operator used")
				switch next {
				case '+':
					return l.makeSimpleToken(TOK_EQ_PLUS, 2)
				case '-':
					return l.makeSimpleToken(TOK_EQ_MINUS, 2)
				case '*':
					return l.makeSimpleToken(TOK_EQ_STAR, 2)
				case '/':
					return l.makeSimpleToken(TOK_EQ_SLASH, 2)
				case '%':
					return l.makeSimpleToken(TOK_EQ_REM, 2)
				case '&':
					return l.makeSimpleToken(TOK_EQ_AND, 2)
				case '|':
					return l.makeSimpleToken(TOK_EQ_OR, 2)
				case '^':
					return l.makeSimpleToken(TOK_EQ_XOR, 2)
				}
			}
		}
		return makeCharToken(TOK_EQ)
	}

	Error(l.currentToken(), "Unexpected character: '%c' (ASCII %d)", c, c)
	return l.makeToken(TOK_EOF, "", startCol, 0) // Unreachable
}

// currentToken creates a token representing the current character for error reporting.
func (l *Lexer) currentToken() Token {
	return Token{
		Type:      0, // Type doesn't matter for error reporting
		Value:     string(l.peek()),
		FileIndex: l.fileIndex,
		Line:      l.line,
		Column:    l.column,
		Len:       1,
	}
}

// Helpers
func (l *Lexer) skipWhitespace() {
	for {
		c := l.peek()
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			l.advance()
			continue
		}
		if c == '/' && l.peekNext() == '*' {
			startTok := l.currentToken()
			l.advance() // consume /
			l.advance() // consume *
			for {
				if l.peek() == 0 {
					Error(startTok, "Unterminated block comment")
					return
				}
				if l.peek() == '*' && l.peekNext() == '/' {
					l.advance()
					l.advance()
					break
				}
				l.advance()
			}
			continue
		}
		if IsFeatureEnabled(FEAT_C_COMMENTS) {
			if c == '/' && l.peekNext() == '/' {
				Warning(WARN_C_COMMENTS, l.currentToken(), "Using non-standard C-style '//' comment.")
				for l.peek() != '\n' && l.peek() != 0 {
					l.advance()
				}
				continue
			}
		}
		break
	}
}

func (l *Lexer) identifierOrKeyword() Token {
	startCol := l.column
	startLine := l.line
	startPos := l.pos
	for {
		c := l.peek()
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			break
		}
		l.advance()
	}
	value := string(l.source[startPos:l.pos])
	tok := l.makeToken(TOK_IDENT, value, startCol, len(value))
	tok.Line = startLine
	if tokType, isKeyword := keywordMap[value]; isKeyword {
		tok.Type = tokType
		tok.Value = "" // Keywords have no value
	}
	return tok
}

func (l *Lexer) numberLiteral() Token {
	startCol := l.column
	startLine := l.line
	startPos := l.pos
	for unicode.IsDigit(l.peek()) || (l.peek() == 'x' || l.peek() == 'X') || (l.peek() >= 'a' && l.peek() <= 'f') || (l.peek() >= 'A' && l.peek() <= 'F') {
		l.advance()
	}
	valueStr := string(l.source[startPos:l.pos])
	tok := l.makeToken(TOK_NUMBER, "", startCol, len(valueStr))
	tok.Line = startLine

	val, err := strconv.ParseUint(valueStr, 0, 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			Warning(WARN_OVERFLOW, tok, "Integer constant is out of range.")
		} else {
			Error(tok, "Invalid number literal: %s", valueStr)
		}
	}
	tok.Value = strconv.FormatInt(int64(val), 10)
	return tok
}

func (l *Lexer) stringLiteral() Token {
	startCol := l.column
	startLine := l.line
	startPos := l.pos
	l.advance() // consume opening "
	var sb strings.Builder

	for {
		c := l.peek()
		if c == 0 {
			errTok := l.makeToken(TOK_STRING, "", startCol, l.pos-startPos)
			errTok.Line = startLine
			Error(errTok, "Unterminated string literal")
			return l.makeToken(TOK_EOF, "", l.column, 0)
		}
		if c == '"' {
			l.advance() // consume closing "
			break
		}

		errTok := l.currentToken()
		if c == '\\' && IsFeatureEnabled(FEAT_C_ESCAPES) {
			l.advance() // consume '\'
			if !IsFeatureEnabled(FEAT_B_ESCAPES) { // Check if we are in a mode that should warn
				Warning(WARN_C_ESCAPES, errTok, "Using C-style '\\' escape in string literal")
			}
			val := l.decodeEscape('\\')
			sb.WriteRune(rune(val))
		} else if c == '*' && IsFeatureEnabled(FEAT_B_ESCAPES) {
			l.advance() // consume '*'
			if IsFeatureEnabled(FEAT_C_ESCAPES) {
				Warning(WARN_B_ESCAPES, errTok, "Using B-style '*' escape in string literal")
			}
			val := l.decodeEscape('*')
			sb.WriteRune(rune(val))
		} else {
			l.advance()
			sb.WriteRune(c)
		}
	}
	tok := l.makeToken(TOK_STRING, sb.String(), startCol, l.pos-startPos)
	tok.Line = startLine
	return tok
}

func (l *Lexer) charLiteral() Token {
	startCol := l.column
	startLine := l.line
	startPos := l.pos
	l.advance() // consume opening '

	var word int64
	charCount := 0

	for l.peek() != '\'' && l.peek() != 0 {
		var val int64
		c := l.peek()
		errTok := l.currentToken()

		if c == '\\' && IsFeatureEnabled(FEAT_C_ESCAPES) {
			l.advance() // consume '\'
			if !IsFeatureEnabled(FEAT_B_ESCAPES) {
				Warning(WARN_C_ESCAPES, errTok, "Using C-style '\\' escape in character literal")
			}
			val = l.decodeEscape('\\')
		} else if c == '*' && IsFeatureEnabled(FEAT_B_ESCAPES) {
			l.advance() // consume '*'
			if IsFeatureEnabled(FEAT_C_ESCAPES) {
				Warning(WARN_B_ESCAPES, errTok, "Using B-style '*' escape in character literal")
			}
			val = l.decodeEscape('*')
		} else {
			l.advance()
			val = int64(c)
		}

		charCount++
		if charCount > 8 { // Assuming 64-bit words. TODO: We want fully UTF8 strings, but is this correct?
			Warning(WARN_LONG_CHAR_CONST, errTok, "Multi-character constant may overflow word size")
		}
		word = (word << 8) | (val & 0xFF)
	}

	tok := l.makeToken(TOK_NUMBER, "", startCol, l.pos-startPos)
	tok.Line = startLine

	if l.peek() != '\'' {
		Error(tok, "Unterminated character literal")
	}
	l.advance() // consume closing '

	tok.Value = strconv.FormatInt(word, 10)
	return tok
}

func (l *Lexer) decodeEscape(escapeChar rune) int64 {
	c := l.peek()
	errTok := l.currentToken()
	if c == 0 {
		Error(errTok, "Unterminated escape sequence")
		return 0
	}
	l.advance() // consume char after escape

	if escapeChar == '\\' {
		switch c {
		case 'n':
			return '\n'
		case 't':
			return '\t'
		case 'v':
			return '\v'
		case 'b':
			return '\b'
		case 'r':
			return '\r'
		case 'f':
			return '\f'
		case 'a':
			return '\a'
		case '\\':
			return '\\'
		case '\'':
			return '\''
		case '"':
			return '"'
		case '?':
			return '?'
		default:
			Warning(WARN_UNRECOGNIZED_ESCAPE, errTok, "Unrecognized escape sequence '\\%c'", c)
			return int64(c)
		}
	} else { // B-style '*' escapes
		switch c {
		case 'n':
			return '\n'
		case 't':
			return '\t'
		case 'e':
			return 4 // EOT
		case 'b':
			return '\b'
		case 'r':
			return '\r'
		case '0':
			return 0
		case '(':
			return '{'
		case ')':
			return '}'
		case '*':
			return '*'
		case '\'':
			return '\''
		default:
			Warning(WARN_UNRECOGNIZED_ESCAPE, errTok, "Unrecognized escape sequence '*%c'", c)
			return int64(c)
		}
	}
}
