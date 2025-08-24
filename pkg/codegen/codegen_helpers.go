package codegen

import (
	"strings"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/util"
)

// Helper functions for codegenExpr

func (ctx *Context) codegenIdent(node *ast.Node) (ir.Value, bool) {
	name := node.Data.(ast.IdentNode).Name
	sym := ctx.findSymbol(name)

	if sym == nil {
		util.Warn(ctx.cfg, config.WarnImplicitDecl, node.Tok, "Implicit declaration of function '%s'", name)
		sym = ctx.addSymbol(name, symFunc, ast.TypeUntyped, false, node)
		return sym.IRVal, false
	}

	switch sym.Type {
	case symFunc:
		return sym.IRVal, false
	case symExtrn:
		isCall := node.Parent != nil && node.Parent.Type == ast.FuncCall && node.Parent.Data.(ast.FuncCallNode).FuncExpr == node
		if isCall {
			return sym.IRVal, false
		}
		ctx.prog.ExtrnVars[name] = true
		res := ctx.newTemp()
		ctx.addInstr(&ir.Instruction{Op: ir.OpLoad, Typ: ir.TypePtr, Result: res, Args: []ir.Value{sym.IRVal}})
		return res, false
	}

	isArr := sym.IsVector || (sym.BxType != nil && sym.BxType.Kind == ast.TYPE_ARRAY)
	if isArr {
		isParam := false
		if sym.Node != nil && sym.Node.Parent != nil && sym.Node.Parent.Type == ast.FuncDecl {
			funcDecl := sym.Node.Parent.Data.(ast.FuncDeclNode)
			for _, p := range funcDecl.Params {
				if p == sym.Node {
					isParam = true
					break
				}
			}
		}
		_, isLocal := sym.IRVal.(*ir.Temporary)
		if isLocal {
			isDopeVector := sym.IsVector && (sym.BxType == nil || sym.BxType.Kind == ast.TYPE_UNTYPED)
			if isParam || isDopeVector {
				return ctx.genLoad(sym.IRVal, sym.BxType), false
			}
		}
		return sym.IRVal, false
	}

	return ctx.genLoad(sym.IRVal, sym.BxType), false
}

func (ctx *Context) codegenAssign(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.AssignNode)
	lvalAddr := ctx.codegenLvalue(d.Lhs)
	var rval ir.Value

	if d.Op == token.Eq {
		rval, _ = ctx.codegenExpr(d.Rhs)
	} else {
		currentLvalVal := ctx.genLoad(lvalAddr, d.Lhs.Typ)
		rhsVal, _ := ctx.codegenExpr(d.Rhs)
		op, typ := getBinaryOpAndType(d.Op, d.Lhs.Typ, ctx.wordSize)
		rval = ctx.newTemp()
		ctx.addInstr(&ir.Instruction{Op: op, Typ: typ, Result: rval, Args: []ir.Value{currentLvalVal, rhsVal}})
	}

	ctx.genStore(lvalAddr, rval, d.Lhs.Typ)
	return rval, false
}

func (ctx *Context) codegenBinaryOp(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.BinaryOpNode)
	if d.Op == token.OrOr || d.Op == token.AndAnd {
		res := ctx.newTemp()
		trueL, falseL, endL := ctx.newLabel(), ctx.newLabel(), ctx.newLabel()

		ctx.codegenLogicalCond(node, trueL, falseL)

		ctx.startBlock(trueL)
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endL}})

		ctx.startBlock(falseL)
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endL}})

		ctx.startBlock(endL)
		wordType := ir.GetType(nil, ctx.wordSize)
		ctx.addInstr(&ir.Instruction{
			Op:     ir.OpPhi,
			Typ:    wordType,
			Result: res,
			Args: []ir.Value{
				trueL, &ir.Const{Value: 1},
				falseL, &ir.Const{Value: 0},
			},
		})
		return res, false
	}

	l, _ := ctx.codegenExpr(d.Left)
	r, _ := ctx.codegenExpr(d.Right)
	res := ctx.newTemp()
	op, typ := getBinaryOpAndType(d.Op, d.Left.Typ, ctx.wordSize)
	ctx.addInstr(&ir.Instruction{Op: op, Typ: typ, Result: res, Args: []ir.Value{l, r}})
	return res, false
}

func (ctx *Context) codegenUnaryOp(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.UnaryOpNode)
	res := ctx.newTemp()
	val, _ := ctx.codegenExpr(d.Expr)
	valType := ir.GetType(d.Expr.Typ, ctx.wordSize)

	switch d.Op {
	case token.Minus:
		ctx.addInstr(&ir.Instruction{Op: ir.OpSub, Typ: valType, Result: res, Args: []ir.Value{&ir.Const{Value: 0}, val}})
	case token.Plus:
		return val, false
	case token.Not:
		wordType := ir.GetType(nil, ctx.wordSize)
		ctx.addInstr(&ir.Instruction{Op: ir.OpCEq, Typ: wordType, Result: res, Args: []ir.Value{val, &ir.Const{Value: 0}}})
	case token.Complement:
		wordType := ir.GetType(nil, ctx.wordSize)
		ctx.addInstr(&ir.Instruction{Op: ir.OpXor, Typ: wordType, Result: res, Args: []ir.Value{val, &ir.Const{Value: -1}}})
	case token.Inc, token.Dec: // Prefix
		lvalAddr := ctx.codegenLvalue(d.Expr)
		op := map[token.Type]ir.Op{token.Inc: ir.OpAdd, token.Dec: ir.OpSub}[d.Op]
		currentVal := ctx.genLoad(lvalAddr, d.Expr.Typ)
		ctx.addInstr(&ir.Instruction{Op: op, Typ: valType, Result: res, Args: []ir.Value{currentVal, &ir.Const{Value: 1}}})
		ctx.genStore(lvalAddr, res, d.Expr.Typ)
	default:
		util.Error(node.Tok, "Unsupported unary operator")
	}
	return res, false
}

func (ctx *Context) codegenPostfixOp(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.PostfixOpNode)
	lvalAddr := ctx.codegenLvalue(d.Expr)
	res := ctx.genLoad(lvalAddr, d.Expr.Typ)

	newVal := ctx.newTemp()
	op := map[token.Type]ir.Op{token.Inc: ir.OpAdd, token.Dec: ir.OpSub}[d.Op]
	valType := ir.GetType(d.Expr.Typ, ctx.wordSize)
	ctx.addInstr(&ir.Instruction{Op: op, Typ: valType, Result: newVal, Args: []ir.Value{res, &ir.Const{Value: 1}}})
	ctx.genStore(lvalAddr, newVal, d.Expr.Typ)
	return res, false
}

func (ctx *Context) codegenIndirection(node *ast.Node) (ir.Value, bool) {
	exprNode := node.Data.(ast.IndirectionNode).Expr
	addr, _ := ctx.codegenExpr(exprNode)
	loadType := node.Typ
	if !ctx.isTypedPass && exprNode.Type == ast.Ident {
		if sym := ctx.findSymbol(exprNode.Data.(ast.IdentNode).Name); sym != nil && sym.IsByteArray {
			loadType = ast.TypeByte
		}
	}
	return ctx.genLoad(addr, loadType), false
}

func (ctx *Context) codegenSubscriptAddr(node *ast.Node) ir.Value {
	d := node.Data.(ast.SubscriptNode)
	arrayPtr, _ := ctx.codegenExpr(d.Array)
	indexVal, _ := ctx.codegenExpr(d.Index)

	var scale int64 = int64(ctx.wordSize)
	if d.Array.Typ != nil {
		if d.Array.Typ.Kind == ast.TYPE_POINTER || d.Array.Typ.Kind == ast.TYPE_ARRAY {
			if d.Array.Typ.Base != nil {
				scale = ctx.getSizeof(d.Array.Typ.Base)
			}
		}
	} else if !ctx.isTypedPass && d.Array.Type == ast.Ident {
		if sym := ctx.findSymbol(d.Array.Data.(ast.IdentNode).Name); sym != nil && sym.IsByteArray {
			scale = 1
		}
	}

	var scaledIndex ir.Value = indexVal
	if scale > 1 {
		scaledIndex = ctx.newTemp()
		ctx.addInstr(&ir.Instruction{
			Op:     ir.OpMul,
			Typ:    ir.GetType(nil, ctx.wordSize),
			Result: scaledIndex,
			Args:   []ir.Value{indexVal, &ir.Const{Value: scale}},
		})
	}

	resultAddr := ctx.newTemp()
	ctx.addInstr(&ir.Instruction{
		Op:     ir.OpAdd,
		Typ:    ir.GetType(nil, ctx.wordSize),
		Result: resultAddr,
		Args:   []ir.Value{arrayPtr, scaledIndex},
	})
	return resultAddr
}

func (ctx *Context) codegenAddressOf(node *ast.Node) (ir.Value, bool) {
	lvalNode := node.Data.(ast.AddressOfNode).LValue
	if lvalNode.Type == ast.Ident {
		name := lvalNode.Data.(ast.IdentNode).Name
		if sym := ctx.findSymbol(name); sym != nil {
			isTypedArray := sym.BxType != nil && sym.BxType.Kind == ast.TYPE_ARRAY
			if sym.Type == symFunc || isTypedArray {
				return sym.IRVal, false
			}
			if sym.IsVector {
				res, _ := ctx.codegenExpr(lvalNode)
				return res, false
			}
		}
	}
	return ctx.codegenLvalue(lvalNode), false
}

func (ctx *Context) codegenFuncCall(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.FuncCallNode)
	if d.FuncExpr.Type == ast.Ident {
		name := d.FuncExpr.Data.(ast.IdentNode).Name
		if sym := ctx.findSymbol(name); sym != nil && sym.Type == symVar && !sym.IsVector {
			util.Error(d.FuncExpr.Tok, "'%s' is a variable but is used as a function", name)
		}
	}

	argVals := make([]ir.Value, len(d.Args))
	argTypes := make([]ir.Type, len(d.Args))
	for i := len(d.Args) - 1; i >= 0; i-- {
		argVals[i], _ = ctx.codegenExpr(d.Args[i])
		argTypes[i] = ir.GetType(d.Args[i].Typ, ctx.wordSize)
	}
	funcVal, _ := ctx.codegenExpr(d.FuncExpr)

	isStmt := node.Parent != nil && node.Parent.Type == ast.Block
	returnType := ir.GetType(node.Typ, ctx.wordSize)

	var res ir.Value
	if !isStmt && returnType != ir.TypeNone {
		res = ctx.newTemp()
	}

	callArgs := append([]ir.Value{funcVal}, argVals...)
	ctx.addInstr(&ir.Instruction{
		Op:       ir.OpCall,
		Typ:      returnType,
		Result:   res,
		Args:     callArgs,
		ArgTypes: argTypes,
	})
	return res, false
}

func (ctx *Context) codegenTernary(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.TernaryNode)
	thenL, elseL, endL := ctx.newLabel(), ctx.newLabel(), ctx.newLabel()
	res := ctx.newTemp()
	resType := ir.GetType(node.Typ, ctx.wordSize)

	ctx.codegenLogicalCond(d.Cond, thenL, elseL)

	ctx.startBlock(thenL)
	thenVal, thenTerminates := ctx.codegenExpr(d.ThenExpr)
	var thenPred *ir.Label
	if !thenTerminates {
		if ir.GetType(d.ThenExpr.Typ, ctx.wordSize) != resType {
			castedVal := ctx.newTemp()
			ctx.addInstr(&ir.Instruction{Op: ir.OpCast, Typ: resType, Result: castedVal, Args: []ir.Value{thenVal}})
			thenVal = castedVal
		}
		thenPred = ctx.currentBlock.Label
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endL}})
	}

	ctx.startBlock(elseL)
	elseVal, elseTerminates := ctx.codegenExpr(d.ElseExpr)
	var elsePred *ir.Label
	if !elseTerminates {
		if ir.GetType(d.ElseExpr.Typ, ctx.wordSize) != resType {
			castedVal := ctx.newTemp()
			ctx.addInstr(&ir.Instruction{Op: ir.OpCast, Typ: resType, Result: castedVal, Args: []ir.Value{elseVal}})
			elseVal = castedVal
		}
		elsePred = ctx.currentBlock.Label
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endL}})
	}

	terminates := thenTerminates && elseTerminates
	if !terminates {
		ctx.startBlock(endL)
		phiArgs := []ir.Value{}
		if !thenTerminates {
			phiArgs = append(phiArgs, thenPred, thenVal)
		}
		if !elseTerminates {
			phiArgs = append(phiArgs, elsePred, elseVal)
		}
		ctx.addInstr(&ir.Instruction{Op: ir.OpPhi, Typ: resType, Result: res, Args: phiArgs})
	}
	return res, terminates
}

func (ctx *Context) codegenAutoAlloc(node *ast.Node) (ir.Value, bool) {
	d := node.Data.(ast.AutoAllocNode)
	sizeVal, _ := ctx.codegenExpr(d.Size)
	res := ctx.newTemp()

	sizeInBytes := ctx.newTemp()
	wordType := ir.GetType(nil, ctx.wordSize)
	ctx.addInstr(&ir.Instruction{
		Op:     ir.OpMul,
		Typ:    wordType,
		Result: sizeInBytes,
		Args:   []ir.Value{sizeVal, &ir.Const{Value: int64(ctx.wordSize)}},
	})

	ctx.addInstr(&ir.Instruction{
		Op:     ir.OpAlloc,
		Typ:    wordType,
		Result: res,
		Args:   []ir.Value{sizeInBytes},
		Align:  ctx.stackAlign,
	})
	return res, false
}

// Helper functions for codegenStmt

func (ctx *Context) codegenReturn(node *ast.Node) bool {
	var retVal ir.Value
	d := node.Data.(ast.ReturnNode)
	if d.Expr != nil {
		retVal, _ = ctx.codegenExpr(d.Expr)
	} else if ctx.currentFunc != nil && ctx.currentFunc.ReturnType != ir.TypeNone {
		retVal = &ir.Const{Value: 0}
	}
	ctx.addInstr(&ir.Instruction{Op: ir.OpRet, Args: []ir.Value{retVal}})
	ctx.currentBlock = nil
	return true
}

func (ctx *Context) codegenIf(node *ast.Node) bool {
	d := node.Data.(ast.IfNode)
	thenL, endL := ctx.newLabel(), ctx.newLabel()
	elseL := endL
	if d.ElseBody != nil {
		elseL = ctx.newLabel()
	}

	ctx.codegenLogicalCond(d.Cond, thenL, elseL)

	ctx.startBlock(thenL)
	thenTerminates := ctx.codegenStmt(d.ThenBody)
	if !thenTerminates {
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endL}})
	}

	var elseTerminates bool
	if d.ElseBody != nil {
		ctx.startBlock(elseL)
		elseTerminates = ctx.codegenStmt(d.ElseBody)
		if !elseTerminates {
			ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endL}})
		}
	}

	if !thenTerminates || !elseTerminates {
		ctx.startBlock(endL)
	}
	return thenTerminates && (d.ElseBody != nil && elseTerminates)
}

func (ctx *Context) codegenWhile(node *ast.Node) bool {
	d := node.Data.(ast.WhileNode)
	startL, bodyL, endL := ctx.newLabel(), ctx.newLabel(), ctx.newLabel()

	oldBreak, oldContinue := ctx.breakLabel, ctx.continueLabel
	ctx.breakLabel, ctx.continueLabel = endL, startL
	defer func() { ctx.breakLabel, ctx.continueLabel = oldBreak, oldContinue }()

	ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{startL}})
	ctx.startBlock(startL)
	ctx.codegenLogicalCond(d.Cond, bodyL, endL)

	ctx.startBlock(bodyL)
	bodyTerminates := ctx.codegenStmt(d.Body)
	if !bodyTerminates {
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{startL}})
	}

	ctx.startBlock(endL)
	return false
}

func (ctx *Context) codegenSwitch(node *ast.Node) bool {
	d := node.Data.(ast.SwitchNode)
	switchVal, _ := ctx.codegenExpr(d.Expr)
	endLabel := ctx.newLabel()

	oldBreak := ctx.breakLabel
	ctx.breakLabel = endLabel
	defer func() { ctx.breakLabel = oldBreak }()

	ctx.switchStack = append(ctx.switchStack, &switchContext{Node: &d, CaseIndex: 0})
	defer func() { ctx.switchStack = ctx.switchStack[:len(ctx.switchStack)-1] }()

	var defaultTarget *ir.Label
	if d.DefaultLabelName != "" {
		labelName := strings.TrimLeft(d.DefaultLabelName, "@")
		defaultTarget = &ir.Label{Name: labelName}
	} else {
		defaultTarget = endLabel
	}

	wordType := ir.GetType(nil, ctx.wordSize)
	for _, caseLabelInfo := range d.CaseLabels {
		caseValConst := &ir.Const{Value: caseLabelInfo.Value}
		cmpRes := ctx.newTemp()
		nextCheckLabel := ctx.newLabel()
		labelName := strings.TrimLeft(caseLabelInfo.LabelName, "@")
		caseTargetLabel := &ir.Label{Name: labelName}

		ctx.addInstr(&ir.Instruction{Op: ir.OpCEq, Typ: wordType, Result: cmpRes, Args: []ir.Value{switchVal, caseValConst}})
		ctx.addInstr(&ir.Instruction{Op: ir.OpJnz, Args: []ir.Value{cmpRes, caseTargetLabel, nextCheckLabel}})
		ctx.startBlock(nextCheckLabel)
	}
	ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{defaultTarget}})
	ctx.currentBlock = nil

	bodyTerminates := ctx.codegenStmt(d.Body)

	if ctx.currentBlock != nil {
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{endLabel}})
	}

	ctx.startBlock(endLabel)
	return bodyTerminates && d.DefaultLabelName != ""
}

func (ctx *Context) codegenCaseOrDefault(node *ast.Node) bool {
	var body *ast.Node
	var labelName string

	if node.Type == ast.Case {
		d := node.Data.(ast.CaseNode)
		body = d.Body
		if len(ctx.switchStack) > 0 {
			info := ctx.switchStack[len(ctx.switchStack)-1]
			if info.CaseIndex < len(info.Node.CaseLabels) {
				labelName = info.Node.CaseLabels[info.CaseIndex].LabelName
				info.CaseIndex++
			}
		}
	} else { // Default
		d := node.Data.(ast.DefaultNode)
		body = d.Body
		if len(ctx.switchStack) > 0 {
			info := ctx.switchStack[len(ctx.switchStack)-1]
			labelName = info.Node.DefaultLabelName
		}
	}

	if labelName != "" {
		label := &ir.Label{Name: strings.TrimLeft(labelName, "@")}
		if ctx.currentBlock != nil {
			ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{label}})
		}
		ctx.startBlock(label)
	}

	return ctx.codegenStmt(body)
}

// Helper for op mapping
func getBinaryOpAndType(op token.Type, astType *ast.BxType, wordSize int) (ir.Op, ir.Type) {
	typ := ir.GetType(astType, wordSize)
	switch op {
	case token.Plus, token.PlusEq, token.EqPlus:
		return ir.OpAdd, typ
	case token.Minus, token.MinusEq, token.EqMinus:
		return ir.OpSub, typ
	case token.Star, token.StarEq, token.EqStar:
		return ir.OpMul, typ
	case token.Slash, token.SlashEq, token.EqSlash:
		return ir.OpDiv, typ
	case token.Rem, token.RemEq, token.EqRem:
		return ir.OpRem, typ
	case token.And, token.AndEq, token.EqAnd:
		return ir.OpAnd, typ
	case token.Or, token.OrEq, token.EqOr:
		return ir.OpOr, typ
	case token.Xor, token.XorEq, token.EqXor:
		return ir.OpXor, typ
	case token.Shl, token.ShlEq, token.EqShl:
		return ir.OpShl, typ
	case token.Shr, token.ShrEq, token.EqShr:
		return ir.OpShr, typ
	case token.EqEq:
		return ir.OpCEq, typ
	case token.Neq:
		return ir.OpCNeq, typ
	case token.Lt:
		return ir.OpCLt, typ
	case token.Gt:
		return ir.OpCGt, typ
	case token.Lte:
		return ir.OpCLe, typ
	case token.Gte:
		return ir.OpCGe, typ
	}
	return -1, -1
}
