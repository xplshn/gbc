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

func setupFlags(cfg *config.Config, fs *cli.FlagSet) ([]cli.FlagGroupEntry, []cli.FlagGroupEntry) {
	var warningFlags, featureFlags []cli.FlagGroupEntry

	for i := config.Warning(0); i < config.WarnCount; i++ {
		pEnable := new(bool)
		*pEnable = cfg.Warnings[i].Enabled
		pDisable := new(bool)
		warningFlags = append(warningFlags, cli.FlagGroupEntry{
			Name:     cfg.Warnings[i].Name,
			Prefix:   "W",
			Usage:    cfg.Warnings[i].Description,
			Enabled:  pEnable,
			Disabled: pDisable,
		})
	}

	for i := config.Feature(0); i < config.FeatCount; i++ {
		pEnable := new(bool)
		*pEnable = cfg.Features[i].Enabled
		pDisable := new(bool)
		featureFlags = append(featureFlags, cli.FlagGroupEntry{
			Name:     cfg.Features[i].Name,
			Prefix:   "F",
			Usage:    cfg.Features[i].Description,
			Enabled:  pEnable,
			Disabled: pDisable,
		})
	}

	fs.AddFlagGroup("Warning Flags", "Enable or disable specific warnings", "warning flag", "Available Warning Flags:", warningFlags)
	fs.AddFlagGroup("Feature Flags", "Enable or disable specific features", "feature flag", "Available feature flags:", featureFlags)

	return warningFlags, featureFlags
}

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
	)

	fs := app.FlagSet
	fs.String(&outFile, "output", "o", "a.out", "Place the output into <file>", "file")
	fs.String(&std, "std", "", "Bx", "Specify language standard (B, Bx)", "std")
	fs.String(&target, "target", "t", "qbe", "Set the backend and target ABI (e.g., llvm/x86_64-linux-musl)", "backend/target")
	fs.List(&linkerArgs, "linker-arg", "", []string{}, "Pass an argument to the linker", "arg")
	fs.List(&compilerArgs, "compiler-arg", "", []string{}, "Pass a compiler-specific argument (e.g., -C linker_args='-s')", "arg")
	fs.List(&userIncludePaths, "include", "", []string{}, "Add a directory to the include path", "path")
	fs.Special(&libRequests, "l", "Link with a library (e.g., -lb for 'b')", "lib")

	cfg := config.NewConfig()
	warningFlags, featureFlags := setupFlags(cfg, fs)

	// Actual compilation pipeline
	app.Action = func(inputFiles []string) error {
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

		// Process compiler arguments for linker args
		for _, carg := range compilerArgs {
			if parts := strings.SplitN(carg, "=", 2); len(parts) == 2 && parts[0] == "linker_args" {
				linkerArgs = append(linkerArgs, strings.Fields(parts[1])...)
			}
		}

		// Process input files, searching for libraries based on the target configuration
		finalInputFiles := processInputFiles(inputFiles, libRequests, userIncludePaths, cfg)
		if len(finalInputFiles) == 0 {
			util.Error(token.Token{}, "no input files specified.")
		}

		fmt.Println("----------------------")
		isTyped := cfg.IsFeatureEnabled(config.FeatTyped)
		fmt.Printf("Tokenizing %d source file(s) (Typed Pass: %v)...\n", len(finalInputFiles), isTyped)
		records, allTokens := readAndTokenizeFiles(finalInputFiles, cfg)
		util.SetSourceFiles(records)

		fmt.Println("Parsing tokens into AST...")
		p := parser.NewParser(allTokens, cfg)
		astRoot := p.Parse()

		fmt.Println("Constant folding...")
		astRoot = ast.FoldConstants(astRoot)

		if isTyped {
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
		if err := assembleAndLink(outFile, backendOutput.String(), inlineAsm, linkerArgs); err != nil {
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

func processInputFiles(args []string, libRequests []string, userPaths []string, cfg *config.Config) []string {
	inputFiles := args
	for _, libName := range libRequests {
		if libPath := findLibrary(libName, userPaths, cfg); libPath != "" {
			inputFiles = append(inputFiles, libPath)
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
	if len(paths) > 0 { finalFileIndex = len(paths) - 1 }
	allTokens = append(allTokens, token.Token{Type: token.EOF, FileIndex: finalFileIndex})
	return records, allTokens
}
