package parser

import (
	"fmt"
	"strconv"
	"strings"

	"gbc/pkg/ast"
	"gbc/pkg/token"
	"gbc/pkg/util"
)

// Parser holds the state for the parsing process
type Parser struct {
	tokens   []token.Token
	pos      int
	current  token.Token
	previous token.Token
}

// NewParser creates and initializes a new Parser from a token stream
func NewParser(tokens []token.Token) *Parser {
	p := &Parser{tokens: tokens, pos: 0}
	if len(tokens) > 0 {
		p.current = p.tokens[0]
	}
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

func (p *Parser) peek() token.Token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	return p.tokens[len(p.tokens)-1]
}

func (p *Parser) check(tokType token.Type) bool {
	return p.current.Type == tokType
}

func (p *Parser) match(tokType token.Type) bool {
	if !p.check(tokType) {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) expect(tokType token.Type, message string) {
	if p.check(tokType) {
		p.advance()
		return
	}
	util.Error(p.current, message)
}

func isLValue(node *ast.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case ast.Ident, ast.Indirection, ast.Subscript:
		return true
	default:
		return false
	}
}

// Expression Parsing
func getBinaryOpPrecedence(op token.Type) int {
	switch op {
	case token.Star, token.Slash, token.Rem:
		return 13
	case token.Plus, token.Minus:
		return 12
	case token.Shl, token.Shr:
		return 11
	case token.Lt, token.Gt, token.Lte, token.Gte:
		return 10
	case token.EqEq, token.Neq:
		return 9
	case token.And:
		return 8
	case token.Xor:
		return 7
	case token.Or:
		return 6
	default:
		return -1
	}
}

func (p *Parser) parsePrimaryExpr() *ast.Node {
	tok := p.current
	if p.match(token.Number) {
		val, _ := strconv.ParseInt(p.previous.Value, 10, 64)
		return ast.NewNumber(tok, val)
	}
	if p.match(token.String) {
		return ast.NewString(tok, p.previous.Value)
	}
	if p.match(token.Ident) {
		return ast.NewIdent(tok, p.previous.Value)
	}
	if p.match(token.LParen) {
		expr := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after expression.")
		return expr
	}
	util.Error(tok, "Expected an expression.")
	return nil
}

func (p *Parser) parsePostfixExpr() *ast.Node {
	expr := p.parsePrimaryExpr()
	for {
		tok := p.current
		if p.match(token.LParen) {
			var args []*ast.Node
			if !p.check(token.RParen) {
				for {
					args = append(args, p.parseAssignmentExpr())
					if !p.match(token.Comma) {
						break
					}
				}
			}
			p.expect(token.RParen, "Expected ')' after function arguments.")
			expr = ast.NewFuncCall(tok, expr, args)
		} else if p.match(token.LBracket) {
			index := p.parseExpr()
			p.expect(token.RBracket, "Expected ']' after array index.")
			expr = ast.NewSubscript(tok, expr, index)
		} else if p.match(token.Inc) || p.match(token.Dec) {
			if !isLValue(expr) {
				util.Error(p.previous, "Postfix '++' or '--' requires an l-value.")
			}
			expr = ast.NewPostfixOp(p.previous, p.previous.Type, expr)
		} else {
			break
		}
	}
	return expr
}

func (p *Parser) parseUnaryExpr() *ast.Node {
	tok := p.current
	if p.match(token.Not) || p.match(token.Complement) || p.match(token.Minus) ||
		p.match(token.Plus) || p.match(token.Inc) || p.match(token.Dec) ||
		p.match(token.Star) || p.match(token.And) {
		op := p.previous.Type
		opToken := p.previous
		operand := p.parseUnaryExpr()

		if op == token.Star {
			return ast.NewIndirection(tok, operand)
		}
		if op == token.And {
			if !isLValue(operand) {
				util.Error(opToken, "Address-of operator '&' requires an l-value.")
			}
			return ast.NewAddressOf(tok, operand)
		}
		if (op == token.Inc || op == token.Dec) && !isLValue(operand) {
			util.Error(opToken, "Prefix '++' or '--' requires an l-value.")
		}
		return ast.NewUnaryOp(tok, op, operand)
	}
	return p.parsePostfixExpr()
}

func (p *Parser) parseBinaryExpr(minPrec int) *ast.Node {
	left := p.parseUnaryExpr()
	for {
		op := p.current.Type
		prec := getBinaryOpPrecedence(op)
		if prec < minPrec {
			break
		}
		opTok := p.current
		p.advance()
		right := p.parseBinaryExpr(prec + 1)
		left = ast.NewBinaryOp(opTok, op, left, right)
	}
	return left
}

func (p *Parser) parseTernaryExpr() *ast.Node {
	cond := p.parseBinaryExpr(0)
	if p.match(token.Question) {
		tok := p.previous
		thenExpr := p.parseExpr()
		p.expect(token.Colon, "Expected ':' for ternary operator.")
		elseExpr := p.parseAssignmentExpr()
		return ast.NewTernary(tok, cond, thenExpr, elseExpr)
	}
	return cond
}

func isAssignmentOp(op token.Type) bool {
	return op >= token.Eq && op <= token.EqShr
}

func (p *Parser) parseAssignmentExpr() *ast.Node {
	left := p.parseTernaryExpr()
	if isAssignmentOp(p.current.Type) {
		if !isLValue(left) {
			util.Error(p.current, "Invalid target for assignment.")
		}
		op := p.current.Type
		tok := p.current
		p.advance()
		right := p.parseAssignmentExpr()
		return ast.NewAssign(tok, op, left, right)
	}
	return left
}

func (p *Parser) parseExpr() *ast.Node {
	return p.parseAssignmentExpr()
}

// Statement and Declaration Parsing
func (p *Parser) parseBlockStmt() *ast.Node {
	tok := p.current
	p.expect(token.LBrace, "Expected '{' to start a block.")
	var stmts []*ast.Node
	for !p.check(token.RBrace) && !p.check(token.EOF) {
		stmts = append(stmts, p.parseStmt())
	}
	p.expect(token.RBrace, "Expected '}' after block.")
	return ast.NewBlock(tok, stmts, false)
}

func (p *Parser) parseDeclarationList(declType token.Type, declTok token.Token) *ast.Node {
	var decls []*ast.Node
	for {
		p.expect(token.Ident, "Expected identifier in declaration.")
		name := p.previous.Value
		itemToken := p.previous

		if declType == token.Extrn {
			if !util.IsFeatureEnabled(util.FeatExtrn) {
				util.Error(itemToken, "'extrn' is forbidden by the current feature set (-Fno-extrn).")
			}
			decls = append(decls, ast.NewExtrnDecl(itemToken, name))
		} else { // token.Auto
			var sizeExpr *ast.Node
			isVector := false
			isBracketed := false
			if p.match(token.LBracket) {
				isVector = true
				isBracketed = true
				if !p.check(token.RBracket) {
					sizeExpr = p.parseExpr()
					foldedSize := ast.FoldConstants(sizeExpr)
					if foldedSize.Type != ast.Number || foldedSize.Data.(ast.NumberNode).Value < 0 {
						util.Error(p.current, "Vector size must be a non-negative constant integer.")
					}
				}
				p.expect(token.RBracket, "Expected ']' after array size.")
			} else if p.check(token.Number) {
				isVector = true
				val, _ := strconv.ParseInt(p.current.Value, 10, 64)
				if val < 0 {
					util.Error(p.current, "Vector size must be a non-negative constant integer.")
				}
				sizeExpr = ast.NewNumber(p.current, val)
				p.advance()
			}
			var initList []*ast.Node
			if p.check(token.String) {
				isVector = true
				initList = append(initList, p.parsePrimaryExpr())
			} else if p.match(token.Eq) {
				initList = append(initList, p.parseAssignmentExpr())
			}
			decls = append(decls, ast.NewVarDecl(itemToken, name, initList, sizeExpr, isVector, isBracketed))
		}

		if !p.match(token.Comma) {
			break
		}
	}
	p.expect(token.Semi, "Expected ';' after declaration list.")
	return ast.NewBlock(declTok, decls, true)
}

func (p *Parser) parseStmt() *ast.Node {
	tok := p.current
	if p.check(token.Ident) && p.peek().Type == token.Colon {
		labelName := p.current.Value
		p.advance() // consume ident
		p.advance() // consume colon
		if p.check(token.RBrace) {
			return ast.NewLabel(tok, labelName, ast.NewBlock(p.current, nil, true))
		}
		stmt := p.parseStmt()
		return ast.NewLabel(tok, labelName, stmt)
	}

	switch {
	case p.match(token.If):
		p.expect(token.LParen, "Expected '(' after 'if'.")
		cond := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after if condition.")
		thenBody := p.parseStmt()
		var elseBody *ast.Node
		if p.match(token.Else) {
			elseBody = p.parseStmt()
		}
		return ast.NewIf(tok, cond, thenBody, elseBody)
	case p.match(token.While):
		p.expect(token.LParen, "Expected '(' after 'while'.")
		cond := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after while condition.")
		body := p.parseStmt()
		return ast.NewWhile(tok, cond, body)
	case p.match(token.Switch):
		hasParen := p.match(token.LParen)
		expr := p.parseExpr()
		if hasParen {
			p.expect(token.RParen, "Expected ')' after switch expression.")
		}
		body := p.parseStmt()
		switchNode := ast.NewSwitch(tok, expr, body)
		p.buildSwitchJumpTable(switchNode)
		return switchNode
	case p.check(token.LBrace):
		return p.parseBlockStmt()
	case p.match(token.Auto):
		return p.parseDeclarationList(token.Auto, p.previous)
	case p.match(token.Extrn):
		return p.parseDeclarationList(token.Extrn, p.previous)
	case p.match(token.Case):
		value := p.parseExpr()
		p.expect(token.Colon, "Expected ':' after case value.")
		body := p.parseStmt()
		return ast.NewCase(tok, value, body)
	case p.match(token.Default):
		p.expect(token.Colon, "Expected ':' after 'default'.")
		body := p.parseStmt()
		return ast.NewDefault(tok, body)
	case p.match(token.Goto):
		p.expect(token.Ident, "Expected label name after 'goto'.")
		node := ast.NewGoto(tok, p.previous.Value)
		p.expect(token.Semi, "Expected ';' after goto statement.")
		return node
	case p.match(token.Return):
		var expr *ast.Node
		if !p.check(token.Semi) {
			expr = p.parseExpr()
		}
		p.expect(token.Semi, "Expected ';' after return statement.")
		return ast.NewReturn(tok, expr)
	case p.match(token.Break):
		p.expect(token.Semi, "Expected ';' after 'break'.")
		return ast.NewBreak(tok)
	case p.match(token.Semi):
		return ast.NewBlock(tok, nil, true)
	default:
		expr := p.parseExpr()
		p.expect(token.Semi, "Expected ';' after expression statement.")
		return expr
	}
}

// Top-Level Parsing
func (p *Parser) parseAsmFuncDef() *ast.Node {
	nameToken := p.previous
	p.expect(token.Asm, "Expected '__asm__' keyword.")
	asmTok := p.previous
	if !util.IsFeatureEnabled(util.FeatAsm) {
		util.Error(asmTok, "'__asm__' is forbidden by the current feature set (-Fno-asm).")
	}

	p.expect(token.LParen, "Expected '(' after '__asm__'.")
	var codeParts []string
	for !p.check(token.RParen) && !p.check(token.EOF) {
		p.expect(token.String, "Expected string literal in '__asm__' block.")
		codeParts = append(codeParts, p.previous.Value)
		p.match(token.Comma)
	}
	p.expect(token.RParen, "Expected ')' to close '__asm__' block.")
	asmCode := strings.Join(codeParts, "\n")
	body := ast.NewAsmStmt(asmTok, asmCode)

	if !p.check(token.LBrace) {
		p.expect(token.Semi, "Expected ';' or '{' after '__asm__' definition.")
	} else {
		p.parseBlockStmt()
	}

	return ast.NewFuncDecl(nameToken, nameToken.Value, nil, body, false)
}

func (p *Parser) parseFuncDecl() *ast.Node {
	fnToken := p.previous
	p.expect(token.LParen, "Expected '(' after function name.")

	var params []*ast.Node
	hasVarargs := false
	if !p.check(token.RParen) {
		for {
			if p.match(token.Dots) {
				hasVarargs = true
				break
			}
			p.expect(token.Ident, "Expected parameter name or '...'.")
			params = append(params, ast.NewIdent(p.previous, p.previous.Value))
			if !p.match(token.Comma) {
				break
			}
		}
	}
	p.expect(token.RParen, "Expected ')' after parameters.")

	var decls []*ast.Node
	for p.check(token.Auto) || p.check(token.Extrn) {
		declType := p.current.Type
		declTok := p.current
		p.advance()
		declBlock := p.parseDeclarationList(declType, declTok)
		decls = append(decls, declBlock.Data.(ast.BlockNode).Stmts...)
	}

	body := p.parseStmt()

	if len(decls) > 0 {
		if body.Type == ast.Block && !body.Data.(ast.BlockNode).IsSynthetic {
			bodyData := body.Data.(ast.BlockNode)
			bodyData.Stmts = append(decls, bodyData.Stmts...)
			body.Data = bodyData
		} else {
			allStmts := append(decls, body)
			body = ast.NewBlock(body.Tok, allStmts, false)
		}
	}

	return ast.NewFuncDecl(fnToken, fnToken.Value, params, body, hasVarargs)
}

func (p *Parser) parseGlobalDefinition() *ast.Node {
	nameToken := p.previous
	var sizeExpr *ast.Node
	isVector := false
	isBracketed := false

	if p.match(token.LBracket) {
		isVector = true
		isBracketed = true
		if !p.check(token.RBracket) {
			sizeExpr = p.parseExpr()
			sizeExpr = ast.FoldConstants(sizeExpr)
			if sizeExpr.Type != ast.Number || sizeExpr.Data.(ast.NumberNode).Value < 0 {
				util.Error(p.current, "Vector size must be a non-negative constant integer.")
			}
		}
		p.expect(token.RBracket, "Expected ']' for vector definition.")
	}

	var initList []*ast.Node
	if !p.check(token.Semi) {
		for {
			initList = append(initList, p.parseAssignmentExpr())
			if !p.match(token.Comma) {
				break
			}
		}
		if len(initList) > 1 {
			isVector = true
		}
	}

	p.expect(token.Semi, "Expected ';' after global definition.")
	return ast.NewVarDecl(nameToken, nameToken.Value, initList, sizeExpr, isVector, isBracketed)
}

func (p *Parser) Parse() *ast.Node {
	var stmts []*ast.Node
	tok := p.current
	for !p.check(token.EOF) {
		if p.match(token.Semi) {
			continue
		}

		if !p.check(token.Ident) {
			if p.check(token.Extrn) {
				declTok := p.current
				p.advance()
				stmts = append(stmts, p.parseDeclarationList(token.Extrn, declTok))
				continue
			}
			util.Error(p.current, "Expected a top-level definition (function or variable).")
		}

		p.advance()
		switch p.current.Type {
		case token.LParen:
			stmts = append(stmts, p.parseFuncDecl())
		case token.Asm:
			stmts = append(stmts, p.parseAsmFuncDef())
		default:
			stmts = append(stmts, p.parseGlobalDefinition())
		}
	}
	return ast.NewBlock(tok, stmts, true)
}

// Switch statement helpers
func (p *Parser) buildSwitchJumpTable(switchNode *ast.Node) {
	if switchNode == nil || switchNode.Type != ast.Switch {
		return
	}
	p.findCasesRecursive(switchNode.Data.(ast.SwitchNode).Body, switchNode)
}

func (p *Parser) findCasesRecursive(node, switchNode *ast.Node) {
	if node == nil {
		return
	}

	if node.Type == ast.Switch && node != switchNode {
		return
	}

	swData := switchNode.Data.(ast.SwitchNode)

	if node.Type == ast.Case {
		caseData := node.Data.(ast.CaseNode)
		foldedValue := ast.FoldConstants(caseData.Value)
		if foldedValue.Type != ast.Number {
			util.Error(node.Tok, "Case value must be a constant integer.")
		}
		caseData.Value = foldedValue
		caseVal := foldedValue.Data.(ast.NumberNode).Value
		labelName := fmt.Sprintf("@case_%d_%d", caseVal, node.Tok.Line)

		swData.CaseLabels = append(swData.CaseLabels, ast.CaseLabelNode{Value: caseVal, LabelName: labelName})
		caseData.QbeLabel = labelName
		node.Data = caseData
		switchNode.Data = swData

	} else if node.Type == ast.Default {
		defData := node.Data.(ast.DefaultNode)
		if swData.DefaultLabelName != "" {
			util.Error(node.Tok, "Multiple 'default' labels in one switch statement.")
		}
		labelName := fmt.Sprintf("@default_%d", node.Tok.Line)
		swData.DefaultLabelName = labelName
		defData.QbeLabel = labelName
		node.Data = defData
		switchNode.Data = swData
	}

	switch d := node.Data.(type) {
	case ast.IfNode:
		p.findCasesRecursive(d.ThenBody, switchNode)
		p.findCasesRecursive(d.ElseBody, switchNode)
	case ast.WhileNode:
		p.findCasesRecursive(d.Body, switchNode)
	case ast.BlockNode:
		for _, stmt := range d.Stmts {
			p.findCasesRecursive(stmt, switchNode)
		}
	case ast.LabelNode:
		p.findCasesRecursive(d.Stmt, switchNode)
	case ast.CaseNode:
		p.findCasesRecursive(d.Body, switchNode)
	case ast.DefaultNode:
		p.findCasesRecursive(d.Body, switchNode)
	}
}
