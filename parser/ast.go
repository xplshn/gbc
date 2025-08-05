package parser

type AstNodeType int

const (
	// Expressions
	AST_NUMBER AstNodeType = iota
	AST_STRING
	AST_IDENT
	AST_ASSIGN
	AST_BINARY_OP
	AST_UNARY_OP
	AST_POSTFIX_OP
	AST_FUNC_CALL
	AST_INDIRECTION
	AST_ADDRESS_OF
	AST_TERNARY
	AST_SUBSCRIPT

	// Statements
	AST_FUNC_DECL
	AST_VAR_DECL
	AST_EXTRN_DECL
	AST_IF
	AST_WHILE
	AST_RETURN
	AST_BLOCK
	AST_GOTO
	AST_SWITCH
	AST_CASE
	AST_DEFAULT
	AST_BREAK
	AST_LABEL
	AST_ASM_STMT
)

type AstNode struct {
	Type   AstNodeType
	Tok    Token // The primary token associated with this node for error reporting
	Parent *AstNode
	Data   interface{}
}

// AST Node Data Structs
type AstNumber struct {
	Value int64
}
type AstString struct {
	Value string
}
type AstIdent struct {
	Name string
}
type AstAssign struct {
	Op  TokenType
	Lhs *AstNode
	Rhs *AstNode
}
type AstBinaryOp struct {
	Op    TokenType
	Left  *AstNode
	Right *AstNode
}
type AstUnaryOp struct {
	Op   TokenType
	Expr *AstNode
}
type AstPostfixOp struct {
	Op   TokenType
	Expr *AstNode
}
type AstIndirection struct {
	Expr *AstNode
}
type AstAddressOf struct {
	LValue *AstNode
}
type AstTernary struct {
	Cond     *AstNode
	ThenExpr *AstNode
	ElseExpr *AstNode
}
type AstSubscript struct {
	Array *AstNode
	Index *AstNode
}
type AstFuncCall struct {
	FuncExpr *AstNode
	Args     []*AstNode
}
type AstFuncDecl struct {
	Name       string
	Params     []*AstNode
	Body       *AstNode
	HasVarargs bool
}
type AstVarDecl struct {
	Name        string
	InitList    []*AstNode
	SizeExpr    *AstNode
	IsVector    bool
	IsBracketed bool
}
type AstExtrnDecl struct {
	Name string
}
type AstIf struct {
	Cond     *AstNode
	ThenBody *AstNode
	ElseBody *AstNode
}
type AstWhile struct {
	Cond *AstNode
	Body *AstNode
}
type AstReturn struct {
	Expr *AstNode
}
type AstBlock struct {
	Stmts       []*AstNode
	IsSynthetic bool
}
type AstGoto struct {
	Label string
}
type AstCaseLabel struct {
	Value     int64
	LabelName string
}
type AstSwitch struct {
	Expr             *AstNode
	Body             *AstNode
	CaseLabels       []AstCaseLabel
	DefaultLabelName string
}
type AstCase struct {
	Value    *AstNode
	Body     *AstNode
	QbeLabel string
}
type AstDefault struct {
	Body     *AstNode
	QbeLabel string
}
type AstBreak struct{}
type AstLabel struct {
	Name string
	Stmt *AstNode
}
type AstAsmStmt struct {
	Code string
}

// AST Node Constructors
func newAstNode(tok Token, nodeType AstNodeType, data interface{}, children ...*AstNode) *AstNode {
	node := &AstNode{
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

func astNumber(tok Token, value int64) *AstNode {
	return newAstNode(tok, AST_NUMBER, AstNumber{Value: value})
}

func astString(tok Token, value string) *AstNode {
	return newAstNode(tok, AST_STRING, AstString{Value: value})
}

func astIdent(tok Token, name string) *AstNode {
	return newAstNode(tok, AST_IDENT, AstIdent{Name: name})
}

func astAssign(tok Token, op TokenType, lhs, rhs *AstNode) *AstNode {
	return newAstNode(tok, AST_ASSIGN, AstAssign{Op: op, Lhs: lhs, Rhs: rhs}, lhs, rhs)
}

func astBinaryOp(tok Token, op TokenType, left, right *AstNode) *AstNode {
	return newAstNode(tok, AST_BINARY_OP, AstBinaryOp{Op: op, Left: left, Right: right}, left, right)
}

func astUnaryOp(tok Token, op TokenType, expr *AstNode) *AstNode {
	return newAstNode(tok, AST_UNARY_OP, AstUnaryOp{Op: op, Expr: expr}, expr)
}

func astPostfixOp(tok Token, op TokenType, expr *AstNode) *AstNode {
	return newAstNode(tok, AST_POSTFIX_OP, AstPostfixOp{Op: op, Expr: expr}, expr)
}

func astIndirection(tok Token, expr *AstNode) *AstNode {
	return newAstNode(tok, AST_INDIRECTION, AstIndirection{Expr: expr}, expr)
}

func astAddressOf(tok Token, lvalue *AstNode) *AstNode {
	return newAstNode(tok, AST_ADDRESS_OF, AstAddressOf{LValue: lvalue}, lvalue)
}

func astTernary(tok Token, cond, thenExpr, elseExpr *AstNode) *AstNode {
	return newAstNode(tok, AST_TERNARY, AstTernary{Cond: cond, ThenExpr: thenExpr, ElseExpr: elseExpr}, cond, thenExpr, elseExpr)
}

func astSubscript(tok Token, array, index *AstNode) *AstNode {
	return newAstNode(tok, AST_SUBSCRIPT, AstSubscript{Array: array, Index: index}, array, index)
}

func astFuncCall(tok Token, funcExpr *AstNode, args []*AstNode) *AstNode {
	node := newAstNode(tok, AST_FUNC_CALL, AstFuncCall{FuncExpr: funcExpr, Args: args}, funcExpr)
	for _, arg := range args {
		arg.Parent = node
	}
	return node
}

func astFuncDecl(tok Token, name string, params []*AstNode, body *AstNode, hasVarargs bool) *AstNode {
	node := newAstNode(tok, AST_FUNC_DECL, AstFuncDecl{Name: name, Params: params, Body: body, HasVarargs: hasVarargs}, body)
	for _, p := range params {
		p.Parent = node
	}
	return node
}

func astVarDecl(tok Token, name string, initList []*AstNode, sizeExpr *AstNode, isVector bool, isBracketed bool) *AstNode {
	node := newAstNode(tok, AST_VAR_DECL, AstVarDecl{Name: name, InitList: initList, SizeExpr: sizeExpr, IsVector: isVector, IsBracketed: isBracketed}, sizeExpr)
	for _, i := range initList {
		i.Parent = node
	}
	return node
}

func astExtrnDecl(tok Token, name string) *AstNode {
	return newAstNode(tok, AST_EXTRN_DECL, AstExtrnDecl{Name: name})
}

func astIf(tok Token, cond, thenBody, elseBody *AstNode) *AstNode {
	return newAstNode(tok, AST_IF, AstIf{Cond: cond, ThenBody: thenBody, ElseBody: elseBody}, cond, thenBody, elseBody)
}

func astWhile(tok Token, cond, body *AstNode) *AstNode {
	return newAstNode(tok, AST_WHILE, AstWhile{Cond: cond, Body: body}, cond, body)
}

func astReturn(tok Token, expr *AstNode) *AstNode {
	return newAstNode(tok, AST_RETURN, AstReturn{Expr: expr}, expr)
}

func astBlock(tok Token, stmts []*AstNode, isSynthetic bool) *AstNode {
	node := newAstNode(tok, AST_BLOCK, AstBlock{Stmts: stmts, IsSynthetic: isSynthetic})
	for _, s := range stmts {
		s.Parent = node
	}
	return node
}

func astGoto(tok Token, label string) *AstNode {
	return newAstNode(tok, AST_GOTO, AstGoto{Label: label})
}

func astSwitch(tok Token, expr, body *AstNode) *AstNode {
	return newAstNode(tok, AST_SWITCH, AstSwitch{Expr: expr, Body: body}, expr, body)
}

func astCase(tok Token, value, body *AstNode) *AstNode {
	return newAstNode(tok, AST_CASE, AstCase{Value: value, Body: body}, value, body)
}

func astDefault(tok Token, body *AstNode) *AstNode {
	return newAstNode(tok, AST_DEFAULT, AstDefault{Body: body}, body)
}

func astBreak(tok Token) *AstNode {
	return newAstNode(tok, AST_BREAK, AstBreak{})
}

func astLabel(tok Token, name string, stmt *AstNode) *AstNode {
	return newAstNode(tok, AST_LABEL, AstLabel{Name: name, Stmt: stmt}, stmt)
}

func astAsmStmt(tok Token, code string) *AstNode {
	return newAstNode(tok, AST_ASM_STMT, AstAsmStmt{Code: code})
}

// Constant folding
func FoldConstants(node *AstNode) *AstNode {
	if node == nil {
		return nil
	}

	// First, recursively fold all children of the current node.
	switch d := node.Data.(type) {
	case AstAssign:
		d.Rhs = FoldConstants(d.Rhs)
		node.Data = d
	case AstBinaryOp:
		d.Left = FoldConstants(d.Left)
		d.Right = FoldConstants(d.Right)
		node.Data = d
	case AstUnaryOp:
		d.Expr = FoldConstants(d.Expr)
		node.Data = d
	case AstTernary:
		d.Cond = FoldConstants(d.Cond)
		// Special case: if the condition folded to a constant, we can
		// eliminate the ternary expression entirely.
		if d.Cond.Type == AST_NUMBER {
			condData := d.Cond.Data.(AstNumber)
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
	if node.Type == AST_BINARY_OP {
		d := node.Data.(AstBinaryOp)
		if d.Left.Type == AST_NUMBER && d.Right.Type == AST_NUMBER {
			l := d.Left.Data.(AstNumber).Value
			r := d.Right.Data.(AstNumber).Value
			var res int64
			folded := true

			switch d.Op {
			case TOK_PLUS:
				res = l + r
			case TOK_MINUS:
				res = l - r
			case TOK_STAR:
				res = l * r
			case TOK_AND:
				res = l & r
			case TOK_OR:
				res = l | r
			case TOK_XOR:
				res = l ^ r
			case TOK_SHL:
				res = l << uint64(r)
			case TOK_SHR:
				res = l >> uint64(r)
			case TOK_EQEQ:
				if l == r {
					res = 1
				}
			case TOK_NEQ:
				if l != r {
					res = 1
				}
			case TOK_LT:
				if l < r {
					res = 1
				}
			case TOK_GT:
				if l > r {
					res = 1
				}
			case TOK_LTE:
				if l <= r {
					res = 1
				}
			case TOK_GTE:
				if l >= r {
					res = 1
				}
			case TOK_SLASH:
				if r == 0 {
					Error(node.Tok, "Compile-time division by zero.")
					res = 0
				} else {
					res = l / r
				}
			case TOK_REM:
				if r == 0 {
					Error(node.Tok, "Compile-time modulo by zero.")
					res = 0
				} else {
					res = l % r
				}
			default:
				folded = false
			}

			if folded {
				return astNumber(node.Tok, res)
			}
		}
	} else if node.Type == AST_UNARY_OP {
		d := node.Data.(AstUnaryOp)
		// Check if the (potentially folded) child is a number.
		if d.Expr.Type == AST_NUMBER {
			val := d.Expr.Data.(AstNumber).Value
			var res int64
			folded := true

			switch d.Op {
			case TOK_MINUS:
				res = -val
			case TOK_COMPLEMENT:
				res = ^val
			case TOK_NOT:
				if val == 0 {
					res = 1
				} else {
					res = 0
				}
			default:
				folded = false
			}
			if folded {
				return astNumber(node.Tok, res)
			}
		}
	}

	return node
}
