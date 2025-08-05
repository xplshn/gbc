package lexer

import (
	"strconv"
	"strings"
	"unicode"

	"gbc/pkg/token"
	"gbc/pkg/util"
)

// Lexer holds the state required for tokenizing a source string.
type Lexer struct {
	source    []rune // Source code as a slice of runes for correct Unicode handling.
	fileIndex int    // The index of the current file in the global sourceFiles slice.
	pos       int    // Current position in the source (index into the rune slice).
	line      int    // Current line number, for error reporting.
	column    int    // Current column number, for error reporting.
	peeked    token.Token  // A peeked token, for lookahead.
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
func (l *Lexer) Next() token.Token {
	if l.hasPeeked {
		l.hasPeeked = false
		return l.peeked
	}
	return l.getToken()
}

// Peek returns the next token without consuming it
func (l *Lexer) Peek() token.Token {
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
func (l *Lexer) makeToken(tokType token.Type, value string, startCol, length int) token.Token {
	return token.Token{
		Type:      tokType,
		Value:     value,
		FileIndex: l.fileIndex,
		Line:      l.line,
		Column:    startCol,
		Len:       length,
	}
}

func (l *Lexer) makeSimpleToken(tokType token.Type, length int) token.Token {
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
func (l *Lexer) getToken() token.Token {
	l.skipWhitespace()
	startCol := l.column
	startLine := l.line

	c := l.peek()
	if c == 0 {
		return l.makeToken(token.EOF, "", startCol, 0)
	}

	if unicode.IsLetter(c) || c == '_' {
		return l.identifierOrKeyword()
	}
	if unicode.IsDigit(c) {
		return l.numberLiteral()
	}

	// For single-character tokens, we can create a helper
	makeCharToken := func(typ token.Type) token.Token {
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
		return makeCharToken(token.LParen)
	case ')':
		return makeCharToken(token.RParen)
	case '{':
		return makeCharToken(token.LBrace)
	case '}':
		return makeCharToken(token.RBrace)
	case '[':
		return makeCharToken(token.LBracket)
	case ']':
		return makeCharToken(token.RBracket)
	case ';':
		return makeCharToken(token.Semi)
	case ',':
		return makeCharToken(token.Comma)
	case ':':
		return makeCharToken(token.Colon)
	case '?':
		return makeCharToken(token.Question)
	case '~':
		return makeCharToken(token.Complement)
	case '.':
		if l.peekNext() == '.' && l.pos+2 < len(l.source) && l.source[l.pos+2] == '.' {
			return l.makeSimpleToken(token.Dots, 3)
		}
	case '+':
		if l.peekNext() == '+' {
			return l.makeSimpleToken(token.Inc, 2)
		}
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '+=' used")
			return l.makeSimpleToken(token.PlusEq, 2)
		}
		return makeCharToken(token.Plus)
	case '-':
		if l.peekNext() == '-' {
			return l.makeSimpleToken(token.Dec, 2)
		}
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '-=' used")
			return l.makeSimpleToken(token.MinusEq, 2)
		}
		return makeCharToken(token.Minus)
	case '*':
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '*=' used")
			return l.makeSimpleToken(token.StarEq, 2)
		}
		return makeCharToken(token.Star)
	case '/':
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '/=' used")
			return l.makeSimpleToken(token.SlashEq, 2)
		}
		return makeCharToken(token.Slash)
	case '%':
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '%%=' used")
			return l.makeSimpleToken(token.RemEq, 2)
		}
		return makeCharToken(token.Rem)
	case '&':
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '&=' used")
			return l.makeSimpleToken(token.AndEq, 2)
		}
		return makeCharToken(token.And)
	case '|':
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '|=' used")
			return l.makeSimpleToken(token.OrEq, 2)
		}
		return makeCharToken(token.Or)
	case '^':
		if l.peekNext() == '=' && util.IsFeatureEnabled(util.FeatCOps) {
			util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '^=' used")
			return l.makeSimpleToken(token.XorEq, 2)
		}
		return makeCharToken(token.Xor)
	case '!':
		if l.peekNext() == '=' {
			return l.makeSimpleToken(token.Neq, 2)
		}
		return makeCharToken(token.Not)
	case '<':
		if l.peekNext() == '<' {
			if l.pos+2 < len(l.source) && l.source[l.pos+2] == '=' && util.IsFeatureEnabled(util.FeatCOps) {
				util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '<<=' used")
				return l.makeSimpleToken(token.ShlEq, 3)
			}
			return l.makeSimpleToken(token.Shl, 2)
		}
		if l.peekNext() == '=' {
			return l.makeSimpleToken(token.Lte, 2)
		}
		return makeCharToken(token.Lt)
	case '>':
		if l.peekNext() == '>' {
			if l.pos+2 < len(l.source) && l.source[l.pos+2] == '=' && util.IsFeatureEnabled(util.FeatCOps) {
				util.Warn(util.WarnCOps, l.currentToken(), "C-style operator '>>=' used")
				return l.makeSimpleToken(token.ShrEq, 3)
			}
			return l.makeSimpleToken(token.Shr, 2)
		}
		if l.peekNext() == '=' {
			return l.makeSimpleToken(token.Gte, 2)
		}
		return makeCharToken(token.Gt)
	case '=':
		next := l.peekNext()
		if next == '=' {
			return l.makeSimpleToken(token.EqEq, 2)
		}
		if util.IsFeatureEnabled(util.FeatBOps) {
			if next == '<' && l.pos+2 < len(l.source) && l.source[l.pos+2] == '<' {
				util.Warn(util.WarnBOps, l.currentToken(), "B-style operator '=<<' used")
				return l.makeSimpleToken(token.EqShl, 3)
			}
			if next == '>' && l.pos+2 < len(l.source) && l.source[l.pos+2] == '>' {
				util.Warn(util.WarnBOps, l.currentToken(), "B-style operator '=>>' used")
				return l.makeSimpleToken(token.EqShr, 3)
			}
			if strings.ContainsRune("+-*/%&|^", next) {
				util.Warn(util.WarnBOps, l.currentToken(), "B-style assignment operator used")
				switch next {
				case '+':
					return l.makeSimpleToken(token.EqPlus, 2)
				case '-':
					return l.makeSimpleToken(token.EqMinus, 2)
				case '*':
					return l.makeSimpleToken(token.EqStar, 2)
				case '/':
					return l.makeSimpleToken(token.EqSlash, 2)
				case '%':
					return l.makeSimpleToken(token.EqRem, 2)
				case '&':
					return l.makeSimpleToken(token.EqAnd, 2)
				case '|':
					return l.makeSimpleToken(token.EqOr, 2)
				case '^':
					return l.makeSimpleToken(token.EqXor, 2)
				}
			}
		}
		return makeCharToken(token.Eq)
	}

	util.Error(l.currentToken(), "Unexpected character: '%c' (ASCII %d)", c, c)
	return l.makeToken(token.EOF, "", startCol, 0) // Unreachable
}

// currentToken creates a token representing the current character for error reporting.
func (l *Lexer) currentToken() token.Token {
	return token.Token{
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
					util.Error(startTok, "Unterminated block comment")
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
		if util.IsFeatureEnabled(util.FeatCComments) {
			if c == '/' && l.peekNext() == '/' {
				util.Warn(util.WarnCComments, l.currentToken(), "Using non-standard C-style '//' comment.")
				for l.peek() != '\n' && l.peek() != 0 {
					l.advance()
				}
				continue
			}
		}
		break
	}
}

func (l *Lexer) identifierOrKeyword() token.Token {
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
	tok := l.makeToken(token.Ident, value, startCol, len(value))
	tok.Line = startLine
	if tokType, isKeyword := token.KeywordMap[value]; isKeyword {
		tok.Type = tokType
		tok.Value = "" // Keywords have no value
	}
	return tok
}

func (l *Lexer) numberLiteral() token.Token {
	startCol := l.column
	startLine := l.line
	startPos := l.pos
	for unicode.IsDigit(l.peek()) || (l.peek() == 'x' || l.peek() == 'X') || (l.peek() >= 'a' && l.peek() <= 'f') || (l.peek() >= 'A' && l.peek() <= 'F') {
		l.advance()
	}
	valueStr := string(l.source[startPos:l.pos])
	tok := l.makeToken(token.Number, "", startCol, len(valueStr))
	tok.Line = startLine

	val, err := strconv.ParseUint(valueStr, 0, 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			util.Warn(util.WarnOverflow, tok, "Integer constant is out of range.")
		} else {
			util.Error(tok, "Invalid number literal: %s", valueStr)
		}
	}
	tok.Value = strconv.FormatInt(int64(val), 10)
	return tok
}

func (l *Lexer) stringLiteral() token.Token {
	startCol := l.column
	startLine := l.line
	startPos := l.pos
	l.advance() // consume opening "
	var sb strings.Builder

	for {
		c := l.peek()
		if c == 0 {
			errTok := l.makeToken(token.String, "", startCol, l.pos-startPos)
			errTok.Line = startLine
			util.Error(errTok, "Unterminated string literal")
			return l.makeToken(token.EOF, "", l.column, 0)
		}
		if c == '"' {
			l.advance() // consume closing "
			break
		}

		errTok := l.currentToken()
		if c == '\\' && util.IsFeatureEnabled(util.FeatCEscapes) {
			l.advance() // consume '\'
			if !util.IsFeatureEnabled(util.FeatBEscapes) { // Check if we are in a mode that should warn
				util.Warn(util.WarnCEscapes, errTok, "Using C-style '\\' escape in string literal")
			}
			val := l.decodeEscape('\\')
			sb.WriteRune(rune(val))
		} else if c == '*' && util.IsFeatureEnabled(util.FeatBEscapes) {
			l.advance() // consume '*'
			if util.IsFeatureEnabled(util.FeatCEscapes) {
				util.Warn(util.WarnBEscapes, errTok, "Using B-style '*' escape in string literal")
			}
			val := l.decodeEscape('*')
			sb.WriteRune(rune(val))
		} else {
			l.advance()
			sb.WriteRune(c)
		}
	}
	tok := l.makeToken(token.String, sb.String(), startCol, l.pos-startPos)
	tok.Line = startLine
	return tok
}

func (l *Lexer) charLiteral() token.Token {
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

		if c == '\\' && util.IsFeatureEnabled(util.FeatCEscapes) {
			l.advance() // consume '\'
			if !util.IsFeatureEnabled(util.FeatBEscapes) {
				util.Warn(util.WarnCEscapes, errTok, "Using C-style '\\' escape in character literal")
			}
			val = l.decodeEscape('\\')
		} else if c == '*' && util.IsFeatureEnabled(util.FeatBEscapes) {
			l.advance() // consume '*'
			if util.IsFeatureEnabled(util.FeatCEscapes) {
				util.Warn(util.WarnBEscapes, errTok, "Using B-style '*' escape in character literal")
			}
			val = l.decodeEscape('*')
		} else {
			l.advance()
			val = int64(c)
		}

		charCount++
		if charCount > 8 { // Assuming 64-bit words.
			util.Warn(util.WarnLongCharConst, errTok, "Multi-character constant may overflow word size")
		}
		word = (word << 8) | (val & 0xFF)
	}

	tok := l.makeToken(token.Number, "", startCol, l.pos-startPos)
	tok.Line = startLine

	if l.peek() != '\'' {
		util.Error(tok, "Unterminated character literal")
	}
	l.advance() // consume closing '

	tok.Value = strconv.FormatInt(word, 10)
	return tok
}

func (l *Lexer) decodeEscape(escapeChar rune) int64 {
	c := l.peek()
	errTok := l.currentToken()
	if c == 0 {
		util.Error(errTok, "Unterminated escape sequence")
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
			util.Warn(util.WarnUnrecognizedEscape, errTok, "Unrecognized escape sequence '\\%c'", c)
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
			util.Warn(util.WarnUnrecognizedEscape, errTok, "Unrecognized escape sequence '*%c'", c)
			return int64(c)
		}
	}
}
