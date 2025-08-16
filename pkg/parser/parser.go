package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/util"
)

type Parser struct {
	tokens      []token.Token
	pos         int
	current     token.Token
	previous    token.Token
	cfg         *config.Config
	isTypedPass bool
	typeNames   map[string]bool
}

func NewParser(tokens []token.Token, cfg *config.Config) *Parser {
	p := &Parser{
		tokens:      tokens,
		cfg:         cfg,
		isTypedPass: cfg.IsFeatureEnabled(config.FeatTyped),
		typeNames:   make(map[string]bool),
	}
	if len(tokens) > 0 {
		p.current = p.tokens[0]
	}

	if p.isTypedPass {
		for keyword, tokType := range token.KeywordMap {
			if tokType >= token.Void && tokType <= token.Any {
				p.typeNames[keyword] = true
			}
		}
	}
	return p
}

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

func (p *Parser) match(tokTypes ...token.Type) bool {
	for _, tokType := range tokTypes {
		if p.check(tokType) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) expect(tokType token.Type, message string) {
	if p.check(tokType) {
		p.advance()
		return
	}
	util.Error(p.current, message)
}

func (p *Parser) isTypeName(name string) bool {
	if !p.isTypedPass {
		return false
	}
	_, exists := p.typeNames[name]
	return exists
}

func isLValue(node *ast.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case ast.Ident, ast.Indirection, ast.Subscript, ast.MemberAccess:
		return true
	default:
		return false
	}
}

func (p *Parser) Parse() *ast.Node {
	var stmts []*ast.Node
	tok := p.current
	for !p.check(token.EOF) {
		for p.match(token.Semi) {
		}
		if p.check(token.EOF) {
			break
		}

		stmt := p.parseTopLevel()
		if stmt != nil {
			if stmt.Type == ast.MultiVarDecl {
				stmts = append(stmts, stmt.Data.(ast.MultiVarDeclNode).Decls...)
			} else {
				stmts = append(stmts, stmt)
			}
		}
	}
	return ast.NewBlock(tok, stmts, true)
}

func (p *Parser) parseTopLevel() *ast.Node {
	currentTok := p.current
	var stmt *ast.Node

	switch currentTok.Type {
	case token.Directive:
		directiveVal := currentTok.Value
		if strings.HasPrefix(directiveVal, "requires:") {
			flagStr := strings.TrimSpace(strings.TrimPrefix(directiveVal, "requires:"))
			p.cfg.ProcessDirectiveFlags(flagStr)
		} else {
			util.Error(currentTok, "Unknown directive '[b]: %s'", directiveVal)
		}
		stmt = ast.NewDirective(currentTok, directiveVal)
		p.advance()
	case token.TypeKeyword:
		p.advance()
		stmt = p.parseTypeDecl()
	case token.Extrn:
		p.advance()
		stmt = p.parseUntypedDeclarationList(token.Extrn, currentTok)
	case token.Auto:
		if p.isBxDeclarationAhead() {
			stmt = p.parseDeclaration(true)
		} else {
			p.advance()
			stmt = p.parseUntypedDeclarationList(token.Auto, p.previous)
		}
	case token.Ident:
		identTok := p.current
		peekTok := p.peek()

		if peekTok.Type == token.LParen {
			p.advance()
			stmt = p.parseFuncDecl(nil, identTok)
		} else if peekTok.Type == token.Asm {
			p.advance()
			stmt = p.parseAsmFuncDef(identTok)
		} else if p.isTypedPass && p.isTypeName(identTok.Value) {
			stmt = p.parseTypedVarOrFuncDecl(true)
		} else if p.isBxDeclarationAhead() {
			stmt = p.parseDeclaration(false)
		} else {
			stmt = p.parseUntypedGlobalDefinition(identTok)
		}
	default:
		if p.isTypedPass && (p.isBuiltinType(currentTok) || p.check(token.Const)) {
			stmt = p.parseTypedVarOrFuncDecl(true)
		} else {
			stmt = p.parseExpr()
			if stmt != nil {
				if stmt.Type == ast.FuncCall {
					funcCallData := stmt.Data.(ast.FuncCallNode)
					if funcCallData.FuncExpr.Type == ast.Ident {
						funcName := funcCallData.FuncExpr.Data.(ast.IdentNode).Name
						stmt = ast.NewFuncDecl(stmt.Tok, funcName, nil, ast.NewBlock(stmt.Tok, nil, true), false, false, ast.TypeUntyped)
					}
				}
				p.expect(token.Semi, "Expected ';' after top-level expression statement.")
			} else {
				util.Error(p.current, "Expected a top-level definition or expression.")
				p.advance()
			}
		}
	}
	return stmt
}

func (p *Parser) isBxDeclarationAhead() bool {
	originalPos, originalCurrent := p.pos, p.current
	defer func() { p.pos, p.current = originalPos, originalCurrent }()

	if p.check(token.Auto) {
		p.advance()
	}
	if !p.check(token.Ident) {
		return false
	}
	p.advance()
	for p.match(token.Comma) {
		if !p.check(token.Ident) {
			return false
		}
		p.advance()
	}
	return p.check(token.Eq) || p.check(token.Define)
}

func (p *Parser) isBuiltinType(tok token.Token) bool {
	return tok.Type >= token.Void && tok.Type <= token.Any
}

func (p *Parser) parseStmt() *ast.Node {
	tok := p.current

	isLabelAhead := false
	if p.peek().Type == token.Colon {
		if p.check(token.Ident) {
			isLabelAhead = true
		} else {
			for _, kwType := range token.KeywordMap {
				if p.check(kwType) {
					isLabelAhead = true
					break
				}
			}
		}
	}

	if isLabelAhead {
		var labelName string
		if p.check(token.Ident) {
			labelName = p.current.Value
		} else {
			for kw, typ := range token.KeywordMap {
				if p.current.Type == typ {
					labelName = kw
					break
				}
			}
		}
		p.advance() // consume label name
		p.advance() // consume ':'
		if p.check(token.RBrace) {
			return ast.NewLabel(tok, labelName, ast.NewBlock(p.current, nil, true))
		}
		return ast.NewLabel(tok, labelName, p.parseStmt())
	}

	if p.isTypedPass && (p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) || p.check(token.Const)) {
		return p.parseTypedVarOrFuncDecl(false)
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
	case p.check(token.Auto):
		if p.isBxDeclarationAhead() {
			return p.parseDeclaration(true)
		}
		p.advance()
		return p.parseUntypedDeclarationList(token.Auto, p.previous)
	case p.match(token.Extrn):
		return p.parseUntypedDeclarationList(token.Extrn, p.previous)
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
		var labelName string
		if p.check(token.Ident) {
			labelName = p.current.Value
			p.advance()
		} else {
			isKeyword := false
			for kw, typ := range token.KeywordMap {
				if p.current.Type == typ {
					labelName = kw
					isKeyword = true
					break
				}
			}
			if !isKeyword {
				util.Error(p.current, "Expected label name after 'goto'.")
				for !p.check(token.Semi) && !p.check(token.EOF) {
					p.advance()
				}
			} else {
				if labelName == "continue" {
					util.Warn(p.cfg, config.WarnExtra, p.current, "'goto continue' is a workaround for a limitation of -std=B. Please avoid this construct.")
				}
				p.advance()
			}
		}
		node := ast.NewGoto(tok, labelName)
		p.expect(token.Semi, "Expected ';' after goto statement.")
		return node
	case p.match(token.Return):
		var expr *ast.Node
		if !p.check(token.Semi) {
			p.expect(token.LParen, "Expected '(' after 'return' with value.")
			if !p.check(token.RParen) {
				expr = p.parseExpr()
			}
			p.expect(token.RParen, "Expected ')' after return value.")
		}
		p.expect(token.Semi, "Expected ';' after return statement.")
		return ast.NewReturn(tok, expr)
	case p.match(token.Break):
		p.expect(token.Semi, "Expected ';' after 'break'.")
		return ast.NewBreak(tok)
	case p.match(token.Continue):
		if !p.cfg.IsFeatureEnabled(config.FeatContinue) {
			util.Error(p.previous, "'continue' is a Bx extension, not available in -std=B.")
		}
		p.expect(token.Semi, "Expected ';' after 'continue'.")
		return ast.NewContinue(tok)
	case p.match(token.Semi):
		return ast.NewBlock(tok, nil, true)
	default:
		if p.check(token.Ident) {
			isShortDecl := false
			originalPos, originalCurrent := p.pos, p.current
			p.advance()
			for p.match(token.Comma) {
				if !p.check(token.Ident) {
					break
				}
				p.advance()
			}
			if p.check(token.Define) {
				isShortDecl = true
			}
			p.pos, p.current = originalPos, originalCurrent
			if isShortDecl {
				return p.parseDeclaration(false)
			}
		}

		expr := p.parseExpr()
		if expr != nil {
			p.expect(token.Semi, "Expected ';' after expression statement.")
		}
		return expr
	}
}

func (p *Parser) parseBlockStmt() *ast.Node {
	tok := p.current
	p.expect(token.LBrace, "Expected '{' to start a block.")
	var stmts []*ast.Node
	for !p.check(token.RBrace) && !p.check(token.EOF) {
		stmt := p.parseStmt()
		if stmt != nil {
			if stmt.Type == ast.MultiVarDecl {
				stmts = append(stmts, stmt.Data.(ast.MultiVarDeclNode).Decls...)
			} else {
				stmts = append(stmts, stmt)
			}
		}
	}
	p.expect(token.RBrace, "Expected '}' after block.")
	return ast.NewBlock(tok, stmts, false)
}

func (p *Parser) parseDeclaration(hasAuto bool) *ast.Node {
	declTok := p.current
	if hasAuto {
		p.expect(token.Auto, "Expected 'auto' keyword.")
		declTok = p.previous
	}

	var names []*ast.Node
	for {
		p.expect(token.Ident, "Expected identifier in declaration.")
		names = append(names, ast.NewIdent(p.previous, p.previous.Value))
		if !p.match(token.Comma) {
			break
		}
	}

	var op token.Type
	var inits []*ast.Node
	isDefine := false

	if p.match(token.Define) {
		op, isDefine = token.Define, true
	} else if p.match(token.Eq) {
		op = token.Eq
	}

	if op != 0 {
		for {
			inits = append(inits, p.parseAssignmentExpr())
			if !p.match(token.Comma) {
				break
			}
		}
		if len(names) != len(inits) {
			util.Error(declTok, "Mismatched number of variables and initializers (%d vs %d)", len(names), len(inits))
		}
	} else {
		if !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
			util.Error(declTok, "Uninitialized declaration is not allowed in this mode")
		}
	}

	var decls []*ast.Node
	for i, nameNode := range names {
		var initList []*ast.Node
		if i < len(inits) {
			initList = append(initList, inits[i])
		}
		name := nameNode.Data.(ast.IdentNode).Name
		decls = append(decls, ast.NewVarDecl(nameNode.Tok, name, ast.TypeUntyped, initList, nil, false, false, isDefine || op == token.Eq))
	}

	p.expect(token.Semi, "Expected ';' after declaration.")

	if len(decls) == 1 {
		return decls[0]
	}
	return ast.NewMultiVarDecl(declTok, decls)
}

func (p *Parser) parseUntypedDeclarationList(declType token.Type, declTok token.Token) *ast.Node {
	if declType == token.Extrn {
		var names []*ast.Node
		for {
			p.expect(token.Ident, "Expected identifier in 'extrn' list.")
			names = append(names, ast.NewIdent(p.previous, p.previous.Value))
			if !p.match(token.Comma) {
				break
			}
		}
		p.expect(token.Semi, "Expected ';' after 'extrn' declaration.")
		return ast.NewExtrnDecl(declTok, names)
	}

	var decls []*ast.Node
	for {
		var name string
		var itemToken token.Token

		if p.check(token.Ident) {
			itemToken = p.current
			name = p.current.Value
			p.advance()
		} else if p.check(token.TypeKeyword) {
			itemToken = p.current
			name = "type"
			util.Warn(p.cfg, config.WarnExtra, itemToken, "Using keyword 'type' as an identifier.")
			p.advance()
		} else {
			p.expect(token.Ident, "Expected identifier in declaration.")
			if p.check(token.Comma) || p.check(token.Semi) {
				continue
			}
			break
		}

		var sizeExpr *ast.Node
		isVector, isBracketed := false, false

		if p.match(token.LBracket) {
			if declType == token.Auto {
				util.Error(p.previous, "Classic B 'auto' vectors use 'auto name size', not 'auto name[size]'.")
			}
			isVector, isBracketed = true, true
			if !p.check(token.RBracket) {
				sizeExpr = p.parseExpr()
			}
			p.expect(token.RBracket, "Expected ']' after array size.")
		} else if p.check(token.Number) {
			isVector = true
			sizeExpr = p.parsePrimaryExpr()
		}

		if sizeExpr == nil && !isBracketed {
			if !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
				util.Error(itemToken, "Uninitialized declaration of '%s' is not allowed in this mode", name)
			}
		}

		decls = append(decls, ast.NewVarDecl(itemToken, name, nil, nil, sizeExpr, isVector, isBracketed, false))
		if !p.match(token.Comma) {
			break
		}
	}
	p.expect(token.Semi, "Expected ';' after declaration list.")

	if len(decls) == 1 {
		return decls[0]
	}
	return ast.NewMultiVarDecl(declTok, decls)
}

func (p *Parser) parseUntypedGlobalDefinition(nameToken token.Token) *ast.Node {
	name := nameToken.Value
	if p.isTypeName(name) {
		util.Error(nameToken, "Variable name '%s' shadows a type.", name)
	}
	p.advance()

	var sizeExpr *ast.Node
	isVector, isBracketed := false, false

	if p.match(token.LBracket) {
		isVector, isBracketed = true, true
		if !p.check(token.RBracket) {
			sizeExpr = p.parseExpr()
		}
		p.expect(token.RBracket, "Expected ']' for vector definition.")
	}

	var initList []*ast.Node
	if !p.check(token.Semi) {
		initList = append(initList, p.parseUnaryExpr())
		if isBracketed || p.match(token.Comma) || (!p.check(token.Semi) && !p.check(token.EOF)) {
			isVector = true
			if p.previous.Type != token.Comma {
				p.match(token.Comma)
			}
			for !p.check(token.Semi) && !p.check(token.EOF) {
				initList = append(initList, p.parseUnaryExpr())
				if p.check(token.Semi) || p.check(token.EOF) {
					break
				}
				p.match(token.Comma)
			}
		}
	}

	if len(initList) == 0 && sizeExpr == nil && !isBracketed {
		if !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
			util.Error(nameToken, "Uninitialized declaration of '%s' is not allowed in this mode", name)
		}
	}

	p.expect(token.Semi, "Expected ';' after global definition.")
	return ast.NewVarDecl(nameToken, name, nil, initList, sizeExpr, isVector, isBracketed, false)
}

func (p *Parser) parseFuncDecl(returnType *ast.BxType, nameToken token.Token) *ast.Node {
	name := nameToken.Value
	if p.isTypeName(name) {
		util.Error(nameToken, "Function name '%s' shadows a type.", name)
	}
	p.expect(token.LParen, "Expected '(' after function name.")

	var params []*ast.Node
	var hasVarargs bool
	isTyped := p.isTypedPass && p.isTypedParameterList()

	if isTyped {
		params, hasVarargs = p.parseTypedParameters()
	} else {
		params, hasVarargs = p.parseUntypedParameters()
	}
	p.expect(token.RParen, "Expected ')' after parameters.")

	var decls []*ast.Node
	for p.check(token.Auto) || p.check(token.Extrn) {
		decl := p.parseStmt()
		if decl != nil {
			if decl.Type == ast.MultiVarDecl {
				decls = append(decls, decl.Data.(ast.MultiVarDeclNode).Decls...)
			} else {
				decls = append(decls, decl)
			}
		}
	}

	var body *ast.Node
	if p.check(token.LBrace) {
		body = p.parseBlockStmt()
	} else {
		body = p.parseStmt()
	}

	if len(decls) > 0 {
		var allStmts []*ast.Node
		allStmts = append(allStmts, decls...)
		if body != nil {
			if body.Type == ast.Block && !body.Data.(ast.BlockNode).IsSynthetic {
				allStmts = append(allStmts, body.Data.(ast.BlockNode).Stmts...)
			} else {
				allStmts = append(allStmts, body)
			}
		}
		body = ast.NewBlock(nameToken, allStmts, false)
	}

	if returnType == nil {
		returnType = ast.TypeUntyped
		if isTyped {
			returnType = ast.TypeInt
		}
	}

	return ast.NewFuncDecl(nameToken, name, params, body, hasVarargs, isTyped, returnType)
}

func (p *Parser) parseAsmFuncDef(nameToken token.Token) *ast.Node {
	name := nameToken.Value
	if p.isTypeName(name) {
		util.Error(nameToken, "Function name '%s' shadows a type.", name)
	}

	p.expect(token.Asm, "Expected '__asm__' keyword.")
	asmTok := p.previous

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

	return ast.NewFuncDecl(nameToken, nameToken.Value, nil, body, false, false, nil)
}

func (p *Parser) parseTypeDecl() *ast.Node {
	typeTok := p.previous
	var underlyingType *ast.BxType

	if p.check(token.Struct) {
		p.advance()
		underlyingType = p.parseStructDef()
	} else {
		util.Error(typeTok, "Expected 'struct' after 'type'.")
		p.advance()
		return nil
	}

	var name string
	if p.check(token.Ident) {
		name = p.current.Value
		p.advance()
	} else {
		if underlyingType.StructTag == "" {
			util.Error(typeTok, "Typedef for anonymous struct must have a name.")
			return nil
		}
		name = underlyingType.StructTag
	}

	p.typeNames[name] = true
	underlyingType.Name = name

	p.expect(token.Semi, "Expected ';' after type declaration.")
	return ast.NewTypeDecl(typeTok, name, underlyingType)
}

func (p *Parser) parseTypedVarOrFuncDecl(isTopLevel bool) *ast.Node {
	startTok := p.current
	declType := p.parseType()

	if p.match(token.Define) {
		util.Error(p.previous, "Cannot use ':=' in a typed declaration. Use '=' instead.")
		return p.parseTypedVarDeclBody(startTok, declType, p.previous)
	}

	p.expect(token.Ident, "Expected identifier after type.")
	nameToken := p.previous

	if p.check(token.LParen) {
		return p.parseFuncDecl(declType, nameToken)
	}

	return p.parseTypedVarDeclBody(startTok, declType, nameToken)
}

func (p *Parser) parseTypedVarDeclBody(startTok token.Token, declType *ast.BxType, nameToken token.Token) *ast.Node {
	var decls []*ast.Node
	currentNameToken := nameToken

	for {
		name := currentNameToken.Value
		finalType := declType
		var sizeExpr *ast.Node
		isArr, isBracketed := false, false

		if p.match(token.LBracket) {
			isArr, isBracketed = true, true
			if !p.check(token.RBracket) {
				sizeExpr = p.parseExpr()
			}
			p.expect(token.RBracket, "Expected ']' after array size.")
			finalType = &ast.BxType{Kind: ast.TYPE_ARRAY, Base: declType, ArraySize: sizeExpr, IsConst: declType.IsConst}
		}

		var initList []*ast.Node
		if p.match(token.Eq) {
			initList = append(initList, p.parseAssignmentExpr())
		} else {
			if !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
				util.Error(nameToken, "Initialized typed declaration is required in this mode")
			}
		}

		decls = append(decls, ast.NewVarDecl(currentNameToken, name, finalType, initList, sizeExpr, isArr, isBracketed, false))

		if !p.match(token.Comma) {
			break
		}

		p.expect(token.Ident, "Expected identifier after comma in declaration list.")
		currentNameToken = p.previous
	}

	p.expect(token.Semi, "Expected ';' after typed variable declaration.")

	if len(decls) == 1 {
		return decls[0]
	}
	return ast.NewMultiVarDecl(startTok, decls)
}

func (p *Parser) parseType() *ast.BxType {
	if !p.isTypedPass {
		return nil
	}

	isConst := p.match(token.Const)
	var baseType *ast.BxType

	if p.match(token.LBracket) {
		p.expect(token.RBracket, "Expected ']' to complete array type specifier.")
		elemType := p.parseType()
		baseType = &ast.BxType{Kind: ast.TYPE_ARRAY, Base: elemType}
	} else {
		tok := p.current
		if p.match(token.Struct) {
			if p.check(token.Ident) && p.peek().Type != token.LBrace {
				tagName := p.current.Value
				p.advance()
				baseType = &ast.BxType{Kind: ast.TYPE_STRUCT, Name: tagName, StructTag: tagName}
			} else {
				baseType = p.parseStructDef()
			}
		} else if p.isBuiltinType(tok) {
			p.advance()
			var typeName string
			for keyword, t := range token.KeywordMap {
				if t == p.previous.Type {
					typeName = keyword
					break
				}
			}

			if p.previous.Type == token.Void {
				baseType = ast.TypeVoid
			} else if p.previous.Type == token.StringKeyword {
				baseType = ast.TypeString
			} else {
				if typeName == "" {
					util.Error(tok, "Internal parser error: could not find string for builtin type %v", tok.Type)
					return ast.TypeUntyped
				}
				baseType = &ast.BxType{Kind: ast.TYPE_PRIMITIVE, Name: typeName}
			}
		} else if p.check(token.Ident) {
			typeName := p.current.Value
			if !p.isTypeName(typeName) {
				util.Error(p.current, "Unknown type name '%s'.", typeName)
				p.advance()
				return ast.TypeUntyped
			}
			p.advance()
			baseType = &ast.BxType{Kind: ast.TYPE_PRIMITIVE, Name: typeName}
		} else {
			util.Error(p.current, "Expected a type name, 'struct', or '[]'.")
			p.advance()
			return ast.TypeUntyped
		}
	}

	for p.match(token.Star) {
		baseType = &ast.BxType{Kind: ast.TYPE_POINTER, Base: baseType}
	}

	if isConst {
		newType := *baseType
		newType.IsConst = true
		return &newType
	}
	return baseType
}

func (p *Parser) parseStructDef() *ast.BxType {
	structType := &ast.BxType{Kind: ast.TYPE_STRUCT}

	if p.check(token.Ident) {
		structType.StructTag = p.current.Value
		if p.isTypedPass {
			p.typeNames[structType.StructTag] = true
		}
		p.advance()
	}

	p.expect(token.LBrace, "Expected '{' to open struct definition.")

	for !p.check(token.RBrace) && !p.check(token.EOF) {
		p.expect(token.Ident, "Expected field name in struct.")
		nameToken := p.previous
		fieldType := p.parseType()
		fieldDecl := ast.NewVarDecl(nameToken, nameToken.Value, fieldType, nil, nil, false, false, false)
		structType.Fields = append(structType.Fields, fieldDecl)
		p.expect(token.Semi, "Expected ';' after struct field declaration.")
	}

	p.expect(token.RBrace, "Expected '}' to close struct definition.")
	if structType.StructTag != "" {
		structType.Name = structType.StructTag
	}
	return structType
}

func (p *Parser) isTypedParameterList() bool {
	originalPos, originalCurrent := p.pos, p.current
	defer func() { p.pos, p.current = originalPos, originalCurrent }()

	if p.check(token.RParen) {
		return false
	}
	if p.check(token.Void) && p.peek().Type == token.RParen {
		return true
	}
	if p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) {
		return true
	}

	for {
		if !p.check(token.Ident) {
			return false
		}
		p.advance()
		if !p.match(token.Comma) {
			break
		}
	}
	return p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) || p.check(token.LBracket) || p.check(token.Star)
}

func (p *Parser) parseUntypedParameters() ([]*ast.Node, bool) {
	var params []*ast.Node
	var hasVarargs bool
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
	return params, hasVarargs
}

func (p *Parser) parseTypedParameters() ([]*ast.Node, bool) {
	var params []*ast.Node
	var hasVarargs bool
	if p.check(token.RParen) {
		return params, false
	}
	if p.check(token.Void) && p.peek().Type == token.RParen {
		p.advance()
		return params, false
	}

	for {
		if p.check(token.RParen) {
			break
		}
		if p.match(token.Dots) {
			hasVarargs = true
			break
		}

		if p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) {
			paramType := p.parseType()
			name := fmt.Sprintf("anonparam%d", len(params))
			paramNode := ast.NewVarDecl(p.previous, name, paramType, nil, nil, false, false, false)
			params = append(params, paramNode)
		} else {
			var names []token.Token
			p.expect(token.Ident, "Expected parameter name.")
			names = append(names, p.previous)
			for p.match(token.Comma) {
				if p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) || p.check(token.LBracket) || p.check(token.Star) || p.check(token.RParen) || p.check(token.Dots) {
					p.pos--
					p.current = p.tokens[p.pos-1]
					break
				}
				p.expect(token.Ident, "Expected parameter name.")
				names = append(names, p.previous)
			}

			paramType := p.parseType()
			for _, nameTok := range names {
				paramNode := ast.NewVarDecl(nameTok, nameTok.Value, paramType, nil, nil, false, false, false)
				params = append(params, paramNode)
			}
		}

		if !p.match(token.Comma) {
			break
		}
	}
	return params, hasVarargs
}

func getBinaryOpPrecedence(op token.Type) int {
	switch op {
	case token.OrOr:
		return 4
	case token.AndAnd:
		return 5
	case token.Or:
		return 6
	case token.Xor:
		return 7
	case token.And:
		return 8
	case token.EqEq, token.Neq:
		return 9
	case token.Lt, token.Gt, token.Lte, token.Gte:
		return 10
	case token.Shl, token.Shr:
		return 11
	case token.Plus, token.Minus:
		return 12
	case token.Star, token.Slash, token.Rem:
		return 13
	default:
		return -1
	}
}

func (p *Parser) parseExpr() *ast.Node {
	return p.parseAssignmentExpr()
}

func (p *Parser) parseAssignmentExpr() *ast.Node {
	left := p.parseTernaryExpr()
	if op := p.current.Type; op >= token.Eq && op <= token.EqShr {
		if !isLValue(left) {
			util.Error(p.current, "Invalid target for assignment.")
		}
		tok := p.current
		p.advance()
		right := p.parseAssignmentExpr()
		return ast.NewAssign(tok, op, left, right)
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

func (p *Parser) parseBinaryExpr(minPrec int) *ast.Node {
	left := p.parseUnaryExpr()
	for {
		if left == nil {
			return nil
		}
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

func (p *Parser) parseUnaryExpr() *ast.Node {
	tok := p.current
	if p.match(token.Not, token.Complement, token.Minus, token.Plus, token.Inc, token.Dec, token.Star, token.And) {
		op, opToken := p.previous.Type, p.previous
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

func (p *Parser) parsePostfixExpr() *ast.Node {
	expr := p.parsePrimaryExpr()
	for {
		if expr == nil {
			return nil
		}
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
		} else if p.isTypedPass && p.match(token.Dot) {
			p.expect(token.Ident, "Expected member name after '.'.")
			member := ast.NewIdent(p.previous, p.previous.Value)
			expr = ast.NewMemberAccess(tok, expr, member)
		} else if p.match(token.Inc, token.Dec) {
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
	if p.match(token.TypeKeyword) {
		util.Warn(p.cfg, config.WarnExtra, p.previous, "Using keyword 'type' as an identifier.")
		return ast.NewIdent(tok, "type")
	}
	if p.match(token.LParen) {
		if p.isTypedPass && (p.isBuiltinType(p.current) || p.isTypeName(p.current.Value)) {
			castType := p.parseType()
			p.expect(token.RParen, "Expected ')' after type in cast.")
			exprToCast := p.parseUnaryExpr()
			return ast.NewTypeCast(tok, exprToCast, castType)
		}
		expr := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after expression.")
		return expr
	}
	if p.match(token.Auto) {
		if p.check(token.LBracket) {
			allocTok := p.previous
			p.advance()
			sizeExpr := p.parseExpr()
			p.expect(token.RBracket, "Expected ']' after auto allocation size.")
			return ast.NewAutoAlloc(allocTok, sizeExpr)
		}
		p.pos--
		p.current = p.previous
	}

	if !p.check(token.EOF) && !p.check(token.RBrace) && !p.check(token.Semi) {
		util.Error(tok, "Expected an expression.")
	}
	return nil
}

func (p *Parser) buildSwitchJumpTable(switchNode *ast.Node) {
	if switchNode == nil || switchNode.Type != ast.Switch {
		return
	}
	p.findCasesRecursive(switchNode.Data.(ast.SwitchNode).Body, switchNode)
}

func (p *Parser) findCasesRecursive(node, switchNode *ast.Node) {
	if node == nil || (node.Type == ast.Switch && node != switchNode) {
		return
	}

	swData := switchNode.Data.(ast.SwitchNode)

	if node.Type == ast.Case {
		caseData := node.Data.(ast.CaseNode)
		foldedValue := ast.FoldConstants(caseData.Value)
		if foldedValue.Type != ast.Number {
			util.Error(node.Tok, "Case value must be a constant integer.")
		} else {
			caseData.Value = foldedValue
			caseVal := foldedValue.Data.(ast.NumberNode).Value
			labelName := fmt.Sprintf("@case_%d_%d", caseVal, node.Tok.Line)
			swData.CaseLabels = append(swData.CaseLabels, ast.CaseLabelNode{Value: caseVal, LabelName: labelName})
			caseData.QbeLabel = labelName
			node.Data = caseData
			switchNode.Data = swData
		}
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
