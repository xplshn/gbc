package util

import (
	"fmt"
	"os"
	"strings"

	"gbc/pkg/token"
)

type Feature int

const (
	FeatExtrn Feature = iota
	FeatAsm
	FeatBEscapes
	FeatCEscapes
	FeatBOps
	FeatCOps
	FeatCComments
	FeatCount // Just to get the number of features
)

type FeatureInfo struct {
	Name        string
	Enabled     bool
	Description string
}

var Features = map[Feature]FeatureInfo{
	FeatExtrn:      {"extrn", true, "Allow the 'extrn' keyword"},
	FeatAsm:        {"asm", true, "Allow the '__asm__' keyword and blocks"},
	FeatBEscapes:  {"b-escapes", true, "Recognize B-style '*' character escapes"},
	FeatCEscapes:  {"c-escapes", true, "Recognize C-style '\\' character escapes"},
	FeatBOps:      {"b-ops", true, "Recognize B-style assignment operators like '=+'"},
	FeatCOps:      {"c-ops", true, "Recognize C-style assignment operators like '+='"},
	FeatCComments: {"c-comments", true, "Recognize C-style '//' comments"},
}

var FeatureMap = make(map[string]Feature)

type Warning int

const (
	WarnCEscapes Warning = iota
	WarnBEscapes
	WarnBOps
	WarnCOps
	WarnUnrecognizedEscape
	WarnTruncatedChar
	WarnLongCharConst
	WarnCComments
	WarnOverflow
	WarnPedantic
	WarnUnreachableCode
	WarnExtra
	WarnCount // Just to get the number of warnings
)

type WarningInfo struct {
	Name        string
	Enabled     bool
	Description string
}

var Warnings = map[Warning]WarningInfo{
	WarnCEscapes:           {"c-escapes", true, "Using C-style '\\' escapes instead of B's '*'"},
	WarnBEscapes:           {"b-escapes", true, "Using historical B-style '*' escapes instead of C's '\\'"},
	WarnBOps:               {"b-ops", true, "Using historical B assignment operators like '=+'"},
	WarnCOps:               {"c-ops", true, "Using C-style assignment operators like '+=' in -std=B mode"},
	WarnUnrecognizedEscape: {"unrecognized-escape", true, "Using unrecognized escape sequences"},
	WarnTruncatedChar:      {"truncated-char", true, "Character escape value is too large for a byte and has been truncated"},
	WarnLongCharConst:     {"long-char-const", true, "Multi-character constant is too long for a word"},
	WarnCComments:          {"c-comments", false, "Using non-standard C-style '//' comments"},
	WarnOverflow:            {"overflow", true, "Integer constant is out of range for its type"},
	WarnPedantic:            {"pedantic", false, "Issues that violate the current strict -std="},
	WarnUnreachableCode:    {"unreachable-code", true, "Unreachable code"},
	WarnExtra:               {"extra", true, "Extra warnings (e.g., poor choices, unrecognized flags)"},
}

var WarningMap = make(map[string]Warning)

// init function to populate the lookup maps
func init() {
	for ft, info := range Features {
		FeatureMap[info.Name] = ft
	}
	for wt, info := range Warnings {
		WarningMap[info.Name] = wt
	}
}

// SetFeature enables or disables a specific feature
func SetFeature(ft Feature, enabled bool) {
	if info, ok := Features[ft]; ok {
		info.Enabled = enabled
		Features[ft] = info
	}
}

// IsFeatureEnabled checks if a specific feature is currently enabled
func IsFeatureEnabled(ft Feature) bool {
	if info, ok := Features[ft]; ok {
		return info.Enabled
	}
	return false
}

// PrintFeatures prints the current status of all features
func PrintFeatures() {
	for i := Feature(0); i < FeatCount; i++ {
		info := Features[i]
		fmt.Printf("  - %-20s: %v (%s)\n", info.Name, info.Enabled, info.Description)
	}
}

// SetAllWarnings enables or disables all warnings at once
func SetAllWarnings(enabled bool) {
	for i := Warning(0); i < WarnCount; i++ {
		// -Wall should not enable pedantic warnings by default.
		if i == WarnPedantic && enabled {
			continue
		}
		SetWarning(i, enabled)
	}
}

// SetWarning enables or disables a specific warning
func SetWarning(wt Warning, enabled bool) {
	if info, ok := Warnings[wt]; ok {
		info.Enabled = enabled
		Warnings[wt] = info
	}
}

// IsWarningEnabled checks if a specific warning is currently enabled.
func IsWarningEnabled(wt Warning) bool {
	if info, ok := Warnings[wt]; ok {
		return info.Enabled
	}
	return false
}

// PrintWarnings prints the current status of all warnings.
func PrintWarnings() {
	for i := Warning(0); i < WarnCount; i++ {
		info := Warnings[i]
		fmt.Printf("  - %-20s: %v (%s)\n", info.Name, info.Enabled, info.Description)
	}
}

// SourceFileRecord tracks the name and content of a single source file.
type SourceFileRecord struct {
	Name    string
	Content []rune
}

var sourceFiles []SourceFileRecord

// SetSourceFiles stores the source code for all input files for rich error messages
func SetSourceFiles(files []SourceFileRecord) {
	sourceFiles = files
}

// findFileAndLine converts a global token to a file-specific location
func findFileAndLine(tok token.Token) (filename string, line, col int) {
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) {
		return "unknown", tok.Line, tok.Column
	}
	return sourceFiles[tok.FileIndex].Name, tok.Line, tok.Column
}

// printErrorLine prints the source line and a caret indicating the error position
func printErrorLine(stream *os.File, tok token.Token) {
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) || tok.Line == 0 {
		return
	}

	content := sourceFiles[tok.FileIndex].Content
	lineNum := tok.Line
	lineStart := 0
	// Find the start of the error line
	for i, r := range content {
		if lineNum <= 1 {
			break
		}
		if r == '\n' {
			lineNum--
			lineStart = i + 1
		}
	}

	// Find the end of the error line
	lineEnd := len(content)
	for i := lineStart; i < len(content); i++ {
		if content[i] == '\n' {
			lineEnd = i
			break
		}
	}

	// Print the line
	fmt.Fprintf(stream, "  %s\n", string(content[lineStart:lineEnd]))

	// Print the caret
	fmt.Fprintf(stream, "  %s\033[32m^", strings.Repeat(" ", tok.Column-1))
	if tok.Len > 1 {
		fmt.Fprintf(stream, "%s", strings.Repeat("~", tok.Len-1))
	}
	fmt.Fprintln(stream, "\033[0m")
}

// Error prints a formatted error message and exits the program
func Error(tok token.Token, format string, args ...interface{}) {
	filename, line, col := findFileAndLine(tok)
	fmt.Fprintf(os.Stderr, "%s:%d:%d: \033[31merror:\033[0m ", filename, line, col)
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
	printErrorLine(os.Stderr, tok)
	os.Exit(1)
}

// Warning prints a formatted warning message if the corresponding warning is enabled
func Warn(wt Warning, tok token.Token, format string, args ...interface{}) {
	if !IsWarningEnabled(wt) {
		return
	}
	filename, line, col := findFileAndLine(tok)
	warningName := Warnings[wt].Name
	fmt.Fprintf(os.Stderr, "%s:%d:%d: \033[33mwarning:\033[0m ", filename, line, col)
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, " [-W%s]\n", warningName)
	printErrorLine(os.Stderr, tok)
}
