package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gbc/pkg/ast"
	"gbc/pkg/codegen"
	"gbc/pkg/lexer"
	"gbc/pkg/parser"
	"gbc/pkg/token"
	"gbc/pkg/util"

	"modernc.org/libqbe"
)

type linkerArgs []string
type includePaths []string

func (l *linkerArgs) String() string {
	return strings.Join(*l, " ")
}

func (l *linkerArgs) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func (i *includePaths) String() string {
	return strings.Join(*i, ":")
}

func (i *includePaths) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	outFile := flag.String("o", "a.out", "Output file name.")
	std := flag.String("std", "Bx", "Specify language standard (B, Bx).")
	wall := flag.Bool("Wall", false, "Enable most warnings.")
	w_no_all := flag.Bool("Wno-all", false, "Disable all warnings.")
	w_pedantic := flag.Bool("pedantic", false, "Issue all warnings demanded by the current B std.")

	var linkerFlags linkerArgs
	flag.Var(&linkerFlags, "L", "Pass argument to the linker")
	var userIncludePaths includePaths
	flag.Var(&userIncludePaths, "I", "Add a directory to the include path")
	libRequest := flag.String("l", "", "Link with a library (e.g., -lb for library 'b')")

	for _, wInfo := range util.Warnings {
		flag.Bool("W"+wInfo.Name, false, "Enable '"+wInfo.Name+"' warning.")
		flag.Bool("Wno-"+wInfo.Name, false, "Disable '"+wInfo.Name+"' warning.")
	}
	for _, fInfo := range util.Features {
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
		util.SetAllWarnings(false)
	}
	if *wall {
		oWARN_C_COMMENTS := util.IsWarningEnabled(util.WarnCComments)
		oWARN_C_OPS := util.IsWarningEnabled(util.WarnCOps)
		oWARN_B_OPS := util.IsWarningEnabled(util.WarnBOps)
		util.SetAllWarnings(true)
		util.SetWarning(util.WarnCComments, oWARN_C_COMMENTS)
		util.SetWarning(util.WarnCOps, oWARN_C_OPS)
		util.SetWarning(util.WarnBOps, oWARN_B_OPS)
	}
	if *w_pedantic {
		util.SetWarning(util.WarnPedantic, true)
		applyStd(*std)
	}

	flag.Visit(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "W") && f.Name != "Wall" {
			if strings.HasPrefix(f.Name, "Wno-") {
				name := strings.TrimPrefix(f.Name, "Wno-")
				if w, ok := util.WarningMap[name]; ok {
					util.SetWarning(w, false)
				}
			} else {
				name := strings.TrimPrefix(f.Name, "W")
				if w, ok := util.WarningMap[name]; ok {
					util.SetWarning(w, true)
				}
			}
		}
		if strings.HasPrefix(f.Name, "F") {
			if strings.HasPrefix(f.Name, "Fno-") {
				name := strings.TrimPrefix(f.Name, "Fno-")
				if feat, ok := util.FeatureMap[name]; ok {
					util.SetFeature(feat, false)
				}
			} else {
				name := strings.TrimPrefix(f.Name, "F")
				if feat, ok := util.FeatureMap[name]; ok {
					util.SetFeature(feat, true)
				}
			}
		}
	})

	inputFiles := flag.Args()
	if len(inputFiles) == 0 {
		util.Error(token.Token{}, "no input files specified.")
	}

	if *libRequest != "" {
		libPath := findLibrary(*libRequest, userIncludePaths)
		if libPath == "" {
			util.Error(token.Token{}, "could not find library '%s'", *libRequest)
		}
		inputFiles = append(inputFiles, libPath)
	}

	fmt.Println("----------------------")

	fmt.Printf("Tokenizing %d source file(s)...\n", len(inputFiles))
	records, allTokens := readAndTokenizeFiles(inputFiles)
	util.SetSourceFiles(records)

	fmt.Println("Parsing tokens into AST...")
	p := parser.NewParser(allTokens)
	astRoot := p.Parse()

	fmt.Println("Constant folding...")
	astRoot = ast.FoldConstants(astRoot)

	fmt.Println("QBE Codegen...")
	cg := codegen.NewContext(runtime.GOARCH)
	qbeIR, inlineAsm := cg.Generate(astRoot)

	fmt.Println("Calling libqbe on our QBE IR...")
	target := libqbe.DefaultTarget(runtime.GOOS, runtime.GOARCH)
	var asmBuf bytes.Buffer
	err := libqbe.Main(target, "input.ssa", strings.NewReader(qbeIR), &asmBuf, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n--- QBE Compilation Failed ---\nGenerated IR:\n%s\n", qbeIR)
		util.Error(token.Token{}, "libqbe error: %v", err)
	}

	fmt.Printf("Assembling and linking to create '%s'...\n", *outFile)
	err = assembleAndLink(*outFile, asmBuf.String(), inlineAsm, linkerFlags)
	if err != nil {
		util.Error(token.Token{}, "assembler/linker failed: %v", err)
	}

	fmt.Println("----------------------")
	fmt.Println("Compilation successful!")
}

func findLibrary(libName string, userPaths []string) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	filenames := []string{
		fmt.Sprintf("%s_%s_%s.b", libName, goarch, goos),
		fmt.Sprintf("%s_%s.b", libName, goarch),
		fmt.Sprintf("%s_%s.b", libName, goos),
		fmt.Sprintf("%s.b", libName),
	}

	defaultPaths := []string{
		"/usr/local/lib/gbc",
		"/usr/lib/gbc",
		"/lib/gbc",
	}

	searchPaths := append(userPaths, defaultPaths...)

	for _, path := range searchPaths {
		for _, fname := range filenames {
			fullPath := filepath.Join(path, fname)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}

	return ""
}

func assembleAndLink(outFile, mainAsm, inlineAsm string, linkerArgs []string) error {
	mainAsmFile, err := os.CreateTemp("", "gbc-main-*.s")
	if err != nil {
		return fmt.Errorf("failed to create temp file for main asm: %w", err)
	}
	defer os.Remove(mainAsmFile.Name())
	if _, err := mainAsmFile.WriteString(mainAsm); err != nil {
		return fmt.Errorf("failed to write to temp file for main asm: %w", err)
	}
	mainAsmFile.Close()

	var inlineAsmFileName string
	if inlineAsm != "" {
		inlineAsmFile, err := os.CreateTemp("", "gbc-inline-*.s")
		if err != nil {
			return fmt.Errorf("failed to create temp file for inline asm: %w", err)
		}
		defer os.Remove(inlineAsmFile.Name())
		if _, err := inlineAsmFile.WriteString(inlineAsm); err != nil {
			return fmt.Errorf("failed to write to temp file for inline asm: %w", err)
		}
		inlineAsmFile.Close()
		inlineAsmFileName = inlineAsmFile.Name()
	}

	ccArgs := []string{"-s", "-static-pie", "-o", outFile, mainAsmFile.Name()}
	if inlineAsmFileName != "" {
		ccArgs = append(ccArgs, inlineAsmFileName)
	}
	ccArgs = append(ccArgs, linkerArgs...)

	cmd := exec.Command("cc", ccArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cc command failed: %w\nOutput:\n%s", err, string(output))
	}

	return nil
}

func readAndTokenizeFiles(paths []string) ([]util.SourceFileRecord, []token.Token) {
	var records []util.SourceFileRecord
	var allTokens []token.Token

	for i, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			util.Error(token.Token{FileIndex: -1}, "could not open file '%s': %v", path, err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			util.Error(token.Token{FileIndex: -1}, "could not read file '%s': %v", path, err)
		}

		runeContent := []rune(string(content))
		records = append(records, util.SourceFileRecord{Name: path, Content: runeContent})

		l := lexer.NewLexer(runeContent, i)
		for {
			tok := l.Next()
			if tok.Type == token.EOF {
				break
			}
			allTokens = append(allTokens, tok)
		}
	}
	finalFileIndex := 0
	if len(paths) > 0 {
		finalFileIndex = len(paths) - 1
	}
	allTokens = append(allTokens, token.Token{Type: token.EOF, FileIndex: finalFileIndex})
	return records, allTokens
}

func applyStd(stdName string) {
	isPedantic := util.IsWarningEnabled(util.WarnPedantic)

	switch stdName {
	case "B":
		util.SetFeature(util.FeatBOps, true)
		util.SetFeature(util.FeatBEscapes, true)
		util.SetFeature(util.FeatCOps, !isPedantic)
		util.SetFeature(util.FeatCEscapes, !isPedantic)
		util.SetFeature(util.FeatCComments, !isPedantic)
		util.SetWarning(util.WarnCOps, true)
		util.SetWarning(util.WarnCEscapes, true)
		util.SetWarning(util.WarnCComments, true)
		util.SetWarning(util.WarnBOps, false)
		util.SetWarning(util.WarnBEscapes, false)
		util.SetFeature(util.FeatExtrn, !isPedantic)
		util.SetFeature(util.FeatAsm, !isPedantic)

	case "Bx":
		util.SetFeature(util.FeatBOps, false)
		util.SetFeature(util.FeatBEscapes, false)
		util.SetFeature(util.FeatCOps, true)
		util.SetFeature(util.FeatCEscapes, true)
		util.SetFeature(util.FeatCComments, true)
		util.SetWarning(util.WarnBOps, true)
		util.SetWarning(util.WarnBEscapes, true)
		util.SetWarning(util.WarnCOps, false)
		util.SetWarning(util.WarnCEscapes, false)
		util.SetWarning(util.WarnCComments, false)

	default:
		util.Error(token.Token{}, "unsupported standard '%s'. Supported: 'B', 'Bx'.", stdName)
	}
}

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

func printHelp() {
	fmt.Fprintf(os.Stderr, "\nCopyright (c) 2025: xplshn and contributors\n")
	fmt.Fprintf(os.Stderr, "For more details refer to <https://github.com/xplshn/gbc>\n")

	printSection("Synopsis")
	fmt.Fprintf(os.Stderr, "    gbc [options] <input.b> ...\n")

	printSection("Description")
	fmt.Fprintf(os.Stderr, "    A compiler for the B programming language and its extensions, written in Go.\n")

	printSection("Options")
	printItem("-o <file>", "Place the output into <file>.")
	printItem("-I <path>", "Add a directory to the include path.")
	printItem("-L <arg>", "Pass an argument to the linker.")
	printItem("-l<lib>", "Link with a library (e.g., -lb for 'b').")
	printItem("-h, --help", "Display this information.")
	printItem("-std=<std>", "Specify language standard (B, Bx). Default: Bx")
	printItem("-pedantic", "Issue all warnings demanded by the current B std.")

	printSection("Warning Flags")
	printItem("-Wall", "Enable most warnings.")
	printItem("-Wno-all", "Disable all warnings.")
	printItem("-W<warning>", "Enable a specific warning.")
	printItem("-Wno-<warning>", "Disable a specific warning.")
	printListHeader("warnings")
	for i := util.Warning(0); i < util.WarnCount; i++ {
		info := util.Warnings[i]
		printListItem(info.Name, info.Description, info.Enabled)
	}

	printSection("Feature Flags")
	printItem("-F<feature>", "Enable a specific feature.")
	printItem("-Fno-<feature>", "Disable a specific feature.")
	printListHeader("features")
	for i := util.Feature(0); i < util.FeatCount; i++ {
		info := util.Features[i]
		printListItem(info.Name, info.Description, info.Enabled)
	}

	fmt.Fprintf(os.Stderr, "\n")
}
