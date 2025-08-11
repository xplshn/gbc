// Package ast defines the types used to represent the Abstract Syntax Tree (AST)
package ast

import (
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/util"
)

// NodeType defines the kind of a node in the AST
type NodeType int

// Node types enum
const (
	// Expressions
	Number NodeType = iota
	String
	Ident
	Assign
	BinaryOp
	UnaryOp
	PostfixOp
	FuncCall
	Indirection
	AddressOf
	Ternary
	Subscript
	AutoAlloc
	MemberAccess
	TypeCast

	// Statements
	FuncDecl
	VarDecl
	MultiVarDecl
	TypeDecl
	ExtrnDecl
	If
	While
	Return
	Block
	Goto
	Switch
	Case
	Default
	Break
	Continue
	Label
	AsmStmt
	Directive
)

// Node represents a node in the Abstract Syntax Tree
type Node struct {
	Type   NodeType
	Tok    token.Token
	Parent *Node
	Data   interface{}
	Typ    *BxType // Set by the type checker
}

// BxTypeKind defines the kind of a BxType
type BxTypeKind int

// BxType kinds enum
const (
	TYPE_PRIMITIVE BxTypeKind = iota
	TYPE_POINTER
	TYPE_VOID
	TYPE_ARRAY
	TYPE_STRUCT
	TYPE_BOOL
	TYPE_FLOAT
	TYPE_UNTYPED
)

// BxType represents a type in the Bx type system
type BxType struct {
	Kind      BxTypeKind
	Base      *BxType // Base type for pointers or arrays
	Name      string  // Name for primitive types or the typedef name
	ArraySize *Node
	IsConst   bool
	StructTag string  // The name immediately following the 'struct' keyword
	Fields    []*Node // List of *VarDecl nodes for struct members
}

// Pre-defined types
var (
	TypeInt     = &BxType{Kind: TYPE_PRIMITIVE, Name: "int"}
	TypeUint    = &BxType{Kind: TYPE_PRIMITIVE, Name: "uint"}
	TypeInt8    = &BxType{Kind: TYPE_PRIMITIVE, Name: "int8"}
	TypeUint8   = &BxType{Kind: TYPE_PRIMITIVE, Name: "uint8"}
	TypeInt16   = &BxType{Kind: TYPE_PRIMITIVE, Name: "int16"}
	TypeUint16  = &BxType{Kind: TYPE_PRIMITIVE, Name: "uint16"}
	TypeInt32   = &BxType{Kind: TYPE_PRIMITIVE, Name: "int32"}
	TypeUint32  = &BxType{Kind: TYPE_PRIMITIVE, Name: "uint32"}
	TypeInt64   = &BxType{Kind: TYPE_PRIMITIVE, Name: "int64"}
	TypeUint64  = &BxType{Kind: TYPE_PRIMITIVE, Name: "uint64"}
	TypeFloat   = &BxType{Kind: TYPE_FLOAT, Name: "float"}
	TypeFloat32 = &BxType{Kind: TYPE_FLOAT, Name: "float32"}
	TypeFloat64 = &BxType{Kind: TYPE_FLOAT, Name: "float64"}
	TypeByte    = &BxType{Kind: TYPE_PRIMITIVE, Name: "byte"}
	TypeVoid    = &BxType{Kind: TYPE_VOID, Name: "void"}
	TypeBool    = &BxType{Kind: TYPE_BOOL, Name: "bool"}
	TypeUntyped = &BxType{Kind: TYPE_UNTYPED, Name: "untyped"}
	TypeString  = &BxType{Kind: TYPE_POINTER, Base: TypeByte, Name: "string"}
)

// --- Node Data Structs ---
type NumberNode struct{ Value int64 }
type StringNode struct{ Value string }
type IdentNode struct{ Name string }
type AssignNode struct{ Op token.Type; Lhs, Rhs *Node }
type BinaryOpNode struct{ Op token.Type; Left, Right *Node }
type UnaryOpNode struct{ Op token.Type; Expr *Node }
type PostfixOpNode struct{ Op token.Type; Expr *Node }
type IndirectionNode struct{ Expr *Node }
type AddressOfNode struct{ LValue *Node }
type TernaryNode struct{ Cond, ThenExpr, ElseExpr *Node }
type SubscriptNode struct{ Array, Index *Node }
type MemberAccessNode struct{ Expr, Member *Node }
type TypeCastNode struct{ Expr *Node; TargetType *BxType }
type FuncCallNode struct{ FuncExpr *Node; Args []*Node }
type AutoAllocNode struct{ Size *Node }
type FuncDeclNode struct {
	Name       string
	Params     []*Node
	Body       *Node
	HasVarargs bool
	IsTyped    bool
	ReturnType *BxType
}
type VarDeclNode struct {
	Name        string
	Type        *BxType
	InitList    []*Node
	SizeExpr    *Node
	IsVector    bool
	IsBracketed bool
	IsDefine    bool
}
type MultiVarDeclNode struct{ Decls []*Node }
type TypeDeclNode struct{ Name string; Type *BxType }
type ExtrnDeclNode struct{ Names []*Node }
type IfNode struct{ Cond, ThenBody, ElseBody *Node }
type WhileNode struct{ Cond, Body *Node }
type ReturnNode struct{ Expr *Node }
type BlockNode struct{ Stmts []*Node; IsSynthetic bool }
type GotoNode struct{ Label string }
type CaseLabelNode struct{ Value int64; LabelName string }
type SwitchNode struct{ Expr, Body *Node; CaseLabels []CaseLabelNode; DefaultLabelName string }
type CaseNode struct{ Value, Body *Node; QbeLabel string }
type DefaultNode struct{ Body *Node; QbeLabel string }
type BreakNode struct{}
type ContinueNode struct{}
type LabelNode struct{ Name string; Stmt *Node }
type AsmStmtNode struct{ Code string }
type DirectiveNode struct{ Name string }

// --- Node Constructors ---

func newNode(tok token.Token, nodeType NodeType, data interface{}, children ...*Node) *Node {
	node := &Node{Type: nodeType, Tok: tok, Data: data}
	for _, child := range children {
		if child != nil {
			child.Parent = node
		}
	}
	return node
}

func NewNumber(tok token.Token, value int64) *Node {
	return newNode(tok, Number, NumberNode{Value: value})
}
func NewString(tok token.Token, value string) *Node {
	return newNode(tok, String, StringNode{Value: value})
}
func NewIdent(tok token.Token, name string) *Node {
	return newNode(tok, Ident, IdentNode{Name: name})
}
func NewAssign(tok token.Token, op token.Type, lhs, rhs *Node) *Node {
	return newNode(tok, Assign, AssignNode{Op: op, Lhs: lhs, Rhs: rhs}, lhs, rhs)
}
func NewBinaryOp(tok token.Token, op token.Type, left, right *Node) *Node {
	return newNode(tok, BinaryOp, BinaryOpNode{Op: op, Left: left, Right: right}, left, right)
}
func NewUnaryOp(tok token.Token, op token.Type, expr *Node) *Node {
	return newNode(tok, UnaryOp, UnaryOpNode{Op: op, Expr: expr}, expr)
}
func NewPostfixOp(tok token.Token, op token.Type, expr *Node) *Node {
	return newNode(tok, PostfixOp, PostfixOpNode{Op: op, Expr: expr}, expr)
}
func NewIndirection(tok token.Token, expr *Node) *Node {
	return newNode(tok, Indirection, IndirectionNode{Expr: expr}, expr)
}
func NewAddressOf(tok token.Token, lvalue *Node) *Node {
	return newNode(tok, AddressOf, AddressOfNode{LValue: lvalue}, lvalue)
}
func NewTernary(tok token.Token, cond, thenExpr, elseExpr *Node) *Node {
	return newNode(tok, Ternary, TernaryNode{Cond: cond, ThenExpr: thenExpr, ElseExpr: elseExpr}, cond, thenExpr, elseExpr)
}
func NewSubscript(tok token.Token, array, index *Node) *Node {
	return newNode(tok, Subscript, SubscriptNode{Array: array, Index: index}, array, index)
}
func NewMemberAccess(tok token.Token, expr, member *Node) *Node {
	return newNode(tok, MemberAccess, MemberAccessNode{Expr: expr, Member: member}, expr, member)
}
func NewTypeCast(tok token.Token, expr *Node, targetType *BxType) *Node {
	return newNode(tok, TypeCast, TypeCastNode{Expr: expr, TargetType: targetType}, expr)
}
func NewFuncCall(tok token.Token, funcExpr *Node, args []*Node) *Node {
	node := newNode(tok, FuncCall, FuncCallNode{FuncExpr: funcExpr, Args: args}, funcExpr)
	for _, arg := range args {
		arg.Parent = node
	}
	return node
}
func NewAutoAlloc(tok token.Token, size *Node) *Node {
	return newNode(tok, AutoAlloc, AutoAllocNode{Size: size}, size)
}
func NewFuncDecl(tok token.Token, name string, params []*Node, body *Node, hasVarargs bool, isTyped bool, returnType *BxType) *Node {
	node := newNode(tok, FuncDecl, FuncDeclNode{
		Name: name, Params: params, Body: body, HasVarargs: hasVarargs, IsTyped: isTyped, ReturnType: returnType,
	}, body)
	for _, p := range params {
		p.Parent = node
	}
	return node
}
func NewVarDecl(tok token.Token, name string, varType *BxType, initList []*Node, sizeExpr *Node, isVector bool, isBracketed bool, isDefine bool) *Node {
	node := newNode(tok, VarDecl, VarDeclNode{
		Name: name, Type: varType, InitList: initList, SizeExpr: sizeExpr, IsVector: isVector, IsBracketed: isBracketed, IsDefine: isDefine,
	}, sizeExpr)
	for _, init := range initList {
		init.Parent = node
	}
	return node
}
func NewMultiVarDecl(tok token.Token, decls []*Node) *Node {
	node := newNode(tok, MultiVarDecl, MultiVarDeclNode{Decls: decls})
	for _, d := range decls {
		d.Parent = node
	}
	return node
}
func NewTypeDecl(tok token.Token, name string, typ *BxType) *Node {
	return newNode(tok, TypeDecl, TypeDeclNode{Name: name, Type: typ})
}
func NewExtrnDecl(tok token.Token, names []*Node) *Node {
	node := newNode(tok, ExtrnDecl, ExtrnDeclNode{Names: names})
	for _, n := range names {
		n.Parent = node
	}
	return node
}
func NewIf(tok token.Token, cond, thenBody, elseBody *Node) *Node {
	return newNode(tok, If, IfNode{Cond: cond, ThenBody: thenBody, ElseBody: elseBody}, cond, thenBody, elseBody)
}
func NewWhile(tok token.Token, cond, body *Node) *Node {
	return newNode(tok, While, WhileNode{Cond: cond, Body: body}, cond, body)
}
func NewReturn(tok token.Token, expr *Node) *Node {
	return newNode(tok, Return, ReturnNode{Expr: expr}, expr)
}
func NewBlock(tok token.Token, stmts []*Node, isSynthetic bool) *Node {
	node := newNode(tok, Block, BlockNode{Stmts: stmts, IsSynthetic: isSynthetic})
	for _, s := range stmts {
		if s != nil {
			s.Parent = node
		}
	}
	return node
}
func NewGoto(tok token.Token, label string) *Node {
	return newNode(tok, Goto, GotoNode{Label: label})
}
func NewSwitch(tok token.Token, expr, body *Node) *Node {
	return newNode(tok, Switch, SwitchNode{Expr: expr, Body: body}, expr, body)
}
func NewCase(tok token.Token, value, body *Node) *Node {
	return newNode(tok, Case, CaseNode{Value: value, Body: body}, value, body)
}
func NewDefault(tok token.Token, body *Node) *Node {
	return newNode(tok, Default, DefaultNode{Body: body}, body)
}
func NewBreak(tok token.Token) *Node {
	return newNode(tok, Break, BreakNode{})
}
func NewContinue(tok token.Token) *Node {
	return newNode(tok, Continue, ContinueNode{})
}
func NewLabel(tok token.Token, name string, stmt *Node) *Node {
	return newNode(tok, Label, LabelNode{Name: name, Stmt: stmt}, stmt)
}
func NewAsmStmt(tok token.Token, code string) *Node {
	return newNode(tok, AsmStmt, AsmStmtNode{Code: code})
}
func NewDirective(tok token.Token, name string) *Node {
	return newNode(tok, Directive, DirectiveNode{Name: name})
}

// FoldConstants performs compile-time constant evaluation on the AST
func FoldConstants(node *Node) *Node {
	if node == nil {
		return nil
	}

	// Recursively fold children first
	switch d := node.Data.(type) {
	case AssignNode:
		d.Rhs = FoldConstants(d.Rhs)
		node.Data = d
	case BinaryOpNode:
		d.Left = FoldConstants(d.Left)
		d.Right = FoldConstants(d.Right)
		node.Data = d
	case UnaryOpNode:
		d.Expr = FoldConstants(d.Expr)
		node.Data = d
	case TernaryNode:
		d.Cond = FoldConstants(d.Cond)
		if d.Cond.Type == Number {
			if d.Cond.Data.(NumberNode).Value != 0 {
				return FoldConstants(d.ThenExpr)
			}
			return FoldConstants(d.ElseExpr)
		}
		d.ThenExpr = FoldConstants(d.ThenExpr)
		d.ElseExpr = FoldConstants(d.ElseExpr)
		node.Data = d
	}

	// Then, attempt to fold the current node.
	switch node.Type {
	case BinaryOp:
		d := node.Data.(BinaryOpNode)
		if d.Left.Type == Number && d.Right.Type == Number {
			l, r := d.Left.Data.(NumberNode).Value, d.Right.Data.(NumberNode).Value
			var res int64
			folded := true
			switch d.Op {
			case token.Plus: res = l + r
			case token.Minus: res = l - r
			case token.Star: res = l * r
			case token.And: res = l & r
			case token.Or: res = l | r
			case token.Xor: res = l ^ r
			case token.Shl: res = l << uint64(r)
			case token.Shr: res = l >> uint64(r)
			case token.EqEq: if l == r { res = 1 }
			case token.Neq: if l != r { res = 1 }
			case token.Lt: if l < r { res = 1 }
			case token.Gt: if l > r { res = 1 }
			case token.Lte: if l <= r { res = 1 }
			case token.Gte: if l >= r { res = 1 }
			case token.Slash:
				if r == 0 { util.Error(node.Tok, "Compile-time division by zero.") }
				res = l / r
			case token.Rem:
				if r == 0 { util.Error(node.Tok, "Compile-time modulo by zero.") }
				res = l % r
			default:
				folded = false
			}
			if folded {
				return NewNumber(node.Tok, res)
			}
		}
	case UnaryOp:
		d := node.Data.(UnaryOpNode)
		if d.Expr.Type == Number {
			val := d.Expr.Data.(NumberNode).Value
			var res int64
			folded := true
			switch d.Op {
			case token.Minus: res = -val
			case token.Complement: res = ^val
			case token.Not: if val == 0 { res = 1 }
			default:
				folded = false
			}
			if folded {
				return NewNumber(node.Tok, res)
			}
		}
	}

	return node
}
