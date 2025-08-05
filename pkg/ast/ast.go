package ast

import (
	"gbc/pkg/token"
	"gbc/pkg/util"
)

type NodeType int

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

	// Statements
	FuncDecl
	VarDecl
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
	Label
	AsmStmt
)

type Node struct {
	Type   NodeType
	Tok    token.Token // The primary token associated with this node for error reporting
	Parent *Node
	Data   interface{}
}

// Node Data Structs
type NumberNode struct {
	Value int64
}
type StringNode struct {
	Value string
}
type IdentNode struct {
	Name string
}
type AssignNode struct {
	Op  token.Type
	Lhs *Node
	Rhs *Node
}
type BinaryOpNode struct {
	Op    token.Type
	Left  *Node
	Right *Node
}
type UnaryOpNode struct {
	Op   token.Type
	Expr *Node
}
type PostfixOpNode struct {
	Op   token.Type
	Expr *Node
}
type IndirectionNode struct {
	Expr *Node
}
type AddressOfNode struct {
	LValue *Node
}
type TernaryNode struct {
	Cond     *Node
	ThenExpr *Node
	ElseExpr *Node
}
type SubscriptNode struct {
	Array *Node
	Index *Node
}
type FuncCallNode struct {
	FuncExpr *Node
	Args     []*Node
}
type FuncDeclNode struct {
	Name       string
	Params     []*Node
	Body       *Node
	HasVarargs bool
}
type VarDeclNode struct {
	Name        string
	InitList    []*Node
	SizeExpr    *Node
	IsVector    bool
	IsBracketed bool
}
type ExtrnDeclNode struct {
	Name string
}
type IfNode struct {
	Cond     *Node
	ThenBody *Node
	ElseBody *Node
}
type WhileNode struct {
	Cond *Node
	Body *Node
}
type ReturnNode struct {
	Expr *Node
}
type BlockNode struct {
	Stmts       []*Node
	IsSynthetic bool
}
type GotoNode struct {
	Label string
}
type CaseLabelNode struct {
	Value     int64
	LabelName string
}
type SwitchNode struct {
	Expr             *Node
	Body             *Node
	CaseLabels       []CaseLabelNode
	DefaultLabelName string
}
type CaseNode struct {
	Value    *Node
	Body     *Node
	QbeLabel string
}
type DefaultNode struct {
	Body     *Node
	QbeLabel string
}
type BreakNode struct{}
type LabelNode struct {
	Name string
	Stmt *Node
}
type AsmStmtNode struct {
	Code string
}

// Node Constructors
func newNode(tok token.Token, nodeType NodeType, data interface{}, children ...*Node) *Node {
	node := &Node{
		Type: nodeType,
		Tok:  tok,
		Data: data,
	}
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

func NewFuncCall(tok token.Token, funcExpr *Node, args []*Node) *Node {
	node := newNode(tok, FuncCall, FuncCallNode{FuncExpr: funcExpr, Args: args}, funcExpr)
	for _, arg := range args {
		arg.Parent = node
	}
	return node
}

func NewFuncDecl(tok token.Token, name string, params []*Node, body *Node, hasVarargs bool) *Node {
	node := newNode(tok, FuncDecl, FuncDeclNode{Name: name, Params: params, Body: body, HasVarargs: hasVarargs}, body)
	for _, p := range params {
		p.Parent = node
	}
	return node
}

func NewVarDecl(tok token.Token, name string, initList []*Node, sizeExpr *Node, isVector bool, isBracketed bool) *Node {
	node := newNode(tok, VarDecl, VarDeclNode{Name: name, InitList: initList, SizeExpr: sizeExpr, IsVector: isVector, IsBracketed: isBracketed}, sizeExpr)
	for _, i := range initList {
		i.Parent = node
	}
	return node
}

func NewExtrnDecl(tok token.Token, name string) *Node {
	return newNode(tok, ExtrnDecl, ExtrnDeclNode{Name: name})
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
		s.Parent = node
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

func NewLabel(tok token.Token, name string, stmt *Node) *Node {
	return newNode(tok, Label, LabelNode{Name: name, Stmt: stmt}, stmt)
}

func NewAsmStmt(tok token.Token, code string) *Node {
	return newNode(tok, AsmStmt, AsmStmtNode{Code: code})
}

// Constant folding
func FoldConstants(node *Node) *Node {
	if node == nil {
		return nil
	}

	// First, recursively fold all children of the current node.
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
		// Special case: if the condition folded to a constant, we can
		// eliminate the ternary expression entirely.
		if d.Cond.Type == Number {
			condData := d.Cond.Data.(NumberNode)
			if condData.Value != 0 {
				return FoldConstants(d.ThenExpr)
			}
			return FoldConstants(d.ElseExpr)
		}
		// Otherwise, fold the branches.
		d.ThenExpr = FoldConstants(d.ThenExpr)
		d.ElseExpr = FoldConstants(d.ElseExpr)
		node.Data = d
	}

	// Now, try to fold the current node itself.
	if node.Type == BinaryOp {
		d := node.Data.(BinaryOpNode)
		if d.Left.Type == Number && d.Right.Type == Number {
			l := d.Left.Data.(NumberNode).Value
			r := d.Right.Data.(NumberNode).Value
			var res int64
			folded := true

			switch d.Op {
			case token.Plus:
				res = l + r
			case token.Minus:
				res = l - r
			case token.Star:
				res = l * r
			case token.And:
				res = l & r
			case token.Or:
				res = l | r
			case token.Xor:
				res = l ^ r
			case token.Shl:
				res = l << uint64(r)
			case token.Shr:
				res = l >> uint64(r)
			case token.EqEq:
				if l == r {
					res = 1
				}
			case token.Neq:
				if l != r {
					res = 1
				}
			case token.Lt:
				if l < r {
					res = 1
				}
			case token.Gt:
				if l > r {
					res = 1
				}
			case token.Lte:
				if l <= r {
					res = 1
				}
			case token.Gte:
				if l >= r {
					res = 1
				}
			case token.Slash:
				if r == 0 {
					util.Error(node.Tok, "Compile-time division by zero.")
					res = 0
				} else {
					res = l / r
				}
			case token.Rem:
				if r == 0 {
					util.Error(node.Tok, "Compile-time modulo by zero.")
					res = 0
				} else {
					res = l % r
				}
			default:
				folded = false
			}

			if folded {
				return NewNumber(node.Tok, res)
			}
		}
	} else if node.Type == UnaryOp {
		d := node.Data.(UnaryOpNode)
		// Check if the (potentially folded) child is a number.
		if d.Expr.Type == Number {
			val := d.Expr.Data.(NumberNode).Value
			var res int64
			folded := true

			switch d.Op {
			case token.Minus:
				res = -val
			case token.Complement:
				res = ^val
			case token.Not:
				if val == 0 {
					res = 1
				} else {
					res = 0
				}
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
