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

func (tc *TypeChecker) typeErrorOrWarn(tok token.Token, format string, args ...interface{}) {
	if tc.cfg.IsFeatureEnabled(config.FeatStrictTypes) {
		util.Error(tok, format, args...)
	} else {
		util.Warn(tc.cfg, config.WarnType, tok, format, args...)
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
	case ast.EnumDeclNode:
		name, isType = d.Name, true
		typ = &ast.BxType{Kind: ast.TYPE_ENUM, Name: d.Name, EnumMembers: d.Members, Base: ast.TypeInt}
		for _, memberNode := range d.Members {
			memberData := memberNode.Data.(ast.VarDeclNode)
			if tc.findSymbol(memberData.Name, false) == nil {
				memberSym := &Symbol{
					Name: memberData.Name, Type: ast.TypeInt, Node: memberNode, Next: tc.currentScope.Symbols,
				}
				tc.currentScope.Symbols = memberSym
			} else {
				util.Warn(tc.cfg, config.WarnExtra, memberNode.Tok, "Redefinition of '%s' in enum", memberData.Name)
			}
		}
	case ast.ExtrnDeclNode:
		for _, nameNode := range d.Names {
			ident := nameNode.Data.(ast.IdentNode)
			if tc.findSymbol(ident.Name, false) == nil {
				sym := &Symbol{Name: ident.Name, Type: ast.TypeUntyped, IsFunc: true, Node: node, Next: tc.currentScope.Symbols}
				tc.currentScope.Symbols = sym
			}
		}
		return nil
	case ast.IdentNode:
		name, typ = d.Name, ast.TypeUntyped
	default:
		return nil
	}

	if typ == nil { typ = ast.TypeUntyped }

	if existing := tc.findSymbol(name, isType); existing != nil && tc.currentScope == tc.globalScope {
		isExistingExtrn := existing.Node != nil && existing.Node.Type == ast.ExtrnDecl
		if isExistingExtrn || (existing.IsFunc && !isFunc && existing.Type.Kind == ast.TYPE_UNTYPED) {
			existing.Type, existing.IsFunc, existing.IsType, existing.Node = typ, isFunc, isType, node
			return existing
		}
		util.Error(node.Tok, "Redefinition of '%s'", name)
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

func (tc *TypeChecker) getAlignof(typ *ast.BxType) int64 {
	if typ == nil { return int64(tc.wordSize) }

	if (typ.Kind == ast.TYPE_PRIMITIVE || typ.Kind == ast.TYPE_STRUCT) && typ.Name != "" {
		if sym := tc.findSymbol(typ.Name, true); sym != nil {
			if sym.Type != typ { return tc.getAlignof(sym.Type) }
		}
	}

	if typ.Kind == ast.TYPE_UNTYPED { return int64(tc.wordSize) }
	switch typ.Kind {
	case ast.TYPE_VOID: return 1
	case ast.TYPE_POINTER: return int64(tc.wordSize)
	case ast.TYPE_ARRAY: return tc.getAlignof(typ.Base)
	case ast.TYPE_PRIMITIVE, ast.TYPE_FLOAT, ast.TYPE_ENUM: return tc.getSizeof(typ)
	case ast.TYPE_STRUCT:
		var maxAlign int64 = 1
		for _, field := range typ.Fields {
			fieldAlign := tc.getAlignof(field.Data.(ast.VarDeclNode).Type)
			if fieldAlign > maxAlign { maxAlign = fieldAlign }
		}
		return maxAlign
	}
	return int64(tc.wordSize)
}

func (tc *TypeChecker) getSizeof(typ *ast.BxType) int64 {
	if typ == nil || typ.Kind == ast.TYPE_UNTYPED { return int64(tc.wordSize) }
	switch typ.Kind {
	case ast.TYPE_VOID: return 0
	case ast.TYPE_POINTER: return int64(tc.wordSize)
	case ast.TYPE_ARRAY:
		elemSize := tc.getSizeof(typ.Base)
		var arrayLen int64 = 1
		if typ.ArraySize != nil {
			if folded := ast.FoldConstants(typ.ArraySize); folded.Type == ast.Number {
				arrayLen = folded.Data.(ast.NumberNode).Value
			} else {
				util.Error(typ.ArraySize.Tok, "Array size must be a constant expression")
			}
		}
		return elemSize * arrayLen
	case ast.TYPE_PRIMITIVE, ast.TYPE_UNTYPED_INT:
		switch typ.Name {
		case "int", "uint", "string": return int64(tc.wordSize)
		case "int64", "uint64": return 8
		case "int32", "uint32": return 4
		case "int16", "uint16": return 2
		case "byte", "bool", "int8", "uint8": return 1
		default:
			if sym := tc.findSymbol(typ.Name, true); sym != nil { return tc.getSizeof(sym.Type) }
			return int64(tc.wordSize)
		}
	case ast.TYPE_ENUM: return tc.getSizeof(ast.TypeInt)
	case ast.TYPE_FLOAT, ast.TYPE_UNTYPED_FLOAT:
		switch typ.Name {
		case "float", "float32": return 4
		case "float64": return 8
		default: return 4
		}
	case ast.TYPE_STRUCT:
		var totalSize, maxAlign int64 = 0, 1
		for _, field := range typ.Fields {
			fieldData := field.Data.(ast.VarDeclNode)
			fieldAlign := tc.getAlignof(fieldData.Type)
			if fieldAlign > maxAlign { maxAlign = fieldAlign }
			totalSize = util.AlignUp(totalSize, fieldAlign)
			totalSize += tc.getSizeof(fieldData.Type)
		}
		if maxAlign == 0 { maxAlign = 1 }
		return util.AlignUp(totalSize, maxAlign)
	}
	return int64(tc.wordSize)
}

func (tc *TypeChecker) Check(root *ast.Node) {
	if !tc.cfg.IsFeatureEnabled(config.FeatTyped) { return }
	tc.collectGlobals(root)
	tc.checkNode(root)
	tc.annotateGlobalDecls(root)
}

func (tc *TypeChecker) collectGlobals(node *ast.Node) {
	if node == nil || node.Type != ast.Block { return }
	for _, stmt := range node.Data.(ast.BlockNode).Stmts {
		switch stmt.Type {
		case ast.VarDecl, ast.FuncDecl, ast.ExtrnDecl, ast.TypeDecl, ast.EnumDecl:
			tc.addSymbol(stmt)
		case ast.MultiVarDecl:
			for _, subStmt := range stmt.Data.(ast.MultiVarDeclNode).Decls {
				tc.addSymbol(subStmt)
			}
		}
	}
}

func (tc *TypeChecker) annotateGlobalDecls(root *ast.Node) {
	if root == nil || root.Type != ast.Block { return }
	for _, stmt := range root.Data.(ast.BlockNode).Stmts {
		if stmt.Type == ast.VarDecl {
			d, ok := stmt.Data.(ast.VarDeclNode)
			if !ok { continue }
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
	if node == nil { return }
	switch node.Type {
	case ast.Block:
		d := node.Data.(ast.BlockNode)
		if !d.IsSynthetic { tc.enterScope() }
		for _, stmt := range d.Stmts {
			tc.checkNode(stmt)
		}
		if !d.IsSynthetic { tc.exitScope() }
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
		tc.checkExpr(d.Expr)
		tc.checkNode(d.Body)
	case ast.Case:
		for _, valueExpr := range node.Data.(ast.CaseNode).Values {
			tc.checkExpr(valueExpr)
		}
		tc.checkNode(node.Data.(ast.CaseNode).Body)
	case ast.Default:
		tc.checkNode(node.Data.(ast.DefaultNode).Body)
	case ast.Label:
		tc.checkNode(node.Data.(ast.LabelNode).Stmt)
	case ast.ExtrnDecl:
		tc.addSymbol(node)
	case ast.TypeDecl, ast.EnumDecl, ast.Goto, ast.Break, ast.Continue, ast.AsmStmt, ast.Directive:
	default:
		if node.Type <= ast.StructLiteral {
			tc.checkExpr(node)
		}
	}
}

func (tc *TypeChecker) checkFuncDecl(node *ast.Node) {
	d := node.Data.(ast.FuncDeclNode)
	if d.Body == nil || d.Body.Type == ast.AsmStmt { return }
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
		util.Error(node.Tok, "Trying to assign to undeclared identifier, use := or define with a explicit type or auto")
	}
	if tc.currentFunc != nil { tc.addSymbol(node) }
	if len(d.InitList) == 0 {
		if (d.Type == nil || d.Type.Kind == ast.TYPE_UNTYPED) && !tc.cfg.IsFeatureEnabled(config.FeatAllowUninitialized) {
			util.Error(node.Tok, "Uninitialized variable '%s' is not allowed in this mode", d.Name)
		}
		node.Typ = d.Type
		return
	}

	initExpr := d.InitList[0]
	initType := tc.checkExpr(initExpr)
	if initType == nil { return }

	if d.Type == nil || d.Type.Kind == ast.TYPE_UNTYPED {
		d.Type = initType
		node.Data = d
		if sym := tc.findSymbol(d.Name, false); sym != nil {
			sym.Type = initType
		}
	} else if initType.Kind == ast.TYPE_UNTYPED_INT || initType.Kind == ast.TYPE_UNTYPED_FLOAT {
		if tc.isNumericType(d.Type) || d.Type.Kind == ast.TYPE_POINTER || d.Type.Kind == ast.TYPE_BOOL {
			initExpr.Typ = d.Type
			initType = d.Type
		}
	}
	if !tc.areTypesCompatible(d.Type, initType, initExpr) {
		tc.typeErrorOrWarn(node.Tok, "Initializing variable of type '%s' with expression of incompatible type '%s'", typeToString(d.Type), typeToString(initType))
	}
	node.Typ = d.Type
}

func (tc *TypeChecker) isSymbolLocal(name string) bool {
	for s := tc.currentScope; s != nil && s != tc.globalScope; s = s.Parent {
		for sym := s.Symbols; sym != nil; sym = sym.Next {
			if sym.Name == name && !sym.IsType { return true }
		}
	}
	return false
}

func (tc *TypeChecker) checkReturn(node *ast.Node) {
	d := node.Data.(ast.ReturnNode)
	if tc.currentFunc == nil {
		if d.Expr != nil { util.Error(node.Tok, "Return with value used outside of a function") }
		return
	}

	if d.Expr != nil && d.Expr.Type == ast.AddressOf {
		lval := d.Expr.Data.(ast.AddressOfNode).LValue
		if lval.Type == ast.Ident {
			name := lval.Data.(ast.IdentNode).Name
			if tc.isSymbolLocal(name) {
				util.Warn(tc.cfg, config.WarnLocalAddress, d.Expr.Tok, "Returning address of local variable '%s'", name)
			}
		}
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
			tc.typeErrorOrWarn(node.Tok, "Returning type '%s' is incompatible with function return type '%s'", typeToString(exprType), typeToString(retType))
		}
	}
}

func (tc *TypeChecker) checkExprAsCondition(node *ast.Node) {
	typ := tc.checkExpr(node)
	if !(tc.isScalarType(typ) || typ.Kind == ast.TYPE_UNTYPED || typ.Kind == ast.TYPE_UNTYPED_INT) {
		util.Warn(tc.cfg, config.WarnType, node.Tok, "Expression of type '%s' used as a condition", typeToString(typ))
	}
}

func (tc *TypeChecker) checkExpr(node *ast.Node) *ast.BxType {
	if node == nil { return ast.TypeUntyped }
	if node.Typ != nil && node.Typ.Kind != ast.TYPE_UNTYPED_INT && node.Typ.Kind != ast.TYPE_UNTYPED_FLOAT {
		return node.Typ
	}
	var typ *ast.BxType
	switch d := node.Data.(type) {
	case ast.AssignNode:
		lhsType, rhsType := tc.checkExpr(d.Lhs), tc.checkExpr(d.Rhs)

		isLhsScalar := tc.isScalarType(lhsType) && lhsType.Kind != ast.TYPE_POINTER
		isRhsPtr := rhsType != nil && rhsType.Kind == ast.TYPE_POINTER
		if isLhsScalar && isRhsPtr && d.Lhs.Type == ast.Ident {
			if sym := tc.findSymbol(d.Lhs.Data.(ast.IdentNode).Name, false); sym != nil {
				sym.Type = rhsType
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
			tc.typeErrorOrWarn(node.Tok, "Assigning to type '%s' from incompatible type '%s'", typeToString(lhsType), typeToString(rhsType))
		} else if rhsType.Kind == ast.TYPE_UNTYPED_INT || rhsType.Kind == ast.TYPE_UNTYPED_FLOAT {
			if tc.isNumericType(lhsType) || lhsType.Kind == ast.TYPE_POINTER || lhsType.Kind == ast.TYPE_BOOL {
				d.Rhs.Typ = lhsType
			}
		}
		typ = lhsType
	case ast.BinaryOpNode:
		leftType, rightType := tc.checkExpr(d.Left), tc.checkExpr(d.Right)
		typ = tc.getBinaryOpResultType(d.Op, leftType, rightType, node.Tok, d.Left, d.Right)
	case ast.UnaryOpNode:
		operandType := tc.checkExpr(d.Expr)
		switch d.Op {
		case token.Star:
			resolvedOpType := tc.resolveType(operandType)
			if resolvedOpType.Kind == ast.TYPE_POINTER || resolvedOpType.Kind == ast.TYPE_ARRAY {
				typ = resolvedOpType.Base
			} else if resolvedOpType.Kind == ast.TYPE_UNTYPED || tc.isIntegerType(resolvedOpType) {
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
		case token.And:
			typ = &ast.BxType{Kind: ast.TYPE_POINTER, Base: operandType}
		default:
			typ = operandType
		}
	case ast.PostfixOpNode:
		typ = tc.checkExpr(d.Expr)
	case ast.TernaryNode:
		tc.checkExprAsCondition(d.Cond)
		thenType, elseType := tc.checkExpr(d.ThenExpr), tc.checkExpr(d.ElseExpr)
		if !tc.areTypesCompatible(thenType, elseType, d.ElseExpr) {
			tc.typeErrorOrWarn(node.Tok, "Type mismatch in ternary expression branches ('%s' vs '%s')", typeToString(thenType), typeToString(elseType))
		}
		if thenType != nil && thenType.Kind == ast.TYPE_POINTER {
			typ = thenType
		} else if elseType != nil && elseType.Kind == ast.TYPE_POINTER {
			typ = elseType
		} else {
			typ = thenType
		}
	case ast.SubscriptNode:
		arrayType, indexType := tc.checkExpr(d.Array), tc.checkExpr(d.Index)
		if !tc.isIntegerType(indexType) && indexType.Kind != ast.TYPE_UNTYPED && indexType.Kind != ast.TYPE_UNTYPED_INT {
			tc.typeErrorOrWarn(d.Index.Tok, "Array subscript is not an integer type ('%s')", typeToString(indexType))
		}
		resolvedArrayType := tc.resolveType(arrayType)
		if resolvedArrayType.Kind == ast.TYPE_ARRAY || resolvedArrayType.Kind == ast.TYPE_POINTER {
			typ = resolvedArrayType.Base
		} else if resolvedArrayType.Kind == ast.TYPE_UNTYPED || tc.isIntegerType(resolvedArrayType) {
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
	case ast.StructLiteralNode:
		typ = tc.checkStructLiteral(node)
	case ast.NumberNode:
		typ = ast.TypeUntypedInt
	case ast.FloatNumberNode:
		typ = ast.TypeUntypedFloat
	case ast.StringNode:
		typ = ast.TypeString
	case ast.NilNode:
		typ = ast.TypeNil
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

func (tc *TypeChecker) findStructWithMember(memberName string) *ast.BxType {
	for s := tc.currentScope; s != nil; s = s.Parent {
		for sym := s.Symbols; sym != nil; sym = sym.Next {
			if sym.IsType {
				typ := tc.resolveType(sym.Type)
				if typ.Kind == ast.TYPE_STRUCT {
					for _, field := range typ.Fields {
						if field.Data.(ast.VarDeclNode).Name == memberName {
							return typ
						}
					}
				}
			}
		}
	}
	return nil
}

func (tc *TypeChecker) checkMemberAccess(node *ast.Node) *ast.BxType {
	d := node.Data.(ast.MemberAccessNode)
	exprType := tc.checkExpr(d.Expr)

	baseType := tc.resolveType(exprType)

	if baseType != nil && baseType.Kind == ast.TYPE_POINTER { baseType = baseType.Base }

	resolvedStructType := tc.resolveType(baseType)

	if resolvedStructType != nil && resolvedStructType.Kind == ast.TYPE_UNTYPED {
		memberName := d.Member.Data.(ast.IdentNode).Name
		if inferredType := tc.findStructWithMember(memberName); inferredType != nil {
			if d.Expr.Typ.Kind == ast.TYPE_POINTER {
				d.Expr.Typ.Base = inferredType
			} else {
				d.Expr.Typ = inferredType
			}
			if d.Expr.Type == ast.Ident {
				if sym := tc.findSymbol(d.Expr.Data.(ast.IdentNode).Name, false); sym != nil {
					sym.Type = d.Expr.Typ
				}
			}
			resolvedStructType = inferredType
		}
	}

	if resolvedStructType == nil || resolvedStructType.Kind != ast.TYPE_STRUCT {
		memberName := d.Member.Data.(ast.IdentNode).Name
		util.Error(node.Tok, "request for member '%s' in non-struct type '%s'", memberName, typeToString(exprType))
		return ast.TypeUntyped
	}

	memberName := d.Member.Data.(ast.IdentNode).Name
	for _, fieldNode := range resolvedStructType.Fields {
		fieldData := fieldNode.Data.(ast.VarDeclNode)
		if fieldData.Name == memberName {
			node.Typ = fieldData.Type
			return fieldData.Type
		}
	}

	util.Error(node.Tok, "no member named '%s' in struct '%s'", memberName, typeToString(resolvedStructType))
	return ast.TypeUntyped
}

func (tc *TypeChecker) typeFromName(name string) *ast.BxType {
	if sym := tc.findSymbol(name, true); sym != nil && sym.IsType { return sym.Type }

	tokType, isKeyword := token.KeywordMap[name]
	if isKeyword && tokType >= token.Void && tokType <= token.Any {
		if tokType == token.Void { return ast.TypeVoid }
		if tokType == token.StringKeyword { return ast.TypeString }
		if tokType >= token.Float && tokType <= token.Float64 { return &ast.BxType{Kind: ast.TYPE_FLOAT, Name: name} }
		return &ast.BxType{Kind: ast.TYPE_PRIMITIVE, Name: name}
	}
	return nil
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
			if targetType == nil { targetType = tc.checkExpr(arg) }
			if targetType == nil {
				util.Error(arg.Tok, "Cannot determine type for sizeof argument")
				return ast.TypeUntyped
			}
			node.Type, node.Data, node.Typ = ast.Number, ast.NumberNode{Value: tc.getSizeof(targetType)}, ast.TypeInt
			return ast.TypeInt
		}

		if targetType := tc.typeFromName(name); targetType != nil {
			if len(d.Args) != 1 {
				util.Error(node.Tok, "Type cast expects exactly one argument")
			} else {
				tc.checkExpr(d.Args[0])
			}
			node.Type = ast.TypeCast
			node.Data = ast.TypeCastNode{Expr: d.Args[0], TargetType: targetType}
			node.Typ = targetType
			return targetType
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

	funcExprType := tc.checkExpr(d.FuncExpr)

	if d.FuncExpr.Type == ast.Ident {
		name := d.FuncExpr.Data.(ast.IdentNode).Name
		if sym := tc.findSymbol(name, false); sym != nil && sym.IsFunc && sym.Type.Kind == ast.TYPE_UNTYPED {
			funcExprType = ast.TypeUntyped
		} else if sym == nil {
			util.Warn(tc.cfg, config.WarnImplicitDecl, d.FuncExpr.Tok, "Implicit declaration of function '%s'", name)
			sym = tc.addSymbol(ast.NewFuncDecl(d.FuncExpr.Tok, name, nil, nil, false, false, ast.TypeUntyped))
			funcExprType = ast.TypeUntyped
		}
	}

	for _, arg := range d.Args {
		tc.checkExpr(arg)
	}

	resolvedType := tc.resolveType(funcExprType)
	if resolvedType != nil && resolvedType.Kind == ast.TYPE_STRUCT { return resolvedType }

	return funcExprType
}

func (tc *TypeChecker) checkStructLiteral(node *ast.Node) *ast.BxType {
	d := node.Data.(ast.StructLiteralNode)

	typeIdent, ok := d.TypeNode.Data.(ast.IdentNode)
	if !ok {
		util.Error(d.TypeNode.Tok, "Invalid type expression in struct literal")
		return ast.TypeUntyped
	}

	sym := tc.findSymbol(typeIdent.Name, true)
	if sym == nil || !sym.IsType {
		util.Error(d.TypeNode.Tok, "Unknown type name '%s' in struct literal", typeIdent.Name)
		return ast.TypeUntyped
	}

	structType := tc.resolveType(sym.Type)
	if structType.Kind != ast.TYPE_STRUCT {
		util.Error(d.TypeNode.Tok, "'%s' is not a struct type", typeIdent.Name)
		return ast.TypeUntyped
	}

	if d.Names == nil {
		if len(d.Values) > 0 {
			if len(structType.Fields) > 0 {
				firstFieldType := tc.resolveType(structType.Fields[0].Data.(ast.VarDeclNode).Type)
				for i := 1; i < len(structType.Fields); i++ {
					currentFieldType := tc.resolveType(structType.Fields[i].Data.(ast.VarDeclNode).Type)
					if !tc.areTypesEqual(firstFieldType, currentFieldType) {
						util.Error(node.Tok, "positional struct literal for '%s' is only allowed if all fields have the same type, but found '%s' and '%s'",
							typeIdent.Name, typeToString(firstFieldType), typeToString(currentFieldType))
						break
					}
				}
			}
		}

		if len(d.Values) != 0 && len(d.Values) > len(structType.Fields) {
			util.Error(node.Tok, "Wrong number of initializers for struct '%s'. Expected %d, got %d", typeIdent.Name, len(structType.Fields), len(d.Values))
			return structType
		}

		for i, valNode := range d.Values {
			field := structType.Fields[i].Data.(ast.VarDeclNode)
			valType := tc.checkExpr(valNode)
			if !tc.areTypesCompatible(field.Type, valType, valNode) {
				tc.typeErrorOrWarn(valNode.Tok, "Initializer for field '%s' has wrong type. Expected '%s', got '%s'", field.Name, typeToString(field.Type), typeToString(valType))
			}
		}
	} else {
		if len(d.Values) > len(structType.Fields) { util.Error(node.Tok, "Too many initializers for struct '%s'", typeIdent.Name) }

		fieldMap := make(map[string]*ast.Node)
		for _, fieldNode := range structType.Fields {
			fieldData := fieldNode.Data.(ast.VarDeclNode)
			fieldMap[fieldData.Name] = fieldNode
		}

		usedFields := make(map[string]bool)

		for i, nameNode := range d.Names {
			if nameNode == nil { continue }
			fieldName := nameNode.Data.(ast.IdentNode).Name

			if usedFields[fieldName] {
				util.Error(nameNode.Tok, "Duplicate field '%s' in struct literal", fieldName)
				continue
			}
			usedFields[fieldName] = true

			field, ok := fieldMap[fieldName]
			if !ok {
				util.Error(nameNode.Tok, "Struct '%s' has no field named '%s'", typeIdent.Name, fieldName)
				continue
			}

			valNode := d.Values[i]
			valType := tc.checkExpr(valNode)
			fieldType := field.Data.(ast.VarDeclNode).Type

			if !tc.areTypesCompatible(fieldType, valType, valNode) {
				tc.typeErrorOrWarn(valNode.Tok, "Initializer for field '%s' has wrong type. Expected '%s', got '%s'", fieldName, typeToString(fieldType), typeToString(valType))
			}
		}
	}

	return structType
}

func (tc *TypeChecker) getBinaryOpResultType(op token.Type, left, right *ast.BxType, tok token.Token, leftNode, rightNode *ast.Node) *ast.BxType {
	resLeft, resRight := tc.resolveType(left), tc.resolveType(right)
	lType, rType := resLeft, resRight

	if lType.Kind == ast.TYPE_UNTYPED_INT && tc.isIntegerType(rType) {
		if tc.getSizeof(rType) < tc.getSizeof(ast.TypeInt) {
			lType, rType = ast.TypeInt, ast.TypeInt
			if rightNode.Typ.Kind != ast.TYPE_UNTYPED_INT { rightNode.Typ = ast.TypeInt }
		} else {
			lType = rType
		}
		leftNode.Typ = lType
	}
	if rType.Kind == ast.TYPE_UNTYPED_INT && tc.isIntegerType(lType) {
		if tc.getSizeof(lType) < tc.getSizeof(ast.TypeInt) {
			lType, rType = ast.TypeInt, ast.TypeInt
			if leftNode.Typ.Kind != ast.TYPE_UNTYPED_INT { leftNode.Typ = ast.TypeInt }
		} else {
			rType = lType
		}
		rightNode.Typ = rType
	}

	if lType.Kind == ast.TYPE_UNTYPED_FLOAT && tc.isFloatType(rType) {
		lType = rType
		leftNode.Typ = rType
	}
	if rType.Kind == ast.TYPE_UNTYPED_FLOAT && tc.isFloatType(lType) {
		rType = lType
		rightNode.Typ = rType
	}

	resLeft, resRight = lType, rType

	if op >= token.EqEq && op <= token.OrOr { return ast.TypeInt }

	if tc.isNumericType(resLeft) && tc.isNumericType(resRight) {
		if tc.isFloatType(resLeft) || tc.isFloatType(resRight) { return ast.TypeFloat }
		if tc.getSizeof(resLeft) > tc.getSizeof(resRight) { return resLeft }
		return resRight
	}

	if op == token.Plus || op == token.Minus {
		if resLeft.Kind == ast.TYPE_POINTER && tc.isIntegerType(resRight) { return resLeft }
		if tc.isIntegerType(resLeft) && resRight.Kind == ast.TYPE_POINTER && op == token.Plus { return resRight }
		if op == token.Minus && resLeft.Kind == ast.TYPE_POINTER && resRight.Kind == ast.TYPE_POINTER { return ast.TypeInt }
	}

	tc.typeErrorOrWarn(tok, "Invalid binary operation between types '%s' and '%s'", typeToString(left), typeToString(right))
	return ast.TypeInt
}

func (tc *TypeChecker) areTypesCompatible(a, b *ast.BxType, bNode *ast.Node) bool {
	if a == nil || b == nil || a.Kind == ast.TYPE_UNTYPED { return true }

	if b.Kind == ast.TYPE_UNTYPED_INT { return tc.isNumericType(a) || a.Kind == ast.TYPE_POINTER || a.Kind == ast.TYPE_BOOL }
	if b.Kind == ast.TYPE_UNTYPED_FLOAT { return tc.isFloatType(a) }
	if b.Kind == ast.TYPE_UNTYPED { return true }

	resA, resB := tc.resolveType(a), tc.resolveType(b)

	if resA.Kind == ast.TYPE_POINTER && tc.isIntegerType(resB) { return true }
	if tc.isIntegerType(resA) && resB.Kind == ast.TYPE_POINTER { return true }

	if resA.Kind == ast.TYPE_NIL { return resB.Kind == ast.TYPE_POINTER || resB.Kind == ast.TYPE_ARRAY || resB.Kind == ast.TYPE_NIL }
	if resB.Kind == ast.TYPE_NIL { return resA.Kind == ast.TYPE_POINTER || resA.Kind == ast.TYPE_ARRAY }

	if resA.Kind == resB.Kind {
		switch resA.Kind {
		case ast.TYPE_POINTER:
			if (resA.Base != nil && resA.Base.Kind == ast.TYPE_VOID) || (resB.Base != nil && resB.Base.Kind == ast.TYPE_VOID) { return true }
			if (resA.Base != nil && resA.Base == ast.TypeByte) || (resB.Base != nil && resB.Base == ast.TypeByte) { return true }
			return tc.areTypesCompatible(resA.Base, resB.Base, nil)
		case ast.TYPE_ARRAY:
			return tc.areTypesCompatible(resA.Base, resB.Base, nil)
		case ast.TYPE_STRUCT:
			return resA == resB || (resA.Name != "" && resA.Name == resB.Name)
		case ast.TYPE_ENUM:
			return true
		default:
			return true
		}
	}
	if bNode != nil && bNode.Type == ast.Number && bNode.Data.(ast.NumberNode).Value == 0 && resA.Kind == ast.TYPE_POINTER && tc.isIntegerType(resB) { return true }
	if resA.Kind == ast.TYPE_POINTER && resB.Kind == ast.TYPE_ARRAY { return tc.areTypesCompatible(resA.Base, resB.Base, nil) }
	if (resA.Kind == ast.TYPE_ENUM && tc.isIntegerType(resB)) || (tc.isIntegerType(resA) && resB.Kind == ast.TYPE_ENUM) { return true }
	if tc.isNumericType(resA) && tc.isNumericType(resB) { return true }
	if (resA.Kind == ast.TYPE_BOOL && tc.isScalarType(resB)) || (tc.isScalarType(resA) && resB.Kind == ast.TYPE_BOOL) { return true }
	return false
}

func (tc *TypeChecker) areTypesEqual(a, b *ast.BxType) bool {
	if a == nil || b == nil { return a == b }
	resA, resB := tc.resolveType(a), tc.resolveType(b)
	if resA.Kind != resB.Kind { return false }
	switch resA.Kind {
	case ast.TYPE_POINTER, ast.TYPE_ARRAY:
		return tc.areTypesEqual(resA.Base, resB.Base)
	case ast.TYPE_STRUCT, ast.TYPE_ENUM, ast.TYPE_PRIMITIVE, ast.TYPE_FLOAT:
		return resA.Name == resB.Name
	default:
		return true
	}
}

func (tc *TypeChecker) resolveType(typ *ast.BxType) *ast.BxType {
	if typ == nil { return ast.TypeUntyped }
	if tc.resolving[typ] { return typ }
	tc.resolving[typ] = true
	defer func() { delete(tc.resolving, typ) }()
	if (typ.Kind == ast.TYPE_PRIMITIVE || typ.Kind == ast.TYPE_STRUCT || typ.Kind == ast.TYPE_ENUM) && typ.Name != "" {
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
	if t == nil { return false }
	resolved := tc.resolveType(t)
	return resolved.Kind == ast.TYPE_PRIMITIVE || resolved.Kind == ast.TYPE_UNTYPED_INT
}

func (tc *TypeChecker) isFloatType(t *ast.BxType) bool {
	if t == nil { return false }
	resolved := tc.resolveType(t)
	return resolved.Kind == ast.TYPE_FLOAT || resolved.Kind == ast.TYPE_UNTYPED_FLOAT
}

func (tc *TypeChecker) isNumericType(t *ast.BxType) bool {
	return tc.isIntegerType(t) || tc.isFloatType(t)
}
func (tc *TypeChecker) isScalarType(t *ast.BxType) bool {
	if t == nil { return false }
	resolved := tc.resolveType(t)
	return tc.isNumericType(resolved) || resolved.Kind == ast.TYPE_POINTER || resolved.Kind == ast.TYPE_BOOL
}

func typeToString(t *ast.BxType) string {
	if t == nil { return "<nil>" }
	var sb strings.Builder
	if t.IsConst { sb.WriteString("const ") }
	switch t.Kind {
	case ast.TYPE_PRIMITIVE, ast.TYPE_BOOL, ast.TYPE_FLOAT, ast.TYPE_UNTYPED_INT, ast.TYPE_UNTYPED_FLOAT:
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
	case ast.TYPE_ENUM:
		sb.WriteString("enum ")
		if t.Name != "" {
			sb.WriteString(t.Name)
		} else {
			sb.WriteString("<anonymous>")
		}
	case ast.TYPE_VOID:
		sb.WriteString("void")
	case ast.TYPE_UNTYPED:
		sb.WriteString("untyped")
	case ast.TYPE_NIL:
		sb.WriteString("nil")
	default:
		sb.WriteString(fmt.Sprintf("<unknown_type_kind_%d>", t.Kind))
	}
	return sb.String()
}
