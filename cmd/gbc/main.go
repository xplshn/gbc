package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xplshn/gbc/pkg/ast"
	"github.com/xplshn/gbc/pkg/cli"
	"github.com/xplshn/gbc/pkg/codegen"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/lexer"
	"github.com/xplshn/gbc/pkg/parser"
	"github.com/xplshn/gbc/pkg/token"
	"github.com/xplshn/gbc/pkg/typeChecker"
	"github.com/xplshn/gbc/pkg/util"
)

func main() {
	app := cli.NewApp("gbc")
	app.Synopsis = "[options] <input.b> ..."
	app.Description = "A compiler for the B programming language and its extensions, written in Go."
	app.Authors = []string{"xplshn"}
	app.Repository = "<https://github.com/xplshn/gbc>"
	app.Since = 2025

	var (
		outFile          string
		std              string
		target           string
		linkerArgs       []string
		compilerArgs     []string
		userIncludePaths []string
		libRequests      []string
		pedantic         bool
	)

	fs := app.FlagSet
	fs.String(&outFile, "output", "o", "a.out", "Place the output into <file>.", "file")
	fs.String(&target, "target", "t", "qbe", "Set the backend and target ABI.", "backend/target")
	fs.List(&userIncludePaths, "include", "I", []string{}, "Add a directory to the include path.", "path")
	fs.List(&linkerArgs, "linker-arg", "L", []string{}, "Pass an argument to the linker.", "arg")
	fs.List(&compilerArgs, "compiler-arg", "C", []string{}, "Pass a compiler-specific argument (e.g., -C linker_args='-s').", "arg")
	fs.Special(&libRequests, "l", "Link with a library (e.g., -lb for 'b')", "lib")
	fs.String(&std, "std", "", "Bx", "Specify language standard (B, Bx)", "std")
	fs.Bool(&pedantic, "pedantic", "", false, "Issue all warnings demanded by the current B std.")

	cfg := config.NewConfig()
	warningFlags, featureFlags := cfg.SetupFlagGroups(fs)

	// Actual compilation pipeline
	app.Action = func(inputFiles []string) error {
		// Handle pedantic flag first, as it can affect other settings.
		if pedantic {
			cfg.SetWarning(config.WarnPedantic, true)
		}

		// Apply warning flag updates to config
		for i, entry := range warningFlags {
			if entry.Enabled != nil && *entry.Enabled {
				cfg.SetWarning(config.Warning(i), true)
			}
			if entry.Disabled != nil && *entry.Disabled {
				cfg.SetWarning(config.Warning(i), false)
			}
		}

		// Apply feature flag updates to config
		for i, entry := range featureFlags {
			if entry.Enabled != nil && *entry.Enabled {
				cfg.SetFeature(config.Feature(i), true)
			}
			if entry.Disabled != nil && *entry.Disabled {
				cfg.SetFeature(config.Feature(i), false)
			}
		}

		// Apply language standard
		if err := cfg.ApplyStd(std); err != nil {
			util.Error(token.Token{}, err.Error())
		}

		// Set target, defaulting to the host if not specified
		cfg.SetTarget(runtime.GOOS, runtime.GOARCH, target)

		// Populate config from parsed command-line flags
		cfg.LinkerArgs = append(cfg.LinkerArgs, linkerArgs...)
		cfg.LibRequests = append(cfg.LibRequests, libRequests...)
		cfg.UserIncludePaths = append(cfg.UserIncludePaths, userIncludePaths...)

		// Process compiler-specific arguments (-C)
		for _, carg := range compilerArgs {
			if parts := strings.SplitN(carg, "=", 2); len(parts) == 2 && parts[0] == "linker_args" {
				parsedArgs, err := config.ParseCLIString(parts[1])
				if err != nil {
					util.Error(token.Token{}, "invalid -C linker_args value: %v", err)
				}
				cfg.LinkerArgs = append(cfg.LinkerArgs, parsedArgs...)
			}
		}

		// PASS 1: Tokenize and parse initial files to process directives.
		fmt.Println("----------------------")
		fmt.Println("Pass 1: Scanning for directives...")
		records, allTokens := readAndTokenizeFiles(inputFiles, cfg)
		util.SetSourceFiles(records)
		p := parser.NewParser(allTokens, cfg)
		p.Parse() // This populates cfg with directive info.

		// Now that all directives are processed, determine the final list of source files.
		finalInputFiles := processInputFiles(inputFiles, cfg)
		if len(finalInputFiles) == 0 {
			util.Error(token.Token{}, "no input files specified.")
		}

		// PASS 2: Re-tokenize and parse the complete set of files for compilation.
		fmt.Println("Pass 2: Compiling all source files...")
		isTyped := cfg.IsFeatureEnabled(config.FeatTyped)
		fmt.Printf("Tokenizing %d source file(s) (Typed Pass: %v)...\n", len(finalInputFiles), isTyped)
		fullRecords, fullTokens := readAndTokenizeFiles(finalInputFiles, cfg)
		util.SetSourceFiles(fullRecords)

		fmt.Println("Parsing tokens into AST...")
		fullParser := parser.NewParser(fullTokens, cfg)
		astRoot := fullParser.Parse()

		fmt.Println("Constant folding...")
		astRoot = ast.FoldConstants(astRoot)

		if cfg.IsFeatureEnabled(config.FeatTyped) { // Re-check after directives
			fmt.Println("Type checking...")
			tc := typeChecker.NewTypeChecker(cfg)
			tc.Check(astRoot)
		}

		fmt.Println("Generating backend-agnostic IR...")
		cg := codegen.NewContext(cfg)
		irProg, inlineAsm := cg.GenerateIR(astRoot)

		fmt.Printf("Generating target code with '%s' backend...\n", cfg.BackendName)
		backend := selectBackend(cfg.BackendName)
		backendOutput, err := backend.Generate(irProg, cfg)
		if err != nil {
			util.Error(token.Token{}, "backend code generation failed: %v", err)
		}

		fmt.Printf("Assembling and linking to create '%s'...\n", outFile)
		if err := assembleAndLink(outFile, backendOutput.String(), inlineAsm, cfg.LinkerArgs); err != nil {
			util.Error(token.Token{}, "assembler/linker failed: %v", err)
		}

		fmt.Println("----------------------")
		fmt.Println("Compilation successful!")
		return nil
	}

	if err := app.Run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func processInputFiles(args []string, cfg *config.Config) []string {
	// Use a map to avoid duplicate library entries
	uniqueLibs := make(map[string]bool)
	for _, lib := range cfg.LibRequests {
		uniqueLibs[lib] = true
	}

	inputFiles := args
	for libName := range uniqueLibs {
		if libPath := findLibrary(libName, cfg.UserIncludePaths, cfg); libPath != "" {
			// Avoid adding the same library file path multiple times
			found := false
			for _, inFile := range inputFiles {
				if inFile == libPath {
					found = true
					break
				}
			}
			if !found {
				inputFiles = append(inputFiles, libPath)
			}
		} else {
			util.Error(token.Token{}, "could not find library '%s' for target %s/%s", libName, cfg.GOOS, cfg.GOARCH)
		}
	}
	return inputFiles
}

func selectBackend(name string) codegen.Backend {
	switch name {
	case "qbe": return codegen.NewQBEBackend()
	case "llvm": return codegen.NewLLVMBackend()
	default:
		util.Error(token.Token{}, "unsupported backend '%s'", name)
		return nil
	}
}

func findLibrary(libName string, userPaths []string, cfg *config.Config) string {
	// Search for libraries matching the target architecture and OS
	filenames := []string{
		fmt.Sprintf("%s_%s_%s.b", libName, cfg.GOARCH, cfg.GOOS),
		fmt.Sprintf("%s_%s.b", libName, cfg.GOARCH),
		fmt.Sprintf("%s_%s.b", libName, cfg.GOOS),
		fmt.Sprintf("%s.b", libName),
		fmt.Sprintf("%s/%s_%s.b", libName, cfg.GOARCH, cfg.GOOS),
		fmt.Sprintf("%s/%s.b", libName, cfg.GOARCH),
		fmt.Sprintf("%s/%s.b", libName, cfg.GOOS),
	}
	searchPaths := append(userPaths, []string{"./lib", "/usr/local/lib/gbc", "/usr/lib/gbc", "/lib/gbc"}...)
	for _, path := range searchPaths {
		for _, fname := range filenames {
			fullPath := filepath.Join(path, fname)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}
	util.Error(token.Token{}, "could not find library '%s'", libName)
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

	// TODO: We want PIE support
	//       - Fix LLVM backend to achieve that
	//       - Our QBE backend seems to have some issues with PIE as well, but only two cases fail when doing `make examples`
	// We should, by default, use `-static-pie`
	ccArgs := []string{"-no-pie", "-o", outFile, mainAsmFile.Name()}
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
		ccArgs = append(ccArgs, inlineAsmFile.Name())
	}
	ccArgs = append(ccArgs, linkerArgs...)

	cmd := exec.Command("cc", ccArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cc command failed: %w\nOutput:\n%s", err, string(output))
	}
	return nil
}

func readAndTokenizeFiles(paths []string, cfg *config.Config) ([]util.SourceFileRecord, []token.Token) {
	var records []util.SourceFileRecord
	var allTokens []token.Token

	for i, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			util.Error(token.Token{FileIndex: -1}, "could not read file '%s': %v", path, err)
			continue
		}
		runeContent := []rune(string(content))
		records = append(records, util.SourceFileRecord{Name: path, Content: runeContent})
		l := lexer.NewLexer(runeContent, i, cfg)
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
