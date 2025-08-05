package codegen

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"gbc/pkg/ast"
	"gbc/pkg/token"
	"gbc/pkg/util"
)

type symbolType int

const (
	symVar symbolType = iota
	symFunc
	symLabel
)

type symbol struct {
	Name        string
	Type        symbolType
	QbeName     string
	IsVector    bool
	IsByteArray bool
	StackOffset int64
	Next        *symbol
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
	out              strings.Builder
	asmOut           strings.Builder
	strings          []stringEntry
	tempCount        int
	labelCount       int
	currentScope     *scope
	currentFuncFrame string
	breakLabel       string
	wordType         string
	wordSize         int
	warnings         bool
	phiFromLabel     string
}

func NewContext(targetArch string) *Context {
	var ws int
	var wt string
	switch targetArch {
	case "amd64", "arm64", "riscv64", "s390x", "ppc64le", "mips64le":
		ws = 8
		wt = "l"
	case "386", "arm", "mipsle":
		ws = 4
		wt = "w"
	default:
		fmt.Fprintf(os.Stderr, "warning: unrecognized architecture '%s', defaulting to 64-bit\n", runtime.GOARCH)
		ws = 8
		wt = "l"
	}

	return &Context{
		currentScope: newScope(nil),
		wordSize:     ws,
		wordType:     wt,
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

func (ctx *Context) addSymbol(name string, symType symbolType, isVector bool, tok token.Token) *symbol {
	if ctx.findSymbolInCurrentScope(name) != nil {
		if symType == symVar {
			// B allows redeclaration
		}
	}

	var qbeName string
	switch symType {
	case symVar:
		if ctx.currentScope.Parent == nil { // Global
			qbeName = "$" + name
		} else { // Local
			qbeName = fmt.Sprintf("%%.%s_%d", name, ctx.tempCount)
			ctx.tempCount++
		}
	case symFunc:
		qbeName = "$" + name
	case symLabel:
		qbeName = "@" + name
	}

	sym := &symbol{
		Name:     name,
		Type:     symType,
		QbeName:  qbeName,
		IsVector: isVector,
		Next:     ctx.currentScope.Symbols,
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

func (ctx *Context) getCmpOpStr(op token.Type) string {
	var base string
	switch op {
	case token.EqEq:
		base = "ceq"
	case token.Neq:
		base = "cne"
	case token.Lt:
		base = "cslt"
	case token.Gt:
		base = "csgt"
	case token.Lte:
		base = "csle"
	case token.Gte:
		base = "csge"
	default:
		return ""
	}
	return base + ctx.wordType
}

func (ctx *Context) Generate(root *ast.Node) (qbeIR, inlineAsm string) {
	ctx.collectGlobals(root)
	ctx.collectStrings(root)
	ctx.findByteArrays(root)
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
			if ctx.findSymbolInCurrentScope(d.Name) == nil {
				ctx.addSymbol(d.Name, symVar, d.IsVector, node.Tok)
			}
		}
	case ast.FuncDecl:
		d := node.Data.(ast.FuncDeclNode)
		if ctx.findSymbolInCurrentScope(d.Name) == nil {
			ctx.addSymbol(d.Name, symFunc, false, node.Tok)
		}
	case ast.ExtrnDecl:
		d := node.Data.(ast.ExtrnDeclNode)
		if ctx.findSymbolInCurrentScope(d.Name) == nil {
			ctx.addSymbol(d.Name, symFunc, false, node.Tok)
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

func isStringDerivedExpr(node *ast.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case ast.String:
		return true
	case ast.BinaryOp:
		if node.Data.(ast.BinaryOpNode).Op == token.Plus {
			return isStringDerivedExpr(node.Data.(ast.BinaryOpNode).Left) || isStringDerivedExpr(node.Data.(ast.BinaryOpNode).Right)
		}
	}
	return false
}

func (ctx *Context) codegenLvalue(node *ast.Node) string {
	if node == nil {
		util.Error(token.Token{}, "Internal error: null l-value node in codegen")
	}
	switch node.Type {
	case ast.Ident:
		name := node.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			sym = ctx.addSymbol(name, symFunc, false, node.Tok)
		}
		if sym.Type == symFunc {
			return sym.QbeName
		}
		return sym.QbeName

	case ast.Indirection:
		res, _ := ctx.codegenExpr(node.Data.(ast.IndirectionNode).Expr)
		return res

	case ast.Subscript:
		d := node.Data.(ast.SubscriptNode)
		arrayBasePtr, _ := ctx.codegenExpr(d.Array)
		indexVal, _ := ctx.codegenExpr(d.Index)

		scale := ctx.wordSize
		if d.Array.Type == ast.Ident {
			sym := ctx.findSymbol(d.Array.Data.(ast.IdentNode).Name)
			if sym != nil && sym.IsByteArray {
				scale = 1
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
	condVal, _ := ctx.codegenExpr(node)

	isNonZero := ctx.newTemp()
	ctx.out.WriteString(fmt.Sprintf("\t%s =%s cne%s %s, 0\n", isNonZero, ctx.wordType, ctx.wordType, condVal))

	var condValForJnz string
	if ctx.wordType == "l" {
		condValWord := ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =w copy %s\n", condValWord, isNonZero))
		condValForJnz = condValWord
	} else {
		condValForJnz = isNonZero
	}

	ctx.out.WriteString(fmt.Sprintf("\tjnz %s, %s, %s\n", condValForJnz, trueL, falseL))
}

func (ctx *Context) codegenExpr(node *ast.Node) (result string, terminates bool) {
	if node == nil {
		return "0", false
	}

	switch node.Type {
	case ast.Number:
		return fmt.Sprintf("%d", node.Data.(ast.NumberNode).Value), false
	case ast.String:
		return ctx.addString(node.Data.(ast.StringNode).Value), false

	case ast.Ident:
		sym := ctx.findSymbol(node.Data.(ast.IdentNode).Name)
		if sym == nil {
			sym = ctx.addSymbol(node.Data.(ast.IdentNode).Name, symFunc, false, node.Tok)
		}

		isFuncNameInCall := node.Parent != nil && node.Parent.Type == ast.FuncCall && node.Parent.Data.(ast.FuncCallNode).FuncExpr == node
		if sym.Type == symFunc && isFuncNameInCall {
			return sym.QbeName, false
		}

		addr := sym.QbeName
		res := ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s load%s %s\n", res, ctx.wordType, ctx.wordType, addr))
		return res, false

	case ast.Assign:
		d := node.Data.(ast.AssignNode)
		lvalAddr := ctx.codegenLvalue(d.Lhs)
		var rval string
		if d.Op == token.Eq {
			rval, _ = ctx.codegenExpr(d.Rhs)
		} else {
			currentLvalVal := ctx.newTemp()
			loadInstruction := "load" + ctx.wordType
			if d.Lhs.Type == ast.Subscript && d.Lhs.Data.(ast.SubscriptNode).Array.Type == ast.Ident {
				sym := ctx.findSymbol(d.Lhs.Data.(ast.SubscriptNode).Array.Data.(ast.IdentNode).Name)
				if sym != nil && sym.IsByteArray {
					loadInstruction = "loadub"
				}
			}
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", currentLvalVal, ctx.wordType, loadInstruction, lvalAddr))

			rhsVal, _ := ctx.codegenExpr(d.Rhs)
			opStr := getOpStr(d.Op)
			rval = ctx.newTemp()
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", rval, ctx.wordType, opStr, currentLvalVal, rhsVal))
		}

		storeInstruction := "store" + ctx.wordType
		if d.Lhs.Type == ast.Subscript && d.Lhs.Data.(ast.SubscriptNode).Array.Type == ast.Ident {
			sym := ctx.findSymbol(d.Lhs.Data.(ast.SubscriptNode).Array.Data.(ast.IdentNode).Name)
			if sym != nil && sym.IsByteArray {
				storeInstruction = "storeb"
			}
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s %s, %s\n", storeInstruction, rval, lvalAddr))
		return rval, false

	case ast.BinaryOp:
		d := node.Data.(ast.BinaryOpNode)
		l, _ := ctx.codegenExpr(d.Left)
		r, _ := ctx.codegenExpr(d.Right)
		res := ctx.newTemp()
		if opStr := getOpStr(d.Op); opStr != "" {
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, ctx.wordType, opStr, l, r))
		} else if cmpOpStr := ctx.getCmpOpStr(d.Op); cmpOpStr != "" {
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, ctx.wordType, cmpOpStr, l, r))
		} else {
			util.Error(node.Tok, "Invalid binary operator token %d", d.Op)
		}
		return res, false

	case ast.UnaryOp:
		d := node.Data.(ast.UnaryOpNode)
		val, _ := ctx.codegenExpr(d.Expr)
		res := ctx.newTemp()
		switch d.Op {
		case token.Minus:
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s sub 0, %s\n", res, ctx.wordType, val))
		case token.Plus:
			return val, false
		case token.Not:
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s ceq%s %s, 0\n", res, ctx.wordType, ctx.wordType, val))
		case token.Complement:
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s xor %s, -1\n", res, ctx.wordType, val))
		case token.Inc, token.Dec:
			lvalAddr := ctx.codegenLvalue(d.Expr)
			op := map[token.Type]string{token.Inc: "add", token.Dec: "sub"}[d.Op]
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, 1\n", res, ctx.wordType, op, val))
			ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, res, lvalAddr))
		default:
			util.Error(node.Tok, "Unsupported unary operator")
		}
		return res, false

	case ast.PostfixOp:
		d := node.Data.(ast.PostfixOpNode)
		lvalAddr := ctx.codegenLvalue(d.Expr)
		res := ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s load%s %s\n", res, ctx.wordType, ctx.wordType, lvalAddr))
		newVal := ctx.newTemp()
		op := map[token.Type]string{token.Inc: "add", token.Dec: "sub"}[d.Op]
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, 1\n", newVal, ctx.wordType, op, res))
		ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, newVal, lvalAddr))
		return res, false

	case ast.Indirection:
		exprNode := node.Data.(ast.IndirectionNode).Expr
		addr, _ := ctx.codegenExpr(exprNode)
		res := ctx.newTemp()

		loadInstruction := "load" + ctx.wordType
		isBytePointer := false

		if isStringDerivedExpr(exprNode) {
			isBytePointer = true
		} else if exprNode.Type == ast.Ident {
			sym := ctx.findSymbol(exprNode.Data.(ast.IdentNode).Name)
			if sym != nil && sym.IsByteArray {
				isBytePointer = true
			}
		}

		if isBytePointer {
			loadInstruction = "loadub"
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", res, ctx.wordType, loadInstruction, addr))
		return res, false

	case ast.Subscript:
		addr := ctx.codegenLvalue(node)
		res := ctx.newTemp()
		loadInstruction := "load" + ctx.wordType
		if node.Data.(ast.SubscriptNode).Array.Type == ast.Ident {
			sym := ctx.findSymbol(node.Data.(ast.SubscriptNode).Array.Data.(ast.IdentNode).Name)
			if sym != nil && sym.IsByteArray {
				loadInstruction = "loadub"
			}
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", res, ctx.wordType, loadInstruction, addr))
		return res, false

	case ast.AddressOf:
		lvalNode := node.Data.(ast.AddressOfNode).LValue
		if lvalNode.Type == ast.Ident {
			sym := ctx.findSymbol(lvalNode.Data.(ast.IdentNode).Name)
			if sym != nil && sym.IsVector {
				return ctx.codegenExpr(lvalNode)
			}
		}
		return ctx.codegenLvalue(lvalNode), false

	case ast.FuncCall:
		d := node.Data.(ast.FuncCallNode)
		if d.FuncExpr.Type == ast.Ident {
			name := d.FuncExpr.Data.(ast.IdentNode).Name
			sym := ctx.findSymbol(name)
			if sym != nil && sym.Type == symVar && !sym.IsVector {
				util.Error(d.FuncExpr.Tok, "'%s' is a variable but is used as a function", name)
			}
		}

		argVals := make([]string, len(d.Args))
		for i := len(d.Args) - 1; i >= 0; i-- {
			argVals[i], _ = ctx.codegenExpr(d.Args[i])
		}
		funcVal, _ := ctx.codegenExpr(d.FuncExpr)

		isStmt := node.Parent != nil && node.Parent.Type == ast.Block

		res := "0"
		callStr := new(strings.Builder)
		if !isStmt {
			res = ctx.newTemp()
			fmt.Fprintf(callStr, "\t%s =%s call %s(", res, ctx.wordType, funcVal)
		} else {
			fmt.Fprintf(callStr, "\tcall %s(", funcVal)
		}

		for i, arg := range argVals {
			fmt.Fprintf(callStr, "%s %s", ctx.wordType, arg)
			if i < len(argVals)-1 {
				callStr.WriteString(", ")
			}
		}
		callStr.WriteString(")\n")
		ctx.out.WriteString(callStr.String())
		return res, false

	case ast.Ternary:
		d := node.Data.(ast.TernaryNode)
		thenLabel := ctx.newLabel()
		elseLabel := ctx.newLabel()
		endLabel := ctx.newLabel()
		res := ctx.newTemp()

		ctx.codegenLogicalCond(d.Cond, thenLabel, elseLabel)

		ctx.out.WriteString(thenLabel + "\n")
		ctx.phiFromLabel = thenLabel
		thenVal, thenTerminates := ctx.codegenExpr(d.ThenExpr)
		thenFromLabelForPhi := ctx.phiFromLabel
		if !thenTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		ctx.out.WriteString(elseLabel + "\n")
		ctx.phiFromLabel = elseLabel
		elseVal, elseTerminates := ctx.codegenExpr(d.ElseExpr)
		elseFromLabelForPhi := ctx.phiFromLabel
		if !elseTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		if !thenTerminates || !elseTerminates {
			ctx.out.WriteString(endLabel + "\n")

			phiArgs := new(strings.Builder)
			if !thenTerminates {
				fmt.Fprintf(phiArgs, "%s %s", thenFromLabelForPhi, thenVal)
			}
			if !elseTerminates {
				if !thenTerminates {
					phiArgs.WriteString(", ")
				}
				fmt.Fprintf(phiArgs, "%s %s", elseFromLabelForPhi, elseVal)
			}

			ctx.out.WriteString(fmt.Sprintf("\t%s =%s phi %s\n", res, ctx.wordType, phiArgs.String()))

			ctx.phiFromLabel = endLabel

			return res, thenTerminates && elseTerminates
		}

		return "0", true
	}
	util.Error(node.Tok, "Internal error: unhandled expression type in codegen: %v", node.Type)
	return "", false
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
					util.Warn(util.WarnUnreachableCode, stmt.Tok, "Unreachable code.")
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

	case ast.Return:
		val := "0"
		if node.Data.(ast.ReturnNode).Expr != nil {
			val, _ = ctx.codegenExpr(node.Data.(ast.ReturnNode).Expr)
		}
		ctx.out.WriteString(fmt.Sprintf("\tret %s\n", val))
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

		ctx.out.WriteString(thenLabel + "\n")
		thenTerminates := ctx.codegenStmt(d.ThenBody)
		if !thenTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		var elseTerminates bool
		if d.ElseBody != nil {
			ctx.out.WriteString(elseLabel + "\n")
			elseTerminates = ctx.codegenStmt(d.ElseBody)
			if !elseTerminates {
				ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
			}
		}
		ctx.out.WriteString(endLabel + "\n")
		return thenTerminates && elseTerminates

	case ast.While:
		d := node.Data.(ast.WhileNode)
		startLabel := ctx.newLabel()
		bodyLabel := ctx.newLabel()
		endLabel := ctx.newLabel()

		oldBreakLabel := ctx.breakLabel
		ctx.breakLabel = endLabel
		defer func() { ctx.breakLabel = oldBreakLabel }()

		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", startLabel))
		ctx.out.WriteString(bodyLabel + "\n")
		ctx.codegenStmt(d.Body)
		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", startLabel))
		ctx.out.WriteString(startLabel + "\n")

		ctx.codegenLogicalCond(d.Cond, bodyLabel, endLabel)

		ctx.out.WriteString(endLabel + "\n")
		return false

	case ast.Switch:
		return ctx.codegenSwitch(node)

	case ast.Label:
		d := node.Data.(ast.LabelNode)
		ctx.out.WriteString(fmt.Sprintf("@%s\n", d.Name))
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

	case ast.Case:
		d := node.Data.(ast.CaseNode)
		ctx.out.WriteString(d.QbeLabel + "\n")
		return ctx.codegenStmt(d.Body)

	case ast.Default:
		d := node.Data.(ast.DefaultNode)
		ctx.out.WriteString(d.QbeLabel + "\n")
		return ctx.codegenStmt(d.Body)

	default:
		if node.Type <= ast.Subscript {
			_, terminates := ctx.codegenExpr(node)
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
			if varData.IsVector {
				if varData.SizeExpr != nil {
					folded := ast.FoldConstants(varData.SizeExpr)
					if folded.Type != ast.Number {
						util.Error(node.Tok, "Local vector size must be a constant expression.")
					}
					sizeInWords := folded.Data.(ast.NumberNode).Value
					if varData.IsBracketed {
						size = (sizeInWords + 1) * int64(ctx.wordSize)
					} else {
						size = sizeInWords * int64(ctx.wordSize)
					}
				} else if len(varData.InitList) == 1 && varData.InitList[0].Type == ast.String {
					strLen := int64(len(varData.InitList[0].Data.(ast.StringNode).Value))
					numBytes := strLen + 1
					size = (numBytes + int64(ctx.wordSize) - 1) / int64(ctx.wordSize) * int64(ctx.wordSize)
				} else {
					size = int64(len(varData.InitList)) * int64(ctx.wordSize)
				}
				if size == 0 {
					size = int64(ctx.wordSize)
				}
				size += int64(ctx.wordSize)
			} else {
				size = int64(ctx.wordSize)
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

	ctx.enterScope()
	defer ctx.exitScope()

	ctx.tempCount = 0
	ctx.out.WriteString(fmt.Sprintf("export function %s $%s(", ctx.wordType, d.Name))
	for i := range d.Params {
		fmt.Fprintf(&ctx.out, "%s %%p%d", ctx.wordType, i)
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
	ctx.out.WriteString(") {\n@start\n")

	var autoVars []autoVarInfo
	definedInFunc := make(map[string]bool)
	for _, p := range d.Params {
		definedInFunc[p.Data.(ast.IdentNode).Name] = true
	}
	ctx.findAllAutosInFunc(d.Body, &autoVars, definedInFunc)

	for i, j := 0, len(autoVars)-1; i < j; i, j = i+1, j-1 {
		autoVars[i], autoVars[j] = autoVars[j], autoVars[i]
	}

	var totalFrameSize int64
	allLocals := make([]autoVarInfo, 0, len(d.Params)+len(autoVars))
	paramInfos := make([]autoVarInfo, len(d.Params))
	for i, p := range d.Params {
		paramInfos[i] = autoVarInfo{Node: p, Size: int64(ctx.wordSize)}
	}
	for i, j := 0, len(paramInfos)-1; i < j; i, j = i+1, j-1 {
		paramInfos[i], paramInfos[j] = paramInfos[j], paramInfos[i]
	}

	allLocals = append(allLocals, paramInfos...)
	allLocals = append(allLocals, autoVars...)

	for _, local := range allLocals {
		totalFrameSize += local.Size
	}

	if totalFrameSize > 0 {
		ctx.currentFuncFrame = ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s alloc8 %d\n", ctx.currentFuncFrame, ctx.wordType, totalFrameSize))
	}

	ctx.enterScope()
	defer ctx.exitScope()

	var currentOffset int64
	for _, local := range allLocals {
		var sym *symbol
		if local.Node.Type == ast.Ident {
			p := local.Node
			sym = ctx.addSymbol(p.Data.(ast.IdentNode).Name, symVar, false, p.Tok)
			sym.StackOffset = currentOffset
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", sym.QbeName, ctx.wordType, ctx.currentFuncFrame, sym.StackOffset))
			paramIndex := -1
			for j, origP := range d.Params {
				if origP == p {
					paramIndex = j
					break
				}
			}
			ctx.out.WriteString(fmt.Sprintf("\tstore%s %%p%d, %s\n", ctx.wordType, paramIndex, sym.QbeName))
		} else {
			varData := local.Node.Data.(ast.VarDeclNode)
			sym = ctx.addSymbol(varData.Name, symVar, varData.IsVector, local.Node.Tok)
			sym.StackOffset = currentOffset
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", sym.QbeName, ctx.wordType, ctx.currentFuncFrame, sym.StackOffset))

			if varData.IsVector {
				storageAddr := ctx.newTemp()
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", storageAddr, ctx.wordType, sym.QbeName, ctx.wordSize))
				ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, storageAddr, sym.QbeName))
			}
		}
		currentOffset += local.Size
	}

	bodyTerminates := ctx.codegenStmt(d.Body)

	if !bodyTerminates {
		ctx.out.WriteString("\tret 0\n")
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
		}
		if sym.IsVector {
			return "$_" + name + "_storage"
		}
		return sym.QbeName
	case ast.AddressOf:
		lval := folded.Data.(ast.AddressOfNode).LValue
		if lval.Type != ast.Ident {
			util.Error(lval.Tok, "Global initializer must be the address of a global symbol.")
		}
		name := lval.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			util.Error(lval.Tok, "Undefined symbol '%s' in global initializer.", name)
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
		util.Error(node.Tok, "Internal error: symbol '%s' not found during declaration.", d.Name)
	}

	if ctx.currentScope.Parent == nil { // Global
		if d.IsVector {
			storageName := "$_" + d.Name + "_storage"
			sizeInWords := int64(0)
			if d.SizeExpr != nil {
				sizeExprData := ast.FoldConstants(d.SizeExpr).Data.(ast.NumberNode)
				sizeInWords = sizeExprData.Value
				if d.IsBracketed {
					sizeInWords++
				}
			}
			sizeFromInits := int64(len(d.InitList))
			if sizeFromInits > sizeInWords {
				sizeInWords = sizeFromInits
			}
			if sizeInWords == 0 {
				sizeInWords = 1
			}

			ctx.out.WriteString(fmt.Sprintf("data %s = align %d { ", storageName, ctx.wordSize))
			for i, init := range d.InitList {
				val := ctx.codegenGlobalConst(init)
				fmt.Fprintf(&ctx.out, "%s %s", ctx.wordType, val)
				if i < len(d.InitList)-1 {
					ctx.out.WriteString(", ")
				}
			}
			remainingBytes := (sizeInWords - int64(len(d.InitList))) * int64(ctx.wordSize)
			if remainingBytes > 0 {
				if len(d.InitList) > 0 {
					ctx.out.WriteString(", ")
				}
				fmt.Fprintf(&ctx.out, "z %d", remainingBytes)
			}
			ctx.out.WriteString(" }\n")

			ctx.out.WriteString(fmt.Sprintf("data %s = { %s %s }\n", sym.QbeName, ctx.wordType, storageName))

		} else { // Global scalar
			ctx.out.WriteString(fmt.Sprintf("data %s = align %d { ", sym.QbeName, ctx.wordSize))
			if len(d.InitList) > 0 {
				val := ctx.codegenGlobalConst(d.InitList[0])
				fmt.Fprintf(&ctx.out, "%s %s", ctx.wordType, val)
			} else {
				fmt.Fprintf(&ctx.out, "z %d", ctx.wordSize)
			}
			ctx.out.WriteString(" }\n")
		}

	} else { // Auto
		if len(d.InitList) > 0 {
			if d.IsVector {
				storagePtr := ctx.newTemp()
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", storagePtr, ctx.wordType, sym.QbeName, ctx.wordSize))

				if len(d.InitList) == 1 && d.InitList[0].Type == ast.String {
					strVal := d.InitList[0].Data.(ast.StringNode).Value
					strLabel := ctx.addString(strVal)
					sizeToCopy := len(strVal) + 1
					ctx.out.WriteString(fmt.Sprintf("\tblit %s, %s, %d\n", strLabel, storagePtr, sizeToCopy))
				} else {
					for i, initExpr := range d.InitList {
						offset := int64(i) * int64(ctx.wordSize)
						elemAddr := ctx.newTemp()
						ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", elemAddr, ctx.wordType, storagePtr, offset))
						rval, _ := ctx.codegenExpr(initExpr)
						ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, rval, elemAddr))
					}
				}
			} else {
				rval, _ := ctx.codegenExpr(d.InitList[0])
				ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, rval, sym.QbeName))
			}
		}
	}
}

func (ctx *Context) codegenSwitch(node *ast.Node) bool {
	d := node.Data.(ast.SwitchNode)
	switchVal, _ := ctx.codegenExpr(d.Expr)
	endLabel := ctx.newLabel()

	oldBreakLabel := ctx.breakLabel
	ctx.breakLabel = endLabel
	defer func() { ctx.breakLabel = oldBreakLabel }()

	defaultTarget := endLabel
	if d.DefaultLabelName != "" {
		defaultTarget = d.DefaultLabelName
	}

	for _, caseLabel := range d.CaseLabels {
		caseValConst := fmt.Sprintf("%d", caseLabel.Value)
		cmpResLong := ctx.newTemp()
		nextCheckLabel := ctx.newLabel()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s ceq%s %s, %s\n", cmpResLong, ctx.wordType, ctx.wordType, switchVal, caseValConst))

		var cmpResForJnz string
		if ctx.wordType == "l" {
			cmpResWord := ctx.newTemp()
			ctx.out.WriteString(fmt.Sprintf("\t%s =w copy %s\n", cmpResWord, cmpResLong))
			cmpResForJnz = cmpResWord
		} else {
			cmpResForJnz = cmpResLong
		}

		ctx.out.WriteString(fmt.Sprintf("\tjnz %s, %s, %s\n", cmpResForJnz, caseLabel.LabelName, nextCheckLabel))
		ctx.out.WriteString(nextCheckLabel + "\n")
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

	ctx.out.WriteString(endLabel + "\n")
	return bodyTerminates && d.DefaultLabelName != ""
}
