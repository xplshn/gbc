package lexer

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/util"
)

type Lexer struct {
	source    []rune
	fileIndex int
	pos       int
	line      int
	column    int
	cfg       *config.Config
}

func NewLexer(source []rune, fileIndex int, cfg *config.Config) *Lexer {
	return &Lexer{
		source: source, fileIndex: fileIndex, line: 1, column: 1, cfg: cfg,
	}
}

func (l *Lexer) Next() token.Token {
	for {
		l.skipWhitespaceAndComments()
		startPos, startCol, startLine := l.pos, l.column, l.line

		if l.isAtEnd() {
			return l.makeToken(token.EOF, "", startPos, startCol, startLine)
		}

		// Handle directives and line comments
		if l.peek() == '/' && l.peekNext() == '/' {
			// Try parsing as a directive first. We consume the line
			// if it's a directive, but reset the position if it's not
			if !l.cfg.IsFeatureEnabled(config.FeatNoDirectives) {
				if tok, isDirective := l.lineCommentOrDirective(startPos, startCol, startLine); isDirective {
					return tok
				}
			}

			// If not a directive, treat as a regular C-style comment
			if l.cfg.IsFeatureEnabled(config.FeatCComments) {
				l.lineComment()
				continue // Loop to find the next actual token
			}
		}

		ch := l.advance()
		if unicode.IsLetter(ch) || ch == '_' {
			return l.identifierOrKeyword(startPos, startCol, startLine)
		}
		if unicode.IsDigit(ch) {
			return l.numberLiteral(startPos, startCol, startLine)
		}

		switch ch {
		case '(':
			return l.makeToken(token.LParen, "", startPos, startCol, startLine)
		case ')':
			return l.makeToken(token.RParen, "", startPos, startCol, startLine)
		case '{':
			return l.makeToken(token.LBrace, "", startPos, startCol, startLine)
		case '}':
			return l.makeToken(token.RBrace, "", startPos, startCol, startLine)
		case '[':
			return l.makeToken(token.LBracket, "", startPos, startCol, startLine)
		case ']':
			return l.makeToken(token.RBracket, "", startPos, startCol, startLine)
		case ';':
			return l.makeToken(token.Semi, "", startPos, startCol, startLine)
		case ',':
			return l.makeToken(token.Comma, "", startPos, startCol, startLine)
		case '?':
			return l.makeToken(token.Question, "", startPos, startCol, startLine)
		case '~':
			return l.makeToken(token.Complement, "", startPos, startCol, startLine)
		case ':':
			return l.matchThen('=', token.Define, token.Colon, startPos, startCol, startLine)
		case '!':
			return l.matchThen('=', token.Neq, token.Not, startPos, startCol, startLine)
		case '^':
			return l.matchThen('=', token.XorEq, token.Xor, startPos, startCol, startLine)
		case '%':
			return l.matchThen('=', token.RemEq, token.Rem, startPos, startCol, startLine)
		case '+':
			return l.plus(startPos, startCol, startLine)
		case '-':
			return l.minus(startPos, startCol, startLine)
		case '*':
			return l.star(startPos, startCol, startLine)
		case '/':
			return l.slash(startPos, startCol, startLine)
		case '&':
			return l.ampersand(startPos, startCol, startLine)
		case '|':
			return l.pipe(startPos, startCol, startLine)
		case '<':
			return l.less(startPos, startCol, startLine)
		case '>':
			return l.greater(startPos, startCol, startLine)
		case '=':
			return l.equal(startPos, startCol, startLine)
		case '.':
			if l.match('.') && l.match('.') {
				return l.makeToken(token.Dots, "", startPos, startCol, startLine)
			}
			return l.makeToken(token.Dot, "", startPos, startCol, startLine)
		case '"':
			return l.stringLiteral(startPos, startCol, startLine)
		case '\'':
			return l.charLiteral(startPos, startCol, startLine)
		}

		tok := l.makeToken(token.EOF, "", startPos, startCol, startLine)
		util.Error(tok, "Unexpected character: '%c'", ch)
		return tok
	}
}

func (l *Lexer) peek() rune {
	if l.isAtEnd() {
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
	if l.isAtEnd() {
		return 0
	}
	ch := l.source[l.pos]
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	l.pos++
	return ch
}

func (l *Lexer) match(expected rune) bool {
	if l.isAtEnd() || l.source[l.pos] != expected {
		return false
	}
	l.advance()
	return true
}

func (l *Lexer) isAtEnd() bool {
	return l.pos >= len(l.source)
}

func (l *Lexer) makeToken(tokType token.Type, value string, startPos, startCol, startLine int) token.Token {
	return token.Token{
		Type: tokType, Value: value, FileIndex: l.fileIndex,
		Line: startLine, Column: startCol, Len: l.pos - startPos,
	}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		c := l.peek()
		switch c {
		case ' ', '\t', '\n', '\r':
			l.advance()
		case '/':
			if l.peekNext() == '*' {
				l.blockComment()
			} else {
				return // Next() handles `//` comments
			}
		default:
			return
		}
	}
}

func (l *Lexer) blockComment() {
	startTok := l.makeToken(token.Comment, "", l.pos, l.column, l.line)
	l.advance() // Consume '/'
	l.advance() // Consume '*'
	for !l.isAtEnd() {
		if l.peek() == '*' && l.peekNext() == '/' {
			l.advance()
			l.advance()
			return
		}
		l.advance()
	}
	util.Error(startTok, "Unterminated block comment")
}

func (l *Lexer) lineComment() {
	for !l.isAtEnd() && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) lineCommentOrDirective(startPos, startCol, startLine int) (token.Token, bool) {
	preCommentPos, preCommentCol, preCommentLine := l.pos, l.column, l.line
	l.advance() // Consume '/'
	l.advance() // Consume '/'
	commentStartPos := l.pos
	for !l.isAtEnd() && l.peek() != '\n' {
		l.advance()
	}
	commentContent := string(l.source[commentStartPos:l.pos])
	trimmedContent := strings.TrimSpace(commentContent)

	if strings.HasPrefix(trimmedContent, "[b]:") {
		directiveContent := strings.TrimSpace(strings.TrimPrefix(trimmedContent, "[b]:"))
		return l.makeToken(token.Directive, directiveContent, startPos, startCol, startLine), true
	}

	// It's not a directive, so reset the lexer's position to before the '//'
	l.pos, l.column, l.line = preCommentPos, preCommentCol, preCommentLine
	return token.Token{}, false
}

func (l *Lexer) identifierOrKeyword(startPos, startCol, startLine int) token.Token {
	for unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_' {
		l.advance()
	}
	value := string(l.source[startPos:l.pos])
	tok := l.makeToken(token.Ident, value, startPos, startCol, startLine)

	if tokType, isKeyword := token.KeywordMap[value]; isKeyword {
		isTypedKeyword := tokType >= token.Void && tokType <= token.Any
		if !isTypedKeyword || l.cfg.IsFeatureEnabled(config.FeatTyped) {
			tok.Type = tokType
			tok.Value = ""
		}
	}
	return tok
}

func (l *Lexer) numberLiteral(startPos, startCol, startLine int) token.Token {
	for unicode.IsDigit(l.peek()) || (l.peek() == 'x' || l.peek() == 'X') || (l.peek() >= 'a' && l.peek() <= 'f') || (l.peek() >= 'A' && l.peek() <= 'F') {
		l.advance()
	}
	valueStr := string(l.source[startPos:l.pos])
	tok := l.makeToken(token.Number, "", startPos, startCol, startLine)
	val, err := strconv.ParseUint(valueStr, 0, 64)
	if err != nil {
		util.Error(tok, "Invalid number literal: %s", valueStr)
	}
	tok.Value = strconv.FormatInt(int64(val), 10)
	return tok
}

func (l *Lexer) stringLiteral(startPos, startCol, startLine int) token.Token {
	var sb strings.Builder
	for !l.isAtEnd() {
		c := l.peek()
		if c == '"' {
			l.advance()
			return l.makeToken(token.String, sb.String(), startPos, startCol, startLine)
		}
		if (c == '\\' && l.cfg.IsFeatureEnabled(config.FeatCEsc)) || (c == '*' && l.cfg.IsFeatureEnabled(config.FeatBEsc)) {
			l.advance()
			sb.WriteRune(rune(l.decodeEscape(c, startPos, startCol, startLine)))
		} else {
			l.advance()
			sb.WriteRune(c)
		}
	}
	util.Error(l.makeToken(token.String, "", startPos, startCol, startLine), "Unterminated string literal")
	return l.makeToken(token.EOF, "", l.pos, l.column, l.line)
}

func (l *Lexer) charLiteral(startPos, startCol, startLine int) token.Token {
	var word int64
	for l.peek() != '\'' && !l.isAtEnd() {
		var val int64
		c := l.peek()
		if (c == '\\' && l.cfg.IsFeatureEnabled(config.FeatCEsc)) || (c == '*' && l.cfg.IsFeatureEnabled(config.FeatBEsc)) {
			l.advance()
			val = l.decodeEscape(c, startPos, startCol, startLine)
		} else {
			l.advance()
			val = int64(c)
		}
		word = (word << 8) | (val & 0xFF)
	}

	tok := l.makeToken(token.Number, "", startPos, startCol, startLine)
	if !l.match('\'') {
		util.Error(tok, "Unterminated character literal")
	}
	tok.Value = strconv.FormatInt(word, 10)
	return tok
}

func (l *Lexer) decodeEscape(escapeChar rune, startPos, startCol, startLine int) int64 {
	if l.isAtEnd() {
		util.Error(l.makeToken(token.EOF, "", l.pos, l.column, l.line), "Unterminated escape sequence")
		return 0
	}
	c := l.advance()
	escapes := map[rune]int64{'n': '\n', 't': '\t', 'e': 4, 'b': '\b', 'r': '\r', '0': 0, '(': '{', ')': '}', '\\': '\\', '\'': '\'', '"': '"', '*': '*'}
	if val, ok := escapes[c]; ok {
		return val
	}
	util.Warn(l.cfg, config.WarnUnrecognizedEscape, l.makeToken(token.String, "", startPos, startCol, startLine), "Unrecognized escape sequence '%c%c'", escapeChar, c)
	return int64(c)
}

func (l *Lexer) matchThen(expected rune, thenType, elseType token.Type, sPos, sCol, sLine int) token.Token {
	if l.match(expected) {
		return l.makeToken(thenType, "", sPos, sCol, sLine)
	}
	return l.makeToken(elseType, "", sPos, sCol, sLine)
}

func (l *Lexer) plus(sPos, sCol, sLine int) token.Token {
	if l.match('+') {
		return l.makeToken(token.Inc, "", sPos, sCol, sLine)
	}
	if l.cfg.IsFeatureEnabled(config.FeatCOps) && l.match('=') {
		return l.makeToken(token.PlusEq, "", sPos, sCol, sLine)
	}
	return l.makeToken(token.Plus, "", sPos, sCol, sLine)
}

func (l *Lexer) minus(sPos, sCol, sLine int) token.Token {
	if l.match('-') {
		return l.makeToken(token.Dec, "", sPos, sCol, sLine)
	}
	if l.cfg.IsFeatureEnabled(config.FeatCOps) && l.match('=') {
		return l.makeToken(token.MinusEq, "", sPos, sCol, sLine)
	}
	return l.makeToken(token.Minus, "", sPos, sCol, sLine)
}

func (l *Lexer) star(sPos, sCol, sLine int) token.Token {
	if l.cfg.IsFeatureEnabled(config.FeatCOps) && l.match('=') {
		return l.makeToken(token.StarEq, "", sPos, sCol, sLine)
	}
	return l.makeToken(token.Star, "", sPos, sCol, sLine)
}

func (l *Lexer) slash(sPos, sCol, sLine int) token.Token {
	if l.cfg.IsFeatureEnabled(config.FeatCOps) && l.match('=') {
		return l.makeToken(token.SlashEq, "", sPos, sCol, sLine)
	}
	return l.makeToken(token.Slash, "", sPos, sCol, sLine)
}

func (l *Lexer) ampersand(sPos, sCol, sLine int) token.Token {
	if l.match('&') {
		return l.makeToken(token.AndAnd, "", sPos, sCol, sLine)
	}
	if l.cfg.IsFeatureEnabled(config.FeatCOps) && l.match('=') {
		return l.makeToken(token.AndEq, "", sPos, sCol, sLine)
	}
	return l.makeToken(token.And, "", sPos, sCol, sLine)
}

func (l *Lexer) pipe(sPos, sCol, sLine int) token.Token {
	if l.match('|') {
		return l.makeToken(token.OrOr, "", sPos, sCol, sLine)
	}
	if l.cfg.IsFeatureEnabled(config.FeatCOps) && l.match('=') {
		return l.makeToken(token.OrEq, "", sPos, sCol, sLine)
	}
	return l.makeToken(token.Or, "", sPos, sCol, sLine)
}

func (l *Lexer) less(sPos, sCol, sLine int) token.Token {
	if l.match('<') {
		return l.matchThen('=', token.ShlEq, token.Shl, sPos, sCol, sLine)
	}
	return l.matchThen('=', token.Lte, token.Lt, sPos, sCol, sLine)
}

func (l *Lexer) greater(sPos, sCol, sLine int) token.Token {
	if l.match('>') {
		return l.matchThen('=', token.ShrEq, token.Shr, sPos, sCol, sLine)
	}
	return l.matchThen('=', token.Gte, token.Gt, sPos, sCol, sLine)
}

func (l *Lexer) equal(sPos, sCol, sLine int) token.Token {
	if l.match('=') { return l.makeToken(token.EqEq, "", sPos, sCol, sLine) }
	if l.cfg.IsFeatureEnabled(config.FeatBOps) {
		switch {
		case l.match('+'): return l.makeToken(token.EqPlus, "", sPos, sCol, sLine)
		case l.match('-'): return l.makeToken(token.EqMinus, "", sPos, sCol, sLine)
		case l.match('*'): return l.makeToken(token.EqStar, "", sPos, sCol, sLine)
		case l.match('/'): return l.makeToken(token.EqSlash, "", sPos, sCol, sLine)
		case l.match('%'): return l.makeToken(token.EqRem, "", sPos, sCol, sLine)
		case l.match('&'): return l.makeToken(token.EqAnd, "", sPos, sCol, sLine)
		case l.match('|'): return l.makeToken(token.EqOr, "", sPos, sCol, sLine)
		case l.match('^'): return l.makeToken(token.EqXor, "", sPos, sCol, sLine)
		case l.match('<') && l.match('<'): return l.makeToken(token.EqShl, "", sPos, sCol, sLine)
		case l.match('>') && l.match('>'): return l.makeToken(token.EqShr, "", sPos, sCol, sLine)
		}
	}
	return l.makeToken(token.Eq, "", sPos, sCol, sLine)
}
