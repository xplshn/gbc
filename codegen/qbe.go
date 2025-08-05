// TODO: Backport to CBC's codegen.c
// ---
// I feel like not all of the logic in codegen.go, belongs in codegen.go
// ---
// Cleanups are appreciated
// ---
// Here be dragons
package codegen

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	. "gbc/parser"
)

type SymbolType int

const (
	SYM_VAR SymbolType = iota
	SYM_FUNC
	SYM_LABEL
)

// Symbol represents a named entity (variable or function)
type Symbol struct {
	Name        string
	Type        SymbolType
	QbeName     string // Identifier used in the QBE IR
	IsVector    bool
	IsByteArray bool
	StackOffset int64 // byte offset for local (auto) variables from the base of the stack frame
	Next        *Symbol
}

// Scope holds the symbols declared within a lexical scope
type Scope struct {
	Symbols *Symbol
	Parent  *Scope
}

// StringEntry maps a string literal to its generated data label in the IR
type StringEntry struct {
	Value string
	Label string
}

// AutoVarInfo holds information about a local (auto) variable, used to
// calculate the function's stack frame layout before code generation
type AutoVarInfo struct {
	Node *AstNode
	Size int64 // Size in bytes
}

// CodegenContext holds the state for the code generation pass
type CodegenContext struct {
	out              strings.Builder // out is the buffer for the generated QBE IR
	asmOut           strings.Builder // asmOut is the buffer for inline assembly blocks
	strings          []StringEntry
	tempCount        int
	labelCount       int
	currentScope     *Scope
	currentFuncFrame string // currentFuncFrame is the temporary QBE IR code for the current function's stack frame base pointer
	breakLabel       string
	wordType         string // wordType is "l" for 64-bit, "w" for 32-bit.
	wordSize         int    // wordSize is 8 for 64-bit, 4 for 32-bit.
	warnings         bool
	phiFromLabel     string // phiFromLabel tracks the predecessor label for constructing a phi node (needed for nested ternaries)
}

// NewCodegenContext creates and initializes a new code generation context
func NewCodegenContext() *CodegenContext {
	var ws int
	var wt string
	// Set word size and type based on the host architecture (this approach sucks)
	switch runtime.GOARCH {
	case "amd64", "arm64", "riscv64", "s390x", "ppc64le", "mips64le":
		ws = 8
		wt = "l"
	case "386", "arm", "mipsle":
		ws = 4
		wt = "w"
	default:
		// 64bit is the default
		fmt.Fprintf(os.Stderr, "warning: unrecognized architecture '%s', defaulting to 64-bit\n", runtime.GOARCH)
		ws = 8
		wt = "l"
	}

	return &CodegenContext{
		currentScope: newScope(nil),
		wordSize:     ws,
		wordType:     wt,
	}
}

// Scope and Symbol Management
func newScope(parent *Scope) *Scope {
	return &Scope{Parent: parent}
}

func (ctx *CodegenContext) enterScope() {
	ctx.currentScope = newScope(ctx.currentScope)
}

func (ctx *CodegenContext) exitScope() {
	if ctx.currentScope.Parent != nil {
		ctx.currentScope = ctx.currentScope.Parent
	}
}

func (ctx *CodegenContext) findSymbol(name string) *Symbol {
	for s := ctx.currentScope; s != nil; s = s.Parent {
		for sym := s.Symbols; sym != nil; sym = sym.Next {
			if sym.Name == name {
				return sym
			}
		}
	}
	return nil
}

func (ctx *CodegenContext) findSymbolInCurrentScope(name string) *Symbol {
	for sym := ctx.currentScope.Symbols; sym != nil; sym = sym.Next {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

func (ctx *CodegenContext) addSymbol(name string, symType SymbolType, isVector bool, tok Token) *Symbol {
	if ctx.findSymbolInCurrentScope(name) != nil {
		if symType == SYM_VAR {
			// B allows redeclaration, which is handled by the analysis passes
		}
	}

	var qbeName string
	switch symType {
	case SYM_VAR:
		if ctx.currentScope.Parent == nil { // Global scope
			qbeName = "$" + name
		} else { // Local scope
			qbeName = fmt.Sprintf("%%.%s_%d", name, ctx.tempCount)
			ctx.tempCount++
		}
	case SYM_FUNC:
		qbeName = "$" + name
	case SYM_LABEL:
		qbeName = "@" + name
	}

	sym := &Symbol{
		Name:     name,
		Type:     symType,
		QbeName:  qbeName,
		IsVector: isVector,
		Next:     ctx.currentScope.Symbols,
	}
	ctx.currentScope.Symbols = sym
	return sym
}

// QBE Helpers
func (ctx *CodegenContext) newTemp() string {
	name := fmt.Sprintf("%%.t%d", ctx.tempCount)
	ctx.tempCount++
	return name
}

func (ctx *CodegenContext) newLabel() string {
	name := fmt.Sprintf("@L%d", ctx.labelCount)
	ctx.labelCount++
	return name
}

func (ctx *CodegenContext) addString(value string) string {
	for _, entry := range ctx.strings {
		if entry.Value == value {
			return "$" + entry.Label
		}
	}
	label := fmt.Sprintf("str%d", len(ctx.strings))
	ctx.strings = append(ctx.strings, StringEntry{Value: value, Label: label})
	return "$" + label
}

func getOpStr(op TokenType) string {
	switch op {
	case TOK_PLUS, TOK_PLUS_EQ, TOK_EQ_PLUS:
		return "add"
	case TOK_MINUS, TOK_MINUS_EQ, TOK_EQ_MINUS:
		return "sub"
	case TOK_STAR, TOK_STAR_EQ, TOK_EQ_STAR:
		return "mul"
	case TOK_SLASH, TOK_SLASH_EQ, TOK_EQ_SLASH:
		return "div"
	case TOK_REM, TOK_REM_EQ, TOK_EQ_REM:
		return "rem"
	case TOK_AND, TOK_AND_EQ, TOK_EQ_AND:
		return "and"
	case TOK_OR, TOK_OR_EQ, TOK_EQ_OR:
		return "or"
	case TOK_XOR, TOK_XOR_EQ, TOK_EQ_XOR:
		return "xor"
	case TOK_SHL, TOK_SHL_EQ, TOK_EQ_SHL:
		return "shl"
	case TOK_SHR, TOK_SHR_EQ, TOK_EQ_SHR:
		return "shr"
	default:
		return ""
	}
}

func (ctx *CodegenContext) getCmpOpStr(op TokenType) string {
	var base string
	switch op {
	case TOK_EQEQ:
		base = "ceq"
	case TOK_NEQ:
		base = "cne"
	case TOK_LT:
		base = "cslt"
	case TOK_GT:
		base = "csgt"
	case TOK_LTE:
		base = "csle"
	case TOK_GTE:
		base = "csge"
	default:
		return ""
	}
	return base + ctx.wordType
}

// Main Generation Logic
func (ctx *CodegenContext) Generate(root *AstNode) (qbeIR, inlineAsm string) {
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

// collectGlobals walks the AST to find all top-level declarations (global
// variables and functions) and adds them to the global scope
func (ctx *CodegenContext) collectGlobals(node *AstNode) {
	if node == nil {
		return
	}

	switch node.Type {
	case AST_BLOCK:
		// Recurse into top-level blocks
		for _, stmt := range node.Data.(AstBlock).Stmts {
			ctx.collectGlobals(stmt)
		}
	case AST_VAR_DECL:
		// Add global variables to the symbol table. Auto variables are handled later
		if ctx.currentScope.Parent == nil {
			d := node.Data.(AstVarDecl)
			// Do not redefine an existing symbol (e.g., from an extrn)
			if ctx.findSymbolInCurrentScope(d.Name) == nil {
				ctx.addSymbol(d.Name, SYM_VAR, d.IsVector, node.Tok)
			}
		}
	case AST_FUNC_DECL:
		d := node.Data.(AstFuncDecl)
		// Do not redefine an existing symbol
		if ctx.findSymbolInCurrentScope(d.Name) == nil {
			ctx.addSymbol(d.Name, SYM_FUNC, false, node.Tok)
		}
	case AST_EXTRN_DECL:
		d := node.Data.(AstExtrnDecl)
		// An `extrn` can declare a variable or a function. The type is resolved
		// by usage. We initially declare it as a function
		if ctx.findSymbolInCurrentScope(d.Name) == nil {
			ctx.addSymbol(d.Name, SYM_FUNC, false, node.Tok)
		}
	}
}

// findByteArrays performs a flow-sensitive analysis to identify variables that
// are used as byte pointers. B does not have a char type, so pointer arithmetic
// must be scaled by the word size for word pointers and by 1 for byte pointers.
//
// This analysis iteratively propagates the "byte pointer" property. A variable
// is marked as a byte pointer if it is assigned a string literal, or if it is
// assigned another variable already marked as a byte pointer. The iteration
// continues until a fixed point is reached.
func (ctx *CodegenContext) findByteArrays(root *AstNode) {
	for { // Loop until a fixed point is reached
		changedInPass := false
		var astWalker func(*AstNode)
		astWalker = func(n *AstNode) {
			if n == nil {
				return
			}

			switch n.Type {
			case AST_VAR_DECL:
				// A vector initialized with a string is a byte array
				d := n.Data.(AstVarDecl)
				if d.IsVector && len(d.InitList) == 1 && d.InitList[0].Type == AST_STRING {
					sym := ctx.findSymbol(d.Name)
					if sym != nil && !sym.IsByteArray {
						sym.IsByteArray = true
						changedInPass = true
					}
				}

			case AST_ASSIGN:
				d := n.Data.(AstAssign)
				if d.Lhs.Type == AST_IDENT {
					lhsSym := ctx.findSymbol(d.Lhs.Data.(AstIdent).Name)
					if lhsSym != nil && !lhsSym.IsByteArray {
						rhsIsByteArray := false
						rhsNode := d.Rhs
						switch rhsNode.Type {
						case AST_STRING:
							// An assignment from a string literal creates a byte pointer
							rhsIsByteArray = true
						case AST_IDENT:
							// The byte pointer property propagates through assignment
							rhsSym := ctx.findSymbol(rhsNode.Data.(AstIdent).Name)
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

			// Recurse through the AST
			switch d := n.Data.(type) {
			case AstAssign:
				astWalker(d.Lhs)
				astWalker(d.Rhs)
			case AstBinaryOp:
				astWalker(d.Left)
				astWalker(d.Right)
			case AstUnaryOp:
				astWalker(d.Expr)
			case AstPostfixOp:
				astWalker(d.Expr)
			case AstIndirection:
				astWalker(d.Expr)
			case AstAddressOf:
				astWalker(d.LValue)
			case AstTernary:
				astWalker(d.Cond)
				astWalker(d.ThenExpr)
				astWalker(d.ElseExpr)
			case AstSubscript:
				astWalker(d.Array)
				astWalker(d.Index)
			case AstFuncCall:
				astWalker(d.FuncExpr)
				for _, arg := range d.Args {
					astWalker(arg)
				}
			case AstFuncDecl:
				astWalker(d.Body)
			case AstVarDecl:
				for _, init := range d.InitList {
					astWalker(init)
				}
				astWalker(d.SizeExpr)
			case AstIf:
				astWalker(d.Cond)
				astWalker(d.ThenBody)
				astWalker(d.ElseBody)
			case AstWhile:
				astWalker(d.Cond)
				astWalker(d.Body)
			case AstReturn:
				astWalker(d.Expr)
			case AstBlock:
				for _, s := range d.Stmts {
					astWalker(s)
				}
			case AstSwitch:
				astWalker(d.Expr)
				astWalker(d.Body)
			case AstCase:
				astWalker(d.Value)
				astWalker(d.Body)
			case AstDefault:
				astWalker(d.Body)
			case AstLabel:
				astWalker(d.Stmt)
			}
		}

		astWalker(root)
		if !changedInPass {
			break
		}
	}
}

func (ctx *CodegenContext) collectStrings(root *AstNode) {
	var walk func(*AstNode)
	walk = func(n *AstNode) {
		if n == nil {
			return
		}
		if n.Type == AST_STRING {
			ctx.addString(n.Data.(AstString).Value)
		}
		switch d := n.Data.(type) {
		case AstAssign:
			walk(d.Lhs)
			walk(d.Rhs)
		case AstBinaryOp:
			walk(d.Left)
			walk(d.Right)
		case AstUnaryOp:
			walk(d.Expr)
		case AstPostfixOp:
			walk(d.Expr)
		case AstIndirection:
			walk(d.Expr)
		case AstAddressOf:
			walk(d.LValue)
		case AstTernary:
			walk(d.Cond)
			walk(d.ThenExpr)
			walk(d.ElseExpr)
		case AstSubscript:
			walk(d.Array)
			walk(d.Index)
		case AstFuncCall:
			walk(d.FuncExpr)
			for _, arg := range d.Args {
				walk(arg)
			}
		case AstFuncDecl:
			walk(d.Body)
		case AstVarDecl:
			for _, init := range d.InitList {
				walk(init)
			}
			walk(d.SizeExpr)
		case AstIf:
			walk(d.Cond)
			walk(d.ThenBody)
			walk(d.ElseBody)
		case AstWhile:
			walk(d.Cond)
			walk(d.Body)
		case AstReturn:
			walk(d.Expr)
		case AstBlock:
			for _, s := range d.Stmts {
				walk(s)
			}
		case AstSwitch:
			walk(d.Expr)
			walk(d.Body)
		case AstCase:
			walk(d.Value)
			walk(d.Body)
		case AstDefault:
			walk(d.Body)
		case AstLabel:
			walk(d.Stmt)
		}
	}
	walk(root)
}

func isStringDerivedExpr(node *AstNode) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case AST_STRING:
		return true
	case AST_BINARY_OP:
		// Check for pointer arithmetic on a string literal, e.g., "hello" + 1
		if node.Data.(AstBinaryOp).Op == TOK_PLUS {
			return isStringDerivedExpr(node.Data.(AstBinaryOp).Left) || isStringDerivedExpr(node.Data.(AstBinaryOp).Right)
		}
	}
	return false
}

func (ctx *CodegenContext) codegenLvalue(node *AstNode) string {
	if node == nil {
		Error(Token{}, "Internal error: null l-value node in codegen")
	}
	switch node.Type {
	case AST_IDENT:
		name := node.Data.(AstIdent).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			// Handle implicit function declarations
			sym = ctx.addSymbol(name, SYM_FUNC, false, node.Tok)
		}
		if sym.Type == SYM_FUNC {
			return sym.QbeName
		}
		return sym.QbeName

	case AST_INDIRECTION:
		res, _ := ctx.codegenExpr(node.Data.(AstIndirection).Expr)
		return res

	case AST_SUBSCRIPT:
		d := node.Data.(AstSubscript)
		arrayBasePtr, _ := ctx.codegenExpr(d.Array)
		indexVal, _ := ctx.codegenExpr(d.Index)

		// Scale the index by word size, unless it's a byte array
		scale := ctx.wordSize
		// Check if the base of the subscript is a known byte array
		if d.Array.Type == AST_IDENT {
			sym := ctx.findSymbol(d.Array.Data.(AstIdent).Name)
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
		Error(node.Tok, "Expression is not a valid l-value.")
		return ""
	}
}

func (ctx *CodegenContext) codegenLogicalCond(node *AstNode, trueL, falseL string) {
	condVal, _ := ctx.codegenExpr(node)

	// Compare the condition value against zero to get a canonical 0 or 1
	isNonZero := ctx.newTemp()
	ctx.out.WriteString(fmt.Sprintf("\t%s =%s cne%s %s, 0\n", isNonZero, ctx.wordType, ctx.wordType, condVal))

	var condValForJnz string
	// The `jnz` instruction requires a word-sized argument. On 64-bit targets,
	// the result of a comparison is a long, so it must be truncated. The
	// truncation is safe as the value is only 0 or 1.
	if ctx.wordType == "l" {
		condValWord := ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =w copy %s\n", condValWord, isNonZero))
		condValForJnz = condValWord
	} else {
		condValForJnz = isNonZero
	}

	ctx.out.WriteString(fmt.Sprintf("\tjnz %s, %s, %s\n", condValForJnz, trueL, falseL))
}

func (ctx *CodegenContext) codegenExpr(node *AstNode) (result string, terminates bool) {
	if node == nil {
		return "0", false
	}

	switch node.Type {
	case AST_NUMBER:
		return fmt.Sprintf("%d", node.Data.(AstNumber).Value), false
	case AST_STRING:
		return ctx.addString(node.Data.(AstString).Value), false

	case AST_IDENT:
		sym := ctx.findSymbol(node.Data.(AstIdent).Name)
		if sym == nil {
			// Implicitly declare undefined functions as extrn
			sym = ctx.addSymbol(node.Data.(AstIdent).Name, SYM_FUNC, false, node.Tok)
		}

		// A function name used in a call expression evaluates to its label
		isFuncNameInCall := node.Parent != nil && node.Parent.Type == AST_FUNC_CALL && node.Parent.Data.(AstFuncCall).FuncExpr == node
		if sym.Type == SYM_FUNC && isFuncNameInCall {
			return sym.QbeName, false
		}

		// For a variable, load its value from memory
		addr := sym.QbeName
		res := ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s load%s %s\n", res, ctx.wordType, ctx.wordType, addr))
		return res, false

	case AST_ASSIGN:
		d := node.Data.(AstAssign)
		lvalAddr := ctx.codegenLvalue(d.Lhs)
		var rval string
		if d.Op == TOK_EQ {
			rval, _ = ctx.codegenExpr(d.Rhs)
		} else {
			// Compound assignment
			currentLvalVal := ctx.newTemp()
			loadInstruction := "load" + ctx.wordType
			if d.Lhs.Type == AST_SUBSCRIPT && d.Lhs.Data.(AstSubscript).Array.Type == AST_IDENT {
				sym := ctx.findSymbol(d.Lhs.Data.(AstSubscript).Array.Data.(AstIdent).Name)
				if sym != nil && sym.IsByteArray {
					loadInstruction = "loadub" // Load from byte array requires an unsigned byte load
				}
			}
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", currentLvalVal, ctx.wordType, loadInstruction, lvalAddr))

			rhsVal, _ := ctx.codegenExpr(d.Rhs)
			opStr := getOpStr(d.Op)
			rval = ctx.newTemp()
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", rval, ctx.wordType, opStr, currentLvalVal, rhsVal))
		}

		storeInstruction := "store" + ctx.wordType
		if d.Lhs.Type == AST_SUBSCRIPT && d.Lhs.Data.(AstSubscript).Array.Type == AST_IDENT {
			sym := ctx.findSymbol(d.Lhs.Data.(AstSubscript).Array.Data.(AstIdent).Name)
			if sym != nil && sym.IsByteArray {
				storeInstruction = "storeb"
			}
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s %s, %s\n", storeInstruction, rval, lvalAddr))
		return rval, false

	case AST_BINARY_OP:
		d := node.Data.(AstBinaryOp)
		l, _ := ctx.codegenExpr(d.Left)
		r, _ := ctx.codegenExpr(d.Right)
		res := ctx.newTemp()
		if opStr := getOpStr(d.Op); opStr != "" {
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, ctx.wordType, opStr, l, r))
		} else if cmpOpStr := ctx.getCmpOpStr(d.Op); cmpOpStr != "" {
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, %s\n", res, ctx.wordType, cmpOpStr, l, r))
		} else {
			Error(node.Tok, "Invalid binary operator token %d", d.Op)
		}
		return res, false

	case AST_UNARY_OP:
		d := node.Data.(AstUnaryOp)
		val, _ := ctx.codegenExpr(d.Expr)
		res := ctx.newTemp()
		switch d.Op {
		case TOK_MINUS:
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s sub 0, %s\n", res, ctx.wordType, val))
		case TOK_PLUS:
			return val, false // Unary plus is a no-op
		case TOK_NOT:
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s ceq%s %s, 0\n", res, ctx.wordType, ctx.wordType, val))
		case TOK_COMPLEMENT:
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s xor %s, -1\n", res, ctx.wordType, val))
		case TOK_INC, TOK_DEC: // Pre-increment/decrement
			lvalAddr := ctx.codegenLvalue(d.Expr)
			op := map[TokenType]string{TOK_INC: "add", TOK_DEC: "sub"}[d.Op]
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, 1\n", res, ctx.wordType, op, val))
			ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, res, lvalAddr))
		default:
			Error(node.Tok, "Unsupported unary operator")
		}
		return res, false

	case AST_POSTFIX_OP:
		d := node.Data.(AstPostfixOp)
		lvalAddr := ctx.codegenLvalue(d.Expr)
		res := ctx.newTemp()
		// Load the original value before the operation
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s load%s %s\n", res, ctx.wordType, ctx.wordType, lvalAddr))
		newVal := ctx.newTemp()
		op := map[TokenType]string{TOK_INC: "add", TOK_DEC: "sub"}[d.Op]
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s, 1\n", newVal, ctx.wordType, op, res))
		ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, newVal, lvalAddr))
		return res, false // Postfix operations return the value before modification

	case AST_INDIRECTION:
		exprNode := node.Data.(AstIndirection).Expr
		addr, _ := ctx.codegenExpr(exprNode)
		res := ctx.newTemp()

		loadInstruction := "load" + ctx.wordType
		isBytePointer := false

		if isStringDerivedExpr(exprNode) {
			// An expression derived from a string literal is a byte pointer
			isBytePointer = true
		} else if exprNode.Type == AST_IDENT {
			// A variable marked as a byte array is a byte pointer
			sym := ctx.findSymbol(exprNode.Data.(AstIdent).Name)
			if sym != nil && sym.IsByteArray {
				isBytePointer = true
			}
		}

		if isBytePointer {
			// Dereferencing a byte pointer requires an unsigned byte load
			loadInstruction = "loadub"
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", res, ctx.wordType, loadInstruction, addr))
		return res, false

	case AST_SUBSCRIPT:
		addr := ctx.codegenLvalue(node)
		res := ctx.newTemp()
		loadInstruction := "load" + ctx.wordType
		if node.Data.(AstSubscript).Array.Type == AST_IDENT {
			sym := ctx.findSymbol(node.Data.(AstSubscript).Array.Data.(AstIdent).Name)
			if sym != nil && sym.IsByteArray {
				loadInstruction = "loadub" // Use unsigned load for bytes
			}
		}
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s %s %s\n", res, ctx.wordType, loadInstruction, addr))
		return res, false

	case AST_ADDRESS_OF:
		lvalNode := node.Data.(AstAddressOf).LValue
		if lvalNode.Type == AST_IDENT {
			sym := ctx.findSymbol(lvalNode.Data.(AstIdent).Name)
			// The value of a vector identifier is already its address
			if sym != nil && sym.IsVector {
				return ctx.codegenExpr(lvalNode)
			}
		}
		// codegenLvalue returns the address of the l-value
		return ctx.codegenLvalue(lvalNode), false

	case AST_FUNC_CALL:
		d := node.Data.(AstFuncCall)
		if d.FuncExpr.Type == AST_IDENT {
			name := d.FuncExpr.Data.(AstIdent).Name
			sym := ctx.findSymbol(name)
			if sym != nil && sym.Type == SYM_VAR && !sym.IsVector {
				Error(d.FuncExpr.Tok, "'%s' is a variable but is used as a function", name)
			}
		}

		argVals := make([]string, len(d.Args))
		for i := len(d.Args) - 1; i >= 0; i-- {
			argVals[i], _ = ctx.codegenExpr(d.Args[i])
		}
		funcVal, _ := ctx.codegenExpr(d.FuncExpr)

		// A call is a statement if its result is unused
		isStmt := node.Parent != nil && node.Parent.Type == AST_BLOCK

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

	case AST_TERNARY:
		d := node.Data.(AstTernary)
		thenLabel := ctx.newLabel()
		elseLabel := ctx.newLabel()
		endLabel := ctx.newLabel()
		res := ctx.newTemp() // res will hold the result from the phi node

		ctx.codegenLogicalCond(d.Cond, thenLabel, elseLabel)

		// The 'then' branch.
		ctx.out.WriteString(thenLabel + "\n")
		ctx.phiFromLabel = thenLabel // Track predecessor for phi
		thenVal, thenTerminates := ctx.codegenExpr(d.ThenExpr)
		thenFromLabelForPhi := ctx.phiFromLabel // Predecessor might change in nested ternary
		if !thenTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		// The 'else' branch
		ctx.out.WriteString(elseLabel + "\n")
		ctx.phiFromLabel = elseLabel // Track predecessor for phi
		elseVal, elseTerminates := ctx.codegenExpr(d.ElseExpr)
		elseFromLabelForPhi := ctx.phiFromLabel // Predecessor might change in nested ternary
		if !elseTerminates {
			ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", endLabel))
		}

		// Join point for the branches.
		if !thenTerminates || !elseTerminates {
			ctx.out.WriteString(endLabel + "\n")

			// The phi node merges the values from the 'then' and 'else' branches
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

			// For nested ternaries, the end of this ternary is the predecessor
			// for the outer one.
			ctx.phiFromLabel = endLabel

			return res, thenTerminates && elseTerminates
		}

		// If both branches of the ternary terminate, the expression as a whole terminates
		return "0", true
	}
	Error(node.Tok, "Internal error: unhandled expression type in codegen: %v", node.Type)
	return "", false
}

func (ctx *CodegenContext) codegenStmt(node *AstNode) (terminates bool) {
	if node == nil {
		return false
	}
	switch node.Type {
	case AST_BLOCK:
		isRealBlock := !node.Data.(AstBlock).IsSynthetic
		if isRealBlock {
			ctx.enterScope()
		}
		var blockTerminates bool
		for _, stmt := range node.Data.(AstBlock).Stmts {
			if blockTerminates {
				isLabel := stmt.Type == AST_LABEL || stmt.Type == AST_CASE || stmt.Type == AST_DEFAULT
				if !isLabel {
					Warning(WARN_UNREACHABLE_CODE, stmt.Tok, "Unreachable code.")
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

	case AST_FUNC_DECL:
		ctx.codegenFuncDecl(node)
		return false

	case AST_VAR_DECL:
		ctx.codegenVarDecl(node)
		return false

	case AST_RETURN:
		val := "0"
		if node.Data.(AstReturn).Expr != nil {
			val, _ = ctx.codegenExpr(node.Data.(AstReturn).Expr)
		}
		ctx.out.WriteString(fmt.Sprintf("\tret %s\n", val))
		return true

	case AST_IF:
		d := node.Data.(AstIf)
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

	case AST_WHILE:
		d := node.Data.(AstWhile)
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

	case AST_SWITCH:
		return ctx.codegenSwitch(node)

	case AST_LABEL:
		d := node.Data.(AstLabel)
		ctx.out.WriteString(fmt.Sprintf("@%s\n", d.Name))
		return ctx.codegenStmt(d.Stmt)

	case AST_GOTO:
		d := node.Data.(AstGoto)
		ctx.out.WriteString(fmt.Sprintf("\tjmp @%s\n", d.Label))
		return true

	case AST_BREAK:
		if ctx.breakLabel == "" {
			Error(node.Tok, "'break' not in a loop or switch.")
		}
		ctx.out.WriteString(fmt.Sprintf("\tjmp %s\n", ctx.breakLabel))
		return true

	case AST_CASE:
		d := node.Data.(AstCase)
		ctx.out.WriteString(d.QbeLabel + "\n")
		return ctx.codegenStmt(d.Body)

	case AST_DEFAULT:
		d := node.Data.(AstDefault)
		ctx.out.WriteString(d.QbeLabel + "\n")
		return ctx.codegenStmt(d.Body)

	default:
		// Handle expression statements like `a + b;` or `foo();`
		if node.Type <= AST_SUBSCRIPT {
			_, terminates := ctx.codegenExpr(node)
			return terminates
		}
		return false
	}
}

func (ctx *CodegenContext) findAllAutosInFunc(node *AstNode, autoVars *[]AutoVarInfo, definedNames map[string]bool) {
	if node == nil {
		return
	}
	if node.Type == AST_VAR_DECL {
		varData := node.Data.(AstVarDecl)
		if !definedNames[varData.Name] {
			definedNames[varData.Name] = true
			var size int64
			if varData.IsVector {
				if varData.SizeExpr != nil {
					folded := FoldConstants(varData.SizeExpr)
					if folded.Type != AST_NUMBER {
						Error(node.Tok, "Local vector size must be a constant expression.")
					}
					sizeInWords := folded.Data.(AstNumber).Value
					if varData.IsBracketed {
						size = (sizeInWords + 1) * int64(ctx.wordSize)
					} else {
						size = sizeInWords * int64(ctx.wordSize)
					}
				} else if len(varData.InitList) == 1 && varData.InitList[0].Type == AST_STRING {
					// Size of a vector initialized by a string literal
					strLen := int64(len(varData.InitList[0].Data.(AstString).Value))
					numBytes := strLen + 1 // Include null terminator
					// Align size to word boundary
					size = (numBytes + int64(ctx.wordSize) - 1) / int64(ctx.wordSize) * int64(ctx.wordSize)
				} else {
					size = int64(len(varData.InitList)) * int64(ctx.wordSize)
				}
				if size == 0 {
					// A vector must have space for at least one element
					size = int64(ctx.wordSize)
				}
				// A vector variable requires space for a pointer to its data,
				// in addition to the data itself
				size += int64(ctx.wordSize)
			} else {
				size = int64(ctx.wordSize)
			}
			*autoVars = append(*autoVars, AutoVarInfo{Node: node, Size: size})
		}
	}

	// Recurse into scopes
	switch d := node.Data.(type) {
	case AstIf:
		ctx.findAllAutosInFunc(d.ThenBody, autoVars, definedNames)
		ctx.findAllAutosInFunc(d.ElseBody, autoVars, definedNames)
	case AstWhile:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case AstBlock:
		for _, s := range d.Stmts {
			ctx.findAllAutosInFunc(s, autoVars, definedNames)
		}
	case AstSwitch:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case AstCase:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case AstDefault:
		ctx.findAllAutosInFunc(d.Body, autoVars, definedNames)
	case AstLabel:
		ctx.findAllAutosInFunc(d.Stmt, autoVars, definedNames)
	}
}

func (ctx *CodegenContext) codegenFuncDecl(node *AstNode) {
	d := node.Data.(AstFuncDecl)
	// Inline assembly functions are emitted directly
	if d.Body != nil && d.Body.Type == AST_ASM_STMT {
		asmCode := d.Body.Data.(AstAsmStmt).Code
		ctx.asmOut.WriteString(fmt.Sprintf(".globl %s\n%s:\n\t%s\n", d.Name, d.Name, strings.ReplaceAll(asmCode, "\n", "\n\t")))
		return
	}
	// Do not generate code for declarations (extrn functions)
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

	// Pass 1: Find all auto variables to calculate stack frame size
	var autoVars []AutoVarInfo
	definedInFunc := make(map[string]bool)
	for _, p := range d.Params {
		definedInFunc[p.Data.(AstIdent).Name] = true
	}
	ctx.findAllAutosInFunc(d.Body, &autoVars, definedInFunc)

	// The list of autos is built by recursive traversal, so it must be
	// reversed to match declaration order for stack layout
	for i, j := 0, len(autoVars)-1; i < j; i, j = i+1, j-1 {
		autoVars[i], autoVars[j] = autoVars[j], autoVars[i]
	}

	var totalFrameSize int64
	allLocals := make([]AutoVarInfo, 0, len(d.Params)+len(autoVars))
	paramInfos := make([]AutoVarInfo, len(d.Params))
	for i, p := range d.Params {
		paramInfos[i] = AutoVarInfo{Node: p, Size: int64(ctx.wordSize)}
	}
	// Parameters are laid out on the stack in reverse order
	for i, j := 0, len(paramInfos)-1; i < j; i, j = i+1, j-1 {
		paramInfos[i], paramInfos[j] = paramInfos[j], paramInfos[i]
	}

	allLocals = append(allLocals, paramInfos...)
	allLocals = append(allLocals, autoVars...)

	for _, local := range allLocals {
		totalFrameSize += local.Size
	}

	// Allocate space for the stack frame
	if totalFrameSize > 0 {
		ctx.currentFuncFrame = ctx.newTemp()
		ctx.out.WriteString(fmt.Sprintf("\t%s =%s alloc8 %d\n", ctx.currentFuncFrame, ctx.wordType, totalFrameSize))
	}

	ctx.enterScope()
	defer ctx.exitScope()

	// Pass 2: Create symbols for parameters and locals and initialize them
	var currentOffset int64
	for _, local := range allLocals {
		var sym *Symbol
		if local.Node.Type == AST_IDENT { // A parameter
			p := local.Node
			sym = ctx.addSymbol(p.Data.(AstIdent).Name, SYM_VAR, false, p.Tok)
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
		} else { // An auto variable
			varData := local.Node.Data.(AstVarDecl)
			sym = ctx.addSymbol(varData.Name, SYM_VAR, varData.IsVector, local.Node.Tok)
			sym.StackOffset = currentOffset
			ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", sym.QbeName, ctx.wordType, ctx.currentFuncFrame, sym.StackOffset))

			// The vector variable holds a pointer to the start of its allocated data
			if varData.IsVector {
				storageAddr := ctx.newTemp()
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", storageAddr, ctx.wordType, sym.QbeName, ctx.wordSize))
				ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, storageAddr, sym.QbeName))
			}
		}
		currentOffset += local.Size
	}

	bodyTerminates := ctx.codegenStmt(d.Body)

	// Functions that do not explicitly return are assumed to return 0
	if !bodyTerminates {
		ctx.out.WriteString("\tret 0\n")
	}
	ctx.out.WriteString("}\n\n")
}

func (ctx *CodegenContext) codegenGlobalConst(node *AstNode) string {
	folded := FoldConstants(node)
	switch folded.Type {
	case AST_NUMBER:
		return fmt.Sprintf("%d", folded.Data.(AstNumber).Value)
	case AST_STRING:
		return ctx.addString(folded.Data.(AstString).Value)
	case AST_IDENT:
		name := folded.Data.(AstIdent).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			Error(node.Tok, "Undefined symbol '%s' in global initializer.", name)
		}
		if sym.IsVector {
			// A vector's name refers to the pointer, not the data. The data is
			// in a separate storage symbol
			return "$_" + name + "_storage"
		}
		return sym.QbeName
	case AST_ADDRESS_OF:
		lval := folded.Data.(AstAddressOf).LValue
		if lval.Type != AST_IDENT {
			Error(lval.Tok, "Global initializer must be the address of a global symbol.")
		}
		name := lval.Data.(AstIdent).Name
		sym := ctx.findSymbol(name)
		if sym == nil {
			Error(lval.Tok, "Undefined symbol '%s' in global initializer.", name)
		}
		if sym.IsVector {
			return "$_" + name + "_storage"
		}
		return sym.QbeName
	default:
		Error(node.Tok, "Global initializer must be a constant expression.")
		return ""
	}
}

func (ctx *CodegenContext) codegenVarDecl(node *AstNode) {
	d := node.Data.(AstVarDecl)
	sym := ctx.findSymbol(d.Name)
	if sym == nil {
		Error(node.Tok, "Internal error: symbol '%s' not found during declaration.", d.Name)
	}

	if ctx.currentScope.Parent == nil { // Global
		if d.IsVector {
			storageName := "$_" + d.Name + "_storage"
			sizeInWords := int64(0)
			if d.SizeExpr != nil {
				sizeExprData := FoldConstants(d.SizeExpr).Data.(AstNumber)
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

			// The global vector variable is a pointer to its data
			ctx.out.WriteString(fmt.Sprintf("data %s = { %s %s }\n", sym.QbeName, ctx.wordType, storageName))

		} else { // Global scalar
			ctx.out.WriteString(fmt.Sprintf("data %s = align %d { ", sym.QbeName, ctx.wordSize))
			if len(d.InitList) > 0 {
				val := ctx.codegenGlobalConst(d.InitList[0])
				fmt.Fprintf(&ctx.out, "%s %s", ctx.wordType, val)
			} else {
				// Uninitialized globals are zeroed
				fmt.Fprintf(&ctx.out, "z %d", ctx.wordSize)
			}
			ctx.out.WriteString(" }\n")
		}

	} else { // Auto
		if len(d.InitList) > 0 {
			if d.IsVector {
				// Get the pointer to the vector's data on the stack
				storagePtr := ctx.newTemp()
				ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", storagePtr, ctx.wordType, sym.QbeName, ctx.wordSize))

				if len(d.InitList) == 1 && d.InitList[0].Type == AST_STRING {
					// `blit` is used for efficient block memory copy
					strVal := d.InitList[0].Data.(AstString).Value
					strLabel := ctx.addString(strVal)
					sizeToCopy := len(strVal) + 1 // Copy the null terminator
					ctx.out.WriteString(fmt.Sprintf("\tblit %s, %s, %d\n", strLabel, storagePtr, sizeToCopy))
				} else {
					// Initialize vector elements from a list of expressions
					for i, initExpr := range d.InitList {
						offset := int64(i) * int64(ctx.wordSize)
						elemAddr := ctx.newTemp()
						ctx.out.WriteString(fmt.Sprintf("\t%s =%s add %s, %d\n", elemAddr, ctx.wordType, storagePtr, offset))
						rval, _ := ctx.codegenExpr(initExpr)
						ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, rval, elemAddr))
					}
				}
			} else {
				// Initialize a scalar auto
				rval, _ := ctx.codegenExpr(d.InitList[0])
				ctx.out.WriteString(fmt.Sprintf("\tstore%s %s, %s\n", ctx.wordType, rval, sym.QbeName))
			}
		}
	}
}

func (ctx *CodegenContext) codegenSwitch(node *AstNode) bool {
	d := node.Data.(AstSwitch)
	switchVal, _ := ctx.codegenExpr(d.Expr)
	endLabel := ctx.newLabel()

	oldBreakLabel := ctx.breakLabel
	ctx.breakLabel = endLabel
	defer func() { ctx.breakLabel = oldBreakLabel }()

	defaultTarget := endLabel
	if d.DefaultLabelName != "" {
		defaultTarget = d.DefaultLabelName
	}

	// Generate a series of conditional jumps for the case values
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

	// Generate the code for the switch's body block
	bodyTerminates := true
	if d.Body != nil && d.Body.Type == AST_BLOCK {
		codegenStarted := false
		allPathsTerminate := true
		bodyStmts := d.Body.Data.(AstBlock).Stmts
		for _, stmt := range bodyStmts {
			isLabel := stmt.Type == AST_CASE || stmt.Type == AST_DEFAULT || stmt.Type == AST_LABEL
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
	// A switch statement terminates if all its case paths, including
	// the default
	return bodyTerminates && d.DefaultLabelName != ""
}
