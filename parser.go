package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser holds the state for the parsing process
type Parser struct {
	tokens   []Token
	pos      int
	current  Token
	previous Token
}

// NewParser creates and initializes a new Parser from a token stream
func NewParser(tokens []Token) *Parser {
	p := &Parser{tokens: tokens, pos: 0}
	if len(tokens) > 0 {
		p.current = p.tokens[0]
	}
	// No need to advance, current is already set
	return p
}

// Parser helpers
func (p *Parser) advance() {
	if p.pos < len(p.tokens) {
		p.previous = p.current
		p.pos++
		if p.pos < len(p.tokens) {
			p.current = p.tokens[p.pos]
		}
	}
}

func (p *Parser) peek() Token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	// Return the EOF token if at the end
	return p.tokens[len(p.tokens)-1]
}

func (p *Parser) check(tokType TokenType) bool {
	return p.current.Type == tokType
}

func (p *Parser) match(tokType TokenType) bool {
	if !p.check(tokType) {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) expect(tokType TokenType, message string) {
	if p.check(tokType) {
		p.advance()
		return
	}
	Error(p.current, message) // linter says: non-constant format string in call to gbc.Error
}

func isLValue(node *AstNode) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case AST_IDENT, AST_INDIRECTION, AST_SUBSCRIPT:
		return true
	default:
		return false
	}
}

// Expression Parsing

// getBinaryOpPrecedence returns the precedence level for a binary operator.
// A higher number indicates higher precedence (tighter binding). This table
// implements standard C operator precedence.
func getBinaryOpPrecedence(op TokenType) int {
	switch op {
	case TOK_STAR, TOK_SLASH, TOK_REM:
		return 13
	case TOK_PLUS, TOK_MINUS:
		return 12
	case TOK_SHL, TOK_SHR:
		return 11
	case TOK_LT, TOK_GT, TOK_LTE, TOK_GTE:
		return 10
	case TOK_EQEQ, TOK_NEQ:
		return 9
	case TOK_AND:
		return 8
	case TOK_XOR:
		return 7
	case TOK_OR:
		return 6
	default:
		return -1 // Not a binary operator
	}
}

func (p *Parser) parsePrimaryExpr() *AstNode {
	tok := p.current
	if p.match(TOK_NUMBER) {
		val, _ := strconv.ParseInt(p.previous.Value, 10, 64)
		return astNumber(tok, val)
	}
	if p.match(TOK_STRING) {
		return astString(tok, p.previous.Value)
	}
	if p.match(TOK_IDENT) {
		return astIdent(tok, p.previous.Value)
	}
	if p.match(TOK_LPAREN) {
		expr := p.parseExpr()
		p.expect(TOK_RPAREN, "Expected ')' after expression.")
		return expr
	}
	Error(tok, "Expected an expression.")
	return nil // Unreachable
}

func (p *Parser) parsePostfixExpr() *AstNode {
	expr := p.parsePrimaryExpr()
	for {
		tok := p.current
		if p.match(TOK_LPAREN) { // Function Call
			var args []*AstNode
			if !p.check(TOK_RPAREN) {
				for {
					args = append(args, p.parseAssignmentExpr())
					if !p.match(TOK_COMMA) {
						break
					}
				}
			}
			p.expect(TOK_RPAREN, "Expected ')' after function arguments.")
			expr = astFuncCall(tok, expr, args)
		} else if p.match(TOK_LBRACKET) { // Subscript
			index := p.parseExpr()
			p.expect(TOK_RBRACKET, "Expected ']' after array index.")
			expr = astSubscript(tok, expr, index)
		} else if p.match(TOK_INC) || p.match(TOK_DEC) { // Postfix inc/dec
			if !isLValue(expr) {
				Error(p.previous, "Postfix '++' or '--' requires an l-value.")
			}
			expr = astPostfixOp(p.previous, p.previous.Type, expr)
		} else {
			break
		}
	}
	return expr
}

func (p *Parser) parseUnaryExpr() *AstNode {
	tok := p.current
	if p.match(TOK_NOT) || p.match(TOK_COMPLEMENT) || p.match(TOK_MINUS) ||
		p.match(TOK_PLUS) || p.match(TOK_INC) || p.match(TOK_DEC) ||
		p.match(TOK_STAR) || p.match(TOK_AND) {
		op := p.previous.Type
		opToken := p.previous
		operand := p.parseUnaryExpr()

		if op == TOK_STAR {
			return astIndirection(tok, operand)
		}
		if op == TOK_AND {
			if !isLValue(operand) {
				Error(opToken, "Address-of operator '&' requires an l-value.")
			}
			return astAddressOf(tok, operand)
		}
		if (op == TOK_INC || op == TOK_DEC) && !isLValue(operand) {
			Error(opToken, "Prefix '++' or '--' requires an l-value.")
		}
		return astUnaryOp(tok, op, operand)
	}
	return p.parsePostfixExpr()
}

func (p *Parser) parseBinaryExpr(minPrec int) *AstNode {
	left := p.parseUnaryExpr()
	for {
		op := p.current.Type
		prec := getBinaryOpPrecedence(op)
		if prec < minPrec {
			break
		}
		opTok := p.current
		p.advance()
		// For left-associative operators, recurse with prec + 1
		right := p.parseBinaryExpr(prec + 1)
		left = astBinaryOp(opTok, op, left, right)
	}
	return left
}

func (p *Parser) parseTernaryExpr() *AstNode {
	// The condition of a ternary operator is a full binary expression.
	// We start parsing with the lowest precedence to capture all operators
	cond := p.parseBinaryExpr(0)
	if p.match(TOK_QUESTION) {
		tok := p.previous
		// The middle part is a full expression
		thenExpr := p.parseExpr()
		p.expect(TOK_COLON, "Expected ':' for ternary operator.")
		// The right-hand side is an assignment expression to handle right-associativity
		elseExpr := p.parseAssignmentExpr()
		return astTernary(tok, cond, thenExpr, elseExpr)
	}
	return cond
}

func isAssignmentOp(op TokenType) bool {
	return op >= TOK_EQ && op <= TOK_EQ_SHR
}

func (p *Parser) parseAssignmentExpr() *AstNode {
	left := p.parseTernaryExpr()
	if isAssignmentOp(p.current.Type) {
		if !isLValue(left) {
			Error(p.current, "Invalid target for assignment.")
		}
		op := p.current.Type
		tok := p.current
		p.advance()
		right := p.parseAssignmentExpr() // Right-associative
		return astAssign(tok, op, left, right)
	}
	return left
}

func (p *Parser) parseExpr() *AstNode {
	return p.parseAssignmentExpr()
}

// Statement and Declaration Parsing
func (p *Parser) parseBlockStmt() *AstNode {
	tok := p.current
	p.expect(TOK_LBRACE, "Expected '{' to start a block.")
	var stmts []*AstNode
	for !p.check(TOK_RBRACE) && !p.check(TOK_EOF) {
		stmts = append(stmts, p.parseStmt())
	}
	p.expect(TOK_RBRACE, "Expected '}' after block.")
	return astBlock(tok, stmts, false)
}

func (p *Parser) parseDeclarationList(declType TokenType, declTok Token) *AstNode {
	var decls []*AstNode
	for {
		p.expect(TOK_IDENT, "Expected identifier in declaration.")
		name := p.previous.Value
		itemToken := p.previous

		if declType == TOK_EXTRN {
			if !IsFeatureEnabled(FEAT_EXTRN) {
				Error(itemToken, "'extrn' is forbidden by the current feature set (-Fno-extrn).")
			}
			decls = append(decls, astExtrnDecl(itemToken, name))
		} else { // TOK_AUTO
			var sizeExpr *AstNode
			isVector := false
			isBracketed := false
			if p.match(TOK_LBRACKET) {
				isVector = true
				isBracketed = true
				if !p.check(TOK_RBRACKET) {
					sizeExpr = p.parseExpr()
					foldedSize := FoldConstants(sizeExpr)
					if foldedSize.Type != AST_NUMBER || foldedSize.Data.(AstNumber).Value < 0 {
						Error(p.current, "Vector size must be a non-negative constant integer.")
					}
				}
				p.expect(TOK_RBRACKET, "Expected ']' after array size.")
			} else if p.check(TOK_NUMBER) {
				isVector = true
				val, _ := strconv.ParseInt(p.current.Value, 10, 64)
				if val < 0 {
					Error(p.current, "Vector size must be a non-negative constant integer.")
				}
				sizeExpr = astNumber(p.current, val)
				p.advance()
			}
			var initList []*AstNode
			if p.check(TOK_STRING) {
				// Special case for `auto vec "string";`
				isVector = true
				initList = append(initList, p.parsePrimaryExpr())
			} else if p.match(TOK_EQ) {
				initList = append(initList, p.parseAssignmentExpr())
			}
			decls = append(decls, astVarDecl(itemToken, name, initList, sizeExpr, isVector, isBracketed))
		}

		if !p.match(TOK_COMMA) {
			break
		}
	}
	p.expect(TOK_SEMI, "Expected ';' after declaration list.")
	return astBlock(declTok, decls, true) // Synthetic block
}

func (p *Parser) parseStmt() *AstNode {
	tok := p.current
	if p.check(TOK_IDENT) && p.peek().Type == TOK_COLON {
		labelName := p.current.Value
		p.advance() // consume ident
		p.advance() // consume colon
		// Handle label at the end of a block, e.g., "label: }"
		if p.check(TOK_RBRACE) {
			return astLabel(tok, labelName, astBlock(p.current, nil, true)) // Labeled null statement
		}
		stmt := p.parseStmt()
		return astLabel(tok, labelName, stmt)
	}

	switch {
	case p.match(TOK_IF):
		p.expect(TOK_LPAREN, "Expected '(' after 'if'.")
		cond := p.parseExpr()
		p.expect(TOK_RPAREN, "Expected ')' after if condition.")
		thenBody := p.parseStmt()
		var elseBody *AstNode
		if p.match(TOK_ELSE) {
			elseBody = p.parseStmt()
		}
		return astIf(tok, cond, thenBody, elseBody)
	case p.match(TOK_WHILE):
		p.expect(TOK_LPAREN, "Expected '(' after 'while'.")
		cond := p.parseExpr()
		p.expect(TOK_RPAREN, "Expected ')' after while condition.")
		body := p.parseStmt()
		return astWhile(tok, cond, body)
	case p.match(TOK_SWITCH):
		hasParen := p.match(TOK_LPAREN)
		expr := p.parseExpr()
		if hasParen {
			p.expect(TOK_RPAREN, "Expected ')' after switch expression.")
		}
		body := p.parseStmt()
		switchNode := astSwitch(tok, expr, body)
		p.buildSwitchJumpTable(switchNode)
		return switchNode
	case p.check(TOK_LBRACE):
		return p.parseBlockStmt()
	case p.match(TOK_AUTO):
		return p.parseDeclarationList(TOK_AUTO, p.previous)
	case p.match(TOK_EXTRN):
		return p.parseDeclarationList(TOK_EXTRN, p.previous)
	case p.match(TOK_CASE):
		value := p.parseExpr()
		p.expect(TOK_COLON, "Expected ':' after case value.")
		body := p.parseStmt()
		return astCase(tok, value, body)
	case p.match(TOK_DEFAULT):
		p.expect(TOK_COLON, "Expected ':' after 'default'.")
		body := p.parseStmt()
		return astDefault(tok, body)
	case p.match(TOK_GOTO):
		p.expect(TOK_IDENT, "Expected label name after 'goto'.")
		node := astGoto(tok, p.previous.Value)
		p.expect(TOK_SEMI, "Expected ';' after goto statement.")
		return node
	case p.match(TOK_RETURN):
		var expr *AstNode
		if !p.check(TOK_SEMI) {
			expr = p.parseExpr()
		}
		p.expect(TOK_SEMI, "Expected ';' after return statement.")
		return astReturn(tok, expr)
	case p.match(TOK_BREAK):
		p.expect(TOK_SEMI, "Expected ';' after 'break'.")
		return astBreak(tok)
	case p.match(TOK_SEMI):
		return astBlock(tok, nil, true) // Null statement
	default:
		expr := p.parseExpr()
		p.expect(TOK_SEMI, "Expected ';' after expression statement.")
		return expr
	}
}

// Top-Level Parsing
func (p *Parser) parseAsmFuncDef() *AstNode {
	nameToken := p.previous
	p.expect(TOK___ASM__, "Expected '__asm__' keyword.")
	asmTok := p.previous
	if !IsFeatureEnabled(FEAT_ASM) {
		Error(asmTok, "'__asm__' is forbidden by the current feature set (-Fno-asm).")
	}

	p.expect(TOK_LPAREN, "Expected '(' after '__asm__'.")
	var codeParts []string
	for !p.check(TOK_RPAREN) && !p.check(TOK_EOF) {
		p.expect(TOK_STRING, "Expected string literal in '__asm__' block.")
		codeParts = append(codeParts, p.previous.Value)
		p.match(TOK_COMMA) // Optional comma
	}
	p.expect(TOK_RPAREN, "Expected ')' to close '__asm__' block.")
	asmCode := strings.Join(codeParts, "\n")
	body := astAsmStmt(asmTok, asmCode)

	if !p.check(TOK_LBRACE) {
		p.expect(TOK_SEMI, "Expected ';' or '{' after '__asm__' definition.")
	} else {
		// Consume empty block
		p.parseBlockStmt()
	}

	return astFuncDecl(nameToken, nameToken.Value, nil, body, false)
}

func (p *Parser) parseFuncDecl() *AstNode {
	fnToken := p.previous
	p.expect(TOK_LPAREN, "Expected '(' after function name.")

	var params []*AstNode
	hasVarargs := false
	if !p.check(TOK_RPAREN) {
		for {
			if p.match(TOK_DOTS) {
				hasVarargs = true
				break
			}
			p.expect(TOK_IDENT, "Expected parameter name or '...'.")
			params = append(params, astIdent(p.previous, p.previous.Value))
			if !p.match(TOK_COMMA) {
				break
			}
		}
	}
	p.expect(TOK_RPAREN, "Expected ')' after parameters.")

	var decls []*AstNode
	for p.check(TOK_AUTO) || p.check(TOK_EXTRN) {
		declType := p.current.Type
		declTok := p.current
		p.advance()
		declBlock := p.parseDeclarationList(declType, declTok)
		decls = append(decls, declBlock.Data.(AstBlock).Stmts...)
	}

	body := p.parseStmt()

	if len(decls) > 0 {
		if body.Type == AST_BLOCK && !body.Data.(AstBlock).IsSynthetic {
			bodyData := body.Data.(AstBlock)
			bodyData.Stmts = append(decls, bodyData.Stmts...)
			body.Data = bodyData
		} else {
			allStmts := append(decls, body)
			body = astBlock(body.Tok, allStmts, false)
		}
	}

	return astFuncDecl(fnToken, fnToken.Value, params, body, hasVarargs)
}

func (p *Parser) parseGlobalDefinition() *AstNode {
	nameToken := p.previous
	var sizeExpr *AstNode
	isVector := false
	isBracketed := false

	if p.match(TOK_LBRACKET) {
		isVector = true
		isBracketed = true
		if !p.check(TOK_RBRACKET) {
			sizeExpr = p.parseExpr()
			sizeExpr = FoldConstants(sizeExpr)
			if sizeExpr.Type != AST_NUMBER || sizeExpr.Data.(AstNumber).Value < 0 {
				Error(p.current, "Vector size must be a non-negative constant integer.")
			}
		}
		p.expect(TOK_RBRACKET, "Expected ']' for vector definition.")
	}

	var initList []*AstNode
	if !p.check(TOK_SEMI) {
		for {
			initList = append(initList, p.parseAssignmentExpr())
			if !p.match(TOK_COMMA) {
				break
			}
		}
		if len(initList) > 1 {
			isVector = true
		}
	}

	p.expect(TOK_SEMI, "Expected ';' after global definition.")
	return astVarDecl(nameToken, nameToken.Value, initList, sizeExpr, isVector, isBracketed)
}

func (p *Parser) Parse() *AstNode {
	var stmts []*AstNode
	tok := p.current
	for !p.check(TOK_EOF) {
		if p.match(TOK_SEMI) {
			continue
		}

		if !p.check(TOK_IDENT) {
			if p.check(TOK_EXTRN) {
				declTok := p.current
				p.advance()
				stmts = append(stmts, p.parseDeclarationList(TOK_EXTRN, declTok))
				continue
			}
			Error(p.current, "Expected a top-level definition (function or variable).")
		}

		p.advance() // Consume identifier
		switch p.current.Type {
		case TOK_LPAREN:
			stmts = append(stmts, p.parseFuncDecl())
		case TOK___ASM__:
			stmts = append(stmts, p.parseAsmFuncDef())
		default:
			stmts = append(stmts, p.parseGlobalDefinition())
		}
	}
	return astBlock(tok, stmts, true)
}

// Switch statement helpers
func (p *Parser) buildSwitchJumpTable(switchNode *AstNode) {
	if switchNode == nil || switchNode.Type != AST_SWITCH {
		return
	}
	p.findCasesRecursive(switchNode.Data.(AstSwitch).Body, switchNode)
}

func (p *Parser) findCasesRecursive(node, switchNode *AstNode) {
	if node == nil {
		return
	}

	if node.Type == AST_SWITCH && node != switchNode {
		return
	}

	swData := switchNode.Data.(AstSwitch)

	if node.Type == AST_CASE {
		caseData := node.Data.(AstCase)
		foldedValue := FoldConstants(caseData.Value)
		if foldedValue.Type != AST_NUMBER {
			Error(node.Tok, "Case value must be a constant integer.")
		}
		caseData.Value = foldedValue
		caseVal := foldedValue.Data.(AstNumber).Value
		labelName := fmt.Sprintf("@case_%d_%d", caseVal, node.Tok.Line)

		swData.CaseLabels = append(swData.CaseLabels, AstCaseLabel{Value: caseVal, LabelName: labelName})
		caseData.QbeLabel = labelName
		node.Data = caseData
		switchNode.Data = swData

	} else if node.Type == AST_DEFAULT {
		defData := node.Data.(AstDefault)
		if swData.DefaultLabelName != "" {
			Error(node.Tok, "Multiple 'default' labels in one switch statement.")
		}
		labelName := fmt.Sprintf("@default_%d", node.Tok.Line)
		swData.DefaultLabelName = labelName
		defData.QbeLabel = labelName
		node.Data = defData
		switchNode.Data = swData
	}

	switch d := node.Data.(type) {
	case AstIf:
		p.findCasesRecursive(d.ThenBody, switchNode)
		p.findCasesRecursive(d.ElseBody, switchNode)
	case AstWhile:
		p.findCasesRecursive(d.Body, switchNode)
	case AstBlock:
		for _, stmt := range d.Stmts {
			p.findCasesRecursive(stmt, switchNode)
		}
	case AstLabel:
		p.findCasesRecursive(d.Stmt, switchNode)
	case AstCase:
		p.findCasesRecursive(d.Body, switchNode)
	case AstDefault:
		p.findCasesRecursive(d.Body, switchNode)
	}
}
