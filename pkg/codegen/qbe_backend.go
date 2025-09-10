package codegen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
	"modernc.org/libqbe"
)

type qbeBackend struct{
	out         *strings.Builder
	prog        *ir.Program
	currentFn   *ir.Func
	structTypes map[string]bool
	extCounter  int
}

func NewQBEBackend() Backend { return &qbeBackend{structTypes: make(map[string]bool)} }

func (b *qbeBackend) Generate(prog *ir.Program, cfg *config.Config) (*bytes.Buffer, error) {
	qbeIR, err := b.GenerateIR(prog, cfg)
	if err != nil { return nil, err }

	var asmBuf bytes.Buffer
	err = libqbe.Main(cfg.BackendTarget, "input.ssa", strings.NewReader(qbeIR), &asmBuf, nil)
	if err != nil { return nil, fmt.Errorf("\n--- QBE Compilation Failed ---\nGenerated IR:\n%s\n\nlibqbe error: %w", qbeIR, err) }
	return &asmBuf, nil
}

func (b *qbeBackend) GenerateIR(prog *ir.Program, cfg *config.Config) (string, error) {
	var qbeIRBuilder strings.Builder
	b.out = &qbeIRBuilder
	b.prog = prog

	b.gen()

	return qbeIRBuilder.String(), nil
}

func (b *qbeBackend) gen() {
	b.genStructTypes()

	for _, g := range b.prog.Globals {
		b.genGlobal(g)
	}

	if len(b.prog.Strings) > 0 {
		b.out.WriteString("\n")
		for s, label := range b.prog.Strings {
			b.out.WriteString(fmt.Sprintf("data $%s = { ", label))

			if len(s) == 0 {
				b.out.WriteString("b 0 }\n")
				continue
			}

			for i := 0; i < len(s); i++ {
				if i > 0 { b.out.WriteString(", ") }
				b.out.WriteString(fmt.Sprintf("b %d", s[i]))
			}
			b.out.WriteString(", b 0 }\n")
		}
	}

	for _, fn := range b.prog.Funcs {
		b.genFunc(fn)
	}
}

func (b *qbeBackend) formatFieldType(t *ast.BxType) (string, bool) {
	if t == nil { return b.formatType(ir.GetType(nil, b.prog.WordSize)), true }
	switch t.Kind {
	case ast.TYPE_STRUCT:
		if t.Name != "" {
			if _, defined := b.structTypes[t.Name]; defined { return ":" + t.Name, true }
		}
		return "", false
	case ast.TYPE_POINTER, ast.TYPE_ARRAY: return b.formatType(ir.GetType(nil, b.prog.WordSize)), true
	default: return b.formatType(ir.GetType(t, b.prog.WordSize)), true
	}
}

func (b *qbeBackend) genStructTypes() {
	allStructs := make(map[string]*ast.BxType)

	var collect func(t *ast.BxType)
	collect = func(t *ast.BxType) {
		if t == nil {
			return
		}
		if t.Kind == ast.TYPE_STRUCT {
			if _, exists := allStructs[t.Name]; !exists && t.Name != "" {
				allStructs[t.Name] = t
				for _, f := range t.Fields {
					collect(f.Data.(ast.VarDeclNode).Type)
				}
			}
		} else if t.Kind == ast.TYPE_POINTER || t.Kind == ast.TYPE_ARRAY {
			collect(t.Base)
		}
	}

	for _, g := range b.prog.Globals {
		collect(g.AstType)
	}
	for _, f := range b.prog.Funcs {
		collect(f.AstReturnType)
		if f.AstParams != nil {
			for _, pNode := range f.AstParams {
				if pNode.Type == ast.VarDecl {
					collect(pNode.Data.(ast.VarDeclNode).Type)
				}
			}
		} else if len(f.Params) > 0 && f.Name != "" {
			if symNode := b.prog.FindFuncSymbol(f.Name); symNode != nil {
				if decl, ok := symNode.Data.(ast.FuncDeclNode); ok {
					for _, p := range decl.Params {
						if p.Type == ast.VarDecl {
							collect(p.Data.(ast.VarDeclNode).Type)
						}
					}
				}
			}
		}
	}

	if len(allStructs) == 0 {
		return
	}

	b.out.WriteString("\n")
	definedCount := -1
	for len(b.structTypes) < len(allStructs) && len(b.structTypes) != definedCount {
		definedCount = len(b.structTypes)
		for name, typ := range allStructs {
			if b.structTypes[name] {
				continue
			}

			var fieldTypes []string
			canDefine := true
			for _, field := range typ.Fields {
				fType := field.Data.(ast.VarDeclNode).Type
				typeStr, ok := b.formatFieldType(fType)
				if !ok {
					canDefine = false
					break
				}
				fieldTypes = append(fieldTypes, typeStr)
			}

			if canDefine {
				fmt.Fprintf(b.out, "type :%s = { %s }\n", name, strings.Join(fieldTypes, ", "))
				b.structTypes[name] = true
			}
		}
	}
}

func (b *qbeBackend) genGlobal(g *ir.Data) {
	alignStr := ""
	if g.Align > 0 {
		alignStr = fmt.Sprintf("align %d ", g.Align)
	}

	fmt.Fprintf(b.out, "data $%s = %s{ ", g.Name, alignStr)
	for i, item := range g.Items {
		if item.Count > 0 {
			size := int64(item.Count)
			if item.Typ != ir.TypeB {
				size *= ir.SizeOfType(item.Typ, b.prog.WordSize)
			}
			fmt.Fprintf(b.out, "z %d", size)
		} else {
			fmt.Fprintf(b.out, "%s %s", b.formatType(item.Typ), b.formatValue(item.Value))
		}
		if i < len(g.Items)-1 {
			b.out.WriteString(", ")
		}
	}
	b.out.WriteString(" }\n")
}

func (b *qbeBackend) genFunc(fn *ir.Func) {
	b.currentFn = fn
	var retTypeStr string
	if fn.AstReturnType != nil && fn.AstReturnType.Kind == ast.TYPE_STRUCT {
		retTypeStr = " :" + fn.AstReturnType.Name
	} else {
		retTypeStr = b.formatType(fn.ReturnType)
		if retTypeStr != "" {
			retTypeStr = " " + retTypeStr
		}
	}

	fmt.Fprintf(b.out, "\nexport function%s $%s(", retTypeStr, fn.Name)

	for i, p := range fn.Params {
		paramType := p.Typ
		if paramType == ir.TypeB || paramType == ir.TypeH {
			paramType = ir.GetType(nil, b.prog.WordSize)
		}
		fmt.Fprintf(b.out, "%s %s", b.formatType(paramType), b.formatValue(p.Val))
		if i < len(fn.Params)-1 {
			b.out.WriteString(", ")
		}
	}

	if fn.HasVarargs {
		if len(fn.Params) > 0 {
			b.out.WriteString(", ")
		}
		b.out.WriteString("...")
	}
	b.out.WriteString(") {\n")

	for _, block := range fn.Blocks {
		b.genBlock(block)
	}

	b.out.WriteString("}\n")
}

func (b *qbeBackend) genBlock(block *ir.BasicBlock) {
	fmt.Fprintf(b.out, "@%s\n", block.Label.Name)
	for _, instr := range block.Instructions {
		b.genInstr(instr)
	}
}

func (b *qbeBackend) genInstr(instr *ir.Instruction) {
	if instr.Op == ir.OpCall {
		b.out.WriteString("\t")
		b.genCall(instr)
		return
	}

	// Handle special case for byte arithmetic operations
	isArithmetic := (instr.Op >= ir.OpAdd && instr.Op <= ir.OpShr) || (instr.Op >= ir.OpAddF && instr.Op <= ir.OpNegF)
	if isArithmetic && (instr.Typ == ir.TypeB || instr.Typ == ir.TypeSB || instr.Typ == ir.TypeUB) {
		// For byte arithmetic, generate as word arithmetic with appropriate conversions
		b.genByteArithmetic(instr)
		return
	}

	// Handle special case for float arithmetic operations requiring type conversion
	isFloatArithmetic := instr.Op >= ir.OpAddF && instr.Op <= ir.OpRemF
	if isFloatArithmetic && instr.Typ == ir.TypeD {
		// For double precision float arithmetic, generate with appropriate conversions
		b.genFloatArithmetic(instr)
		return
	}

	b.out.WriteString("\t")
	if instr.Result != nil {
		resultType := instr.Typ
		isComparison := instr.Op >= ir.OpCEq && instr.Op <= ir.OpCGe

		if isComparison {
			resultType = ir.GetType(nil, b.prog.WordSize)
		}

		// In QBE, temporaries can only have base types. On 64-bit systems, promote sub-word types to long (l)
		if instr.Op == ir.OpLoad && b.isSubWordType(instr.Typ) {
			resultType = ir.GetType(nil, b.prog.WordSize)
		}

		// For cast operations, ensure result types are base types
		if instr.Op == ir.OpCast && b.isSubWordType(resultType) {
			resultType = ir.GetType(nil, b.prog.WordSize)
		}

		fmt.Fprintf(b.out, "%s =%s ", b.formatValue(instr.Result), b.formatType(resultType))
	}

	opStr, _ := b.formatOp(instr)
	b.out.WriteString(opStr)

	if instr.Op == ir.OpPhi {
		for i := 0; i < len(instr.Args); i += 2 {
			fmt.Fprintf(b.out, " @%s %s", instr.Args[i].String(), b.formatValue(instr.Args[i+1]))
			if i+2 < len(instr.Args) {
				b.out.WriteString(",")
			}
		}
	} else {
		for i, arg := range instr.Args {
			b.out.WriteString(" ")
			if arg != nil {
				b.out.WriteString(b.formatValue(arg))
			}
			if i < len(instr.Args)-1 {
				b.out.WriteString(",")
			}
		}
	}
	b.out.WriteString("\n")
}

func (b *qbeBackend) genFloatArithmetic(instr *ir.Instruction) {
	// For float arithmetic operations, handle operands appropriately
	resultType := instr.Typ

	// Build arguments - no extension needed if operands match result type
	var args []string
	for _, arg := range instr.Args {
		if arg == nil {
			args = append(args, "")
			continue
		}

		if floatConst, ok := arg.(*ir.FloatConst); ok {
			// Float constants can be directly used with the proper format
			if resultType == ir.TypeD {
				args = append(args, fmt.Sprintf("d_%f", floatConst.Value))
			} else {
				args = append(args, b.formatValue(arg))
			}
		} else {
			// For temporaries, use directly - the type system ensures consistency
			args = append(args, b.formatValue(arg))
		}
	}

	// Generate the arithmetic instruction
	b.out.WriteString("\t")
	if instr.Result != nil {
		fmt.Fprintf(b.out, "%s =%s ", b.formatValue(instr.Result), b.formatType(resultType))
	}

	opStr, _ := b.formatOp(instr)
	b.out.WriteString(opStr)

	for i, argStr := range args {
		b.out.WriteString(" " + argStr)
		if i < len(args)-1 {
			b.out.WriteString(",")
		}
	}
	b.out.WriteString("\n")
}

func (b *qbeBackend) genByteArithmetic(instr *ir.Instruction) {
	// For byte arithmetic operations, we need to handle operand type conversion
	// Generate intermediate temporaries with proper extensions when needed
	resultType := ir.GetType(nil, b.prog.WordSize) // long on 64-bit

	// Convert operands to the target type by generating extension instructions
	var convertedArgs []string
	for _, arg := range instr.Args {
		if arg == nil {
			convertedArgs = append(convertedArgs, "")
			continue
		}

		if const_arg, ok := arg.(*ir.Const); ok {
			// Constants can be directly used with the right value
			convertedArgs = append(convertedArgs, fmt.Sprintf("%d", const_arg.Value))
		} else {
			// For temporaries, we need to generate an extension instruction
			extTemp := fmt.Sprintf("%%ext_%d", b.extCounter)
			b.extCounter++
			// Generate extension instruction: extTemp =l extsw arg (word to long)
			b.out.WriteString(fmt.Sprintf("\t%s =%s extsw %s\n",
				extTemp, b.formatType(resultType), b.formatValue(arg)))
			convertedArgs = append(convertedArgs, extTemp)
		}
	}

	// Now generate the actual arithmetic instruction with converted operands
	b.out.WriteString("\t")
	if instr.Result != nil {
		fmt.Fprintf(b.out, "%s =%s ", b.formatValue(instr.Result), b.formatType(resultType))
	}

	opStr, _ := b.formatOp(instr)
	b.out.WriteString(opStr)

	for i, argStr := range convertedArgs {
		b.out.WriteString(" " + argStr)
		if i < len(convertedArgs)-1 {
			b.out.WriteString(",")
		}
	}
	b.out.WriteString("\n")
}

// isSubWordType checks if a type needs promotion for QBE function calls
func (b *qbeBackend) isSubWordType(t ir.Type) bool {
	return t == ir.TypeB || t == ir.TypeSB || t == ir.TypeUB ||
		t == ir.TypeH || t == ir.TypeSH || t == ir.TypeUH || t == ir.TypeW
}

func (b *qbeBackend) genCall(instr *ir.Instruction) {
	callee := instr.Args[0]
	calleeName := ""
	if g, ok := callee.(*ir.Global); ok {
		calleeName = g.Name
	}

	// Pre-generate all needed extension instructions
	var processedArgs []struct {
		value      string
		targetType ir.Type
	}

	for i, arg := range instr.Args[1:] {
		argType := ir.GetType(nil, b.prog.WordSize)
		if instr.ArgTypes != nil && i < len(instr.ArgTypes) {
			argType = instr.ArgTypes[i]
		}

		argValue := b.formatValue(arg)
		targetType := argType

		// Promote sub-word types to target word size and generate extension if needed
		if b.isSubWordType(argType) {
			targetType = ir.GetType(nil, b.prog.WordSize)
			if argType != targetType {
				extTemp := fmt.Sprintf("%%ext_%d", b.extCounter)
				b.extCounter++

				// Select extension operation based on source type
				var extOp string
				switch argType {
				case ir.TypeW: extOp = "extsw"
				case ir.TypeUB: extOp = "extub"
				case ir.TypeSB: extOp = "extsb"
				case ir.TypeUH: extOp = "extuh"
				case ir.TypeSH: extOp = "extsh"
				default:
					extOp = "extub" // Default for ambiguous b/h types
				}

				// Generate extension instruction before the call
				fmt.Fprintf(b.out, "\t%s =%s %s %s\n", extTemp, b.formatType(targetType), extOp, argValue)
				argValue = extTemp
			}
		}

		processedArgs = append(processedArgs, struct {
			value      string
			targetType ir.Type
		}{argValue, targetType})
	}

	// Generate result assignment if needed
	if instr.Result != nil {
		var retTypeStr string
		calledFunc := b.prog.FindFunc(calleeName)
		if calledFunc != nil && calledFunc.AstReturnType != nil && calledFunc.AstReturnType.Kind == ast.TYPE_STRUCT {
			retTypeStr = " :" + calledFunc.AstReturnType.Name
		} else {
			retTypeStr = b.formatType(instr.Typ)
		}
		fmt.Fprintf(b.out, "\t%s =%s ", b.formatValue(instr.Result), retTypeStr)
	} else {
		b.out.WriteString("\t")
	}

	// Generate call with processed arguments
	fmt.Fprintf(b.out, "call %s(", b.formatValue(callee))
	for i, arg := range processedArgs {
		if i > 0 {
			b.out.WriteString(", ")
		}
		fmt.Fprintf(b.out, "%s %s", b.formatType(arg.targetType), arg.value)
	}
	b.out.WriteString(")\n")
}

func (b *qbeBackend) formatValue(v ir.Value) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case *ir.Const: return fmt.Sprintf("%d", val.Value)
	case *ir.FloatConst:
		if val.Typ == ir.TypeS {
			// For 32-bit floats, truncate to float32 precision first
			float32Val := float32(val.Value)
			return fmt.Sprintf("s_%f", float64(float32Val))
		}
		return fmt.Sprintf("%s_%f", b.formatType(val.Typ), val.Value)
	case *ir.Global: return "$" + val.Name
	case *ir.Temporary:
		safeName := strings.NewReplacer(".", "_", "[", "_", "]", "_").Replace(val.Name)
		if val.ID == -1 {
			return "%" + safeName
		}
		if safeName != "" {
			return fmt.Sprintf("%%.%s_%d", safeName, val.ID)
		}
		return fmt.Sprintf("%%t%d", val.ID)
	case *ir.Label: return "@" + val.Name
	default:
		return ""
	}
}

func (b *qbeBackend) formatType(t ir.Type) string {
	switch t {
	case ir.TypeB, ir.TypeSB, ir.TypeUB:
		return "b"
	case ir.TypeH, ir.TypeSH, ir.TypeUH:
		return "h"
	case ir.TypeW:
		return "w"
	case ir.TypeL:
		return "l"
	case ir.TypeS:
		return "s"
	case ir.TypeD:
		return "d"
	case ir.TypePtr:
		return b.formatType(ir.GetType(nil, b.prog.WordSize))
	default:
		return ""
	}
}

func (b *qbeBackend) getCmpInstType(argType ir.Type) string {
	if b.isSubWordType(argType) {
		return b.formatType(ir.GetType(nil, b.prog.WordSize))
	}
	return b.formatType(argType)
}

func (b *qbeBackend) formatOp(instr *ir.Instruction) (opStr string, isCall bool) {
	typ := instr.Typ
	argType := instr.OperandType
	if argType == ir.TypeNone {
		argType = instr.Typ
	}

	typeStr := b.formatType(typ)
	argTypeStr := b.getCmpInstType(argType)

	switch instr.Op {
	case ir.OpAlloc:
		if instr.Align <= 4 {
			return "alloc4", false
		}
		if instr.Align <= 8 {
			return "alloc8", false
		}
		return "alloc16", false
	case ir.OpLoad:
		switch typ {
		case ir.TypeB:
			return "loadub", false // ambiguous, default to unsigned
		case ir.TypeSB:
			return "loadsb", false // signed byte
		case ir.TypeUB:
			return "loadub", false // unsigned byte
		case ir.TypeH:
			return "loaduh", false // ambiguous, default to unsigned
		case ir.TypeSH:
			return "loadsh", false // signed half
		case ir.TypeUH:
			return "loaduh", false // unsigned half
		case ir.TypePtr:
			return "load" + b.formatType(ir.GetType(nil, b.prog.WordSize)), false
		default:
			return "load" + typeStr, false
		}
	case ir.OpStore:
		return "store" + typeStr, false
	case ir.OpBlit:
		return "blit", false
	case ir.OpAdd, ir.OpAddF:
		return "add", false
	case ir.OpSub, ir.OpSubF:
		return "sub", false
	case ir.OpMul, ir.OpMulF:
		return "mul", false
	case ir.OpDiv, ir.OpDivF:
		return "div", false
	case ir.OpRem, ir.OpRemF:
		return "rem", false
	case ir.OpAnd:
		return "and", false
	case ir.OpOr:
		return "or", false
	case ir.OpXor:
		return "xor", false
	case ir.OpShl:
		return "shl", false
	case ir.OpShr:
		return "shr", false
	case ir.OpNegF:
		return "neg", false
	case ir.OpCEq:
		return "ceq" + argTypeStr, false
	case ir.OpCNeq:
		return "cne" + argTypeStr, false
	case ir.OpCLt:
		if argType == ir.TypeS || argType == ir.TypeD {
			return "clt" + argTypeStr, false
		}
		return "cslt" + argTypeStr, false
	case ir.OpCGt:
		if argType == ir.TypeS || argType == ir.TypeD {
			return "cgt" + argTypeStr, false
		}
		return "csgt" + argTypeStr, false
	case ir.OpCLe:
		if argType == ir.TypeS || argType == ir.TypeD {
			return "cle" + argTypeStr, false
		}
		return "csle" + argTypeStr, false
	case ir.OpCGe:
		if argType == ir.TypeS || argType == ir.TypeD {
			return "cge" + argTypeStr, false
		}
		return "csge" + argTypeStr, false
	case ir.OpJmp:
		return "jmp", false
	case ir.OpJnz:
		return "jnz", false
	case ir.OpRet:
		return "ret", false
	case ir.OpCall:
		return "call", true
	case ir.OpPhi:
		return "phi", false
	case ir.OpSWToF:
		return "swtof", false
	case ir.OpSLToF:
		return "sltof", false
	case ir.OpFToF:
		if typ == ir.TypeD {
			return "exts", false
		}
		return "truncd", false
	case ir.OpExtSB, ir.OpExtUB, ir.OpExtSH, ir.OpExtUH, ir.OpExtSW, ir.OpExtUW:
		return "exts" + string(b.formatType(argType)[0]), false
	case ir.OpFToSI:
		return "ftosi", false
	case ir.OpFToUI:
		return "ftoui", false
	case ir.OpCast:
		return "copy", false
	default:
		return "unknown_op", false
	}
}
