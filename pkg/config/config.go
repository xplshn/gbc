package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/xplshn/gbc/pkg/cli"
	"modernc.org/libqbe"
)

type Feature int

const (
	FeatExtrn Feature = iota
	FeatAsm
	FeatBEsc
	FeatCEsc
	FeatBOps
	FeatCOps
	FeatCComments
	FeatTyped
	FeatShortDecl
	FeatBxDeclarations
	FeatAllowUninitialized
	FeatStrictDecl
	FeatNoDirectives
	FeatContinue
	FeatCount
)

type Warning int

const (
	WarnCEsc Warning = iota
	WarnBEsc
	WarnBOps
	WarnCOps
	WarnUnrecognizedEscape
	WarnTruncatedChar
	WarnLongCharConst
	WarnCComments
	WarnOverflow
	WarnPedantic
	WarnUnreachableCode
	WarnImplicitDecl
	WarnType
	WarnExtra
	WarnCount
)

type Info struct {
	Name        string
	Enabled     bool
	Description string
}

// Backend-specific properties
type Target struct {
	GOOS           string
	GOARCH         string
	BackendName    string // "qbe", "llvm"
	BackendTarget  string // "amd64_sysv", "x86_64-unknown-linux-musl"
	WordSize       int
	WordType       string // QBE type char
	StackAlignment int
}

var archTranslations = map[string]string{
	"amd64":   "x86_64",
	"386":     "i686",
	"arm64":   "aarch64",
	"arm":     "arm",
	"riscv64": "riscv64",
	"x86_64":  "amd64",
	"i386":    "386",
	"i686":    "386",
	"aarch64": "arm64",
}

var archProperties = map[string]struct {
	WordSize       int
	WordType       string
	StackAlignment int
}{
	"amd64":   {WordSize: 8, WordType: "l", StackAlignment: 16},
	"arm64":   {WordSize: 8, WordType: "l", StackAlignment: 16},
	"386":     {WordSize: 4, WordType: "w", StackAlignment: 8},
	"arm":     {WordSize: 4, WordType: "w", StackAlignment: 8},
	"riscv64": {WordSize: 8, WordType: "l", StackAlignment: 16},
}

type Config struct {
	Features   map[Feature]Info
	Warnings   map[Warning]Info
	FeatureMap map[string]Feature
	WarningMap map[string]Warning
	StdName    string
	Target
}

func NewConfig() *Config {
	cfg := &Config{
		Features:   make(map[Feature]Info),
		Warnings:   make(map[Warning]Info),
		FeatureMap: make(map[string]Feature),
		WarningMap: make(map[string]Warning),
	}

	features := map[Feature]Info{
		FeatExtrn:              {"extrn", true, "Allow the 'extrn' keyword."},
		FeatAsm:                {"asm", true, "Allow `__asm__` blocks for inline assembly."},
		FeatBEsc:               {"b-esc", false, "Recognize B-style '*' character escapes."},
		FeatCEsc:               {"c-esc", true, "Recognize C-style '\\' character escapes."},
		FeatBOps:               {"b-ops", false, "Recognize B-style assignment operators like '=+'."},
		FeatCOps:               {"c-ops", true, "Recognize C-style assignment operators like '+='."},
		FeatCComments:          {"c-comments", true, "Recognize C-style '//' line comments."},
		FeatTyped:              {"typed", true, "Enable the Bx opt-in & backwards-compatible type system."},
		FeatShortDecl:          {"short-decl", true, "Enable Bx-style short declaration `:=`."},
		FeatBxDeclarations:     {"bx-decl", true, "Enable Bx-style `auto name = val` declarations."},
		FeatAllowUninitialized: {"allow-uninitialized", true, "Allow declarations without an initializer (`var;` or `auto var;`)."},
		FeatStrictDecl:         {"strict-decl", false, "Require all declarations to be initialized."},
		FeatContinue:           {"continue", true, "Allow the Bx keyword `continue` to be used."},
		FeatNoDirectives:       {"no-directives", false, "Disable `// [b]:` directives."},
	}

	warnings := map[Warning]Info{
		WarnCEsc:               {"c-esc", true, "Warn on usage of C-style '\\' escapes."},
		WarnBEsc:               {"b-esc", true, "Warn on usage of B-style '*' escapes."},
		WarnBOps:               {"b-ops", true, "Warn on usage of B-style assignment operators like '=+'."},
		WarnCOps:               {"c-ops", true, "Warn on usage of C-style assignment operators like '+='."},
		WarnUnrecognizedEscape: {"u-esc", true, "Warn on unrecognized character escape sequences."},
		WarnTruncatedChar:      {"truncated-char", true, "Warn when a character escape value is truncated."},
		WarnLongCharConst:      {"long-char-const", true, "Warn when a multi-character constant is too long for a word."},
		WarnCComments:          {"c-comments", false, "Warn on usage of non-standard C-style '//' comments."},
		WarnOverflow:           {"overflow", true, "Warn when an integer constant is out of range for its type."},
		WarnPedantic:           {"pedantic", false, "Issue all warnings demanded by the strict standard."},
		WarnUnreachableCode:    {"unreachable-code", true, "Warn about code that will never be executed."},
		WarnImplicitDecl:       {"implicit-decl", true, "Warn about implicit function or variable declarations."},
		WarnType:               {"type", true, "Warn about type mismatches in expressions and assignments."},
		WarnExtra:              {"extra", true, "Enable extra miscellaneous warnings."},
	}

	cfg.Features, cfg.Warnings = features, warnings
	for ft, info := range features {
		cfg.FeatureMap[info.Name] = ft
	}
	for wt, info := range warnings {
		cfg.WarningMap[info.Name] = wt
	}

	return cfg
}

// SetTarget configures the compiler for a specific architecture and backend target
func (c *Config) SetTarget(hostOS, hostArch, targetFlag string) {
	// Init with host defaults
	c.GOOS, c.GOARCH, c.BackendName = hostOS, hostArch, "qbe"

	// Parse target flag: <backend>/<target_string>
	if targetFlag != "" {
		parts := strings.SplitN(targetFlag, "/", 2)
		c.BackendName = parts[0]
		if len(parts) > 1 {
			c.BackendTarget = parts[1]
		}
	}

	// Valid QBE targets |https://pkg.go.dev/modernc.org/libqbe#hdr-Supported_targets|
	validQBETargets := map[string]string{
		"amd64_apple": "amd64",
		"amd64_sysv":  "amd64",
		"arm64":       "arm64",
		"arm64_apple": "arm64",
		"rv64":        "riscv64",
	}

	if c.BackendName == "qbe" {
		if c.BackendTarget == "" {
			c.BackendTarget = libqbe.DefaultTarget(hostOS, hostArch)
			fmt.Fprintf(os.Stderr, "gbc: info: no target specified, defaulting to host target '%s' for backend '%s'\n", c.BackendTarget, c.BackendName)
		}
		if goArch, ok := validQBETargets[c.BackendTarget]; ok {
			c.GOARCH = goArch
		} else {
			fmt.Fprintf(os.Stderr, "gbc: warning: unsupported QBE target '%s', defaulting to GOARCH '%s'\n", c.BackendTarget, c.GOARCH)
		}
	} else { // llvm
		if c.BackendTarget == "" {
			tradArch := archTranslations[hostArch]
			if tradArch == "" { tradArch = hostArch } // No target architecture specified
			// TODO: ? Infer env ("musl", "gnu", etc..?)
			c.BackendTarget = fmt.Sprintf("%s-unknown-%s-unknown", tradArch, hostOS)
			fmt.Fprintf(os.Stderr, "gbc: info: no target specified, defaulting to host target '%s' for backend '%s'\n", c.BackendTarget, c.BackendName)
		}
		parts := strings.Split(c.BackendTarget, "-")
		if len(parts) > 0 {
			if goArch, ok := archTranslations[parts[0]]; ok {
				c.GOARCH = goArch
			} else {
				c.GOARCH = parts[0]
			}
		}
		if len(parts) > 2 && parts[2] != "unknown" {
			c.GOOS = parts[2]
		}
	}

	// Set architecture-specific properties
	if props, ok := archProperties[c.GOARCH]; ok {
		c.WordSize, c.WordType, c.StackAlignment = props.WordSize, props.WordType, props.StackAlignment
	} else {
		fmt.Fprintf(os.Stderr, "gbc: warning: unrecognized architecture '%s'.\n", c.GOARCH)
		fmt.Fprintf(os.Stderr, "gbc: warning: defaulting to 64-bit properties. Compilation may fail.\n")
		c.WordSize, c.WordType, c.StackAlignment = 8, "l", 16
	}

	fmt.Fprintf(os.Stderr, "gbc: info: using backend '%s' with target '%s' (GOOS=%s, GOARCH=%s)\n", c.BackendName, c.BackendTarget, c.GOOS, c.GOARCH)
}

func (c *Config) SetFeature(ft Feature, enabled bool) {
	if info, ok := c.Features[ft]; ok {
		info.Enabled = enabled
		c.Features[ft] = info
	}
}

func (c *Config) IsFeatureEnabled(ft Feature) bool { return c.Features[ft].Enabled }

func (c *Config) SetWarning(wt Warning, enabled bool) {
	if info, ok := c.Warnings[wt]; ok {
		info.Enabled = enabled
		c.Warnings[wt] = info
	}
}

func (c *Config) IsWarningEnabled(wt Warning) bool { return c.Warnings[wt].Enabled }

func (c *Config) ApplyStd(stdName string) error {
	c.StdName = stdName
	isPedantic := c.IsWarningEnabled(WarnPedantic)

	type stdSettings struct {
		feature Feature
		bValue  bool
		bxValue bool
	}

	settings := []stdSettings{
		{FeatAllowUninitialized, true, !isPedantic},
		{FeatBOps, true, false},
		{FeatBEsc, true, false},
		{FeatCOps, !isPedantic, true},
		{FeatCEsc, !isPedantic, true},
		{FeatCComments, !isPedantic, true},
		{FeatExtrn, !isPedantic, true},
		{FeatAsm, !isPedantic, true},
		{FeatTyped, false, true},
		{FeatShortDecl, false, true},
		{FeatBxDeclarations, false, true},
		{FeatStrictDecl, false, isPedantic},
	}

	switch stdName {
	case "B":
		for _, s := range settings {
			c.SetFeature(s.feature, s.bValue)
		}
		c.SetWarning(WarnBOps, false)
		c.SetWarning(WarnBEsc, false)
		c.SetWarning(WarnCOps, true)
		c.SetWarning(WarnCEsc, true)
		c.SetWarning(WarnCComments, true)
	case "Bx":
		for _, s := range settings {
			c.SetFeature(s.feature, s.bxValue)
		}
		c.SetWarning(WarnBOps, true)
		c.SetWarning(WarnBEsc, true)
		c.SetWarning(WarnCOps, false)
		c.SetWarning(WarnCEsc, false)
		c.SetWarning(WarnCComments, false)
	default:
		return fmt.Errorf("unsupported standard '%s'. Supported: 'B', 'Bx'", stdName)
	}
	return nil
}

// ProcessDirectiveFlags parses flags from a directive string using a temporary FlagSet.
func (c *Config) ProcessDirectiveFlags(flagStr string) {
	fs := cli.NewFlagSet("directive")
	var warningFlags, featureFlags []cli.FlagGroupEntry

	// Build warning flags for the directive parser.
	for i := Warning(0); i < WarnCount; i++ {
		pEnable := new(bool)
		pDisable := new(bool)
		entry := cli.FlagGroupEntry{Name: c.Warnings[i].Name, Prefix: "W", Enabled: pEnable, Disabled: pDisable}
		warningFlags = append(warningFlags, entry)
		fs.Bool(entry.Enabled, entry.Prefix+entry.Name, "", false, "")
		fs.Bool(entry.Disabled, entry.Prefix+"no-"+entry.Name, "", false, "")
	}

	// Build feature flags for the directive parser.
	for i := Feature(0); i < FeatCount; i++ {
		pEnable := new(bool)
		pDisable := new(bool)
		entry := cli.FlagGroupEntry{Name: c.Features[i].Name, Prefix: "F", Enabled: pEnable, Disabled: pDisable}
		featureFlags = append(featureFlags, entry)
		fs.Bool(entry.Enabled, entry.Prefix+entry.Name, "", false, "")
		fs.Bool(entry.Disabled, entry.Prefix+"no-"+entry.Name, "", false, "")
	}

	// The cli parser expects arguments to start with '-'.
	args := strings.Fields(flagStr)
	processedArgs := make([]string, len(args))
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			processedArgs[i] = "-" + arg
		} else {
			processedArgs[i] = arg
		}
	}

	if err := fs.Parse(processedArgs); err != nil {
		// Silently ignore errors in directives, as they shouldn't halt compilation.
		return
	}

	// Apply parsed directive flags to the configuration.
	for i, entry := range warningFlags {
		if *entry.Enabled {
			c.SetWarning(Warning(i), true)
		}
		if *entry.Disabled {
			c.SetWarning(Warning(i), false)
		}
	}

	for i, entry := range featureFlags {
		if *entry.Enabled {
			c.SetFeature(Feature(i), true)
		}
		if *entry.Disabled {
			c.SetFeature(Feature(i), false)
		}
	}
}
