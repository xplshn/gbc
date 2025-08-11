package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/util"
)

type symbolType int

const (
	symVar symbolType = iota
	symFunc
	symLabel
	symType
	symExtrn
)

type symbol struct {
	Name        string
	Type        symbolType
	BxType      *ast.BxType
	QbeName     string
	IsVector    bool
	IsByteArray bool
	StackOffset int64
	Next        *symbol
	Node        *ast.Node
}

type scope struct {
	Symbols *symbol
	Parent  *scope
}

type stringEntry struct {
	Value string
	Label string
}

type autoVarInfo struct {
	Node *ast.Node
	Size int64
}

type Context struct {
	out               strings.Builder
	asmOut            strings.Builder
	strings           []stringEntry
	tempCount         int
	labelCount        int
	currentScope      *scope
	currentFunc       *ast.FuncDeclNode
	currentFuncFrame  string
	currentBlockLabel string
	breakLabel        string
	continueLabel     string
	wordType          string
	wordSize          int
	stackAlign        int
	isTypedPass       bool
	cfg               *config.Config
}

func NewContext(cfg *config.Config) *Context {
	return &Context{
		currentScope: newScope(nil),
		wordSize:     cfg.WordSize,
		wordType:     cfg.WordType,
		stackAlign:   cfg.StackAlignment,
		isTypedPass:  cfg.IsFeatureEnabled(config.FeatTyped),
		cfg:          cfg,
	}
}

func newScope(parent *scope) *scope {
	return &scope{Parent: parent}
}

func (ctx *Context) enterScope() {
	ctx.currentScope = newScope(ctx.currentScope)
}

func (ctx *Context) exitScope() {
	if ctx.currentScope.Parent != nil {
		ctx.currentScope = ctx.currentScope.Parent
	}
}

func (ctx *Context) findSymbol(name string) *symbol {
	for s := ctx.currentScope; s != nil; s = s.Parent {
		for sym := s.Symbols; sym != nil; sym = sym.Next {
			if sym.Name == name {
				return sym
			}
		}
	}
	return nil
}

func (ctx *Context) findSymbolInCurrentScope(name string) *symbol {
	for sym := ctx.currentScope.Symbols; sym != nil; sym = sym.Next {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

func (ctx *Context) addSymbol(name string, symType symbolType, bxType *ast.BxType, isVector bool, node *ast.Node) *symbol {
	var qbeName string
	switch symType {
	case symVar:
		if ctx.currentScope.Parent == nil {
			qbeName = "$" + name
		} else {
			qbeName = fmt.Sprintf("%%.%s_%d", name, ctx.tempCount)
			ctx.tempCount++
		}
	case symFunc, symExtrn:
		qbeName = "$" + name
	case symLabel:
		qbeName = "@" + name
	case symType:
		qbeName = ":" + name
	}

	sym := &symbol{
		Name:     name,
		Type:     symType,
		BxType:   bxType,
		QbeName:  qbeName,
		IsVector: isVector,
		Next:     ctx.currentScope.Symbols,
		Node:     node,
	}
	ctx.currentScope.Symbols = sym
	return sym
}

func (ctx *Context) newTemp() string {
	name := fmt.Sprintf("%%.t%d", ctx.tempCount)
	ctx.tempCount++
	return name
}

func (ctx *Context) newLabel() string {
	name := fmt.Sprintf("@L%d", ctx.labelCount)
	ctx.labelCount++
	return name
}

func (ctx *Context) writeLabel(label string) {
	ctx.out.WriteString(label + "\n")
	ctx.currentBlockLabel = label
}

func (ctx *Context) addString(value string) string {
	for _, entry := range ctx.strings {
		if entry.Value == value {
			return "$" + entry.Label
		}
	}
	label := fmt.Sprintf("str%d", len(ctx.strings))
	ctx.strings = append(ctx.strings, stringEntry{Value: value, Label: label})
	return "$" + label
}

func (ctx *Context) getSizeof(typ *ast.BxType) int64 {
	if typ == nil || typ.Kind == ast.TYPE_UNTYPED {
		return int64(ctx.wordSize)
	}
	switch typ.Kind {
	case ast.TYPE_VOID:
		return 0
	case ast.TYPE_POINTER:
		return int64(ctx.wordSize)
	case ast.TYPE_ARRAY:
		elemSize := ctx.getSizeof(typ.Base)
		var arrayLen int64 = 1
		if typ.ArraySize != nil {
			folded := ast.FoldConstants(typ.ArraySize)
			if folded.Type == ast.Number {
				arrayLen = folded.Data.(ast.NumberNode).Value
			} else if folded.Type == ast.Ident {
				identName := folded.Data.(ast.IdentNode).Name
				sym := ctx.findSymbol(identName)
				if sym == nil || sym.Node == nil || sym.Node.Type != ast.VarDecl {
					util.Error(typ.ArraySize.Tok, "Array size '%s' is not a constant variable.", identName)
					return elemSize
				}
				decl := sym.Node.Data.(ast.VarDeclNode)
				if len(decl.InitList) != 1 {
					util.Error(typ.ArraySize.Tok, "Array size '%s' is not a simple constant.", identName)
					return elemSize
				}
				constVal := ast.FoldConstants(decl.InitList[0])
				if constVal.Type != ast.Number {
					util.Error(typ.ArraySize.Tok, "Array size '%s' is not a constant number.", identName)
					return elemSize
				}
				arrayLen = constVal.Data.(ast.NumberNode).Value
			} else {
				util.Error(typ.ArraySize.Tok, "Array size must be a constant expression.")
			}
		}
		return elemSize * arrayLen
	case ast.TYPE_PRIMITIVE:
		switch typ.Name {
		case "int", "uint", "string":
			return int64(ctx.wordSize)
		case "int64", "uint64":
			return 8
		case "int32", "uint32":
			return 4
		case "int16", "uint16":
			return 2
		case "byte", "bool", "int8", "uint8":
			return 1
		default:
			if sym := ctx.findSymbol(typ.Name); sym != nil {
				return ctx.getSizeof(sym.BxType)
			}
			return int64(ctx.wordSize)
		}
	case ast.TYPE_FLOAT:
		switch typ.Name {
		case "float", "float32":
			return 4
		case "float64":
			return 8
		default:
			return 4
		}
	case ast.TYPE_STRUCT:
		var totalSize int64
		for _, field := range typ.Fields {
			totalSize += ctx.getSizeof(field.Data.(ast.VarDeclNode).Type)
		}
		return totalSize
	}
	return int64(ctx.wordSize)
}

func (ctx *Context) getQbeType(typ *ast.BxType) string {
	if typ == nil || typ.Kind == ast.TYPE_UNTYPED {
		return ctx.wordType
	}
	switch typ.Kind {
	case ast.TYPE_VOID:
		return ""
	case ast.TYPE_POINTER, ast.TYPE_ARRAY:
		return ctx.wordType
	case ast.TYPE_FLOAT:
		switch typ.Name {
		case "float", "float32":
			return "s"
		case "float64":
			return "d"
		default:
			return "s"
		}
	case ast.TYPE_PRIMITIVE:
		switch typ.Name {
		case "int", "uint", "string":
			return ctx.wordType
		case "int64", "uint64":
			return "l"
		case "int32", "uint32":
			return "w"
		case "int16", "uint16":
			return "h"
		case "byte", "bool", "int8", "uint8":
			return "b"
		default:
			if sym := ctx.findSymbol(typ.Name); sym != nil {
				return ctx.getQbeType(sym.BxType)
			}
			return ctx.wordType
		}
	case ast.TYPE_STRUCT:
		return ctx.wordType
	}
	return ctx.wordType
}

func getOpStr(op token.Type) string {
	switch op {
	case token.Plus, token.PlusEq, token.EqPlus:
		return "add"
	case token.Minus, token.MinusEq, token.EqMinus:
		return "sub"
	case token.Star, token.StarEq, token.EqStar:
		return "mul"
	case token.Slash, token.SlashEq, token.EqSlash:
		return "div"
	case token.Rem, token.RemEq, token.EqRem:
		return "rem"
	case token.And, token.AndEq, token.EqAnd:
		return "and"
	case token.Or, token.OrEq, token.EqOr:
		return "or"
	case token.Xor, token.XorEq, token.EqXor:
		return "xor"
	case token.Shl, token.ShlEq, token.EqShl:
		return "shl"
	case token.Shr, token.ShrEq, token.EqShr:
		return "shr"
	default:
		return ""
	}
}

func (ctx *Context) getCmpOpStr(op token.Type, operandType string) string {
	isFloat := operandType == "s" || operandType == "d"
	isSigned := true

	var cmpType string

	switch op {
	case token.EqEq:
		cmpType = "eq"
	case token.Neq:
		cmpType = "ne"
	case token.Lt:
		if isFloat {
			cmpType = "lt"
		} else if isSigned {
			cmpType = "slt"
		} else {
			cmpType = "ult"
		}
	case token.Gt:
		if isFloat {
			cmpType = "gt"
		} else if isSigned {
			cmpType = "sgt"
		} else {
			cmpType = "ugt"
		}
	case token.Lte:
		if isFloat {
			cmpType = "le"
		} else if isSigned {
			cmpType = "sle"
		} else {
			cmpType = "ule"
		}
	case token.Gte:
		if isFloat {
			cmpType = "ge"
		} else if isSigned {
			cmpType = "sge"
		} else {
			cmpType = "uge"
		}
	default:
		return ""
	}

	return "c" + cmpType + operandType
}

func (ctx *Context) getLoadInstruction(typ *ast.BxType) (inst, resType string) {
	qbeType := ctx.getQbeType(typ)
	switch qbeType {
	case "b":
		return "loadub", ctx.wordType
	case "h":
		return "loaduh", ctx.wordType
	case "w":
		return "loadw", ctx.wordType
	case "l":
		return "loadl", "l"
	case "s":
		return "loads", "s"
	case "d":
		return "loadd", "d"
	default:
		return "load" + qbeType, qbeType
	}
}

func (ctx *Context) getStoreInstruction(typ *ast.BxType) string {
	qbeType := ctx.getQbeType(typ)
	switch qbeType {
	case "b":
		return "storeb"
	case "h":
		return "storeh"
	case "w":
		return "storew"
	case "l":
		return "storel"
	case "s":
		return "stores"
	case "d":
		return "stored"
	default:
		return "store" + qbeType
	}
}

func (ctx *Context) getAllocInstruction() string {
	switch ctx.stackAlign {
	case 16:
		return "alloc16"
	case 8:
		return "alloc8"
	default:
		return "alloc4"
	}
}

func (ctx *Context) genLoad(addr string, typ *ast.BxType) string {
	res := ctx.newTemp()
	loadInst, resType := ctx.getLoadInstruction(typ)
	ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", res, resType, loadInst, addr))
	return res
}

func (ctx *Context) genStore(addr, value string, typ *ast.BxType) {
	storeInst := ctx.getStoreInstruction(typ)
	ctx.out.WriteString(fmt.Sprintf("\t%s %s, %s\n", storeInst, value, addr))
}

func (ctx *Context) Generate(root *ast.Node) (qbeIR, inlineAsm string) {
	ctx.collectGlobals(root)
	ctx.collectStrings(root)
	if !ctx.isTypedPass {
		ctx.findByteArrays(root)
	}
	ctx.codegenStmt(root)

	if len(ctx.strings) > 0 {
		ctx.out.WriteString("\n# --- Data Section ---\n")
		for _, s := range ctx.strings {
			escaped := strconv.Quote(s.Value)
			ctx.out.WriteString(fmt.Sprintf("data $%s = { b %s, b 0 }\n", s.Label, escaped))
		}
	}

	return ctx.out.String(), ctx.asmOut.String()
}

func (ctx *Context) collectGlobals(node *ast.Node) {
	if node == nil {
		return
	}

	switch node.Type {
	case ast.Block:
		for _, stmt := range node.Data.(ast.BlockNode).Stmts {
			ctx.collectGlobals(stmt)
		}
	case ast.VarDecl:
		if ctx.currentScope.Parent == nil {
			d := node.Data.(ast.VarDeclNode)
			existingSym := ctx.findSymbolInCurrentScope(d.Name)
			if existingSym == nil {
				ctx.addSymbol(d.Name, symVar, d.Type, d.IsVector, node)
			} else if existingSym.Type == symFunc || existingSym.Type == symExtrn {
				util.Warn(ctx.cfg, config.WarnExtra, node.Tok, "Definition of '%s' overrides previous external declaration.", d.Name)
				existingSym.Type = symVar
				existingSym.IsVector = d.IsVector
				existingSym.BxType = d.Type
				existingSym.Node = node
			} else if existingSym.Type == symVar {
				util.Warn(ctx.cfg, config.WarnExtra, node.Tok, "Redefinition of variable '%s'.", d.Name)
				existingSym.IsVector = d.IsVector
				existingSym.BxType = d.Type
				existingSym.Node = node
			}
		}
	case ast.MultiVarDecl:
		if ctx.currentScope.Parent == nil {
			for _, decl := range node.Data.(ast.MultiVarDeclNode).Decls {
				ctx.collectGlobals(decl)
			}
		}
	case ast.FuncDecl:
		d := node.Data.(ast.FuncDeclNode)
		existingSym := ctx.findSymbolInCurrentScope(d.Name)
		if existingSym == nil {
			ctx.addSymbol(d.Name, symFunc, d.ReturnType, false, node)
		} else if existingSym.Type != symFunc {
			util.Warn(ctx.cfg, config.WarnExtra, node.Tok, "Redefinition of '%s' as a function.", d.Name)
			existingSym.Type = symFunc
			existingSym.IsVector = false
			existingSym.BxType = d.ReturnType
			existingSym.Node = node
		}
	case ast.ExtrnDecl:
		d := node.Data.(ast.ExtrnDeclNode)
		for _, nameNode := range d.Names {
			name := nameNode.Data.(ast.IdentNode).Name
			if ctx.findSymbolInCurrentScope(name) == nil {
				ctx.addSymbol(name, symExtrn, ast.TypeUntyped, false, nameNode)
			}
		}
	case ast.TypeDecl:
		d := node.Data.(ast.TypeDeclNode)
		if ctx.findSymbolInCurrentScope(d.Name) == nil {
			ctx.addSymbol(d.Name, symType, d.Type, false, node)
		}
	}
}

func (ctx *Context) findByteArrays(root *ast.Node) {
	for {
		changedInPass := false
		var astWalker func(*ast.Node)
		astWalker = func(n *ast.Node) {
			if n == nil {
				return
			}

			switch n.Type {
			case ast.VarDecl:
				d := n.Data.(ast.VarDeclNode)
				if d.IsVector && len(d.InitList) == 1 && d.InitList[0].Type == ast.String {
					sym := ctx.findSymbol(d.Name)
					if sym != nil && !sym.IsByteArray {
						sym.IsByteArray = true
						changedInPass = true
					}
				}

			case ast.Assign:
				d := n.Data.(ast.AssignNode)
				if d.Lhs.Type == ast.Ident {
					lhsSym := ctx.findSymbol(d.Lhs.Data.(ast.IdentNode).Name)
					if lhsSym != nil && !lhsSym.IsByteArray {
						rhsIsByteArray := false
						rhsNode := d.Rhs
						switch rhsNode.Type {
						case ast.String:
							rhsIsByteArray = true
						case ast.Ident:
							rhsSym := ctx.findSymbol(rhsNode.Data.(ast.IdentNode).Name)
							if rhsSym != nil && rhsSym.IsByteArray {
								rhsIsByteArray = true
							}
						}

						if rhsIsByteArray {
							lhsSym.IsByteArray = true
							changedInPass = true
						}
					}
				}
			}

			switch d := n.Data.(type) {
			case ast.AssignNode:
				astWalker(d.Lhs)
				astWalker(d.Rhs)
			case ast.BinaryOpNode:
				astWalker(d.Left)
				astWalker(d.Right)
			case ast.UnaryOpNode:
				astWalker(d.Expr)
			case ast.PostfixOpNode:
				astWalker(d.Expr)
			case ast.IndirectionNode:
				astWalker(d.Expr)
			case ast.AddressOfNode:
				astWalker(d.LValue)
			case ast.TernaryNode:
				astWalker(d.Cond)
				astWalker(d.ThenExpr)
				astWalker(d.ElseExpr)
			case ast.SubscriptNode:
				astWalker(d.Array)
				astWalker(d.Index)
			case ast.FuncCallNode:
				astWalker(d.FuncExpr)
				for _, arg := range d.Args {
					astWalker(arg)
				}
			case ast.FuncDeclNode:
				astWalker(d.Body)
			case ast.VarDeclNode:
				for _, init := range d.InitList {
					astWalker(init)
				}
				astWalker(d.SizeExpr)
			case ast.MultiVarDeclNode:
				for _, decl := range d.Decls {
					astWalker(decl)
				}
			case ast.IfNode:
				astWalker(d.Cond)
				astWalker(d.ThenBody)
				astWalker(d.ElseBody)
			case ast.WhileNode:
				astWalker(d.Cond)
				astWalker(d.Body)
			case ast.ReturnNode:
				astWalker(d.Expr)
			case ast.BlockNode:
				for _, s := range d.Stmts {
					astWalker(s)
				}
			case ast.SwitchNode:
				astWalker(d.Expr)
				astWalker(d.Body)
			case ast.CaseNode:
				astWalker(d.Value)
				astWalker(d.Body)
			case ast.DefaultNode:
				astWalker(d.Body)
			case ast.LabelNode:
				astWalker(d.Stmt)
			}
		}

		astWalker(root)
		if !changedInPass {
			break
		}
	}
}

func (ctx *Context) collectStrings(root *ast.Node) {
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		if n == nil {
			return
		}
		if n.Type == ast.String {
			ctx.addString(n.Data.(ast.StringNode).Value)
		}
		switch d := n.Data.(type) {
		case ast.AssignNode:
			walk(d.Lhs)
			walk(d.Rhs)
		case ast.BinaryOpNode:
			walk(d.Left)
			walk(d.Right)
		case ast.UnaryOpNode:
			walk(d.Expr)
		case ast.PostfixOpNode:
			walk(d.Expr)
		case ast.IndirectionNode:
			walk(d.Expr)
		case ast.AddressOfNode:
			walk(d.LValue)
		case ast.TernaryNode:
			walk(d.Cond)
			walk(d.ThenExpr)
			walk(d.ElseExpr)
		case ast.SubscriptNode:
			walk(d.Array)
			walk(d.Index)
		case ast.FuncCallNode:
			walk(d.FuncExpr)
			for _, arg := range d.Args {
				walk(arg)
			}
		case ast.FuncDeclNode:
			walk(d.Body)
		case ast.VarDeclNode:
			for _, init := range d.InitList {
				walk(init)
			}
			walk(d.SizeExpr)
		case ast.MultiVarDeclNode:
			for _, decl := range d.Decls {
				walk(decl)
			}
		case ast.IfNode:
			walk(d.Cond)
			walk(d.ThenBody)
			walk(d.ElseBody)
		case ast.WhileNode:
			walk(d.Cond)
			walk(d.Body)
		case ast.ReturnNode:
			walk(d.Expr)
		case ast.BlockNode:
			for _, s := range d.Stmts {
				walk(s)
			}
		case ast.SwitchNode:
			walk(d.Expr)
			walk(d.Body)
		case ast.CaseNode:
			walk(d.Value)
			walk(d.Body)
		case ast.DefaultNode:
			walk(d.Body)
		case ast.LabelNode:
			walk(d.Stmt)
		}
	}
	walk(root)
}

func (ctx *Context) codegenLvalue(node *ast.Node) string {
	if node == nil {
		util.Error(token.Token{}, "Internal error: null l-value node in codegen")
		return ""
	}
	switch node.Type {
	case ast.Ident:
		name := node.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			util.Warn(ctx.cfg, config.WarnImplicitDecl, node.Tok, "Implicit declaration of variable '%s'", name)
			sym = ctx.addSymbol(name, symVar, ast.TypeUntyped, false, node)
		}

		if sym.Type == symFunc {
			util.Error(node.Tok, "Cannot assign to function '%s'.", name)
			return ""
		}
		if sym.BxType != nil && sym.BxType.Kind == ast.TYPE_ARRAY {
			return sym.QbeName
		}
		if sym.IsVector && sym.Node != nil && sym.Node.Type == ast.VarDecl {
			d := sym.Node.Data.(ast.VarDeclNode)
			if !d.IsBracketed && len(d.InitList) <= 1 && d.Type == nil {
				util.Error(node.Tok, "Cannot assign to '%s', it is a constant.", name)
				return ""
			}
		}

		return sym.QbeName

	case ast.Indirection:
		res, _, _ := ctx.codegenExpr(node.Data.(ast.IndirectionNode).Expr)
		return res

	case ast.Subscript:
		d := node.Data.(ast.SubscriptNode)
		arrayBasePtr, _, _ := ctx.codegenExpr(d.Array)
		indexVal, _, _ := ctx.codegenExpr(d.Index)

		var scale int64 = int64(ctx.wordSize)
		if d.Array.Typ != nil {
			if d.Array.Typ.Kind == ast.TYPE_POINTER || d.Array.Typ.Kind == ast.TYPE_ARRAY {
				if d.Array.Typ.Base != nil {
					scale = ctx.getSizeof(d.Array.Typ.Base)
				}
			}
		} else {
			if !ctx.isTypedPass && d.Array.Type == ast.Ident {
				sym := ctx.findSymbol(d.Array.Data.(ast.IdentNode).Name)
				if sym != nil && sym.IsByteArray {
					scale = 1
				}
			}
		}

		var scaledIndex string
		if scale > 1 {
			scaledIndex = ctx.newTemp()
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s mul %s, %d\n", scaledIndex, ctx.wordType, indexVal, scale))
		} else {
			scaledIndex = indexVal
		}

		resultAddr := ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %s\n", resultAddr, ctx.wordType, arrayBasePtr, scaledIndex))
		return resultAddr

	default:
		util.Error(node.Tok, "Expression is not a valid l-value.")
		return ""
	}
}

func (ctx *Context) codegenLogicalCond(node *ast.Node, trueL, falseL string) {
	if node.Type == ast.BinaryOp {
		d := node.Data.(ast.BinaryOpNode)
		if d.Op == token.OrOr {
			newFalseL := ctx.newLabel()
			ctx.codegenLogicalCond(d.Left, trueL, newFalseL)
			ctx.writeLabel(newFalseL)
			ctx.codegenLogicalCond(d.Right, trueL, falseL)
			return
		}
		if d.Op == token.AndAnd {
			newTrueL := ctx.newLabel()
			ctx.codegenLogicalCond(d.Left, newTrueL, falseL)
			ctx.writeLabel(newTrueL)
			ctx.codegenLogicalCond(d.Right, trueL, falseL)
			return
		}
	}

	condVal, _, _ := ctx.codegenExpr(node)
	ctx.out.WriteString(fmt.Sprintf("\tjnz %s, %s, %s\n", condVal, trueL, falseL))
}

func (ctx *Context) codegenExpr(node *ast.Node) (result, predLabel string, terminates bool) {
	if node == nil {
		return "0", ctx.currentBlockLabel, false
	}
	startBlockLabel := ctx.currentBlockLabel

	switch node.Type {
	case ast.Number:
		return fmt.Sprintf("%d", node.Data.(ast.NumberNode).Value), startBlockLabel, false
	case ast.String:
		return ctx.addString(node.Data.(ast.StringNode).Value), startBlockLabel, false

	case ast.Ident:
		name := node.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)

		if sym == nil {
			util.Warn(ctx.cfg, config.WarnImplicitDecl, node.Tok, "Implicit declaration of function '%s'", name)
			sym = ctx.addSymbol(name, symFunc, ast.TypeUntyped, false, node)
			return sym.QbeName, startBlockLabel, false
		}

		if sym.Type == symFunc || sym.Type == symExtrn {
			return sym.QbeName, startBlockLabel, false
		}

		isArr := sym.IsVector || (sym.BxType != nil && sym.BxType.Kind == ast.TYPE_ARRAY)
		isPtr := sym.BxType != nil && sym.BxType.Kind == ast.TYPE_POINTER

		if isArr {
			isParam := false
			if ctx.currentFunc != nil {
				for _, p := range ctx.currentFunc.Params {
					var pName string
					if p.Type == ast.Ident {
						pName = p.Data.(ast.IdentNode).Name
					} else {
						pName = p.Data.(ast.VarDeclNode).Name
					}
					if pName == name {
						isParam = true
						break
					}
				}
			}
			if isParam {
				return ctx.genLoad(sym.QbeName, sym.BxType), startBlockLabel, false
			}
			return sym.QbeName, startBlockLabel, false
		}

		if isPtr {
			return ctx.genLoad(sym.QbeName, sym.BxType), startBlockLabel, false
		}

		return ctx.genLoad(sym.QbeName, sym.BxType), startBlockLabel, false

	case ast.Assign:
		d := node.Data.(ast.AssignNode)
		lvalAddr := ctx.codegenLvalue(node.Data.(ast.AssignNode).Lhs)
		var rval string

		if d.Op == token.Eq {
			rval, _, _ = ctx.codegenExpr(d.Rhs)
		} else {
			currentLvalVal := ctx.genLoad(lvalAddr, d.Lhs.Typ)
			rhsVal, _, _ := ctx.codegenExpr(d.Rhs)
			opStr := getOpStr(d.Op)
			rval = ctx.newTemp()
			valType := ctx.getQbeType(d.Lhs.Typ)
			if valType == "b" || valType == "h" {
				valType = ctx.wordType
			}
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", rval, valType, opStr, currentLvalVal, rhsVal))
		}

		ctx.genStore(lvalAddr, rval, d.Lhs.Typ)
		return rval, startBlockLabel, false

	case ast.BinaryOp:
		d := node.Data.(ast.BinaryOpNode)
		if d.Op == token.OrOr || d.Op == token.AndAnd {
			res := ctx.newTemp()
			trueL, falseL, endL := ctx.newLabel(), ctx.newLabel(), ctx.newLabel()
			ctx.codegenLogicalCond(node, trueL, falseL)
			ctx.writeLabel(trueL)
			truePred := ctx.currentBlockLabel
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endL))
			ctx.writeLabel(falseL)
			falsePred := ctx.currentBlockLabel
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endL))
			ctx.writeLabel(endL)
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s phi %s 1, %s 0\n", res, ctx.wordType, truePred, falsePred))
			return res, endL, false
		}

		l, _, _ := ctx.codegenExpr(d.Left)
		r, _, _ := ctx.codegenExpr(d.Right)
		res := ctx.newTemp()

		lType, rType := ctx.getQbeType(d.Left.Typ), ctx.getQbeType(d.Right.Typ)
		opType := ctx.wordType
		if lType == "s" || lType == "d" {
			opType = lType
		} else if rType == "s" || rType == "d" {
			opType = rType
		}

		if opStr := getOpStr(d.Op); opStr != "" {
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, opType, opStr, l, r))
		} else if cmpOpStr := ctx.getCmpOpStr(d.Op, opType); cmpOpStr != "" {
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, ctx.wordType, cmpOpStr, l, r))
		} else {
			util.Error(node.Tok, "Invalid binary operator token %d", d.Op)
		}
		return res, startBlockLabel, false

	case ast.UnaryOp:
		d := node.Data.(ast.UnaryOpNode)
		res := ctx.newTemp()
		switch d.Op {
		case token.Minus:
			val, _, _ := ctx.codegenExpr(d.Expr)
			valType := ctx.getQbeType(d.Expr.Typ)
			if valType == "s" || valType == "d" {
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s neg %s\n", res, valType, val))
			} else {
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s sub 0, %s\n", res, valType, val))
			}
		case token.Plus:
			val, _, _ := ctx.codegenExpr(d.Expr)
			return val, startBlockLabel, false
		case token.Not:
			val, _, _ := ctx.codegenExpr(d.Expr)
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s ceq%s %s, 0\n", res, ctx.wordType, ctx.wordType, val))
		case token.Complement:
			val, _, _ := ctx.codegenExpr(d.Expr)
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s xor %s, -1\n", res, ctx.wordType, val))
		case token.Inc, token.Dec: // Prefix
			lvalAddr := ctx.codegenLvalue(d.Expr)
			op := map[token.Type]string{token.Inc: "add", token.Dec: "sub"}[d.Op]
			currentVal := ctx.genLoad(lvalAddr, d.Expr.Typ)
			valType := ctx.getQbeType(d.Expr.Typ)
			if valType == "b" || valType == "h" {
				valType = ctx.wordType
			}

			var oneConst string
			if valType == "s" || valType == "d" {
				oneConst = fmt.Sprintf("%s_1.0", valType)
			} else {
				oneConst = "1"
			}
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, valType, op, currentVal, oneConst))
			ctx.genStore(lvalAddr, res, d.Expr.Typ)
		default:
			util.Error(node.Tok, "Unsupported unary operator")
		}
		return res, startBlockLabel, false

	case ast.PostfixOp:
		d := node.Data.(ast.PostfixOpNode)
		lvalAddr := ctx.codegenLvalue(d.Expr)
		res := ctx.genLoad(lvalAddr, d.Expr.Typ) // Original value

		newVal := ctx.newTemp()
		op := map[token.Type]string{token.Inc: "add", token.Dec: "sub"}[d.Op]
		valType := ctx.getQbeType(d.Expr.Typ)
		if valType == "b" || valType == "h" {
			valType = ctx.wordType
		}

		var oneConst string
		if valType == "s" || valType == "d" {
			oneConst = fmt.Sprintf("%s_1.0", valType)
		} else {
			oneConst = "1"
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", newVal, valType, op, res, oneConst))
		ctx.genStore(lvalAddr, newVal, d.Expr.Typ)
		return res, startBlockLabel, false

	case ast.Indirection:
		exprNode := node.Data.(ast.IndirectionNode).Expr
		addr, _, _ := ctx.codegenExpr(exprNode)
		loadType := node.Typ
		if !ctx.isTypedPass {
			if exprNode.Type == ast.Ident {
				if sym := ctx.findSymbol(exprNode.Data.(ast.IdentNode).Name); sym != nil && sym.IsByteArray {
					loadType = ast.TypeByte
				}
			}
		}
		return ctx.genLoad(addr, loadType), startBlockLabel, false

	case ast.Subscript:
		addr := ctx.codegenLvalue(node)
		loadType := node.Typ
		if !ctx.isTypedPass && node.Data.(ast.SubscriptNode).Array.Type == ast.Ident {
			if sym := ctx.findSymbol(node.Data.(ast.SubscriptNode).Array.Data.(ast.IdentNode).Name); sym != nil && sym.IsByteArray {
				loadType = ast.TypeByte
			}
		}
		return ctx.genLoad(addr, loadType), startBlockLabel, false

	case ast.AddressOf:
		lvalNode := node.Data.(ast.AddressOfNode).LValue
		if lvalNode.Type == ast.Ident {
			name := lvalNode.Data.(ast.IdentNode).Name
			sym := ctx.findSymbol(name)
			if sym != nil {
				isTypedArray := sym.BxType != nil && sym.BxType.Kind == ast.TYPE_ARRAY
				if sym.Type == symFunc || isTypedArray {
					return sym.QbeName, startBlockLabel, false
				}
				if sym.IsVector {
					res, _, _ := ctx.codegenExpr(lvalNode)
					return res, startBlockLabel, false
				}
			}
		}
		return ctx.codegenLvalue(lvalNode), startBlockLabel, false

	case ast.FuncCall:
		d := node.Data.(ast.FuncCallNode)
		if d.FuncExpr.Type == ast.Ident {
			name := d.FuncExpr.Data.(ast.IdentNode).Name
			if sym := ctx.findSymbol(name); sym != nil && sym.Type == symVar && !sym.IsVector {
				util.Error(d.FuncExpr.Tok, "'%s' is a variable but is used as a function", name)
			}
		}

		argVals := make([]string, len(d.Args))
		for i := len(d.Args) - 1; i >= 0; i-- {
			argVals[i], _, _ = ctx.codegenExpr(d.Args[i])
		}
		funcVal, _, _ := ctx.codegenExpr(d.FuncExpr)

		isStmt := node.Parent != nil && node.Parent.Type == ast.Block

		res := "0"
		callStr := new(strings.Builder)

		var returnQbeType string
		var funcSym *symbol
		if d.FuncExpr.Type == ast.Ident {
			funcSym = ctx.findSymbol(d.FuncExpr.Data.(ast.IdentNode).Name)
		}
		if funcSym != nil && funcSym.BxType != nil && funcSym.BxType.Kind != ast.TYPE_UNTYPED {
			returnQbeType = ctx.getQbeType(funcSym.BxType)
		} else {
			returnQbeType = ctx.wordType
		}

		if !isStmt && returnQbeType != "" {
			res = ctx.newTemp()
			resultTempType := returnQbeType
			if resultTempType == "b" || resultTempType == "h" {
				resultTempType = "w"
			}
			fmt.Fprintf(callStr, "\t%s =%s call %s(", res, resultTempType, funcVal)
		} else {
			fmt.Fprintf(callStr, "\tcall %s(", funcVal)
		}

		for i, argVal := range argVals {
			argType := ctx.getQbeType(d.Args[i].Typ)
			switch argType {
			case "b":
				if d.Args[i].Typ != nil && d.Args[i].Typ.Name == "int8" {
					argType = "sb"
				} else {
					argType = "ub"
				}
			case "h":
				if d.Args[i].Typ != nil && d.Args[i].Typ.Name == "uint16" {
					argType = "uh"
				} else {
					argType = "sh"
				}
			}
			fmt.Fprintf(callStr, "%s %s", argType, argVal)
			if i < len(argVals)-1 {
				callStr.WriteString(", ")
			}
		}
		callStr.WriteString(")\n")
		ctx.out.WriteString(callStr.String())
		return res, startBlockLabel, false

	case ast.Ternary:
		d := node.Data.(ast.TernaryNode)
		thenLabel := ctx.newLabel()
		endLabel := ctx.newLabel()
		res := ctx.newTemp()
		elseLabel := ctx.newLabel()

		ctx.codegenLogicalCond(d.Cond, thenLabel, elseLabel)

		ctx.writeLabel(thenLabel)
		thenVal, thenPred, thenTerminates := ctx.codegenExpr(d.ThenExpr)
		if !thenTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		ctx.writeLabel(elseLabel)
		elseVal, elsePred, elseTerminates := ctx.codegenExpr(d.ElseExpr)
		if !elseTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		if !thenTerminates || !elseTerminates {
			ctx.writeLabel(endLabel)
			resType := ctx.getQbeType(node.Typ)

			phiArgs := new(strings.Builder)
			if !thenTerminates {
				fmt.Fprintf(phiArgs, "%s %s", thenPred, thenVal)
			}
			if !elseTerminates {
				if !thenTerminates {
					phiArgs.WriteString(", ")
				}
				fmt.Fprintf(phiArgs, "%s %s", elsePred, elseVal)
			}
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s phi %s\n", res, resType, phiArgs.String()))
			return res, endLabel, thenTerminates && elseTerminates
		}

		return "0", endLabel, true
	case ast.AutoAlloc:
		d := node.Data.(ast.AutoAllocNode)
		sizeVal, _, _ := ctx.codegenExpr(d.Size)
		res := ctx.newTemp()
		allocInst := "alloc" + strconv.Itoa(ctx.wordSize)
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", res, ctx.wordType, allocInst, sizeVal))
		return res, startBlockLabel, false
	}
	util.Error(node.Tok, "Internal error: unhandled expression type in codegen: %v", node.Type)
	return "", startBlockLabel, true
}

func (ctx *Context) codegenStmt(node *ast.Node) (terminates bool) {
	if node == nil {
		return false
	}
	switch node.Type {
	case ast.Block:
		isRealBlock := !node.Data.(ast.BlockNode).IsSynthetic
		if isRealBlock {
			ctx.enterScope()
		}
		var blockTerminates bool
		for _, stmt := range node.Data.(ast.BlockNode).Stmts {
			if blockTerminates {
				isLabel := stmt.Type == ast.Label || stmt.Type == ast.Case || stmt.Type == ast.Default
				if !isLabel {
					util.Warn(ctx.cfg, config.WarnUnreachableCode, stmt.Tok, "Unreachable code.")
					continue
				}
				blockTerminates = false
			}
			blockTerminates = ctx.codegenStmt(stmt)
		}
		if isRealBlock {
			ctx.exitScope()
		}
		return blockTerminates

	case ast.FuncDecl:
		ctx.codegenFuncDecl(node)
		return false

	case ast.VarDecl:
		ctx.codegenVarDecl(node)
		return false

	case ast.MultiVarDecl:
		for _, decl := range node.Data.(ast.MultiVarDeclNode).Decls {
			ctx.codegenVarDecl(decl)
		}
		return false

	case ast.TypeDecl, ast.Directive:
		return false

	case ast.ExtrnDecl:
		d := node.Data.(ast.ExtrnDeclNode)
		for _, nameNode := range d.Names {
			name := nameNode.Data.(ast.IdentNode).Name
			if ctx.findSymbol(name) == nil {
				ctx.addSymbol(name, symExtrn, ast.TypeUntyped, false, nameNode)
			}
		}
		return false

	case ast.Return:
		if node.Data.(ast.ReturnNode).Expr != nil {
			val, _, _ := ctx.codegenExpr(node.Data.(ast.ReturnNode).Expr)
			ctx.out.WriteString(fmt.Sprintf("\tret %s\n", val))
		} else {
			ctx.out.WriteString("\tret\n")
		}
		return true

	case ast.If:
		d := node.Data.(ast.IfNode)
		thenLabel := ctx.newLabel()
		endLabel := ctx.newLabel()
		elseLabel := endLabel
		if d.ElseBody != nil {
			elseLabel = ctx.newLabel()
		}

		ctx.codegenLogicalCond(d.Cond, thenLabel, elseLabel)

		ctx.writeLabel(thenLabel)
		thenTerminates := ctx.codegenStmt(d.ThenBody)
		if !thenTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		var elseTerminates bool
		if d.ElseBody != nil {
			ctx.writeLabel(elseLabel)
			elseTerminates = ctx.codegenStmt(d.ElseBody)
			if !elseTerminates {
				ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
			}
		}
		ctx.writeLabel(endLabel)
		return thenTerminates && elseTerminates

	case ast.While:
		d := node.Data.(ast.WhileNode)
		startLabel := ctx.newLabel()
		bodyLabel := ctx.newLabel()
		endLabel := ctx.newLabel()

		oldBreakLabel := ctx.breakLabel
		oldContinueLabel := ctx.continueLabel
		ctx.breakLabel = endLabel
		ctx.continueLabel = startLabel
		defer func() {
			ctx.breakLabel = oldBreakLabel
			ctx.continueLabel = oldContinueLabel
		}()

		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", startLabel))
		ctx.writeLabel(startLabel)

		ctx.codegenLogicalCond(d.Cond, bodyLabel, endLabel)

		ctx.writeLabel(bodyLabel)
		ctx.codegenStmt(d.Body)
		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", startLabel))

		ctx.writeLabel(endLabel)
		return false

	case ast.Switch:
		return ctx.codegenSwitch(node)

	case ast.Label:
		d := node.Data.(ast.LabelNode)
		labelName := "@" + d.Name
		ctx.writeLabel(labelName)
		return ctx.codegenStmt(d.Stmt)

	case ast.Goto:
		d := node.Data.(ast.GotoNode)
		ctx.out.WriteString(fmt.Sprintf("\tjmp @%s\n", d.Label))
		return true

	case ast.Break:
		if ctx.breakLabel == "" {
			util.Error(node.Tok, "'break' not in a loop or switch.")
		}
		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", ctx.breakLabel))
		return true

	case ast.Continue:
		if ctx.continueLabel == "" {
			util.Error(node.Tok, "'continue' not in a loop.")
		}
		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", ctx.continueLabel))
		return true

	case ast.Case:
		d := node.Data.(ast.CaseNode)
		ctx.writeLabel(d.QbeLabel)
		return ctx.codegenStmt(d.Body)

	case ast.Default:
		d := node.Data.(ast.DefaultNode)
		ctx.writeLabel(d.QbeLabel)
		return ctx.codegenStmt(d.Body)

	default:
		if node.Type <= ast.AutoAlloc {
			_, _, terminates := ctx.codegenExpr(node)
			return terminates
		}
		return false
	}
}

func (ctx *Context) findAllAutosInFunc(node *ast.Node, autoVars *[]autoVarInfo, definedNames map[string]bool) {
	if node == nil {
		return
	}
	if node.Type == ast.VarDecl {
		varData := node.Data.(ast.VarDeclNode)
		if !definedNames[varData.Name] {
			definedNames[varData.Name] = true
			var size int64
			if varData.Type != nil && varData.Type.Kind != ast.TYPE_UNTYPED {
				size = ctx.getSizeof(varData.Type)
			} else {
				if varData.IsVector {
					dataSizeInWords := int64(0)
					if varData.SizeExpr != nil {
						folded := ast.FoldConstants(varData.SizeExpr)
						if folded.Type != ast.Number {
							util.Error(node.Tok, "Local vector size must be a constant expression.")
						}
						dataSizeInWords = folded.Data.(ast.NumberNode).Value
					} else if len(varData.InitList) == 1 && varData.InitList[0].Type == ast.String {
						strLen := int64(len(varData.InitList[0].Data.(ast.StringNode).Value))
						numBytes := strLen + 1
						dataSizeInWords = (numBytes + int64(ctx.wordSize) - 1) / int64(ctx.wordSize)
					} else {
						dataSizeInWords = int64(len(varData.InitList))
					}
					if varData.IsBracketed {
						dataSizeInWords++
					}
					size = int64(ctx.wordSize) + dataSizeInWords*int64(ctx.wordSize)
				} else {
					size = int64(ctx.wordSize)
				}
			}
			*autoVars = append(*autoVars, autoVarInfo{Node: node, Size: size})
		}
	}

	switch d := node.Data.(type) {
	case ast.IfNode:
		ctx.findAllAutosInFunc(d.ThenBody, autoVars, definedNames)
		ctx.findAllAutosInFunc(d.ElseBody, autoVars, definedNames)
	case ast.WhileNode:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case ast.BlockNode:
		for _, s := range d.Stmts {
			ctx.findAllAutosInFunc(s, autoVars, definedNames)
		}
	case ast.MultiVarDeclNode:
		for _, decl := range d.Decls {
			ctx.findAllAutosInFunc(decl, autoVars, definedNames)
		}
	case ast.SwitchNode:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case ast.CaseNode:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case ast.DefaultNode:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case ast.LabelNode:
		ctx.findAllAutosInFunc(d.Stmt, autoVars, definedNames)
	}
}

func (ctx *Context) codegenFuncDecl(node *ast.Node) {
	d := node.Data.(ast.FuncDeclNode)
	if d.Body != nil && d.Body.Type == ast.AsmStmt {
		asmCode := d.Body.Data.(ast.AsmStmtNode).Code
		ctx.asmOut.WriteString(fmt.Sprintf(".globl %s\n%s:\n\t%s\n", d.Name, d.Name, strings.ReplaceAll(asmCode, "\n", "\n\t")))
		return
	}
	if d.Body == nil {
		return
	}

	prevFunc := ctx.currentFunc
	ctx.currentFunc = &d
	defer func() { ctx.currentFunc = prevFunc }()

	ctx.enterScope()
	defer ctx.exitScope()

	ctx.tempCount = 0
	returnQbeType := ctx.getQbeType(d.ReturnType)
	switch returnQbeType {
	case "b":
		if d.ReturnType != nil && d.ReturnType.Name == "int8" {
			returnQbeType = "sb"
		} else {
			returnQbeType = "ub"
		}
	case "h":
		if d.ReturnType != nil && d.ReturnType.Name == "uint16" {
			returnQbeType = "uh"
		} else {
			returnQbeType = "sh"
		}
	}

	if returnQbeType != "" {
		ctx.out.WriteString(fmt.Sprintf("export function %s $%s(", returnQbeType, d.Name))
	} else {
		ctx.out.WriteString(fmt.Sprintf("export function $%s(", d.Name))
	}

	for i, p := range d.Params {
		var paramQbeType string
		if d.IsTyped {
			paramData := p.Data.(ast.VarDeclNode)
			paramQbeType = ctx.getQbeType(paramData.Type)
			switch paramQbeType {
			case "b":
				if paramData.Type != nil && paramData.Type.Name == "int8" {
					paramQbeType = "sb"
				} else {
					paramQbeType = "ub"
				}
			case "h":
				if paramData.Type != nil && paramData.Type.Name == "uint16" {
					paramQbeType = "uh"
				} else {
					paramQbeType = "sh"
				}
			}
		} else {
			paramQbeType = ctx.wordType
		}
		fmt.Fprintf(&ctx.out, "%s %%p%d", paramQbeType, i)
		if i < len(d.Params)-1 {
			ctx.out.WriteString(", ")
		}
	}

	if d.HasVarargs {
		if len(d.Params) > 0 {
			ctx.out.WriteString(", ")
		}
		ctx.out.WriteString("...")
	}
	ctx.out.WriteString(") {\n")
	ctx.writeLabel("@start")

	var autoVars []autoVarInfo
	definedInFunc := make(map[string]bool)
	for _, p := range d.Params {
		var name string
		if d.IsTyped {
			name = p.Data.(ast.VarDeclNode).Name
		} else {
			name = p.Data.(ast.IdentNode).Name
		}
		definedInFunc[name] = true
	}
	ctx.findAllAutosInFunc(d.Body, &autoVars, definedInFunc)

	for i, j := 0, len(autoVars)-1; i < j; i, j = i+1, j-1 {
		autoVars[i], autoVars[j] = autoVars[j], autoVars[i]
	}

	paramInfos := make([]autoVarInfo, len(d.Params))
	for i, p := range d.Params {
		paramInfos[i] = autoVarInfo{Node: p, Size: int64(ctx.wordSize)}
	}
	for i, j := 0, len(paramInfos)-1; i < j; i, j = i+1, j-1 {
		paramInfos[i], paramInfos[j] = paramInfos[j], paramInfos[i]
	}

	allLocals := append(paramInfos, autoVars...)

	var totalFrameSize int64
	for _, local := range allLocals {
		totalFrameSize += local.Size
	}

	if totalFrameSize > 0 {
		align := int64(ctx.stackAlign)
		totalFrameSize = (totalFrameSize + align - 1) &^ (align - 1)
		ctx.currentFuncFrame = ctx.newTemp()
		allocInstr := ctx.getAllocInstruction()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %d\n", ctx.currentFuncFrame, ctx.wordType, allocInstr, totalFrameSize))
	}

	ctx.enterScope()
	defer ctx.exitScope()

	var currentOffset int64
	for i, local := range allLocals {
		var sym *symbol
		isParam := i < len(paramInfos)

		if local.Node.Type == ast.Ident {
			p := local.Node
			sym = ctx.addSymbol(p.Data.(ast.IdentNode).Name, symVar, nil, false, p)
		} else {
			varData := local.Node.Data.(ast.VarDeclNode)
			sym = ctx.addSymbol(varData.Name, symVar, varData.Type, varData.IsVector, local.Node)
		}

		sym.StackOffset = currentOffset
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", sym.QbeName, ctx.wordType, ctx.currentFuncFrame, sym.StackOffset))

		if isParam {
			paramIndex := len(d.Params) - 1 - i
			ctx.genStore(sym.QbeName, fmt.Sprintf("%%p%d", paramIndex), sym.BxType)
		} else {
			varData := local.Node.Data.(ast.VarDeclNode)
			if varData.IsVector && (varData.Type == nil || varData.Type.Kind == ast.TYPE_UNTYPED) {
				storageAddr := ctx.newTemp()
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", storageAddr, ctx.wordType, sym.QbeName, ctx.wordSize))
				ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, storageAddr, sym.QbeName))
			}
		}

		currentOffset += local.Size
	}

	bodyTerminates := ctx.codegenStmt(d.Body)

	if !bodyTerminates {
		if d.ReturnType != nil && d.ReturnType.Kind == ast.TYPE_VOID {
			ctx.out.WriteString("\tret\n")
		} else {
			ctx.out.WriteString("\tret 0\n")
		}
	}
	ctx.out.WriteString("}\n\n")
}

func (ctx *Context) codegenGlobalConst(node *ast.Node) string {
	folded := ast.FoldConstants(node)
	switch folded.Type {
	case ast.Number:
		return fmt.Sprintf("%d", folded.Data.(ast.NumberNode).Value)
	case ast.String:
		return ctx.addString(folded.Data.(ast.StringNode).Value)
	case ast.Ident:
		name := folded.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			util.Error(node.Tok, "Undefined symbol '%s' in global initializer.", name)
			return ""
		}
		if sym.IsVector && sym.Node != nil && sym.Node.Type == ast.VarDecl {
			d := sym.Node.Data.(ast.VarDeclNode)
			if !d.IsBracketed && len(d.InitList) <= 1 && d.Type == nil {
				return "$_" + name + "_storage"
			}
		}
		return sym.QbeName
	case ast.AddressOf:
		lval := folded.Data.(ast.AddressOfNode).LValue
		if lval.Type != ast.Ident {
			util.Error(lval.Tok, "Global initializer must be the address of a global symbol.")
			return ""
		}
		name := lval.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			util.Error(lval.Tok, "Undefined symbol '%s' in global initializer.", name)
			return ""
		}
		if sym.IsVector {
			return "$_" + name + "_storage"
		}
		return sym.QbeName
	default:
		util.Error(node.Tok, "Global initializer must be a constant expression.")
		return ""
	}
}

func (ctx *Context) codegenVarDecl(node *ast.Node) {
	d := node.Data.(ast.VarDeclNode)
	sym := ctx.findSymbol(d.Name)
	if sym == nil {
		if ctx.currentFunc == nil {
			sym = ctx.addSymbol(d.Name, symVar, d.Type, d.IsVector, node)
		} else {
			util.Error(node.Tok, "Internal error: symbol '%s' not found during declaration.", d.Name)
			return
		}
	}

	if ctx.currentFunc == nil {
		ctx.codegenGlobalVarDecl(d, sym)
	} else {
		ctx.codegenLocalVarDecl(d, sym)
	}
}

func (ctx *Context) codegenLocalVarDecl(d ast.VarDeclNode, sym *symbol) {
	if len(d.InitList) == 0 {
		return
	}

	if d.IsVector || (d.Type != nil && d.Type.Kind == ast.TYPE_ARRAY) {
		vectorPtr := ctx.genLoad(sym.QbeName, sym.BxType)

		if len(d.InitList) == 1 && d.InitList[0].Type == ast.String {
			strVal := d.InitList[0].Data.(ast.StringNode).Value
			strLabel := ctx.addString(strVal)
			sizeToCopy := len(strVal) + 1
			ctx.out.WriteString(fmt.Sprintf("\tblit %s, %s, %d\n", strLabel, vectorPtr, sizeToCopy))
		} else {
			for i, initExpr := range d.InitList {
				offset := int64(i) * int64(ctx.wordSize)
				elemAddr := ctx.newTemp()
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", elemAddr, ctx.wordType, vectorPtr, offset))
				rval, _, _ := ctx.codegenExpr(initExpr)
				ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, rval, elemAddr))
			}
		}
		return
	}

	rval, _, _ := ctx.codegenExpr(d.InitList[0])
	ctx.genStore(sym.QbeName, rval, d.Type)
}

func (ctx *Context) codegenGlobalVarDecl(d ast.VarDeclNode, sym *symbol) {
	ctx.out.WriteString(fmt.Sprintf("data %s = align %d { ", sym.QbeName, ctx.wordSize))

	var elemSize int64 = int64(ctx.wordSize)
	var elemQbeType string = ctx.wordType

	isTypedArray := d.Type != nil && (d.Type.Kind == ast.TYPE_ARRAY || d.Type.Kind == ast.TYPE_POINTER)
	if isTypedArray {
		if d.Type.Base != nil {
			elemSize = ctx.getSizeof(d.Type.Base)
			elemQbeType = ctx.getQbeType(d.Type.Base)
		}
	} else if d.Type != nil {
		elemSize = ctx.getSizeof(d.Type)
		elemQbeType = ctx.getQbeType(d.Type)
	}

	var totalSize int64
	if d.Type != nil && d.Type.Kind == ast.TYPE_ARRAY && d.Type.ArraySize != nil {
		totalSize = ctx.getSizeof(d.Type)
	} else if d.IsVector && d.SizeExpr != nil {
		if folded := ast.FoldConstants(d.SizeExpr); folded.Type == ast.Number {
			totalSize = folded.Data.(ast.NumberNode).Value * elemSize
		}
	} else {
		totalSize = int64(len(d.InitList)) * elemSize
		if totalSize == 0 {
			totalSize = elemSize
		}
	}

	if len(d.InitList) > 0 {
		var items []string
		for _, init := range d.InitList {
			val := ctx.codegenGlobalConst(init)
			itemType := elemQbeType
			if strings.HasPrefix(val, "$") {
				itemType = ctx.wordType
			}
			items = append(items, fmt.Sprintf("%s %s", itemType, val))
		}
		ctx.out.WriteString(strings.Join(items, ", "))

		initializedBytes := int64(len(d.InitList)) * elemSize
		if totalSize > initializedBytes {
			ctx.out.WriteString(fmt.Sprintf(", z %d", totalSize-initializedBytes))
		}
	} else {
		ctx.out.WriteString(fmt.Sprintf("z %d", totalSize))
	}

	ctx.out.WriteString(" }\n")
}

func (ctx *Context) codegenSwitch(node *ast.Node) bool {
	d := node.Data.(ast.SwitchNode)
	switchVal, _, _ := ctx.codegenExpr(d.Expr)
	endLabel := ctx.newLabel()

	oldBreakLabel := ctx.breakLabel
	ctx.breakLabel = endLabel
	defer func() { ctx.breakLabel = oldBreakLabel }()

	defaultTarget := endLabel
	if d.DefaultLabelName != "" {
		defaultTarget = d.DefaultLabelName
	}

	switchType := ctx.getQbeType(d.Expr.Typ)

	for _, caseLabel := range d.CaseLabels {
		caseValConst := fmt.Sprintf("%d", caseLabel.Value)
		cmpRes := ctx.newTemp()
		nextCheckLabel := ctx.newLabel()
		cmpInst := ctx.getCmpOpStr(token.EqEq, switchType)
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", cmpRes, ctx.wordType, cmpInst, switchVal, caseValConst))
		ctx.out.WriteString(fmt.Sprintf("\tjnz %s, %s, %s\n", cmpRes, caseLabel.LabelName, nextCheckLabel))
		ctx.writeLabel(nextCheckLabel)
	}
	ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", defaultTarget))

	bodyTerminates := true
	if d.Body != nil && d.Body.Type == ast.Block {
		codegenStarted := false
		allPathsTerminate := true
		bodyStmts := d.Body.Data.(ast.BlockNode).Stmts
		for _, stmt := range bodyStmts {
			isLabel := stmt.Type == ast.Case || stmt.Type == ast.Default || stmt.Type == ast.Label
			if !codegenStarted && isLabel {
				codegenStarted = true
			}
			if codegenStarted {
				if !ctx.codegenStmt(stmt) {
					allPathsTerminate = false
				}
			}
		}
		bodyTerminates = allPathsTerminate
	}

	ctx.writeLabel(endLabel)
	return bodyTerminates && d.DefaultLabelName != ""
}
