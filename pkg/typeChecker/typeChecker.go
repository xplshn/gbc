package typeChecker

import (
	"fmt"
	"strings"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/util"
)

type Symbol struct {
	Name   string
	Type   *ast.BxType
	IsFunc bool
	IsType bool
	Node   *ast.Node
	Next   *Symbol
}

type Scope struct {
	Symbols *Symbol
	Parent  *Scope
}

type TypeChecker struct {
	currentScope *Scope
	currentFunc  *ast.FuncDeclNode
	globalScope  *Scope
	cfg          *config.Config
	resolving    map[*ast.BxType]bool
	wordSize     int
}

func NewTypeChecker(cfg *config.Config) *TypeChecker {
	globalScope := newScope(nil)
	return &TypeChecker{
		currentScope: globalScope,
		globalScope:  globalScope,
		cfg:          cfg,
		resolving:    make(map[*ast.BxType]bool),
		wordSize:     cfg.WordSize,
	}
}

func newScope(parent *Scope) *Scope { return &Scope{Parent: parent} }
func (tc *TypeChecker) enterScope()    { tc.currentScope = newScope(tc.currentScope) }
func (tc *TypeChecker) exitScope() {
	if tc.currentScope.Parent != nil {
		tc.currentScope = tc.currentScope.Parent
	}
}

func (tc *TypeChecker) addSymbol(node *ast.Node) *Symbol {
	var name string
	var typ *ast.BxType
	isFunc, isType := false, false

	switch d := node.Data.(type) {
	case ast.VarDeclNode:
		name, typ = d.Name, d.Type
	case ast.FuncDeclNode:
		name, typ, isFunc = d.Name, d.ReturnType, true
	case ast.TypeDeclNode:
		name, typ, isType = d.Name, d.Type, true
	case ast.ExtrnDeclNode:
		for _, nameNode := range d.Names {
			ident := nameNode.Data.(ast.IdentNode)
			if tc.findSymbol(ident.Name, false) == nil {
				sym := &Symbol{Name: ident.Name, Type: ast.TypeUntyped, IsFunc: true, Node: node, Next: tc.currentScope.Symbols}
				tc.currentScope.Symbols = sym
			}
		}
		return nil
	case ast.IdentNode: // untyped function parameters
		name, typ = d.Name, ast.TypeUntyped
	default:
		return nil
	}

	if typ == nil {
		typ = ast.TypeUntyped
	}

	if existing := tc.findSymbol(name, isType); existing != nil && tc.currentScope == tc.globalScope {
		isExistingExtrn := existing.Node != nil && existing.Node.Type == ast.ExtrnDecl
		if isExistingExtrn || (existing.IsFunc && !isFunc && existing.Type.Kind == ast.TYPE_UNTYPED) {
			existing.Type, existing.IsFunc, existing.IsType, existing.Node = typ, isFunc, isType, node
			return existing
		}
		util.Warn(tc.cfg, config.WarnExtra, node.Tok, "Redefinition of '%s'", name)
		existing.Type, existing.IsFunc, existing.IsType, existing.Node = typ, isFunc, isType, node
		return existing
	}

	sym := &Symbol{Name: name, Type: typ, IsFunc: isFunc, IsType: isType, Node: node, Next: tc.currentScope.Symbols}
	tc.currentScope.Symbols = sym
	return sym
}

func (tc *TypeChecker) findSymbol(name string, findTypes bool) *Symbol {
	for s := tc.currentScope; s != nil; s = s.Parent {
		for sym := s.Symbols; sym != nil; sym = sym.Next {
			if sym.Name == name && sym.IsType == findTypes {
				return sym
			}
		}
	}
	return nil
}

func (tc *TypeChecker) getSizeof(typ *ast.BxType) int64 {
	if typ == nil || typ.Kind == ast.TYPE_UNTYPED {
		return int64(tc.wordSize)
	}
	switch typ.Kind {
	case ast.TYPE_VOID:
		return 0
	case ast.TYPE_POINTER:
		return int64(tc.wordSize)
	case ast.TYPE_ARRAY:
		elemSize := tc.getSizeof(typ.Base)
		var arrayLen int64 = 1
		if typ.ArraySize != nil {
			if folded := ast.FoldConstants(typ.ArraySize); folded.Type == ast.Number {
				arrayLen = folded.Data.(ast.NumberNode).Value
			} else {
				util.Error(typ.ArraySize.Tok, "Array size must be a constant expression.")
			}
		}
		return elemSize * arrayLen
	case ast.TYPE_PRIMITIVE:
		switch typ.Name {
		case "int", "uint", "string":
			return int64(tc.wordSize)
		case "int64", "uint64":
			return 8
		case "int32", "uint32":
			return 4
		case "int16", "uint16":
			return 2
		case "byte", "bool", "int8", "uint8":
			return 1
		default:
			if sym := tc.findSymbol(typ.Name, true); sym != nil {
				return tc.getSizeof(sym.Type)
			}
			return int64(tc.wordSize)
		}
	case ast.TYPE_STRUCT:
		var totalSize int64
		for _, field := range typ.Fields {
			totalSize += tc.getSizeof(field.Data.(ast.VarDeclNode).Type)
		}
		// NOTE: does not account for alignment/padding
		return totalSize
	}
	return int64(tc.wordSize)
}

func (tc *TypeChecker) Check(root *ast.Node) {
	if !tc.cfg.IsFeatureEnabled(config.FeatTyped) {
		return
	}
	tc.collectGlobals(root)
	tc.checkNode(root)
	tc.annotateGlobalDecls(root)
}

func (tc *TypeChecker) collectGlobals(node *ast.Node) {
	if node == nil || node.Type != ast.Block {
		return
	}
	for _, stmt := range node.Data.(ast.BlockNode).Stmts {
		switch stmt.Type {
		case ast.VarDecl, ast.FuncDecl, ast.ExtrnDecl, ast.TypeDecl:
			tc.addSymbol(stmt)
		case ast.MultiVarDecl:
			for _, subStmt := range stmt.Data.(ast.MultiVarDeclNode).Decls {
				tc.addSymbol(subStmt)
			}
		}
	}
}

func (tc *TypeChecker) annotateGlobalDecls(root *ast.Node) {
	if root == nil || root.Type != ast.Block {
		return
	}
	for _, stmt := range root.Data.(ast.BlockNode).Stmts {
		if stmt.Type == ast.VarDecl {
			d, ok := stmt.Data.(ast.VarDeclNode)
			if !ok {
				continue
			}
			if globalSym := tc.findSymbol(d.Name, false); globalSym != nil {
				if (d.Type == nil || d.Type.Kind == ast.TYPE_UNTYPED) && (globalSym.Type != nil && globalSym.Type.Kind != ast.TYPE_UNTYPED) {
					d.Type = globalSym.Type
					stmt.Data, stmt.Typ = d, globalSym.Type
				}
			}
		}
	}
}

func (tc *TypeChecker) checkNode(node *ast.Node) {
	if node == nil {
		return
	}
	switch node.Type {
	case ast.Block:
		d := node.Data.(ast.BlockNode)
		if !d.IsSynthetic {
			tc.enterScope()
		}
		for _, stmt := range d.Stmts {
			tc.checkNode(stmt)
		}
		if !d.IsSynthetic {
			tc.exitScope()
		}
	case ast.FuncDecl:
		tc.checkFuncDecl(node)
	case ast.VarDecl:
		tc.checkVarDecl(node)
	case ast.MultiVarDecl:
		for _, decl := range node.Data.(ast.MultiVarDeclNode).Decls {
			tc.checkVarDecl(decl)
		}
	case ast.If:
		d := node.Data.(ast.IfNode)
		tc.checkExprAsCondition(d.Cond)
		tc.checkNode(d.ThenBody)
		tc.checkNode(d.ElseBody)
	case ast.While:
		d := node.Data.(ast.WhileNode)
		tc.checkExprAsCondition(d.Cond)
		tc.checkNode(d.Body)
	case ast.Return:
		tc.checkReturn(node)
	case ast.Switch:
		d := node.Data.(ast.SwitchNode)
		tc.checkExprAsCondition(d.Expr)
		tc.checkNode(d.Body)
	case ast.Case:
		tc.checkExpr(node.Data.(ast.CaseNode).Value)
		tc.checkNode(node.Data.(ast.CaseNode).Body)
	case ast.Default:
		tc.checkNode(node.Data.(ast.DefaultNode).Body)
	case ast.Label:
		tc.checkNode(node.Data.(ast.LabelNode).Stmt)
	case ast.ExtrnDecl:
		tc.addSymbol(node)
	case ast.TypeDecl, ast.Goto, ast.Break, ast.Continue, ast.AsmStmt, ast.Directive:
	default:
		if node.Type <= ast.TypeCast {
			tc.checkExpr(node)
		}
	}
}

func (tc *TypeChecker) checkFuncDecl(node *ast.Node) {
	d := node.Data.(ast.FuncDeclNode)
	if d.Body == nil || d.Body.Type == ast.AsmStmt {
		return
	}
	prevFunc := tc.currentFunc
	tc.currentFunc = &d
	defer func() { tc.currentFunc = prevFunc }()
	tc.enterScope()
	for _, pNode := range d.Params {
		tc.addSymbol(pNode)
	}
	tc.checkNode(d.Body)
	tc.exitScope()
}

func (tc *TypeChecker) checkVarDecl(node *ast.Node) {
	d := node.Data.(ast.VarDeclNode)
	if d.IsDefine && tc.findSymbol(d.Name, false) != nil {
		util.Error(node.Tok, "No new variables on left side of :=")
	}
	if tc.currentFunc != nil {
		tc.addSymbol(node)
	}
	if len(d.InitList) == 0 {
		if (d.Type == nil || d.Type.Kind == ast.TYPE_UNTYPED) && !tc.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
			util.Error(node.Tok, "Uninitialized variable '%s' is not allowed in this mode", d.Name)
		}
		node.Typ = d.Type
		return
	}

	initExpr := d.InitList[0]
	initType := tc.checkExpr(initExpr)
	if initType == nil || initType.Kind == ast.TYPE_UNTYPED {
		return
	}

	if d.Type == nil || d.Type.Kind == ast.TYPE_UNTYPED {
		d.Type = initType
		node.Data = d
		if sym := tc.findSymbol(d.Name, false); sym != nil {
			sym.Type = initType
		}
	}
	if !tc.areTypesCompatible(d.Type, initType, initExpr) {
		util.Warn(tc.cfg, config.WarnType, node.Tok, "Initializing variable of type '%s' with expression of incompatible type '%s'", typeToString(d.Type), typeToString(initType))
	}
	node.Typ = d.Type
}

func (tc *TypeChecker) checkReturn(node *ast.Node) {
	d := node.Data.(ast.ReturnNode)
	if tc.currentFunc == nil {
		if d.Expr != nil {
			util.Error(node.Tok, "Return with value used outside of a function.")
		}
		return
	}
	if !tc.currentFunc.IsTyped {
		tc.checkExpr(d.Expr)
		if d.Expr == nil {
			tc.currentFunc.ReturnType = ast.TypeVoid
			if sym := tc.findSymbol(tc.currentFunc.Name, false); sym != nil {
				sym.Type = ast.TypeVoid
			}
		}
		return
	}

	retType := tc.currentFunc.ReturnType
	if d.Expr == nil {
		if retType.Kind != ast.TYPE_VOID {
			util.Error(node.Tok, "Return with no value in function returning non-void type ('%s')", typeToString(retType))
		}
	} else {
		exprType := tc.checkExpr(d.Expr)
		if retType.Kind == ast.TYPE_VOID {
			util.Error(node.Tok, "Return with a value in function returning void")
		} else if !tc.areTypesCompatible(retType, exprType, d.Expr) {
			util.Warn(tc.cfg, config.WarnType, node.Tok, "Returning type '%s' is incompatible with function return type '%s'", typeToString(exprType), typeToString(retType))
		}
	}
}

func (tc *TypeChecker) checkExprAsCondition(node *ast.Node) {
	typ := tc.checkExpr(node)
	if !(tc.isScalarType(typ) || typ.Kind == ast.TYPE_UNTYPED) {
		util.Warn(tc.cfg, config.WarnType, node.Tok, "Expression of type '%s' used as a condition", typeToString(typ))
	}
}

func (tc *TypeChecker) checkExpr(node *ast.Node) *ast.BxType {
	if node == nil {
		return ast.TypeUntyped
	}
	if node.Typ != nil {
		return node.Typ
	}
	var typ *ast.BxType
	switch d := node.Data.(type) {
	case ast.AssignNode:
		lhsType, rhsType := tc.checkExpr(d.Lhs), tc.checkExpr(d.Rhs)

		// Handle type promotion on assignment (e.g., int var = ptr_val).
		// This is common in B where variables can change type implicitly.
		isLhsScalar := tc.isScalarType(lhsType) && lhsType.Kind != ast.TYPE_POINTER
		isRhsPtr := rhsType != nil && rhsType.Kind == ast.TYPE_POINTER
		if isLhsScalar && isRhsPtr && d.Lhs.Type == ast.Ident {
			if sym := tc.findSymbol(d.Lhs.Data.(ast.IdentNode).Name, false); sym != nil {
				sym.Type = rhsType // Promote the variable's type to the pointer type
				lhsType = rhsType
			}
		}

		if d.Lhs.Type == ast.Subscript {
			subscript := d.Lhs.Data.(ast.SubscriptNode)
			arrayExpr := subscript.Array
			if arrayExpr.Typ != nil && arrayExpr.Typ.Kind == ast.TYPE_POINTER && arrayExpr.Typ.Base.Kind == ast.TYPE_UNTYPED {
				arrayExpr.Typ.Base = rhsType
				lhsType = rhsType
				if arrayExpr.Type == ast.Ident {
					if sym := tc.findSymbol(arrayExpr.Data.(ast.IdentNode).Name, false); sym != nil {
						sym.Type = arrayExpr.Typ
					}
				}
			}
		}
		if lhsType.Kind == ast.TYPE_UNTYPED && d.Lhs.Type == ast.Ident {
			if sym := tc.findSymbol(d.Lhs.Data.(ast.IdentNode).Name, false); sym != nil {
				sym.Type, sym.IsFunc = rhsType, false
				lhsType = rhsType
			}
		}
		if !tc.areTypesCompatible(lhsType, rhsType, d.Rhs) {
			util.Warn(tc.cfg, config.WarnType, node.Tok, "Assigning to type '%s' from incompatible type '%s'", typeToString(lhsType), typeToString(rhsType))
		}
		typ = lhsType
	case ast.BinaryOpNode:
		leftType, rightType := tc.checkExpr(d.Left), tc.checkExpr(d.Right)
		typ = tc.getBinaryOpResultType(d.Op, leftType, rightType, node.Tok)
	case ast.UnaryOpNode:
		operandType := tc.checkExpr(d.Expr)
		switch d.Op {
		case token.Star: // Dereference
			resolvedOpType := tc.resolveType(operandType)
			if resolvedOpType.Kind == ast.TYPE_POINTER || resolvedOpType.Kind == ast.TYPE_ARRAY {
				typ = resolvedOpType.Base
			} else if resolvedOpType.Kind == ast.TYPE_UNTYPED || tc.isIntegerType(resolvedOpType) {
				// An untyped or integer variable is being dereferenced.
				// Very common pattern in B. Promote it to a pointer to an untyped base
				promotedType := &ast.BxType{Kind: ast.TYPE_POINTER, Base: ast.TypeUntyped}
				d.Expr.Typ = promotedType
				if d.Expr.Type == ast.Ident {
					if sym := tc.findSymbol(d.Expr.Data.(ast.IdentNode).Name, false); sym != nil {
						if sym.Type == nil || sym.Type.Kind == ast.TYPE_UNTYPED || tc.isIntegerType(sym.Type) {
							sym.Type = promotedType
						}
					}
				}
				typ = promotedType.Base
			} else {
				util.Error(node.Tok, "Cannot dereference non-pointer type '%s'", typeToString(operandType))
				typ = ast.TypeUntyped
			}
		case token.And: // Address-of
			typ = &ast.BxType{Kind: ast.TYPE_POINTER, Base: operandType}
		default: // ++, --, -, +, !, ~
			typ = operandType
		}
	case ast.PostfixOpNode:
		typ = tc.checkExpr(d.Expr)
	case ast.TernaryNode:
		tc.checkExprAsCondition(d.Cond)
		thenType, elseType := tc.checkExpr(d.ThenExpr), tc.checkExpr(d.ElseExpr)
		if !tc.areTypesCompatible(thenType, elseType, d.ElseExpr) {
			util.Warn(tc.cfg, config.WarnType, node.Tok, "Type mismatch in ternary expression branches ('%s' vs '%s')", typeToString(thenType), typeToString(elseType))
		}
		// Type promotion rules for ternary operator: pointer types take precedence.
		if thenType != nil && thenType.Kind == ast.TYPE_POINTER {
			typ = thenType
		} else if elseType != nil && elseType.Kind == ast.TYPE_POINTER {
			typ = elseType
		} else {
			typ = thenType // Default to 'then' type if no pointers involved
		}
	case ast.SubscriptNode:
		arrayType, indexType := tc.checkExpr(d.Array), tc.checkExpr(d.Index)
		if !tc.isIntegerType(indexType) && indexType.Kind != ast.TYPE_UNTYPED {
			util.Warn(tc.cfg, config.WarnType, d.Index.Tok, "Array subscript is not an integer type ('%s')", typeToString(indexType))
		}
		resolvedArrayType := tc.resolveType(arrayType)
		if resolvedArrayType.Kind == ast.TYPE_ARRAY || resolvedArrayType.Kind == ast.TYPE_POINTER {
			typ = resolvedArrayType.Base
		} else if resolvedArrayType.Kind == ast.TYPE_UNTYPED || tc.isIntegerType(resolvedArrayType) {
			// An untyped or integer variable is being used as a pointer
			// Another super common pattern in B. Promote it to a pointer to an untyped base
			// The base type will be inferred from usage (e.g., assignment)
			promotedType := &ast.BxType{Kind: ast.TYPE_POINTER, Base: ast.TypeUntyped}
			d.Array.Typ = promotedType

			if d.Array.Type == ast.Ident {
				if sym := tc.findSymbol(d.Array.Data.(ast.IdentNode).Name, false); sym != nil {
					if sym.Type == nil || sym.Type.Kind == ast.TYPE_UNTYPED || tc.isIntegerType(sym.Type) {
						sym.Type = promotedType
					}
				}
			}
			typ = promotedType.Base
		} else {
			util.Error(node.Tok, "Cannot subscript non-array/pointer type '%s'", typeToString(arrayType))
			typ = ast.TypeUntyped
		}
	case ast.MemberAccessNode:
		typ = tc.checkMemberAccess(node)
	case ast.FuncCallNode:
		typ = tc.checkFuncCall(node)
	case ast.TypeCastNode:
		tc.checkExpr(d.Expr)
		typ = d.TargetType
	case ast.NumberNode:
		typ = ast.TypeInt
	case ast.StringNode:
		typ = ast.TypeString
	case ast.IdentNode:
		if sym := tc.findSymbol(d.Name, false); sym != nil {
			if node.Parent != nil && node.Parent.Type == ast.FuncCall && node.Parent.Data.(ast.FuncCallNode).FuncExpr == node && !sym.IsFunc {
				sym.IsFunc, sym.Type = true, ast.TypeInt
			}
			t := sym.Type
			if t != nil && t.Kind == ast.TYPE_ARRAY {
				typ = &ast.BxType{Kind: ast.TYPE_POINTER, Base: t.Base, IsConst: t.IsConst}
			} else {
				typ = t
			}
		} else {
			util.Warn(tc.cfg, config.WarnImplicitDecl, node.Tok, "Implicit declaration of variable '%s'", d.Name)
			sym := tc.addSymbol(ast.NewVarDecl(node.Tok, d.Name, ast.TypeUntyped, nil, nil, false, false, false))
			typ = sym.Type
		}
	default:
		typ = ast.TypeUntyped
	}
	node.Typ = typ
	return typ
}

func (tc *TypeChecker) checkMemberAccess(node *ast.Node) *ast.BxType {
	d := node.Data.(ast.MemberAccessNode)
	exprType := tc.checkExpr(d.Expr)
	baseType := exprType
	if exprType.Kind == ast.TYPE_POINTER {
		baseType = exprType.Base
	}
	resolvedBaseType := tc.resolveType(baseType)
	if resolvedBaseType.Kind != ast.TYPE_STRUCT {
		util.Error(node.Tok, "Request for member '%s' in non-struct type", d.Member.Data.(ast.IdentNode).Name)
		return ast.TypeUntyped
	}

	var offset int64
	var memberType *ast.BxType
	found := false
	memberName := d.Member.Data.(ast.IdentNode).Name
	for _, fieldNode := range resolvedBaseType.Fields {
		fieldData := fieldNode.Data.(ast.VarDeclNode)
		if fieldData.Name == memberName {
			memberType, found = fieldData.Type, true
			break
		}
		offset += tc.getSizeof(fieldData.Type)
	}
	if !found {
		util.Error(node.Tok, "No member named '%s' in struct '%s'", memberName, typeToString(resolvedBaseType))
		return ast.TypeUntyped
	}

	var structAddrNode *ast.Node
	if exprType.Kind == ast.TYPE_POINTER {
		structAddrNode = d.Expr
	} else {
		structAddrNode = ast.NewAddressOf(d.Expr.Tok, d.Expr)
		tc.checkExpr(structAddrNode)
	}

	offsetNode := ast.NewNumber(d.Member.Tok, offset)
	offsetNode.Typ = ast.TypeInt
	addNode := ast.NewBinaryOp(node.Tok, token.Plus, structAddrNode, offsetNode)
	addNode.Typ = &ast.BxType{Kind: ast.TYPE_POINTER, Base: memberType}
	node.Type, node.Data, node.Typ = ast.Indirection, ast.IndirectionNode{Expr: addNode}, memberType
	return memberType
}

func (tc *TypeChecker) checkFuncCall(node *ast.Node) *ast.BxType {
	d := node.Data.(ast.FuncCallNode)
	if d.FuncExpr.Type == ast.Ident {
		name := d.FuncExpr.Data.(ast.IdentNode).Name
		if name == "sizeof" {
			if len(d.Args) != 1 {
				util.Error(node.Tok, "sizeof expects exactly one argument")
				return ast.TypeUntyped
			}
			arg := d.Args[0]
			var targetType *ast.BxType
			if arg.Type == ast.Ident {
				if sym := tc.findSymbol(arg.Data.(ast.IdentNode).Name, true); sym != nil && sym.IsType {
					targetType = sym.Type
				}
			}
			if targetType == nil {
				targetType = tc.checkExpr(arg)
			}
			if targetType == nil {
				util.Error(arg.Tok, "Cannot determine type for sizeof argument")
				return ast.TypeUntyped
			}
			node.Type, node.Data, node.Typ = ast.Number, ast.NumberNode{Value: tc.getSizeof(targetType)}, ast.TypeInt
			return ast.TypeInt
		}
	}

	if len(d.Args) == 1 {
		if sizeArgCall, ok := d.Args[0].Data.(ast.FuncCallNode); ok && sizeArgCall.FuncExpr.Type == ast.Ident && sizeArgCall.FuncExpr.Data.(ast.IdentNode).Name == "sizeof" {
			if len(sizeArgCall.Args) == 1 {
				sizeofArg := sizeArgCall.Args[0]
				var targetType *ast.BxType
				if sizeofArg.Type == ast.Ident {
					if sym := tc.findSymbol(sizeofArg.Data.(ast.IdentNode).Name, true); sym != nil && sym.IsType {
						targetType = sym.Type
					}
				} else {
					targetType = tc.checkExpr(sizeofArg)
				}
				if targetType != nil {
					node.Typ = &ast.BxType{Kind: ast.TYPE_POINTER, Base: targetType}
					tc.checkExpr(d.FuncExpr)
					for _, arg := range d.Args {
						tc.checkExpr(arg)
					}
					return node.Typ
				}
			}
		}
	}

	if d.FuncExpr.Type == ast.Ident {
		name := d.FuncExpr.Data.(ast.IdentNode).Name
		if sym := tc.findSymbol(name, false); sym == nil {
			util.Warn(tc.cfg, config.WarnImplicitDecl, d.FuncExpr.Tok, "Implicit declaration of function '%s'", name)
			tc.globalScope.Symbols = &Symbol{Name: name, Type: ast.TypeInt, IsFunc: true, Node: d.FuncExpr, Next: tc.globalScope.Symbols}
		} else {
			sym.IsFunc = true
		}
	}
	funcExprType := tc.checkExpr(d.FuncExpr)
	for _, arg := range d.Args {
		tc.checkExpr(arg)
	}
	return funcExprType
}

func (tc *TypeChecker) getBinaryOpResultType(op token.Type, left, right *ast.BxType, tok token.Token) *ast.BxType {
	resLeft, resRight := tc.resolveType(left), tc.resolveType(right)
	if resLeft.Kind == ast.TYPE_UNTYPED {
		return resRight
	}
	if resRight.Kind == ast.TYPE_UNTYPED {
		return resLeft
	}
	if op >= token.EqEq && op <= token.OrOr {
		return ast.TypeInt
	}

	switch op {
	case token.Plus, token.Minus:
		if resLeft.Kind == ast.TYPE_POINTER && tc.isIntegerType(resRight) {
			return resLeft
		}
		if tc.isIntegerType(resLeft) && resRight.Kind == ast.TYPE_POINTER && op == token.Plus {
			return resRight
		}
		if op == token.Minus && resLeft.Kind == ast.TYPE_POINTER && resRight.Kind == ast.TYPE_POINTER {
			return ast.TypeInt
		}
	}

	if tc.isNumericType(resLeft) && tc.isNumericType(resRight) {
		if resLeft.Kind == ast.TYPE_FLOAT || resRight.Kind == ast.TYPE_FLOAT {
			return ast.TypeFloat
		}
		return ast.TypeInt
	}

	util.Warn(tc.cfg, config.WarnType, tok, "Invalid binary operation between types '%s' and '%s'", typeToString(left), typeToString(right))
	return ast.TypeInt
}

func (tc *TypeChecker) areTypesCompatible(a, b *ast.BxType, bNode *ast.Node) bool {
	if a == nil || b == nil || a.Kind == ast.TYPE_UNTYPED || b.Kind == ast.TYPE_UNTYPED {
		return true
	}
	resA, resB := tc.resolveType(a), tc.resolveType(b)
	if resA.Kind == resB.Kind {
		switch resA.Kind {
		case ast.TYPE_POINTER:
			if (resA.Base != nil && resA.Base.Kind == ast.TYPE_VOID) || (resB.Base != nil && resB.Base.Kind == ast.TYPE_VOID) {
				return true
			}
			if (resA.Base != nil && resA.Base == ast.TypeByte) || (resB.Base != nil && resB.Base == ast.TypeByte) {
				return true
			}
			return tc.areTypesCompatible(resA.Base, resB.Base, nil)
		case ast.TYPE_ARRAY:
			return tc.areTypesCompatible(resA.Base, resB.Base, nil)
		case ast.TYPE_STRUCT:
			return resA == resB || (resA.Name != "" && resA.Name == resB.Name)
		default:
			return true
		}
	}
	if bNode != nil && bNode.Type == ast.Number && bNode.Data.(ast.NumberNode).Value == 0 && resA.Kind == ast.TYPE_POINTER && tc.isIntegerType(resB) {
		return true
	}
	if resA.Kind == ast.TYPE_POINTER && resB.Kind == ast.TYPE_ARRAY {
		return tc.areTypesCompatible(resA.Base, resB.Base, nil)
	}
	if tc.isNumericType(resA) && tc.isNumericType(resB) {
		return true
	}
	if (resA.Kind == ast.TYPE_BOOL && tc.isScalarType(resB)) || (tc.isScalarType(resA) && resB.Kind == ast.TYPE_BOOL) {
		return true
	}
	return false
}

func (tc *TypeChecker) resolveType(typ *ast.BxType) *ast.BxType {
	if typ == nil {
		return ast.TypeUntyped
	}
	if tc.resolving[typ] {
		return typ
	}
	tc.resolving[typ] = true
	defer func() { delete(tc.resolving, typ) }()
	if (typ.Kind == ast.TYPE_PRIMITIVE || typ.Kind == ast.TYPE_STRUCT) && typ.Name != "" {
		if sym := tc.findSymbol(typ.Name, true); sym != nil {
			resolved := tc.resolveType(sym.Type)
			if typ.IsConst {
				newType := *resolved
				newType.IsConst = true
				return &newType
			}
			return resolved
		}
	}
	return typ
}

func (tc *TypeChecker) isIntegerType(t *ast.BxType) bool {
	if t == nil {
		return false
	}
	resolved := tc.resolveType(t)
	return resolved.Kind == ast.TYPE_PRIMITIVE && resolved.Name != "float" && resolved.Name != "float32" && resolved.Name != "float64"
}

func (tc *TypeChecker) isFloatType(t *ast.BxType) bool {
	if t == nil {
		return false
	}
	return tc.resolveType(t).Kind == ast.TYPE_FLOAT
}

func (tc *TypeChecker) isNumericType(t *ast.BxType) bool { return tc.isIntegerType(t) || tc.isFloatType(t) }
func (tc *TypeChecker) isScalarType(t *ast.BxType) bool {
	if t == nil {
		return false
	}
	resolved := tc.resolveType(t)
	return tc.isNumericType(resolved) || resolved.Kind == ast.TYPE_POINTER || resolved.Kind == ast.TYPE_BOOL
}

func typeToString(t *ast.BxType) string {
	if t == nil {
		return "<nil>"
	}
	var sb strings.Builder
	if t.IsConst {
		sb.WriteString("const ")
	}
	switch t.Kind {
	case ast.TYPE_PRIMITIVE, ast.TYPE_BOOL, ast.TYPE_FLOAT:
		sb.WriteString(t.Name)
	case ast.TYPE_POINTER:
		sb.WriteString(typeToString(t.Base))
		sb.WriteString("*")
	case ast.TYPE_ARRAY:
		sb.WriteString("[]")
		sb.WriteString(typeToString(t.Base))
	case ast.TYPE_STRUCT:
		sb.WriteString("struct ")
		if t.Name != "" {
			sb.WriteString(t.Name)
		} else if t.StructTag != "" {
			sb.WriteString(t.StructTag)
		} else {
			sb.WriteString("<anonymous>")
		}
	case ast.TYPE_VOID:
		sb.WriteString("void")
	case ast.TYPE_UNTYPED:
		sb.WriteString("untyped")
	default:
		sb.WriteString(fmt.Sprintf("<unknown_type_kind_%d>", t.Kind))
	}
	return sb.String()
}
