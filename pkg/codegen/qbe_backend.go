package codegen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
	"modernc.org/libqbe"
)

// qbeBackend implements the Backend interface for the QBE intermediate language.
type qbeBackend struct {
	out  *strings.Builder
	prog *ir.Program
}

// NewQBEBackend creates a new instance of the QBE backend.
func NewQBEBackend() Backend {
	return &qbeBackend{}
}

// Generate translates a generic IR program into QBE intermediate language text.
func (b *qbeBackend) Generate(prog *ir.Program, cfg *config.Config) (*bytes.Buffer, error) {
	var qbeIRBuilder strings.Builder
	b.out = &qbeIRBuilder
	b.prog = prog

	b.gen()

	qbeIR := qbeIRBuilder.String()
	var asmBuf bytes.Buffer
	if err := libqbe.Main(cfg.BackendTarget, "input.ssa", strings.NewReader(qbeIR), &asmBuf, nil); err != nil {
		return nil, fmt.Errorf("\n--- QBE Compilation Failed ---\nGenerated IR:\n%s\n\nlibqbe error: %w", qbeIR, err)
	}
	return &asmBuf, nil
}

func (b *qbeBackend) gen() {
	for _, g := range b.prog.Globals {
		b.genGlobal(g)
	}

	if len(b.prog.Strings) > 0 {
		b.out.WriteString("\n# --- String Literals ---\n")
		for s, label := range b.prog.Strings {
			escaped := strconv.Quote(s)
			fmt.Fprintf(b.out, "data $%s = { b %s, b 0 }\n", label, escaped)
		}
	}

	for _, fn := range b.prog.Funcs {
		b.genFunc(fn)
	}
}

func (b *qbeBackend) genGlobal(g *ir.Data) {
	fmt.Fprintf(b.out, "data $%s = align %d { ", g.Name, g.Align)
	for i, item := range g.Items {
		if item.Count > 0 { // Zero-initialized
			fmt.Fprintf(b.out, "z %d", item.Count*int(ir.SizeOfType(item.Typ, b.prog.WordSize)))
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
	retTypeStr := b.formatType(fn.ReturnType)
	if retTypeStr != "" {
		retTypeStr = " " + retTypeStr
	}

	fmt.Fprintf(b.out, "\nexport function%s $%s(", retTypeStr, fn.Name)

	for i, p := range fn.Params {
		fmt.Fprintf(b.out, "%s %s", b.formatType(p.Typ), b.formatValue(p.Val))
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
	b.out.WriteString("\t")
	if instr.Result != nil {
		resultType := instr.Typ
		isComparison := false
		switch instr.Op {
		case ir.OpCEq, ir.OpCNeq, ir.OpCLt, ir.OpCGt, ir.OpCLe, ir.OpCGe:
			isComparison = true
		}

		if isComparison {
			resultType = ir.GetType(nil, b.prog.WordSize)
		}

		if instr.Op == ir.OpLoad && (instr.Typ == ir.TypeB || instr.Typ == ir.TypeH) {
			resultType = ir.GetType(nil, b.prog.WordSize)
		}

		fmt.Fprintf(b.out, "%s =%s ", b.formatValue(instr.Result), b.formatType(resultType))
	}

	opStr, isCall := b.formatOp(instr)
	b.out.WriteString(opStr)

	if isCall {
		fmt.Fprintf(b.out, " %s(", b.formatValue(instr.Args[0]))
		for i, arg := range instr.Args[1:] {
			argType := ir.GetType(nil, b.prog.WordSize)
			if instr.ArgTypes != nil && i < len(instr.ArgTypes) {
				argType = instr.ArgTypes[i]
			}
			if argType == ir.TypeB || argType == ir.TypeH {
				argType = ir.GetType(nil, b.prog.WordSize)
			}

			fmt.Fprintf(b.out, "%s %s", b.formatType(argType), b.formatValue(arg))
			if i < len(instr.Args)-2 {
				b.out.WriteString(", ")
			}
		}
		b.out.WriteString(")\n")
		return
	}

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

func (b *qbeBackend) formatValue(v ir.Value) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case *ir.Const:
		return fmt.Sprintf("%d", val.Value)
	case *ir.FloatConst:
		return fmt.Sprintf("%s_%f", b.formatType(val.Typ), val.Value)
	case *ir.Global:
		return "$" + val.Name
	case *ir.Temporary:
		safeName := strings.NewReplacer(".", "_", "[", "_", "]", "_").Replace(val.Name)
		if safeName != "" {
			return fmt.Sprintf("%%.%s_%d", safeName, val.ID)
		}
		return fmt.Sprintf("%%t%d", val.ID)
	case *ir.Label:
		return "@" + val.Name
	default:
		return ""
	}
}

func (b *qbeBackend) formatType(t ir.Type) string {
	switch t {
	case ir.TypeB:
		return "b"
	case ir.TypeH:
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

func (b *qbeBackend) getCmpInstType(t ir.Type) string {
	if t == ir.TypeB || t == ir.TypeH {
		return b.formatType(ir.GetType(nil, b.prog.WordSize))
	}
	return b.formatType(t)
}

func (b *qbeBackend) formatOp(instr *ir.Instruction) (opStr string, isCall bool) {
	typ := instr.Typ
	typeStr := b.formatType(typ)
	switch instr.Op {
	case ir.OpAlloc:
		// QBE's stack alloc instruction is based on word size, not arbitrary alignment.
		return "alloc" + strconv.Itoa(b.prog.WordSize), false
	case ir.OpLoad:
		switch typ {
		case ir.TypeB:
			return "loadub", false
		case ir.TypeH:
			return "loaduh", false
		case ir.TypePtr:
			return "load" + b.formatType(ir.GetType(nil, b.prog.WordSize)), false
		default:
			return "load" + typeStr, false
		}
	case ir.OpStore:
		return "store" + typeStr, false
	case ir.OpBlit:
		return "blit", false
	case ir.OpAdd:
		return "add", false
	case ir.OpSub:
		return "sub", false
	case ir.OpMul:
		return "mul", false
	case ir.OpDiv:
		return "div", false
	case ir.OpRem:
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
	case ir.OpCEq:
		return "ceq" + b.getCmpInstType(typ), false
	case ir.OpCNeq:
		return "cne" + b.getCmpInstType(typ), false
	case ir.OpCLt:
		if typ == ir.TypeS || typ == ir.TypeD {
			return "clt" + typeStr, false
		}
		return "cslt" + b.getCmpInstType(typ), false
	case ir.OpCGt:
		if typ == ir.TypeS || typ == ir.TypeD {
			return "cgt" + typeStr, false
		}
		return "csgt" + b.getCmpInstType(typ), false
	case ir.OpCLe:
		if typ == ir.TypeS || typ == ir.TypeD {
			return "cle" + typeStr, false
		}
		return "csle" + b.getCmpInstType(typ), false
	case ir.OpCGe:
		if typ == ir.TypeS || typ == ir.TypeD {
			return "cge" + typeStr, false
		}
		return "csge" + b.getCmpInstType(typ), false
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
	default:
		return "unknown_op", false
	}
}
