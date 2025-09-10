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

func (p *Parser) check(tokType token.Type) bool { return p.current.Type == tokType }

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
	if !p.isTypedPass { return false }
	_, exists := p.typeNames[name]
	return exists
}

func isLValue(node *ast.Node) bool {
	if node == nil { return false }
	switch node.Type {
	case ast.Ident, ast.Indirection, ast.Subscript, ast.MemberAccess: return true
	default: return false
	}
}

func (p *Parser) Parse() *ast.Node {
	var stmts []*ast.Node
	tok := p.current
	for !p.check(token.EOF) {
		for p.match(token.Semi) {}
		if p.check(token.EOF) { break }

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
			if err := p.cfg.ProcessDirectiveFlags(flagStr, currentTok); err != nil {
				util.Error(currentTok, err.Error())
			}
		} else {
			util.Error(currentTok, "Unknown directive '[b]: %s'", directiveVal)
		}
		stmt = ast.NewDirective(currentTok, directiveVal)
		p.advance()
	case token.TypeKeyword: p.advance(); stmt = p.parseTypeDecl()
	case token.Extrn: p.advance(); stmt = p.parseUntypedDeclarationList(token.Extrn, currentTok)
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
		} else if peekTok.Type == token.Extrn {
			// Handle typed external declaration: type_name extrn function_name;
			if p.isTypedPass && p.isTypeName(identTok.Value) {
				returnType := p.typeFromName(identTok.Value)
				if returnType != nil {
					p.advance() // consume type name
					p.advance() // consume 'extrn'
					stmt = p.parseTypedExtrnDecl(identTok, returnType)
				} else {
					// Fallback to regular parsing if type not found
					stmt = p.parseUntypedGlobalDefinition(identTok)
				}
			} else {
				stmt = p.parseUntypedGlobalDefinition(identTok)
			}
		} else if p.isTypedPass && p.isTypeName(identTok.Value) && peekTok.Type != token.Define {
			stmt = p.parseTypedVarOrFuncDecl(true)
		} else if p.isBxDeclarationAhead() {
			stmt = p.parseDeclaration(false)
		} else if p.isMultiAssignmentAhead() {
			stmt = p.parseExpr()
			if stmt != nil {
				p.expect(token.Semi, "Expected ';' after multi-assignment statement")
			}
		} else {
			stmt = p.parseUntypedGlobalDefinition(identTok)
		}
	default:
		if p.isTypedPass && (p.isBuiltinType(p.current) || p.check(token.Const)) {
			stmt = p.parseTypedVarOrFuncDecl(true)
		} else {
			stmt = p.parseExpr()
			if stmt != nil {
				p.expect(token.Semi, "Expected ';' after top-level expression statement")
			} else {
				util.Error(p.current, "Expected a top-level definition or expression")
				p.advance()
			}
		}
	}
	return stmt
}

func (p *Parser) isBxDeclarationAhead() bool {
	originalPos, originalCurrent := p.pos, p.current
	defer func() { p.pos, p.current = originalPos, originalCurrent }()

	hasAuto := p.match(token.Auto)
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

	if p.check(token.Define) {
		return true
	}
	if p.check(token.Eq) {
		return hasAuto
	}
	return false
}

func (p *Parser) isMultiAssignmentAhead() bool {
	originalPos, originalCurrent := p.pos, p.current
	defer func() { p.pos, p.current = originalPos, originalCurrent }()

	if !p.check(token.Ident) {
		return false
	}
	p.advance()

	hasMultipleVars := false
	for p.match(token.Comma) {
		hasMultipleVars = true
		if !p.check(token.Ident) {
			return false
		}
		p.advance()
	}

	return hasMultipleVars && p.check(token.Eq)
}

func (p *Parser) isBuiltinType(tok token.Token) bool {
	return tok.Type >= token.Void && tok.Type <= token.Any
}

// isPointerCastAhead checks if the current position looks like a pointer cast: (type*)
// This allows complex pointer casts while disallowing simple scalar C-style casts
func (p *Parser) isPointerCastAhead() bool {
	if !p.isTypedPass { return false }

	originalPos, originalCurrent := p.pos, p.current
	defer func() { p.pos, p.current = originalPos, originalCurrent }()

	if p.match(token.Const) {}

	if p.isBuiltinType(p.current) {
		p.advance()
	} else if p.check(token.Ident) && p.isTypeName(p.current.Value) {
		p.advance()
	} else {
		return false
	}

	hasPointer := false
	if p.match(token.Star) {
		hasPointer = true
		for p.match(token.Star) {}
	}

	if p.match(token.LBracket) {
		hasPointer = true
		for !p.check(token.RBracket) && !p.check(token.EOF) { p.advance() }
		if p.check(token.RBracket) { p.advance() }
	}

	return hasPointer && p.check(token.RParen)
}

func (p *Parser) parseStmt() *ast.Node {
	tok := p.current

	isLabelAhead := (p.check(token.Ident) || p.current.Type >= token.Auto) && p.peek().Type == token.Colon

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
		p.advance()
		p.advance()
		if p.check(token.RBrace) { return ast.NewLabel(tok, labelName, ast.NewBlock(p.current, nil, true)) }
		return ast.NewLabel(tok, labelName, p.parseStmt())
	}

	if p.isTypedPass && (p.isBuiltinType(p.current) || (p.isTypeName(p.current.Value) && p.peek().Type != token.Define) || p.check(token.Const)) {
		return p.parseTypedVarOrFuncDecl(false)
	}

	switch {
	case p.match(token.If):
		p.expect(token.LParen, "Expected '(' after 'if'")
		cond := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after if condition")
		thenBody := p.parseStmt()
		var elseBody *ast.Node
		if p.match(token.Else) {
			elseBody = p.parseStmt()
		}
		return ast.NewIf(tok, cond, thenBody, elseBody)
	case p.match(token.While):
		p.expect(token.LParen, "Expected '(' after 'while'")
		cond := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after while condition")
		body := p.parseStmt()
		return ast.NewWhile(tok, cond, body)
	case p.match(token.Switch):
		hasParen := p.match(token.LParen)
		expr := p.parseExpr()
		if hasParen {
			p.expect(token.RParen, "Expected ')' after switch expression")
		}
		body := p.parseStmt()
		return ast.NewSwitch(tok, expr, body)
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
		var values []*ast.Node
		for {
			values = append(values, p.parseExpr())
			if !p.match(token.Comma) {
				break
			}
		}
		p.expect(token.Colon, "Expected ':' after case value")
		body := p.parseStmt()
		return ast.NewCase(tok, values, body)
	case p.match(token.Default):
		p.expect(token.Colon, "Expected ':' after 'default'")
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
					labelName, isKeyword = kw, true
					break
				}
			}
			if !isKeyword {
				util.Error(p.current, "Expected label name after 'goto'")
				for !p.check(token.Semi) && !p.check(token.EOF) {
					p.advance()
				}
			} else {
				if labelName == "continue" {
					util.Warn(p.cfg, config.WarnExtra, p.current, "'goto continue' is a workaround for a limitation of -std=B; please avoid this construct")
				}
				p.advance()
			}
		}
		node := ast.NewGoto(tok, labelName)
		p.expect(token.Semi, "Expected ';' after goto statement")
		return node
	case p.match(token.Return):
		var expr *ast.Node
		if !p.check(token.Semi) {
			p.expect(token.LParen, "Expected '(' after 'return' with value")
			if !p.check(token.RParen) {
				expr = p.parseExpr()
			}
			p.expect(token.RParen, "Expected ')' after return value")
		}
		p.expect(token.Semi, "Expected ';' after return statement")
		return ast.NewReturn(tok, expr)
	case p.match(token.Break):
		p.expect(token.Semi, "Expected ';' after 'break'")
		return ast.NewBreak(tok)
	case p.match(token.Continue):
		if !p.cfg.IsFeatureEnabled(config.FeatContinue) {
			util.Error(p.previous, "'continue' is a Bx extension, not available in -std=B")
		}
		p.expect(token.Semi, "Expected ';' after 'continue'")
		return ast.NewContinue(tok)
	case p.match(token.Semi): return ast.NewBlock(tok, nil, true)
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
			p.expect(token.Semi, "Expected ';' after expression statement")
		}
		return expr
	}
}

func (p *Parser) parseBlockStmt() *ast.Node {
	tok := p.current
	p.expect(token.LBrace, "Expected '{' to start a block")
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
	p.expect(token.RBrace, "Expected '}' after block")
	return ast.NewBlock(tok, stmts, false)
}

func (p *Parser) parseDeclaration(hasAuto bool) *ast.Node {
	declTok := p.current
	if hasAuto {
		p.expect(token.Auto, "Expected 'auto' keyword")
		declTok = p.previous
	}

	var names []*ast.Node
	for {
		p.expect(token.Ident, "Expected identifier in declaration")
		names = append(names, ast.NewIdent(p.previous, p.previous.Value))
		if !p.match(token.Comma) {
			break
		}
	}

	var op token.Type
	var inits []*ast.Node
	isDefine := false

	if p.match(token.Define) {
		if hasAuto {
			util.Error(p.previous, "Cannot use ':=' in a typed declaration; use '=' instead")
			return ast.NewVarDecl(declTok, "", ast.TypeUntyped, nil, nil, false, false, false)
		}
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
	} else if !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
		util.Error(declTok, "Uninitialized declaration is not allowed in this mode")
	}

	var decls []*ast.Node
	for i, nameNode := range names {
		var initList []*ast.Node
		if i < len(inits) {
			initList = append(initList, inits[i])
		}
		name := nameNode.Data.(ast.IdentNode).Name
		decls = append(decls, ast.NewVarDecl(nameNode.Tok, name, ast.TypeUntyped, initList, nil, false, false, isDefine))
	}

	p.expect(token.Semi, "Expected ';' after declaration")

	if len(decls) == 1 {
		return decls[0]
	}
	return ast.NewMultiVarDecl(declTok, decls)
}

func (p *Parser) parseUntypedDeclarationList(declType token.Type, declTok token.Token) *ast.Node {
	if declType == token.Extrn {
		var names []*ast.Node
		for {
			p.expect(token.Ident, "Expected identifier in 'extrn' list")
			names = append(names, ast.NewIdent(p.previous, p.previous.Value))
			if !p.match(token.Comma) {
				break
			}
		}
		p.expect(token.Semi, "Expected ';' after 'extrn' declaration")
		return ast.NewExtrnDecl(declTok, names, nil)
	}

	var decls []*ast.Node
	for {
		var name string
		var itemToken token.Token

		if p.check(token.Ident) {
			itemToken, name = p.current, p.current.Value
			p.advance()
		} else if p.check(token.TypeKeyword) {
			itemToken, name = p.current, "type"
			util.Warn(p.cfg, config.WarnExtra, itemToken, "Using keyword 'type' as an identifier")
			p.advance()
		} else {
			p.expect(token.Ident, "Expected identifier in declaration")
			if p.check(token.Comma) || p.check(token.Semi) {
				continue
			}
			break
		}

		var sizeExpr *ast.Node
		isVector, isBracketed := false, false

		if p.match(token.LBracket) {
			if declType == token.Auto {
				util.Error(p.previous, "Classic B 'auto' vectors use 'auto name size', not 'auto name[size]'")
			}
			isVector, isBracketed = true, true
			if !p.check(token.RBracket) {
				sizeExpr = p.parseExpr()
			}
			p.expect(token.RBracket, "Expected ']' after array size")
		} else if p.check(token.Number) {
			isVector = true
			sizeExpr = p.parsePrimaryExpr()
		}

		if sizeExpr == nil && !isBracketed && !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
			util.Error(itemToken, "Uninitialized declaration of '%s' is not allowed in this mode", name)
		}

		decls = append(decls, ast.NewVarDecl(itemToken, name, nil, nil, sizeExpr, isVector, isBracketed, false))
		if !p.match(token.Comma) {
			break
		}
	}
	p.expect(token.Semi, "Expected ';' after declaration list")

	if len(decls) == 1 {
		return decls[0]
	}
	return ast.NewMultiVarDecl(declTok, decls)
}

func (p *Parser) parseUntypedGlobalDefinition(nameToken token.Token) *ast.Node {
	name := nameToken.Value
	if p.isTypeName(name) {
		util.Error(nameToken, "Variable name '%s' shadows a type", name)
	}
	p.advance()

	var sizeExpr *ast.Node
	isVector, isBracketed := false, false

	if p.match(token.LBracket) {
		isVector, isBracketed = true, true
		if !p.check(token.RBracket) {
			sizeExpr = p.parseExpr()
		}
		p.expect(token.RBracket, "Expected ']' for vector definition")
	}

	var initList []*ast.Node
	if !p.check(token.Semi) {
		// Check if this is an attempt to assign without declaration
		if p.check(token.Eq) {
			util.Error(nameToken, "Assignment without declaration is not allowed at global scope. Use ':=' or make it a typed declaration and initialization")
			return nil
		}
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

	if len(initList) == 0 && sizeExpr == nil && !isBracketed && !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
		util.Error(nameToken, "Uninitialized declaration of '%s' is not allowed in this mode", name)
	}

	p.expect(token.Semi, "Expected ';' after global definition")
	return ast.NewVarDecl(nameToken, name, nil, initList, sizeExpr, isVector, isBracketed, false)
}

func (p *Parser) parseFuncDecl(returnType *ast.BxType, nameToken token.Token) *ast.Node {
	name := nameToken.Value
	if p.isTypeName(name) {
		util.Error(nameToken, "Function name '%s' shadows a type", name)
	}
	p.expect(token.LParen, "Expected '(' after function name")

	var params []*ast.Node
	var hasVarargs bool
	isTyped := p.isTypedPass && p.isTypedParameterList()

	if isTyped {
		params, hasVarargs = p.parseTypedParameters()
	} else {
		params, hasVarargs = p.parseUntypedParameters()
	}
	p.expect(token.RParen, "Expected ')' after parameters")

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
		util.Error(nameToken, "Function name '%s' shadows a type", name)
	}

	p.expect(token.Asm, "Expected '__asm__' keyword")
	asmTok := p.previous

	p.expect(token.LParen, "Expected '(' after '__asm__'")
	var codeParts []string
	for !p.check(token.RParen) && !p.check(token.EOF) {
		p.expect(token.String, "Expected string literal in '__asm__' block")
		codeParts = append(codeParts, p.previous.Value)
		p.match(token.Comma)
	}
	p.expect(token.RParen, "Expected ')' to close '__asm__' block")
	asmCode := strings.Join(codeParts, "\n")
	body := ast.NewAsmStmt(asmTok, asmCode)

	if !p.check(token.LBrace) {
		p.expect(token.Semi, "Expected ';' or '{' after '__asm__' definition")
	} else {
		p.parseBlockStmt()
	}

	return ast.NewFuncDecl(nameToken, nameToken.Value, nil, body, false, false, nil)
}

func (p *Parser) parseTypeDecl() *ast.Node {
	typeTok := p.previous

	if p.match(token.Enum) {
		return p.parseEnumDef(typeTok)
	}

	if p.match(token.Struct) {
		underlyingType := p.parseStructDef()
		var name string
		if p.check(token.Ident) {
			name = p.current.Value
			p.advance()
		} else {
			if underlyingType.StructTag == "" {
				util.Error(typeTok, "Typedef for anonymous struct must have a name")
				return nil
			}
			name = underlyingType.StructTag
		}

		p.typeNames[name] = true
		underlyingType.Name = name

		p.expect(token.Semi, "Expected ';' after type declaration")
		return ast.NewTypeDecl(typeTok, name, underlyingType)
	}

	// Handle type alias: type <underlying_type> <new_name>;
	underlyingType := p.parseType()
	if underlyingType == nil {
		util.Error(p.current, "Expected a type for type alias after 'type'")
		for !p.check(token.Semi) && !p.check(token.EOF) {
			p.advance()
		}
		return nil
	}

	p.expect(token.Ident, "Expected new type name for alias")
	nameToken := p.previous
	name := nameToken.Value

	if p.isTypeName(name) {
		util.Error(nameToken, "Redefinition of type '%s'", name)
	}
	p.typeNames[name] = true

	p.expect(token.Semi, "Expected ';' after type alias declaration")
	return ast.NewTypeDecl(typeTok, name, underlyingType)
}

func (p *Parser) parseEnumDef(typeTok token.Token) *ast.Node {
	p.expect(token.Ident, "Expected enum name")
	nameToken := p.previous
	name := nameToken.Value
	p.typeNames[name] = true

	p.expect(token.LBrace, "Expected '{' to open enum definition")

	var members []*ast.Node
	var currentValue int64 = 0

	for !p.check(token.RBrace) && !p.check(token.EOF) {
		p.expect(token.Ident, "Expected enum member name")
		memberToken := p.previous
		memberName := memberToken.Value

		if p.match(token.Eq) {
			valExpr := p.parseExpr()
			foldedVal := ast.FoldConstants(valExpr)
			if foldedVal.Type != ast.Number {
				util.Error(valExpr.Tok, "Enum member initializer must be a constant integer")
				currentValue++
			} else {
				currentValue = foldedVal.Data.(ast.NumberNode).Value
			}
		}

		initExpr := ast.NewNumber(memberToken, currentValue)
		memberDecl := ast.NewVarDecl(memberToken, memberName, ast.TypeInt, []*ast.Node{initExpr}, nil, false, false, true)
		members = append(members, memberDecl)

		currentValue++

		if !p.match(token.Comma) {
			break
		}
	}

	p.expect(token.RBrace, "Expected '}' to close enum definition")
	p.expect(token.Semi, "Expected ';' after enum declaration")

	return ast.NewEnumDecl(typeTok, name, members)
}

func (p *Parser) parseTypedVarOrFuncDecl(isTopLevel bool) *ast.Node {
	startTok := p.current
	declType := p.parseType()

	if p.match(token.Define) {
		util.Error(p.previous, "Cannot use ':=' in a typed declaration; use '=' instead")
		return p.parseTypedVarDeclBody(startTok, declType, p.previous)
	}

	// Check for typed external declaration: type extrn function_name;
	if p.match(token.Extrn) {
		return p.parseTypedExtrnDecl(startTok, declType)
	}

	p.expect(token.Ident, "Expected identifier after type")
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
			p.expect(token.RBracket, "Expected ']' after array size")
			finalType = &ast.BxType{Kind: ast.TYPE_ARRAY, Base: declType, ArraySize: sizeExpr, IsConst: declType.IsConst}
		}

		var initList []*ast.Node
		if p.match(token.Eq) {
			initList = append(initList, p.parseAssignmentExpr())
		} else if p.check(token.Define) {
			util.Error(p.current, "Cannot use ':=' in a typed declaration; use '=' instead")
			return ast.NewVarDecl(currentNameToken, name, finalType, nil, sizeExpr, isArr, isBracketed, false)
		} else if !p.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
			util.Error(nameToken, "Initialized typed declaration is required in this mode")
		}

		decls = append(decls, ast.NewVarDecl(currentNameToken, name, finalType, initList, sizeExpr, isArr, isBracketed, false))

		if !p.match(token.Comma) {
			break
		}

		p.expect(token.Ident, "Expected identifier after comma in declaration list")
		currentNameToken = p.previous
	}

	p.expect(token.Semi, "Expected ';' after typed variable declaration")

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
		p.expect(token.RBracket, "Expected ']' to complete array type specifier")
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
		} else if p.match(token.Enum) {
			if p.check(token.Ident) {
				tagName := p.current.Value
				p.advance()
				baseType = &ast.BxType{Kind: ast.TYPE_ENUM, Name: tagName}
			} else {
				util.Error(tok, "Anonymous enums are not supported as types")
				baseType = ast.TypeUntyped
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
			} else if p.previous.Type >= token.Float && p.previous.Type <= token.Float64 {
				baseType = &ast.BxType{Kind: ast.TYPE_FLOAT, Name: typeName}
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
				util.Error(p.current, "Unknown type name '%s'", typeName)
				p.advance()
				return ast.TypeUntyped
			}
			p.advance()
			baseType = &ast.BxType{Kind: ast.TYPE_PRIMITIVE, Name: typeName}
		} else {
			util.Error(p.current, "Expected a type name, 'struct', 'enum', or '[]'")
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

	p.expect(token.LBrace, "Expected '{' to open struct definition")

	for !p.check(token.RBrace) && !p.check(token.EOF) {
		var names []token.Token
		p.expect(token.Ident, "Expected field name in struct")
		names = append(names, p.previous)

		for p.match(token.Comma) {
			if p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) || p.check(token.LBracket) || p.check(token.Star) || p.check(token.Struct) {
				p.pos--
				p.current = p.tokens[p.pos-1]
				break
			}
			p.expect(token.Ident, "Expected field name after comma")
			names = append(names, p.previous)
		}

		fieldType := p.parseType()

		for _, nameToken := range names {
			fieldDecl := ast.NewVarDecl(nameToken, nameToken.Value, fieldType, nil, nil, false, false, false)
			structType.Fields = append(structType.Fields, fieldDecl)
		}

		p.expect(token.Semi, "Expected ';' after struct field declaration")
	}

	p.expect(token.RBrace, "Expected '}' to close struct definition")
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
			p.expect(token.Ident, "Expected parameter name or '...'")
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
			p.expect(token.Ident, "Expected parameter name")
			names = append(names, p.previous)
			for p.match(token.Comma) {
				if p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) || p.check(token.LBracket) || p.check(token.Star) || p.check(token.RParen) || p.check(token.Dots) {
					p.pos--
					p.current = p.tokens[p.pos-1]
					break
				}
				p.expect(token.Ident, "Expected parameter name")
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
	case token.OrOr: return 4
	case token.AndAnd: return 5
	case token.Or: return 6
	case token.Xor: return 7
	case token.And: return 8
	case token.EqEq, token.Neq: return 9
	case token.Lt, token.Gt, token.Lte, token.Gte: return 10
	case token.Shl, token.Shr: return 11
	case token.Plus, token.Minus: return 12
	case token.Star, token.Slash, token.Rem: return 13
	default:
		return -1
	}
}

func (p *Parser) parseExpr() *ast.Node { return p.parseAssignmentExpr() }

func (p *Parser) parseAssignmentExpr() *ast.Node {
	// Try to detect multi-assignment pattern: lhs1, lhs2, ... = rhs1, rhs2, ...
	startPos := p.pos
	startCurrent := p.current
	
	// Parse first expression
	left := p.parseTernaryExpr()
	
	// Check if this could be a multi-assignment (comma followed by more expressions then equals)
	if p.check(token.Comma) {
		var lhsList []*ast.Node
		lhsList = append(lhsList, left)
		
		// Parse comma-separated lvalues
		for p.match(token.Comma) {
			expr := p.parseTernaryExpr()
			lhsList = append(lhsList, expr)
		}
		
		// Check if we have an assignment operator
		if op := p.current.Type; op >= token.Eq && op <= token.EqShr {
			// Validate all lhs expressions are lvalues
			for _, lhs := range lhsList {
				if !isLValue(lhs) {
					util.Error(p.current, "Invalid target for assignment")
				}
			}
			
			tok := p.current
			p.advance()
			
			// Parse comma-separated rvalues
			var rhsList []*ast.Node
			for {
				rhs := p.parseAssignmentExpr()
				rhsList = append(rhsList, rhs)
				if !p.match(token.Comma) {
					break
				}
			}
			
			// Check that number of lhs and rhs match
			if len(lhsList) != len(rhsList) {
				util.Error(tok, "Mismatched number of variables and values in assignment (%d vs %d)", len(lhsList), len(rhsList))
			}
			
			return ast.NewMultiAssign(tok, op, lhsList, rhsList)
		}
		
		// Not a multi-assignment, backtrack and treat as comma expression
		p.pos = startPos
		p.current = startCurrent
		left = p.parseTernaryExpr()
	}
	
	// Regular single assignment
	if op := p.current.Type; op >= token.Eq && op <= token.EqShr {
		if !isLValue(left) {
			util.Error(p.current, "Invalid target for assignment")
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
		p.expect(token.Colon, "Expected ':' for ternary operator")
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
				util.Error(opToken, "Address-of operator '&' requires an l-value")
			}
			return ast.NewAddressOf(tok, operand)
		}
		if (op == token.Inc || op == token.Dec) && !isLValue(operand) {
			util.Error(opToken, "Prefix '++' or '--' requires an l-value")
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
			p.expect(token.RParen, "Expected ')' after function arguments")
			expr = ast.NewFuncCall(tok, expr, args)
		} else if p.match(token.LBracket) {
			index := p.parseExpr()
			p.expect(token.RBracket, "Expected ']' after array index")
			expr = ast.NewSubscript(tok, expr, index)
		} else if p.isTypedPass && p.match(token.Dot) {
			p.expect(token.Ident, "Expected member name after '.'")
			member := ast.NewIdent(p.previous, p.previous.Value)
			expr = ast.NewMemberAccess(tok, expr, member)
		} else if p.match(token.Inc, token.Dec) {
			if !isLValue(expr) {
				util.Error(p.previous, "Postfix '++' or '--' requires an l-value")
			}
			expr = ast.NewPostfixOp(p.previous, p.previous.Type, expr)
		} else {
			break
		}
	}
	return expr
}

func (p *Parser) parseStructLiteral(typeNode *ast.Node) *ast.Node {
	startTok := p.current
	p.expect(token.LBrace, "Expected '{' for struct literal")

	var values []*ast.Node
	var names []*ast.Node
	hasNames, hasPositional := false, false

	for !p.check(token.RBrace) && !p.check(token.EOF) {
		if p.check(token.Ident) && p.peek().Type == token.Colon {
			hasNames = true
			if hasPositional {
				util.Error(p.current, "Cannot mix named and positional fields in struct literal")
			}
			p.expect(token.Ident, "Expected field name")
			names = append(names, ast.NewIdent(p.previous, p.previous.Value))
			p.expect(token.Colon, "Expected ':' after field name")
			values = append(values, p.parseAssignmentExpr())
		} else {
			hasPositional = true
			if hasNames {
				util.Error(p.current, "Cannot mix named and positional fields in struct literal")
			}
			names = append(names, nil)
			values = append(values, p.parseAssignmentExpr())
		}

		if !p.match(token.Comma) {
			break
		}
	}

	p.expect(token.RBrace, "Expected '}' to close struct literal")

	if hasPositional && !hasNames {
		names = nil
	}

	return ast.NewStructLiteral(startTok, typeNode, values, names)
}

func (p *Parser) parseArrayLiteral(startTok token.Token, elemType *ast.BxType) *ast.Node {
	p.expect(token.LBrace, "Expected '{' for array literal")

	var values []*ast.Node
	for !p.check(token.RBrace) && !p.check(token.EOF) {
		values = append(values, p.parseAssignmentExpr())
		if !p.match(token.Comma) {
			break
		}
	}

	p.expect(token.RBrace, "Expected '}' to close array literal")

	return ast.NewArrayLiteral(startTok, elemType, values)
}

func (p *Parser) parsePrimaryExpr() *ast.Node {
	tok := p.current
	if p.match(token.Number) {
		valStr := p.previous.Value
		val, err := strconv.ParseInt(valStr, 0, 64)
		if err != nil {
			uval, uerr := strconv.ParseUint(valStr, 0, 64)
			if uerr != nil {
				util.Error(tok, "Invalid integer literal: %s", valStr)
			}
			val = int64(uval)
		}
		return ast.NewNumber(tok, val)
	}
	if p.match(token.FloatNumber) {
		val, _ := strconv.ParseFloat(p.previous.Value, 64)
		return ast.NewFloatNumber(tok, val)
	}
	if p.match(token.String) {
		return ast.NewString(tok, p.previous.Value)
	}
	if p.match(token.Nil) {
		return ast.NewNil(tok)
	}
	if p.match(token.Null) {
		util.Warn(p.cfg, config.WarnExtra, tok, "Use of 'null' is discouraged, prefer 'nil' for idiomatic Bx code")
		return ast.NewNil(tok)
	}
	if p.match(token.Ident) {
		identTok := p.previous
		if p.isTypedPass && p.isTypeName(identTok.Value) && p.check(token.LBrace) {
			typeNode := ast.NewIdent(identTok, identTok.Value)
			return p.parseStructLiteral(typeNode)
		}
		return ast.NewIdent(tok, p.previous.Value)
	}
	if p.isTypedPass && p.isBuiltinType(p.current) {
		tokType := p.current.Type
		p.advance()
		if keyword, ok := token.TypeStrings[tokType]; ok {
			return ast.NewIdent(tok, keyword)
		}
	}
	if p.match(token.TypeKeyword) {
		util.Warn(p.cfg, config.WarnExtra, p.previous, "Using keyword 'type' as an identifier")
		return ast.NewIdent(tok, "type")
	}
	if p.match(token.TypeOf) {
		p.expect(token.LParen, "Expected '(' after 'typeof'")
		expr := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after typeof expression")
		return ast.NewTypeOf(tok, expr)
	}
	if p.match(token.LParen) {
		// Only allow C-style casts for pointer types, not simple scalar types
		if p.isTypedPass && p.isPointerCastAhead() {
			castType := p.parseType()
			p.expect(token.RParen, "Expected ')' after type in cast")
			exprToCast := p.parseUnaryExpr()
			return ast.NewTypeCast(tok, exprToCast, castType)
		}
		expr := p.parseExpr()
		p.expect(token.RParen, "Expected ')' after expression")
		return expr
	}
	if p.match(token.Auto) {
		if p.check(token.LBracket) {
			allocTok := p.previous
			p.advance()
			sizeExpr := p.parseExpr()
			p.expect(token.RBracket, "Expected ']' after auto allocation size")
			return ast.NewAutoAlloc(allocTok, sizeExpr)
		}
		p.pos--
		p.current = p.previous
	}

	// Handle array literals: []type{ ... }
	if p.isTypedPass && p.match(token.LBracket) {
		arrayTok := p.previous
		p.expect(token.RBracket, "Expected ']' for array literal")
		if p.isBuiltinType(p.current) || p.isTypeName(p.current.Value) || p.check(token.Star) {
			elemType := p.parseType()
			if elemType != nil && p.check(token.LBrace) {
				return p.parseArrayLiteral(arrayTok, elemType)
			}
		}
		// Not an array literal, backtrack
		util.Error(arrayTok, "Expected type after '[]' for array literal")
		return nil
	}

	if !p.check(token.EOF) && !p.check(token.RBrace) && !p.check(token.Semi) {
		util.Error(tok, "Expected an expression")
	}
	return nil
}

// typeFromName converts a type name string to a BxType
func (p *Parser) typeFromName(name string) *ast.BxType {
	// Check if it's a built-in type keyword
	if tokType, isKeyword := token.KeywordMap[name]; isKeyword {
		if tokType == token.Void {
			return ast.TypeVoid
		} else if tokType == token.StringKeyword {
			return ast.TypeString
		} else if tokType >= token.Float && tokType <= token.Float64 {
			return &ast.BxType{Kind: ast.TYPE_FLOAT, Name: name}
		} else if tokType >= token.Byte && tokType <= token.Any {
			return &ast.BxType{Kind: ast.TYPE_PRIMITIVE, Name: name}
		}
	}

	// Check if it's a user-defined type name
	if p.isTypeName(name) {
		return &ast.BxType{Kind: ast.TYPE_PRIMITIVE, Name: name}
	}

	return nil
}

// parseTypedExtrnDecl parses typed external function declarations like "tm extrn localtime;"
func (p *Parser) parseTypedExtrnDecl(typeTok token.Token, returnType *ast.BxType) *ast.Node {
	var names []*ast.Node
	for {
		p.expect(token.Ident, "Expected identifier in typed 'extrn' declaration")
		names = append(names, ast.NewIdent(p.previous, p.previous.Value))
		if !p.match(token.Comma) {
			break
		}
	}
	p.expect(token.Semi, "Expected ';' after typed 'extrn' declaration")
	return ast.NewExtrnDecl(typeTok, names, returnType)
}
