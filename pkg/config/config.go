package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xplshn/gbc/pkg/cli"
	"github.com/xplshn/gbc/pkg/token"
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
	Features         map[Feature]Info
	Warnings         map[Warning]Info
	FeatureMap       map[string]Feature
	WarningMap       map[string]Warning
	StdName          string
	Target
	LinkerArgs       []string
	LibRequests      []string
	UserIncludePaths []string
}

func NewConfig() *Config {
	cfg := &Config{
		Features:         make(map[Feature]Info),
		Warnings:         make(map[Warning]Info),
		FeatureMap:       make(map[string]Feature),
		WarningMap:       make(map[string]Warning),
		LinkerArgs:       make([]string, 0),
		LibRequests:      make([]string, 0),
		UserIncludePaths: make([]string, 0),
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
		WarnCEsc:               {"c-esc", false, "Warn on usage of C-style '\\' escapes."},
		WarnBEsc:               {"b-esc", true, "Warn on usage of B-style '*' escapes."},
		WarnBOps:               {"b-ops", true, "Warn on usage of B-style assignment operators like '=+'."},
		WarnCOps:               {"c-ops", false, "Warn on usage of C-style assignment operators like '+='."},
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
			if tradArch == "" {
				tradArch = hostArch
			} // No target architecture specified
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

// SetupFlagGroups populates a FlagSet with warning and feature flag groups
// and returns the corresponding entry slices for processing results.
func (c *Config) SetupFlagGroups(fs *cli.FlagSet) ([]cli.FlagGroupEntry, []cli.FlagGroupEntry) {
	var warningFlags, featureFlags []cli.FlagGroupEntry

	for i := Warning(0); i < WarnCount; i++ {
		pEnable := new(bool)
		*pEnable = c.Warnings[i].Enabled
		pDisable := new(bool)
		warningFlags = append(warningFlags, cli.FlagGroupEntry{
			Name:     c.Warnings[i].Name,
			Prefix:   "W",
			Usage:    c.Warnings[i].Description,
			Enabled:  pEnable,
			Disabled: pDisable,
		})
	}

	for i := Feature(0); i < FeatCount; i++ {
		pEnable := new(bool)
		*pEnable = c.Features[i].Enabled
		pDisable := new(bool)
		featureFlags = append(featureFlags, cli.FlagGroupEntry{
			Name:     c.Features[i].Name,
			Prefix:   "F",
			Usage:    c.Features[i].Description,
			Enabled:  pEnable,
			Disabled: pDisable,
		})
	}

	fs.AddFlagGroup("Warning Flags", "Enable or disable specific warnings", "warning flag", "Available Warning Flags:", warningFlags)
	fs.AddFlagGroup("Feature Flags", "Enable or disable specific features", "feature flag", "Available feature flags:", featureFlags)

	return warningFlags, featureFlags
}

// ParseCLIString splits a string into arguments, respecting single quotes.
func ParseCLIString(s string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '\'':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if inQuote {
		return nil, errors.New("unterminated single quote in argument string")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

// ProcessArgs parses a slice of command-line style arguments and updates the configuration.
func (c *Config) ProcessArgs(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "-l"):
			c.LibRequests = append(c.LibRequests, strings.TrimPrefix(arg, "-l"))
		case strings.HasPrefix(arg, "-L"):
			val := strings.TrimPrefix(arg, "-L")
			if val == "" { // Space separated: -L <val>
				if i+1 >= len(args) {
					return fmt.Errorf("missing argument for flag: %s", arg)
				}
				i++
				val = args[i]
			}
			c.LinkerArgs = append(c.LinkerArgs, "-L"+val)
		case strings.HasPrefix(arg, "-I"):
			val := strings.TrimPrefix(arg, "-I")
			if val == "" { // Space separated: -I <val>
				if i+1 >= len(args) {
					return fmt.Errorf("missing argument for flag: %s", arg)
				}
				i++
				val = args[i]
			}
			c.UserIncludePaths = append(c.UserIncludePaths, val)
		case strings.HasPrefix(arg, "-C"):
			val := strings.TrimPrefix(arg, "-C")
			if val == "" { // Space separated: -C <val>
				if i+1 >= len(args) {
					return fmt.Errorf("missing argument for flag: %s", arg)
				}
				i++
				val = args[i]
			}
			if parts := strings.SplitN(val, "=", 2); len(parts) == 2 && parts[0] == "linker_args" {
				linkerArgs, err := ParseCLIString(parts[1])
				if err != nil {
					return fmt.Errorf("failed to parse linker_args: %w", err)
				}
				c.LinkerArgs = append(c.LinkerArgs, linkerArgs...)
			}
		case strings.HasPrefix(arg, "-W"):
			flagName := strings.TrimPrefix(arg, "-W")
			if strings.HasPrefix(flagName, "no-") {
				warnName := strings.TrimPrefix(flagName, "no-")
				if wt, ok := c.WarningMap[warnName]; ok {
					c.SetWarning(wt, false)
				} else {
					return fmt.Errorf("unknown warning flag: %s", arg)
				}
			} else {
				if wt, ok := c.WarningMap[flagName]; ok {
					c.SetWarning(wt, true)
				} else {
					return fmt.Errorf("unknown warning flag: %s", arg)
				}
			}
		case strings.HasPrefix(arg, "-F"):
			flagName := strings.TrimPrefix(arg, "-F")
			if strings.HasPrefix(flagName, "no-") {
				featName := strings.TrimPrefix(flagName, "no-")
				if ft, ok := c.FeatureMap[featName]; ok {
					c.SetFeature(ft, false)
				} else {
					return fmt.Errorf("unknown feature flag: %s", arg)
				}
			} else {
				if ft, ok := c.FeatureMap[flagName]; ok {
					c.SetFeature(ft, true)
				} else {
					return fmt.Errorf("unknown feature flag: %s", arg)
				}
			}
		default:
			return fmt.Errorf("unrecognized argument: %s", arg)
		}
	}
	return nil
}

// ProcessDirectiveFlags parses flags from a directive string.
func (c *Config) ProcessDirectiveFlags(flagStr string, tok token.Token) error {
	args, err := ParseCLIString(flagStr)
	if err != nil {
		return err
	}
	return c.ProcessArgs(args)
}
