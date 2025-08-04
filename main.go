package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"modernc.org/libqbe"
)

func main() {
	outFile := flag.String("o", "a.out", "Output file name.")
	std := flag.String("std", "Bx", "Specify language standard (B, Bx).")
	wall := flag.Bool("Wall", false, "Enable most warnings.")
	w_no_all := flag.Bool("Wno-all", false, "Disable all warnings.")
	w_pedantic := flag.Bool("pedantic", false, "Issue all warnings demanded by the current B std.")
	for _, wInfo := range warnings {
		flag.Bool("W"+wInfo.Name, false, "Enable '"+wInfo.Name+"' warning.")
		flag.Bool("Wno-"+wInfo.Name, false, "Disable '"+wInfo.Name+"' warning.")
	}
	for _, fInfo := range features {
		flag.Bool("F"+fInfo.Name, false, "Enable '"+fInfo.Name+"' feature.")
		flag.Bool("Fno-"+fInfo.Name, false, "Disable '"+fInfo.Name+"' feature.")
	}
	help := flag.Bool("h", false, "Display this information.")
	flag.BoolVar(help, "help", false, "Display this information.")
	flag.Usage = printHelp
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	applyStd(*std)

	if *w_no_all {
		setAllWarnings(false)
	}
	if *wall {
		oWARN_C_COMMENTS := IsWarningEnabled(WARN_C_COMMENTS)
		oWARN_C_OPS := IsWarningEnabled(WARN_C_OPS)
		oWARN_B_OPS := IsWarningEnabled(WARN_B_OPS)
		setAllWarnings(true)
		SetWarning(WARN_C_COMMENTS, oWARN_C_COMMENTS)
		SetWarning(WARN_C_OPS, oWARN_C_OPS)
		SetWarning(WARN_B_OPS, oWARN_B_OPS)
	}
	if *w_pedantic {
		SetWarning(WARN_PEDANTIC, true)
		applyStd(*std)
	}

	// Apply user-specified overrides
	flag.Visit(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "W") && f.Name != "Wall" {
			if strings.HasPrefix(f.Name, "Wno-") {
				name := strings.TrimPrefix(f.Name, "Wno-")
				if w, ok := warningMap[name]; ok {
					SetWarning(w, false)
				}
			} else {
				name := strings.TrimPrefix(f.Name, "W")
				if w, ok := warningMap[name]; ok {
					SetWarning(w, true)
				}
			}
		}
		if strings.HasPrefix(f.Name, "F") {
			if strings.HasPrefix(f.Name, "Fno-") {
				name := strings.TrimPrefix(f.Name, "Fno-")
				if feat, ok := featureMap[name]; ok {
					SetFeature(feat, false)
				}
			} else {
				name := strings.TrimPrefix(f.Name, "F")
				if feat, ok := featureMap[name]; ok {
					SetFeature(feat, true)
				}
			}
		}
	})

	inputFiles := flag.Args()
	if len(inputFiles) == 0 {
		Error(Token{}, "no input files specified.")
	}

	fmt.Println("----------------------")

	fmt.Printf("Tokenizing %d source file(s)...\n", len(inputFiles))
	records, allTokens := readAndTokenizeFiles(inputFiles)
	SetSourceFiles(records)

	fmt.Println("Parsing tokens into AST...")
	parser := NewParser(allTokens)
	ast := parser.Parse()

	fmt.Println("Constant folding...")
	ast = FoldConstants(ast)

	fmt.Println("QBE Codegen...")
	codegen := NewCodegenContext()
	qbeIR, inlineAsm := codegen.Generate(ast)

	fmt.Println("Calling libqbe on our QBE IR...")
	target := libqbe.DefaultTarget(runtime.GOOS, runtime.GOARCH)
	var asmBuf bytes.Buffer
	err := libqbe.Main(target, "input.ssa", strings.NewReader(qbeIR), &asmBuf, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n--- QBE Compilation Failed ---\nGenerated IR:\n%s\n", qbeIR)
		Error(Token{}, "libqbe error: %v", err)
	}

	fmt.Printf("Assembling and linking to create '%s'...\n", *outFile)
	err = assembleAndLink(*outFile, asmBuf.String(), inlineAsm)
	if err != nil {
		Error(Token{}, "assembler/linker failed: %v", err)
	}

	fmt.Println("----------------------")
	fmt.Println("✅ Compilation successful!")
}

func assembleAndLink(outFile, mainAsm, inlineAsm string) error {
	mainAsmFile, err := os.CreateTemp("", "gbc-main-*.s")
	if err != nil {
		return fmt.Errorf("failed to create temp file for main asm: %w", err)
	}
	defer os.Remove(mainAsmFile.Name())
	if _, err := mainAsmFile.WriteString(mainAsm); err != nil {
		return fmt.Errorf("failed to write to temp file for main asm: %w", err)
	}
	mainAsmFile.Close()

	inlineAsmFile, err := os.CreateTemp("", "gbc-inline-*.s")
	if err != nil {
		return fmt.Errorf("failed to create temp file for inline asm: %w", err)
	}
	defer os.Remove(inlineAsmFile.Name())
	if _, err := inlineAsmFile.WriteString(inlineAsm); err != nil {
		return fmt.Errorf("failed to write to temp file for inline asm: %w", err)
	}
	inlineAsmFile.Close()

	cmd := exec.Command("cc", "-s", "-static-pie", "-o", outFile, mainAsmFile.Name(), inlineAsmFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cc command failed: %w\nOutput:\n%s", err, string(output))
	}

	return nil
}

func readAndTokenizeFiles(paths []string) ([]SourceFileRecord, []Token) {
	var records []SourceFileRecord
	var allTokens []Token

	for i, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			Error(Token{FileIndex: -1}, "could not open file '%s': %v", path, err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			Error(Token{FileIndex: -1}, "could not read file '%s': %v", path, err)
		}

		runeContent := []rune(string(content))
		records = append(records, SourceFileRecord{Name: path, Content: runeContent})

		lexer := NewLexer(runeContent, i)
		for {
			tok := lexer.Next()
			if tok.Type == TOK_EOF {
				break
			}
			allTokens = append(allTokens, tok)
		}
	}
	finalFileIndex := 0
	if len(paths) > 0 {
		finalFileIndex = len(paths) - 1
	}
	allTokens = append(allTokens, Token{Type: TOK_EOF, FileIndex: finalFileIndex})
	return records, allTokens
}

func applyStd(stdName string) {
	isPedantic := IsWarningEnabled(WARN_PEDANTIC)

	switch stdName {
	case "B":
		// Flexible B: enable modern features unless pedantic
		SetFeature(FEAT_B_OPS, true)
		SetFeature(FEAT_B_ESCAPES, true)
		SetFeature(FEAT_C_OPS, !isPedantic)
		SetFeature(FEAT_C_ESCAPES, !isPedantic)
		SetFeature(FEAT_C_COMMENTS, !isPedantic)

		// Warn about modern features when in B mode
		SetWarning(WARN_C_OPS, true)
		SetWarning(WARN_C_ESCAPES, true)
		SetWarning(WARN_C_COMMENTS, true)
		// Don't warn about B features
		SetWarning(WARN_B_OPS, false)
		SetWarning(WARN_B_ESCAPES, false)

		// Strict B: disable non-standard features
		SetFeature(FEAT_EXTRN, !isPedantic)
		SetFeature(FEAT_ASM, !isPedantic)

	case "Bx":
		// Bx (Modern B): enable C-style features, disable old B-style features
		SetFeature(FEAT_B_OPS, false)
		SetFeature(FEAT_B_ESCAPES, false)

		SetFeature(FEAT_C_OPS, true)
		SetFeature(FEAT_C_ESCAPES, true)
		SetFeature(FEAT_C_COMMENTS, true)

		// Warn about historical B syntax when in Bx mode
		SetWarning(WARN_B_OPS, true) // Won't really change anything because FEAT_B_OPS, FEAT_B_ESCAPES is disabled
		SetWarning(WARN_B_ESCAPES, true)
		// Don't warn about C-style features
		SetWarning(WARN_C_OPS, false)
		SetWarning(WARN_C_ESCAPES, false)
		SetWarning(WARN_C_COMMENTS, false)

	default:
		Error(Token{}, "unsupported standard '%s'. Supported: 'B', 'Bx'.", stdName)
	}
}

// Help page formatting woes
func printSection(title string) {
	fmt.Fprintf(os.Stderr, "\n  %s\n", title)
}
func printItem(key, desc string) {
	fmt.Fprintf(os.Stderr, "    %-22s %s\n", key, desc)
}
func printListHeader(name string) {
	fmt.Fprintf(os.Stderr, "    Available %s:\n", name)
}
func printListItem(name, desc string, enabled bool) {
	state := "[-]"
	if enabled {
		state = "[x]"
	}
	const descWidth = 75
	descPadded := desc
	if len(desc) < descWidth {
		descPadded += strings.Repeat(" ", descWidth-len(desc))
	}
	fmt.Fprintf(os.Stderr, "      %-20s %s %s\n", name, descPadded, state)
}
// printHelp displays the compiler's command-line help information.
func printHelp() {
	fmt.Fprintf(os.Stderr, "\nCopyright (c) 2025: xplshn and contributors\n")
	fmt.Fprintf(os.Stderr, "For more details refer to <https://github.com/xplshn/gbc>\n")

	printSection("Synopsis")
	fmt.Fprintf(os.Stderr, "    gbc [options] <input.b> ...\n")

	printSection("Description")
	fmt.Fprintf(os.Stderr, "    A compiler for the B programming language and its extensions, written in Go.\n")

	printSection("Options")
	printItem("-o <file>", "Place the output into <file>.")
	printItem("-h, --help", "Display this information.")
	printItem("-std=<std>", "Specify language standard (B, Bx). Default: Bx")
	printItem("-pedantic", "Issue all warnings demanded by the current B std.")

	printSection("Warning Flags")
	printItem("-Wall", "Enable most warnings.")
	printItem("-Wno-all", "Disable all warnings.")
	printItem("-W<warning>", "Enable a specific warning.")
	printItem("-Wno-<warning>", "Disable a specific warning.")
	printListHeader("warnings")
	for i := WarningType(0); i < WARN_COUNT; i++ {
		info := warnings[i]
		printListItem(info.Name, info.Description, info.Enabled)
	}

	printSection("Feature Flags")
	printItem("-F<feature>", "Enable a specific feature.")
	printItem("-Fno-<feature>", "Disable a specific feature.")
	printListHeader("features")
	for i := FeatureType(0); i < FEAT_COUNT; i++ {
		info := features[i]
		printListItem(info.Name, info.Description, info.Enabled)
	}

	fmt.Fprintf(os.Stderr, "\n")
}
