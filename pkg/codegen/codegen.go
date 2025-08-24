package codegen

import (
	"fmt"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
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
	IRVal       ir.Value
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

type autoVarInfo struct {
	Node *ast.Node
	Size int64
}

type switchContext struct {
	Node      *ast.SwitchNode
	CaseIndex int
}

// Context holds the state for the codegen pass
type Context struct {
	prog          *ir.Program
	inlineAsm     string
	tempCount     int
	labelCount    int
	currentScope  *scope
	currentFunc   *ir.Func
	currentBlock  *ir.BasicBlock
	breakLabel    *ir.Label
	continueLabel *ir.Label
	wordSize      int
	stackAlign    int
	isTypedPass   bool
	cfg           *config.Config
	switchStack   []*switchContext
}

// NewContext creates a new codegen context
func NewContext(cfg *config.Config) *Context {
	return &Context{
		prog: &ir.Program{
			Strings:    make(map[string]string),
			ExtrnFuncs: make([]string, 0),
			ExtrnVars:  make(map[string]bool),
			WordSize:   cfg.WordSize,
		},
		currentScope: newScope(nil),
		wordSize:     cfg.WordSize,
		stackAlign:   cfg.StackAlignment,
		isTypedPass:  cfg.IsFeatureEnabled(config.FeatTyped),
		cfg:          cfg,
		switchStack:  make([]*switchContext, 0),
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
	var irVal ir.Value
	switch symType {
	case symVar:
		if ctx.currentScope.Parent == nil { // Global
			irVal = &ir.Global{Name: name}
		} else { // Local
			irVal = &ir.Temporary{Name: name, ID: ctx.tempCount}
			ctx.tempCount++
		}
	case symFunc, symExtrn:
		irVal = &ir.Global{Name: name}
	case symLabel:
		irVal = &ir.Label{Name: name}
	case symType:
		// Types don't have a direct IR value in this model
	}

	sym := &symbol{
		Name:     name,
		Type:     symType,
		BxType:   bxType,
		IRVal:    irVal,
		IsVector: isVector,
		Next:     ctx.currentScope.Symbols,
		Node:     node,
	}
	ctx.currentScope.Symbols = sym
	return sym
}

func (ctx *Context) newTemp() *ir.Temporary {
	t := &ir.Temporary{ID: ctx.tempCount}
	ctx.tempCount++
	return t
}

func (ctx *Context) newLabel() *ir.Label {
	l := &ir.Label{Name: fmt.Sprintf("L%d", ctx.labelCount)}
	ctx.labelCount++
	return l
}

func (ctx *Context) startBlock(label *ir.Label) {
	block := &ir.BasicBlock{Label: label}
	ctx.currentFunc.Blocks = append(ctx.currentFunc.Blocks, block)
	ctx.currentBlock = block
}

func (ctx *Context) addInstr(instr *ir.Instruction) {
	if ctx.currentBlock == nil {
		ctx.startBlock(ctx.newLabel())
	}
	ctx.currentBlock.Instructions = append(ctx.currentBlock.Instructions, instr)
}

func (ctx *Context) addString(value string) ir.Value {
	if label, ok := ctx.prog.Strings[value]; ok {
		return &ir.Global{Name: label}
	}
	label := fmt.Sprintf("str%d", len(ctx.prog.Strings))
	ctx.prog.Strings[value] = label
	return &ir.Global{Name: label}
}

// evalConstExpr evaluates a compile-time constant expression node to an integer
func (ctx *Context) evalConstExpr(node *ast.Node) (int64, bool) {
	if node == nil {
		return 0, false
	}
	folded := ast.FoldConstants(node)
	if folded.Type == ast.Number {
		return folded.Data.(ast.NumberNode).Value, true
	}
	if folded.Type == ast.Ident {
		identName := folded.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(identName)
		if sym != nil && sym.Node != nil && sym.Node.Type == ast.VarDecl {
			decl := sym.Node.Data.(ast.VarDeclNode)
			if len(decl.InitList) == 1 {
				// Prevent infinite recursion on `auto x = x;`
				if decl.InitList[0] == node {
					return 0, false
				}
				return ctx.evalConstExpr(decl.InitList[0])
			}
		}
	}
	return 0, false
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
			if val, ok := ctx.evalConstExpr(typ.ArraySize); ok {
				arrayLen = val
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

// GenerateIR translates the entire AST into an IR program
func (ctx *Context) GenerateIR(root *ast.Node) (*ir.Program, string) {
	ctx.collectGlobals(root)
	ctx.collectStrings(root)
	if !ctx.isTypedPass {
		ctx.findByteArrays(root)
	}
	ctx.codegenStmt(root)

	ctx.prog.BackendTempCount = ctx.tempCount
	return ctx.prog, ctx.inlineAsm
}

// walkAST provides a generic way to traverse the AST
func walkAST(node *ast.Node, visitor func(n *ast.Node)) {
	if node == nil {
		return
	}
	visitor(node)

	switch d := node.Data.(type) {
	case ast.AssignNode:
		walkAST(d.Lhs, visitor)
		walkAST(d.Rhs, visitor)
	case ast.BinaryOpNode:
		walkAST(d.Left, visitor)
		walkAST(d.Right, visitor)
	case ast.UnaryOpNode:
		walkAST(d.Expr, visitor)
	case ast.PostfixOpNode:
		walkAST(d.Expr, visitor)
	case ast.IndirectionNode:
		walkAST(d.Expr, visitor)
	case ast.AddressOfNode:
		walkAST(d.LValue, visitor)
	case ast.TernaryNode:
		walkAST(d.Cond, visitor)
		walkAST(d.ThenExpr, visitor)
		walkAST(d.ElseExpr, visitor)
	case ast.SubscriptNode:
		walkAST(d.Array, visitor)
		walkAST(d.Index, visitor)
	case ast.FuncCallNode:
		walkAST(d.FuncExpr, visitor)
		for _, arg := range d.Args {
			walkAST(arg, visitor)
		}
	case ast.FuncDeclNode:
		walkAST(d.Body, visitor)
	case ast.VarDeclNode:
		for _, init := range d.InitList {
			walkAST(init, visitor)
		}
		walkAST(d.SizeExpr, visitor)
	case ast.MultiVarDeclNode:
		for _, decl := range d.Decls {
			walkAST(decl, visitor)
		}
	case ast.IfNode:
		walkAST(d.Cond, visitor)
		walkAST(d.ThenBody, visitor)
		walkAST(d.ElseBody, visitor)
	case ast.WhileNode:
		walkAST(d.Cond, visitor)
		walkAST(d.Body, visitor)
	case ast.ReturnNode:
		walkAST(d.Expr, visitor)
	case ast.BlockNode:
		for _, s := range d.Stmts {
			walkAST(s, visitor)
		}
	case ast.SwitchNode:
		walkAST(d.Expr, visitor)
		walkAST(d.Body, visitor)
	case ast.CaseNode:
		walkAST(d.Value, visitor)
		walkAST(d.Body, visitor)
	case ast.DefaultNode:
		walkAST(d.Body, visitor)
	case ast.LabelNode:
		walkAST(d.Stmt, visitor)
	}
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
				isAlreadyExtrn := false
				for _, extrnName := range ctx.prog.ExtrnFuncs {
					if extrnName == name {
						isAlreadyExtrn = true
						break
					}
				}
				if !isAlreadyExtrn {
					ctx.prog.ExtrnFuncs = append(ctx.prog.ExtrnFuncs, name)
				}
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
		visitor := func(n *ast.Node) {
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
				if d.Lhs.Type != ast.Ident {
					return
				}
				lhsSym := ctx.findSymbol(d.Lhs.Data.(ast.IdentNode).Name)
				if lhsSym == nil || lhsSym.IsByteArray {
					return
				}
				rhsIsByteArray := false
				switch d.Rhs.Type {
				case ast.String:
					rhsIsByteArray = true
				case ast.Ident:
					if rhsSym := ctx.findSymbol(d.Rhs.Data.(ast.IdentNode).Name); rhsSym != nil && rhsSym.IsByteArray {
						rhsIsByteArray = true
					}
				}
				if rhsIsByteArray {
					lhsSym.IsByteArray = true
					changedInPass = true
				}
			}
		}
		walkAST(root, visitor)
		if !changedInPass {
			break
		}
	}
}

func (ctx *Context) collectStrings(root *ast.Node) {
	walkAST(root, func(n *ast.Node) {
		if n != nil && n.Type == ast.String {
			ctx.addString(n.Data.(ast.StringNode).Value)
		}
	})
}

func (ctx *Context) genLoad(addr ir.Value, typ *ast.BxType) ir.Value {
	res := ctx.newTemp()
	loadType := ir.GetType(typ, ctx.wordSize)
	ctx.addInstr(&ir.Instruction{Op: ir.OpLoad, Typ: loadType, Result: res, Args: []ir.Value{addr}})
	return res
}

func (ctx *Context) genStore(addr, value ir.Value, typ *ast.BxType) {
	storeType := ir.GetType(typ, ctx.wordSize)
	ctx.addInstr(&ir.Instruction{Op: ir.OpStore, Typ: storeType, Args: []ir.Value{value, addr}})
}

func (ctx *Context) codegenLvalue(node *ast.Node) ir.Value {
	if node == nil {
		util.Error(token.Token{}, "Internal error: null l-value node in codegen")
		return nil
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
			return nil
		}
		if sym.BxType != nil && sym.BxType.Kind == ast.TYPE_ARRAY {
			return sym.IRVal
		}
		if sym.IsVector && sym.Node != nil && sym.Node.Type == ast.VarDecl {
			d := sym.Node.Data.(ast.VarDeclNode)
			if !d.IsBracketed && len(d.InitList) <= 1 && d.Type == nil {
				util.Error(node.Tok, "Cannot assign to '%s', it is a constant.", name)
				return nil
			}
		}
		return sym.IRVal

	case ast.Indirection:
		res, _ := ctx.codegenExpr(node.Data.(ast.IndirectionNode).Expr)
		return res

	case ast.Subscript:
		return ctx.codegenSubscriptAddr(node)

	default:
		util.Error(node.Tok, "Expression is not a valid l-value.")
		return nil
	}
}

func (ctx *Context) codegenLogicalCond(node *ast.Node, trueL, falseL *ir.Label) {
	if node.Type == ast.BinaryOp {
		d := node.Data.(ast.BinaryOpNode)
		if d.Op == token.OrOr {
			newFalseL := ctx.newLabel()
			ctx.codegenLogicalCond(d.Left, trueL, newFalseL)
			ctx.startBlock(newFalseL)
			ctx.codegenLogicalCond(d.Right, trueL, falseL)
			return
		}
		if d.Op == token.AndAnd {
			newTrueL := ctx.newLabel()
			ctx.codegenLogicalCond(d.Left, newTrueL, falseL)
			ctx.startBlock(newTrueL)
			ctx.codegenLogicalCond(d.Right, trueL, falseL)
			return
		}
	}

	condVal, _ := ctx.codegenExpr(node)
	ctx.addInstr(&ir.Instruction{Op: ir.OpJnz, Args: []ir.Value{condVal, trueL, falseL}})
	ctx.currentBlock = nil // This block is terminated
}

func (ctx *Context) codegenExpr(node *ast.Node) (result ir.Value, terminates bool) {
	if node == nil {
		return &ir.Const{Value: 0}, false
	}

	switch node.Type {
	case ast.Number:
		return &ir.Const{Value: node.Data.(ast.NumberNode).Value}, false
	case ast.String:
		return ctx.addString(node.Data.(ast.StringNode).Value), false
	case ast.Ident:
		return ctx.codegenIdent(node)
	case ast.Assign:
		return ctx.codegenAssign(node)
	case ast.BinaryOp:
		return ctx.codegenBinaryOp(node)
	case ast.UnaryOp:
		return ctx.codegenUnaryOp(node)
	case ast.PostfixOp:
		return ctx.codegenPostfixOp(node)
	case ast.Indirection:
		return ctx.codegenIndirection(node)
	case ast.Subscript:
		addr := ctx.codegenSubscriptAddr(node)
		return ctx.genLoad(addr, node.Typ), false
	case ast.AddressOf:
		return ctx.codegenAddressOf(node)
	case ast.FuncCall:
		return ctx.codegenFuncCall(node)
	case ast.Ternary:
		return ctx.codegenTernary(node)
	case ast.AutoAlloc:
		return ctx.codegenAutoAlloc(node)
	}
	util.Error(node.Tok, "Internal error: unhandled expression type in codegen: %v", node.Type)
	return nil, true
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
				ctx.currentBlock = nil
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
		return ctx.codegenReturn(node)
	case ast.If:
		return ctx.codegenIf(node)
	case ast.While:
		return ctx.codegenWhile(node)
	case ast.Switch:
		return ctx.codegenSwitch(node)
	case ast.Label:
		d := node.Data.(ast.LabelNode)
		label := &ir.Label{Name: d.Name}
		if ctx.currentBlock != nil {
			ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{label}})
		}
		ctx.startBlock(label)
		return ctx.codegenStmt(d.Stmt)

	case ast.Goto:
		d := node.Data.(ast.GotoNode)
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{&ir.Label{Name: d.Label}}})
		ctx.currentBlock = nil
		return true

	case ast.Break:
		if ctx.breakLabel == nil {
			util.Error(node.Tok, "'break' not in a loop or switch.")
		}
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{ctx.breakLabel}})
		ctx.currentBlock = nil
		return true

	case ast.Continue:
		if ctx.continueLabel == nil {
			util.Error(node.Tok, "'continue' not in a loop.")
		}
		ctx.addInstr(&ir.Instruction{Op: ir.OpJmp, Args: []ir.Value{ctx.continueLabel}})
		ctx.currentBlock = nil
		return true

	case ast.Case, ast.Default:
		return ctx.codegenCaseOrDefault(node)

	default:
		// Any other node type is treated as an expression statement
		_, terminates := ctx.codegenExpr(node)
		return terminates
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
					// Dope vector: 1 word for the pointer to data
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
		ctx.inlineAsm += fmt.Sprintf(".globl %s\n%s:\n\t%s\n", d.Name, d.Name, asmCode)
		return
	}
	if d.Body == nil {
		return
	}

	fn := &ir.Func{
		Name:       d.Name,
		ReturnType: ir.GetType(d.ReturnType, ctx.wordSize),
		HasVarargs: d.HasVarargs,
	}
	ctx.prog.Funcs = append(ctx.prog.Funcs, fn)

	prevFunc := ctx.currentFunc
	ctx.currentFunc = fn
	defer func() { ctx.currentFunc = prevFunc }()

	ctx.enterScope()
	defer ctx.exitScope()

	ctx.tempCount = 0
	ctx.startBlock(&ir.Label{Name: "start"})

	for i, p := range d.Params {
		var name string
		var typ *ast.BxType
		if d.IsTyped {
			paramData := p.Data.(ast.VarDeclNode)
			name, typ = paramData.Name, paramData.Type
		} else {
			name = p.Data.(ast.IdentNode).Name
		}
		paramVal := &ir.Temporary{Name: name, ID: i}
		fn.Params = append(fn.Params, &ir.Param{
			Name: name,
			Typ:  ir.GetType(typ, ctx.wordSize),
			Val:  paramVal,
		})
	}

	// Determine stack layout
	var paramInfos []autoVarInfo
	for _, p := range d.Params {
		paramInfos = append(paramInfos, autoVarInfo{Node: p, Size: int64(ctx.wordSize)})
	}
	for i, j := 0, len(paramInfos)-1; i < j; i, j = i+1, j-1 {
		paramInfos[i], paramInfos[j] = paramInfos[j], paramInfos[i]
	}

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
	var autoVars []autoVarInfo
	ctx.findAllAutosInFunc(d.Body, &autoVars, definedInFunc)

	for i, j := 0, len(autoVars)-1; i < j; i, j = i+1, j-1 {
		autoVars[i], autoVars[j] = autoVars[j], autoVars[i]
	}

	allLocals := append(paramInfos, autoVars...)

	var totalFrameSize int64
	for _, av := range allLocals {
		totalFrameSize += av.Size
	}

	var framePtr ir.Value
	if totalFrameSize > 0 {
		align := int64(ctx.stackAlign)
		totalFrameSize = (totalFrameSize + align - 1) &^ (align - 1)
		framePtr = ctx.newTemp()
		ctx.addInstr(&ir.Instruction{
			Op:     ir.OpAlloc,
			Typ:    ir.GetType(nil, ctx.wordSize),
			Result: framePtr,
			Args:   []ir.Value{&ir.Const{Value: totalFrameSize}},
			Align:  ctx.stackAlign,
		})
	}

	var currentOffset int64
	for i, local := range allLocals {
		isParam := i < len(paramInfos)

		var name string
		var typ *ast.BxType
		var isVec bool
		if local.Node.Type == ast.Ident { // Untyped param
			name = local.Node.Data.(ast.IdentNode).Name
			if d.Name == "main" && isParam {
				originalIndex := -1
				for j, p := range d.Params {
					if p == local.Node {
						originalIndex = j
						break
					}
				}
				if originalIndex == 1 {
					isVec = true
				}
			}
		} else { // Typed param or auto var
			varData := local.Node.Data.(ast.VarDeclNode)
			name, typ, isVec = varData.Name, varData.Type, varData.IsVector
		}

		sym := ctx.addSymbol(name, symVar, typ, isVec, local.Node)
		sym.StackOffset = currentOffset

		addr := ctx.newTemp()
		ctx.addInstr(&ir.Instruction{
			Op:     ir.OpAdd,
			Typ:    ir.GetType(nil, ctx.wordSize),
			Result: addr,
			Args:   []ir.Value{framePtr, &ir.Const{Value: currentOffset}},
		})
		sym.IRVal = addr

		if isParam {
			var origParamIndex int = -1
			for j, p := range d.Params {
				if p == local.Node {
					origParamIndex = j
					break
				}
			}

			if origParamIndex != -1 {
				paramVal := fn.Params[origParamIndex].Val
				ctx.genStore(sym.IRVal, paramVal, typ)
			}
		} else { // Is an auto var
			if isVec && (typ == nil || typ.Kind == ast.TYPE_UNTYPED) {
				storageAddr := ctx.newTemp()
				ctx.addInstr(&ir.Instruction{
					Op:     ir.OpAdd,
					Typ:    ir.GetType(nil, ctx.wordSize),
					Result: storageAddr,
					Args:   []ir.Value{addr, &ir.Const{Value: int64(ctx.wordSize)}},
				})
				ctx.genStore(addr, storageAddr, nil)
			}
		}
		currentOffset += local.Size
	}

	bodyTerminates := ctx.codegenStmt(d.Body)

	if !bodyTerminates {
		if d.ReturnType != nil && d.ReturnType.Kind == ast.TYPE_VOID {
			ctx.addInstr(&ir.Instruction{Op: ir.OpRet})
		} else {
			ctx.addInstr(&ir.Instruction{Op: ir.OpRet, Args: []ir.Value{&ir.Const{Value: 0}}})
		}
	}
}

func (ctx *Context) codegenGlobalConst(node *ast.Node) ir.Value {
	folded := ast.FoldConstants(node)
	switch folded.Type {
	case ast.Number:
		return &ir.Const{Value: folded.Data.(ast.NumberNode).Value}
	case ast.String:
		return ctx.addString(folded.Data.(ast.StringNode).Value)
	case ast.Ident:
		name := folded.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			util.Error(node.Tok, "Undefined symbol '%s' in global initializer.", name)
			return nil
		}
		return sym.IRVal
	case ast.AddressOf:
		lval := folded.Data.(ast.AddressOfNode).LValue
		if lval.Type != ast.Ident {
			util.Error(lval.Tok, "Global initializer must be the address of a global symbol.")
			return nil
		}
		name := lval.Data.(ast.IdentNode).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			util.Error(lval.Tok, "Undefined symbol '%s' in global initializer.", name)
			return nil
		}
		return sym.IRVal
	default:
		util.Error(node.Tok, "Global initializer must be a constant expression.")
		return nil
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
		vectorPtr, _ := ctx.codegenExpr(&ast.Node{Type: ast.Ident, Data: ast.IdentNode{Name: d.Name}, Tok: sym.Node.Tok})

		if len(d.InitList) == 1 && d.InitList[0].Type == ast.String {
			strVal := d.InitList[0].Data.(ast.StringNode).Value
			strLabel := ctx.addString(strVal)
			sizeToCopy := len(strVal) + 1
			ctx.addInstr(&ir.Instruction{
				Op:   ir.OpBlit,
				Args: []ir.Value{strLabel, vectorPtr, &ir.Const{Value: int64(sizeToCopy)}},
			})
		} else {
			for i, initExpr := range d.InitList {
				offset := int64(i) * int64(ctx.wordSize)
				elemAddr := ctx.newTemp()
				ctx.addInstr(&ir.Instruction{
					Op:     ir.OpAdd,
					Typ:    ir.GetType(nil, ctx.wordSize),
					Result: elemAddr,
					Args:   []ir.Value{vectorPtr, &ir.Const{Value: offset}},
				})
				rval, _ := ctx.codegenExpr(initExpr)
				ctx.genStore(elemAddr, rval, nil)
			}
		}
		return
	}

	rval, _ := ctx.codegenExpr(d.InitList[0])
	ctx.genStore(sym.IRVal, rval, d.Type)
}

func (ctx *Context) codegenGlobalVarDecl(d ast.VarDeclNode, sym *symbol) {
	globalData := &ir.Data{
		Name:  sym.IRVal.(*ir.Global).Name,
		Align: ctx.wordSize,
	}

	isUntypedStringVec := d.IsVector && (d.Type == nil || d.Type.Kind == ast.TYPE_UNTYPED) &&
		len(d.InitList) == 1 && d.InitList[0].Type == ast.String

	var elemType ir.Type
	if isUntypedStringVec {
		elemType = ir.TypeB
	} else {
		elemType = ir.GetType(d.Type, ctx.wordSize)
		if d.Type != nil && d.Type.Kind == ast.TYPE_ARRAY {
			elemType = ir.GetType(d.Type.Base, ctx.wordSize)
		}
	}

	var numElements int64
	var sizeNode *ast.Node
	if d.Type != nil && d.Type.Kind == ast.TYPE_ARRAY {
		sizeNode = d.Type.ArraySize
	} else if d.IsVector {
		sizeNode = d.SizeExpr
	}

	if sizeNode != nil {
		if val, ok := ctx.evalConstExpr(sizeNode); ok {
			numElements = val
		} else {
			util.Error(sizeNode.Tok, "Global array size must be a constant expression.")
		}
	} else {
		numElements = int64(len(d.InitList))
	}
	if numElements == 0 && !d.IsVector && len(d.InitList) == 0 {
		numElements = 1
	}

	if len(d.InitList) > 0 {
		for _, init := range d.InitList {
			val := ctx.codegenGlobalConst(init)
			itemType := elemType
			if _, ok := val.(*ir.Global); ok {
				itemType = ir.TypePtr
			}
			globalData.Items = append(globalData.Items, ir.DataItem{Typ: itemType, Value: val})
		}
		initializedElements := int64(len(d.InitList))
		if numElements > initializedElements {
			globalData.Items = append(globalData.Items, ir.DataItem{Typ: elemType, Count: int(numElements - initializedElements)})
		}
	} else if numElements > 0 {
		globalData.Items = append(globalData.Items, ir.DataItem{Typ: elemType, Count: int(numElements)})
	}

	if len(globalData.Items) > 0 {
		ctx.prog.Globals = append(ctx.prog.Globals, globalData)
	}
}
