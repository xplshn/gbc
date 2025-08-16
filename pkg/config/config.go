package config

import (
	"fmt"
	"os"
	"strings"

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

type Config struct {
	Features       map[Feature]Info
	Warnings       map[Warning]Info
	FeatureMap     map[string]Feature
	WarningMap     map[string]Warning
	StdName        string
	TargetArch     string
	QbeTarget      string
	WordSize       int
	WordType       string
	StackAlignment int
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

// SetTarget configures the compiler for a specific architecture and QBE target.
func (c *Config) SetTarget(goos, goarch, qbeTarget string) {
	if qbeTarget == "" {
		c.QbeTarget = libqbe.DefaultTarget(goos, goarch)
		fmt.Fprintf(os.Stderr, "gbc: info: no target specified, defaulting to host target '%s'\n", c.QbeTarget)
	} else {
		c.QbeTarget = qbeTarget
		fmt.Fprintf(os.Stderr, "gbc: info: using specified target '%s'\n", c.QbeTarget)
	}

	c.TargetArch = goarch

	switch c.QbeTarget {
	case "amd64_sysv", "amd64_apple", "arm64", "arm64_apple", "rv64":
		c.WordSize, c.WordType, c.StackAlignment = 8, "l", 16
	case "arm", "rv32":
		c.WordSize, c.WordType, c.StackAlignment = 4, "w", 8
	default:
		fmt.Fprintf(os.Stderr, "gbc: warning: unrecognized or unsupported QBE target '%s'.\n", c.QbeTarget)
		fmt.Fprintf(os.Stderr, "gbc: warning: defaulting to 64-bit properties. Compilation may fail.\n")
		c.WordSize, c.WordType, c.StackAlignment = 8, "l", 16
	}
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

func (c *Config) applyFlag(flag string) {
	trimmed := strings.TrimPrefix(flag, "-")
	isNo := strings.HasPrefix(trimmed, "Wno-") || strings.HasPrefix(trimmed, "Fno-")
	enable := !isNo

	var name string
	var isWarning bool

	switch {
	case strings.HasPrefix(trimmed, "W"):
		name = strings.TrimPrefix(trimmed, "W")
		if isNo {
			name = strings.TrimPrefix(name, "no-")
		}
		isWarning = true
	case strings.HasPrefix(trimmed, "F"):
		name = strings.TrimPrefix(trimmed, "F")
		if isNo {
			name = strings.TrimPrefix(name, "no-")
		}
	default:
		name = trimmed
		isWarning = true
	}

	if name == "all" && isWarning {
		for i := Warning(0); i < WarnCount; i++ {
			if i != WarnPedantic {
				c.SetWarning(i, enable)
			}
		}
		return
	}

	if name == "pedantic" && isWarning {
		c.SetWarning(WarnPedantic, true)
		return
	}

	if isWarning {
		if w, ok := c.WarningMap[name]; ok {
			c.SetWarning(w, enable)
		}
	} else {
		if f, ok := c.FeatureMap[name]; ok {
			c.SetFeature(f, enable)
		}
	}
}

func (c *Config) ProcessFlags(visitFlag func(fn func(name string))) {
	visitFlag(func(name string) {
		if name == "Wall" || name == "Wno-all" || name == "pedantic" {
			c.applyFlag("-" + name)
		}
	})
	visitFlag(func(name string) {
		if name != "Wall" && name != "Wno-all" && name != "pedantic" {
			c.applyFlag("-" + name)
		}
	})
}

func (c *Config) ProcessDirectiveFlags(flagStr string) {
	for _, flag := range strings.Fields(flagStr) {
		c.applyFlag(flag)
	}
}
